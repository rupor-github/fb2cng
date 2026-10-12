package envelope

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/amazon-ion/ion-go/ion"
	"github.com/ulikunitz/xz/lzma"
)

// Mode selects the page representation, not encryption.
type Mode string

const (
	Plaintext Mode = "plaintext"
	LZMA      Mode = "lzma"
)

// Encode wraps nonempty opaque bytes using the single-frame plaintext profile.
// Decoded page size adapts to stream length, choosing 4, 16, 32, or 64 KiB.
func Encode(ctx context.Context, inner []byte, mode Mode) ([]byte, Info, error) {
	if err := ctx.Err(); err != nil {
		return nil, Info{}, err
	}
	if len(inner) == 0 || uint64(len(inner)) > MaxDecodedSize {
		return nil, Info{}, errors.New("decoded stream must contain 1 byte to 1 GiB")
	}
	if mode != Plaintext && mode != LZMA {
		return nil, Info{}, fmt.Errorf("unsupported envelope mode %q", mode)
	}
	pageSize := adaptivePageSize(len(inner))
	prefix, err := datagram(func(w ion.Writer) error {
		_ = w.Annotation(ion.NewSymbolTokenFromString("doctype"))
		return w.WriteSymbolFromString(protectedData)
	})
	if err != nil {
		return nil, Info{}, err
	}
	frame, err := datagram(func(w ion.Writer) error {
		_ = w.Annotation(ion.NewSymbolTokenFromString(envelopeType))
		_ = w.BeginList()
		_ = w.Annotation(ion.NewSymbolTokenFromString(metadataType))
		_ = w.BeginStruct()
		writeUintField(w, "size", uint64(len(inner)))
		writeUintField(w, "page_size", uint64(pageSize))
		if mode == LZMA {
			_ = w.FieldName(ion.NewSymbolTokenFromString("compression_algorithm"))
			_ = w.WriteString("LZMA")
		}
		_ = w.EndStruct()
		for offset := 0; offset < len(inner); offset += pageSize {
			if err := ctx.Err(); err != nil {
				return err
			}
			page := inner[offset:min(offset+pageSize, len(inner))]
			if mode == LZMA {
				var err error
				page, err = encodePage(page)
				if err != nil {
					return fmt.Errorf("encode page %d: %w", offset/pageSize, err)
				}
			}
			_ = w.Annotation(ion.NewSymbolTokenFromString(plaintextType))
			_ = w.BeginStruct()
			_ = w.FieldName(ion.NewSymbolTokenFromString("data"))
			if mode == LZMA {
				_ = w.Annotation(ion.NewSymbolTokenFromString(compressedType))
			}
			_ = w.WriteBlob(page)
			_ = w.EndStruct()
		}
		return w.EndList()
	})
	if err != nil {
		return nil, Info{}, fmt.Errorf("encode envelope frame: %w", err)
	}
	prolog, err := datagram(func(ion.Writer) error { return nil })
	if err != nil {
		return nil, Info{}, err
	}
	end, err := datagram(func(w ion.Writer) error {
		_ = w.Annotation(ion.NewSymbolTokenFromString("enddoc"))
		return w.WriteSymbolFromString(protectedData)
	})
	if err != nil {
		return nil, Info{}, err
	}
	if !bytes.HasPrefix(end, prolog) {
		return nil, Info{}, errors.New("inconsistent ProtectedData prolog")
	}
	end = end[len(prolog):]
	indexOffset := uint64(len(prefix)) + uint64(len(frame)) + uint64(len(end))
	index, err := datagram(func(w ion.Writer) error {
		_ = w.Annotation(ion.NewSymbolTokenFromString(indexType))
		_ = w.BeginList()
		_ = w.BeginList()
		for _, n := range []uint64{uint64(len(prefix)), uint64(len(frame)), 0, uint64(len(inner))} {
			_ = w.WriteUint(n)
		}
		_ = w.EndList()
		return w.EndList()
	})
	if err != nil {
		return nil, Info{}, err
	}
	var output bytes.Buffer
	output.WriteString(Magic)
	output.Write(prefix)
	output.Write(frame)
	output.Write(end)
	output.Write(index)
	output.WriteByte(0x28)
	output.Write(binary.BigEndian.AppendUint64(nil, indexOffset))
	data := output.Bytes()
	parsed, err := Inspect(ctx, data)
	if err != nil {
		return nil, Info{}, fmt.Errorf("inspect encoded envelope: %w", err)
	}
	return data, parsed.Info(), nil
}

func datagram(write func(ion.Writer) error) ([]byte, error) {
	var buf bytes.Buffer
	w := ion.NewBinaryWriter(&buf, symbolTable())
	if err := write(w); err != nil {
		return nil, err
	}
	// Ion writers retain all earlier write errors; Finish is the final error check.
	if err := w.Finish(); err != nil {
		return nil, fmt.Errorf("finish ProtectedData Ion: %w", err)
	}
	return buf.Bytes(), nil
}

func writeUintField(w ion.Writer, name string, n uint64) {
	_ = w.FieldName(ion.NewSymbolTokenFromString(name))
	_ = w.WriteUint(n)
}

func encodePage(page []byte) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(0)
	w, err := (lzma.WriterConfig{
		Properties: &lzma.Properties{LC: 3, LP: 0, PB: 2},
		// Each independent stream needs at most its own decoded length of history. A larger
		// encoder window only allocates unused dictionary/match-finder storage.
		DictCap: max(PageSize, len(page)), SizeInHeader: true, Size: int64(len(page)), EOSMarker: true,
	}).NewWriter(&buf)
	if err != nil {
		return nil, err
	}
	_, writeErr := w.Write(page)
	if err := errors.Join(writeErr, w.Close()); err != nil {
		return nil, err
	}
	// Advertise the native-verified 8-MiB decoder dictionary. Every distance
	// produced by the bounded encoder is valid within that dictionary; the
	// on-disk properties, explicit size, and EOS framing remain unchanged.
	binary.LittleEndian.PutUint32(buf.Bytes()[2:6], dictionarySize)
	return buf.Bytes(), nil
}
