package envelope

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/amazon-ion/ion-go/ion"
	"github.com/ulikunitz/xz/lzma"
)

// Envelope is an inspected plaintext envelope. Inspection validates layout and page
// headers without decompressing payloads. The object owns its parsed page bytes.
type Envelope struct {
	info  Info
	pages [][]byte
}

// IsEnvelope detects the complete outer signature, independently of file extension.
func IsEnvelope(data []byte) bool { return bytes.HasPrefix(data, []byte(Magic)) }

// Info returns layout information, independent of the envelope's internal state.
func (e *Envelope) Info() Info {
	i := e.info
	i.Pages = slices.Clone(i.Pages)
	return i
}

// Inspect validates the native-verified plaintext profile without decoding LZMA pages.
func Inspect(ctx context.Context, data []byte) (*Envelope, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !IsEnvelope(data) || len(data) < len(Magic)+9 {
		return nil, errors.New("invalid or truncated KFX envelope signature")
	}
	if data[len(data)-9] != 0x28 {
		return nil, errors.New("invalid fixed-width envelope index trailer")
	}
	body := data[len(Magic) : len(data)-9]
	indexOffset := binary.BigEndian.Uint64(data[len(data)-8:])
	if indexOffset >= uint64(len(body)) {
		return nil, errors.New("envelope index offset out of bounds")
	}
	row, err := readIndex(ctx, body[indexOffset:])
	if err != nil {
		return nil, fmt.Errorf("read envelope index: %w", err)
	}
	if row[0] == 0 || row[0] >= indexOffset || row[1] == 0 || row[1] >= indexOffset-row[0] {
		return nil, errors.New("envelope frame offset/length out of bounds")
	}
	if row[2] != 0 || row[3] == 0 || row[3] > MaxDecodedSize {
		return nil, errors.New("unsupported envelope index row or decoded size (limit 1 GiB)")
	}
	frameEnd := row[0] + row[1]
	prefix := body[:row[0]]
	// The enddoc is a raw value, interpreted in the prefix's symbol context.
	markers := make([]byte, 0, len(prefix)+int(indexOffset-frameEnd))
	markers = append(markers, prefix...)
	markers = append(markers, body[frameEnd:indexOffset]...)
	r, err := newReader(ctx, markers)
	if err != nil {
		return nil, fmt.Errorf("read envelope markers: %w", err)
	}
	for _, annotation := range []string{"doctype", "enddoc"} {
		if err := requireNext(r, ion.SymbolType, annotation); err != nil {
			return nil, err
		}
		token, err := r.SymbolValue()
		if err != nil {
			return nil, err
		}
		if token == nil || token.Text == nil || *token.Text != protectedData {
			return nil, fmt.Errorf("unsupported %s symbol", annotation)
		}
		if err := checkImport(r); err != nil {
			return nil, err
		}
	}
	if err := done(r); err != nil {
		return nil, fmt.Errorf("envelope markers: %w", err)
	}
	// Also require the prefix to contain exactly doctype, not part of the frame/enddoc.
	pr, err := newReader(ctx, prefix)
	if err != nil {
		return nil, err
	}
	if err := requireNext(pr, ion.SymbolType, "doctype"); err != nil {
		return nil, err
	}
	if err := done(pr); err != nil {
		return nil, fmt.Errorf("envelope prefix: %w", err)
	}
	e := &Envelope{info: Info{
		Mode: Plaintext, EncodedSize: uint64(len(data)), DecodedSize: row[3],
		FrameOffset: row[0], FrameLength: row[1], IndexOffset: indexOffset, IndexLength: uint64(len(body)) - indexOffset,
	}}
	if err := e.readFrame(ctx, body[row[0]:frameEnd]); err != nil {
		return nil, fmt.Errorf("read frame 0: %w", err)
	}
	return e, nil
}

func readIndex(ctx context.Context, data []byte) ([4]uint64, error) {
	var row [4]uint64
	r, err := newReader(ctx, data)
	if err != nil {
		return row, err
	}
	if err := requireNext(r, ion.ListType, indexType); err != nil {
		return row, err
	}
	if err := checkImport(r); err != nil {
		return row, err
	}
	if err := r.StepIn(); err != nil {
		return row, err
	}
	if err := requireNext(r, ion.ListType, ""); err != nil {
		return row, err
	}
	if err := r.StepIn(); err != nil {
		return row, err
	}
	for j := range row {
		if err := requireNext(r, ion.IntType, ""); err != nil {
			return row, err
		}
		row[j], err = unsigned(r)
		if err != nil {
			return row, err
		}
	}
	if err := finishContainer(r); err != nil {
		return row, fmt.Errorf("four-number index row required: %w", err)
	}
	if err := finishContainer(r); err != nil {
		return row, fmt.Errorf("single indexed frame required: %w", err)
	}
	return row, done(r)
}

