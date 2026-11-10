package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"

	"fbc/config"
	"fbc/convert/publication"
)

// apnxLayout is a planned sidecar, not a published file. Final book/sidecar
// preflight, staging and publication belong to the output bundle layer.
type apnxLayout struct {
	Path string
	Data []byte
}

type apnxIdentity struct {
	ContentGUID    string `json:"contentGuid"`
	ASIN           string `json:"asin"`
	ContentType    string `json:"cdeType"`
	Format         string `json:"format"`
	FileRevisionID string `json:"fileRevisionId"`
	ACR            string `json:"acr"`
}

type apnxPageHeader struct {
	ASIN    string `json:"asin"`
	PageMap string `json:"pageMap"`
}

func compileAPNX(
	ctx context.Context, bookPath string, pub *publication.Publication, text *textLayout,
	metadata *metadataLayout, cfg *config.DocumentConfig, log *zap.Logger,
) (*apnxLayout, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, errors.New("APNX configuration is required")
	}
	mode := cfg.AZW3.APNX
	switch mode {
	case "", "none":
		return nil, nil
	case "eink", "app":
	default:
		return nil, fmt.Errorf("unsupported AZW3 APNX mode %q: expected none, eink or app", mode)
	}
	if !cfg.PageMap.Enable {
		log.Warn("APNX requested but page maps are disabled; omitting sidecar", zap.String("apnx", mode))
		return nil, nil
	}
	if pub == nil || text == nil || metadata == nil || text.MainLength == 0 || len(text.Documents) == 0 {
		return nil, errors.New("APNX requires a publication, frozen text and compiled metadata")
	}
	if len(pub.Pages) == 0 {
		log.Warn("APNX requested but the publication has no pages; omitting sidecar", zap.String("apnx", mode))
		return nil, nil
	}
	if len(pub.Pages) > math.MaxUint16 {
		return nil, errors.New("APNX page count exceeds 65535")
	}
	identity, err := apnxBookIdentity(metadata)
	if err != nil {
		return nil, err
	}
	path, err := apnxOutputPath(bookPath, mode)
	if err != nil {
		return nil, err
	}
	var mainLength uint64
	documentEnds := make([]uint64, len(text.Documents))
	for i, document := range text.Documents {
		mainLength += uint64(len(document.Data))
		documentEnds[i] = mainLength
	}
	if mainLength != uint64(text.MainLength) {
		return nil, errors.New("APNX main-text length does not match frozen documents")
	}
	offsets := make([]uint32, 0, len(pub.Pages))
	var ranges []string
	lastNumber := 0
	for i, page := range pub.Pages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if page.PageNum <= lastNumber {
			return nil, fmt.Errorf("APNX page %d has a nonpositive or out-of-order number %d", i+1, page.PageNum)
		}
		position, err := resolveMainPosition(text, linkTarget{Filename: page.Filename, ID: page.SpanID})
		if err != nil {
			return nil, fmt.Errorf("resolve APNX page %d: %w", page.PageNum, err)
		}
		if uint64(position.Position) >= documentEnds[position.FID] {
			return nil, fmt.Errorf("APNX page %d lies outside its frozen document", page.PageNum)
		}
		if i > 0 && position.Position < offsets[i-1] {
			return nil, fmt.Errorf("APNX page %d precedes the previous page in frozen text", page.PageNum)
		}
		if i == 0 || page.PageNum != lastNumber+1 {
			ranges = append(ranges, fmt.Sprintf("(%d,a,%d)", i+1, page.PageNum))
		}
		lastNumber = page.PageNum
		offsets = append(offsets, position.Position)
	}
	data, err := encodeAPNX(identity, strings.Join(ranges, ","), offsets)
	if err != nil {
		return nil, err
	}
	log.Debug("APNX sidecar prepared", zap.String("apnx", mode), zap.String("path", path), zap.Int("pages", len(offsets)))
	return &apnxLayout{Path: path, Data: data}, nil
}