func (e *Envelope) readFrame(ctx context.Context, data []byte) error {
	r, err := newReader(ctx, data)
	if err != nil {
		return err
	}
	if err := requireNext(r, ion.ListType, envelopeType); err != nil {
		return err
	}
	if err := checkImport(r); err != nil {
		return err
	}
	if err := r.StepIn(); err != nil {
		return err
	}
	if err := requireNext(r, ion.StructType, metadataType); err != nil {
		return err
	}
	if err := e.readMetadata(r); err != nil {
		return err
	}
	var offset uint64
	for r.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		j := len(e.pages)
		if offset >= e.info.DecodedSize {
			return errors.New("too many envelope pages")
		}
		if err := expect(r, ion.StructType, plaintextType); err != nil {
			return fmt.Errorf("page %d: unsupported/encrypted page: %w", j, err)
		}
		if err := r.StepIn(); err != nil {
			return err
		}
		annotation := ""
		if e.info.Mode == LZMA {
			annotation = compressedType
		}
		if err := requireNext(r, ion.BlobType, annotation); err != nil {
			return fmt.Errorf("page %d data: %w", j, err)
		}
		field, err := r.FieldName()
		if err != nil {
			return err
		}
		if field == nil || field.Text == nil || *field.Text != "data" {
			return fmt.Errorf("page %d: missing data field", j)
		}
		blob, err := r.ByteValue()
		if err != nil {
			return fmt.Errorf("page %d blob: %w", j, err)
		}
		length := min(e.info.PageSize, e.info.DecodedSize-offset)
		p := PageInfo{Number: j, DecodedOffset: offset, DecodedSize: length, StoredSize: uint64(len(blob))}
		if e.info.Mode == LZMA {
			if err := pageHeader(blob, length); err != nil {
				return fmt.Errorf("page %d: %w", j, err)
			}
			p.Filter, p.Properties, p.Dictionary = blob[0], blob[1], binary.LittleEndian.Uint32(blob[2:6])
		} else if uint64(len(blob)) != length {
			return fmt.Errorf("page %d: plaintext length mismatch", j)
		}
		if err := finishContainer(r); err != nil {
			return fmt.Errorf("page %d: %w", j, err)
		}
		e.pages = append(e.pages, blob)
		e.info.Pages = append(e.info.Pages, p)
		offset += length
	}
	if err := r.Err(); err != nil {
		return err
	}
	if offset != e.info.DecodedSize {
		return errors.New("page count does not match decoded size")
	}
	if err := r.StepOut(); err != nil {
		return err
	}
	return done(r)
}

func (e *Envelope) readMetadata(r ion.Reader) error {
	if err := r.StepIn(); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for r.Next() {
		field, err := r.FieldName()
		if err != nil {
			return err
		}
		if field == nil || field.Text == nil {
			return errors.New("unknown envelope metadata field")
		}
		name := *field.Text
		if seen[name] {
			return fmt.Errorf("duplicate envelope metadata %q", name)
		}
		seen[name] = true
		switch name {
		case "size", "page_size":
			if err := expect(r, ion.IntType, ""); err != nil {
				return err
			}
			n, err := unsigned(r)
			if err != nil {
				return err
			}
			if (name == "size" && n != e.info.DecodedSize) || (name == "page_size" && !supportedPageSize(n)) {
				return fmt.Errorf("unsupported or inconsistent %s: %d", name, n)
			}
			if name == "page_size" {
				e.info.PageSize = n
			}
		case "compression_algorithm":
			if err := expect(r, ion.StringType, ""); err != nil {
				return err
			}
			algorithm, err := r.StringValue()
			if err != nil {
				return err
			}
			if algorithm == nil || *algorithm != "LZMA" {
				return errors.New("unsupported envelope compression algorithm")
			}
			e.info.Mode = LZMA
		default:
			return fmt.Errorf("unsupported encryption/signing or metadata field %q", name)
		}
	}
	if err := r.Err(); err != nil {
		return err
	}
	if !seen["size"] || !seen["page_size"] {
		return errors.New("missing envelope size/page_size")
	}
	return r.StepOut()
}

func pageHeader(blob []byte, size uint64) error {
	if len(blob) < 14 {
		return errors.New("truncated flagged LZMA header")
	}
	if blob[0] != 0 {
		return fmt.Errorf("unsupported LZMA filter %d", blob[0])
	}
	if blob[1] != 0x5d || binary.LittleEndian.Uint32(blob[2:6]) != dictionarySize {
		return errors.New("unsupported LZMA properties/dictionary")
	}
	if binary.LittleEndian.Uint64(blob[6:14]) != size {
		return errors.New("LZMA decoded size mismatch")
	}
	return nil
}

// Decode recovers the complete original byte stream, checking every page's real size.
func (e *Envelope) Decode(ctx context.Context) ([]byte, error) {
	var output bytes.Buffer
	for j, blob := range e.pages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page := blob
		if e.info.Mode == LZMA {
			input := bytes.NewReader(blob[1:])
			r, err := (lzma.ReaderConfig{DictCap: dictionarySize}).NewReader(input)
			if err != nil {
				return nil, fmt.Errorf("decode frame 0 page %d: %w", j, err)
			}
			page, err = io.ReadAll(io.LimitReader(r, int64(e.info.PageSize)+1))
			if err != nil {
				return nil, fmt.Errorf("decode frame 0 page %d: %w", j, err)
			}
			if !r.EOSMarker() || input.Len() != 0 {
				return nil, fmt.Errorf("page %d: missing EOS marker or trailing LZMA bytes", j)
			}
		}
		if uint64(len(page)) != e.info.Pages[j].DecodedSize {
			return nil, fmt.Errorf("page %d: actual decoded length mismatch", j)
		}
		output.Write(page)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if uint64(output.Len()) != e.info.DecodedSize {
		return nil, errors.New("actual decoded stream length mismatch")
	}
	return output.Bytes(), nil
}

func newReader(ctx context.Context, data []byte) (ion.Reader, error) {
	if err := checkIon(ctx, data); err != nil {
		return nil, err
	}
	return ion.NewReaderCat(bytes.NewReader(data), ion.NewCatalog(symbolTable())), nil
}

func checkImport(r ion.Reader) error {
	table := r.SymbolTable()
	if table == nil || table.MaxID() != 73 || len(table.Symbols()) != 0 {
		return errors.New("unsupported ProtectedData symbol table")
	}
	count := 0
	for _, imp := range table.Imports() {
		if imp.Name() == "$ion" {
			continue
		}
		if imp.Name() != "ProtectedData" || imp.Version() != 1 || imp.MaxID() != 64 {
			return errors.New("unsupported ProtectedData import")
		}
		count++
	}
	if count != 1 {
		return errors.New("missing ProtectedData v1 import")
	}
	return nil
}

func requireNext(r ion.Reader, typ ion.Type, annotation string) error {
	if !r.Next() {
		if err := r.Err(); err != nil {
			return err
		}
		return errors.New("missing Ion value")
	}
	return expect(r, typ, annotation)
}

func expect(r ion.Reader, typ ion.Type, annotation string) error {
	if r.Type() != typ || r.IsNull() {
		return fmt.Errorf("expected non-null Ion %s, got %s", typ, r.Type())
	}
	annotations, err := r.Annotations()
	if err != nil {
		return err
	}
	if annotation == "" && len(annotations) == 0 {
		return nil
	}
	if len(annotations) != 1 || annotations[0].Text == nil || *annotations[0].Text != annotation {
		return fmt.Errorf("expected annotation %q", annotation)
	}
	return nil
}

func unsigned(r ion.Reader) (uint64, error) {
	n, err := r.Int64Value()
	if err != nil {
		return 0, err
	}
	if n == nil || *n < 0 {
		return 0, errors.New("expected nonnegative Ion integer")
	}
	return uint64(*n), nil
}

func done(r ion.Reader) error {
	if r.Next() {
		return errors.New("unexpected extra Ion value")
	}
	return r.Err()
}

func finishContainer(r ion.Reader) error {
	if err := done(r); err != nil {
		return err
	}
	return r.StepOut()
}