func apnxBookIdentity(metadata *metadataLayout) (apnxIdentity, error) {
	if err := validatePalmDBName(metadata.Database); err != nil {
		return apnxIdentity{}, fmt.Errorf("APNX database identity: %w", err)
	}
	if len(metadata.ASIN) != 10 && len(metadata.ASIN) != 32 {
		return apnxIdentity{}, errors.New("APNX requires the compiled Kindle content identifier")
	}
	for _, char := range []byte(metadata.ASIN) {
		if !(char >= 'A' && char <= 'Z' || char >= '0' && char <= '9') {
			return apnxIdentity{}, errors.New("APNX content identifier must match normalized Kindle metadata")
		}
	}
	if metadata.ContentType != "PDOC" && metadata.ContentType != "EBOK" {
		return apnxIdentity{}, errors.New("APNX requires a compiled PDOC or EBOK content type")
	}
	for _, field := range []struct {
		id   uint32
		want string
	}{{113, metadata.ASIN}, {504, metadata.ASIN}, {501, metadata.ContentType}} {
		count := 0
		for _, record := range metadata.EXTH {
			if record.ID == field.id {
				count++
				if !bytes.Equal(record.Data, []byte(field.want)) {
					return apnxIdentity{}, fmt.Errorf("APNX identity disagrees with EXTH %d", field.id)
				}
			}
		}
		if count != 1 {
			return apnxIdentity{}, fmt.Errorf("APNX requires exactly one EXTH %d identity", field.id)
		}
	}
	return apnxIdentity{
		ContentGUID: fmt.Sprintf("%08x", metadata.UID), ASIN: metadata.ASIN, ContentType: metadata.ContentType,
		Format: "MOBI_8", FileRevisionID: "1", ACR: metadata.Database,
	}, nil
}

func encodeAPNX(identity apnxIdentity, pageMap string, offsets []uint32) ([]byte, error) {
	if len(offsets) == 0 || len(offsets) > math.MaxUint16 {
		return nil, errors.New("APNX requires 1 to 65535 page offsets")
	}
	if !utf8.ValidString(pageMap) {
		return nil, errors.New("APNX page names must be UTF-8")
	}
	content, err := json.Marshal(identity)
	if err != nil {
		return nil, fmt.Errorf("serialize APNX content header: %w", err)
	}
	pages, err := json.Marshal(apnxPageHeader{ASIN: identity.ASIN, PageMap: pageMap})
	if err != nil {
		return nil, fmt.Errorf("serialize APNX page header: %w", err)
	}
	if uint64(len(content))+12 > math.MaxUint32 || len(pages) > math.MaxUint16 {
		return nil, errors.New("APNX JSON headers exceed their bounded byte lengths")
	}
	data := binary.BigEndian.AppendUint16(nil, 1)
	data = binary.BigEndian.AppendUint16(data, 1)
	data = binary.BigEndian.AppendUint32(data, uint32(12+len(content)))
	data = binary.BigEndian.AppendUint32(data, uint32(len(content)))
	data = append(data, content...)
	for _, word := range []uint16{1, uint16(len(pages)), uint16(len(offsets)), 32} {
		data = binary.BigEndian.AppendUint16(data, word)
	}
	data = append(data, pages...)
	for _, offset := range offsets {
		data = binary.BigEndian.AppendUint32(data, offset)
	}
	return data, nil
}

func apnxOutputPath(bookPath, mode string) (string, error) {
	if mode == "" || mode == "none" {
		return "", nil
	}
	if mode != "eink" && mode != "app" {
		return "", fmt.Errorf("unsupported APNX placement %q", mode)
	}
	if bookPath == "" || os.IsPathSeparator(bookPath[len(bookPath)-1]) {
		return "", errors.New("APNX placement requires a final book filename")
	}
	name := filepath.Base(bookPath)
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if stem == "" || stem == "." || stem == ".." {
		return "", errors.New("APNX placement requires a nonempty book stem")
	}
	directory := filepath.Dir(bookPath)
	if mode == "eink" {
		directory = filepath.Join(directory, stem+".sdr")
	}
	path := filepath.Join(directory, stem+".apnx")
	if path == filepath.Clean(bookPath) {
		return "", errors.New("APNX sidecar path collides with the book filename")
	}
	return path, nil
}
