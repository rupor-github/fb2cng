package azw3

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"fbc/config"
	"fbc/content"
	"fbc/convert/publication"
)

// writeDebugArtifacts describes actual compiler inputs, transformations and frozen
// coordinates. It does not pretend to compute the Kindle reader's cascade/layout.
func writeDebugArtifacts(
	c *content.Content, pub *publication.Publication, text *textLayout, navigation *navigationLayout,
	metadata *metadataLayout, book *bookLayout, apnx *apnxLayout, cfg *config.DocumentConfig,
) error {
	artifacts := map[string][]byte{
		"book.rawml": text.Raw, "book.source.css": pub.Stylesheet.Data,
	}
	summary := fmt.Sprintf(
		"format=%s\ncompression=%s\ncompression_source=%s\napnx=%s\nmain_bytes=%d\nall_flow_bytes=%d\nrecords=%d\npages=%d\n",
		c.OutputFormat, book.Compression, cfg.AZW3.CompressionSource, cfg.AZW3.APNX,
		text.MainLength, len(text.Raw), len(book.Records), len(pub.Pages))
	artifacts["book.azw3.txt"] = []byte(summary)
	if apnx != nil {
		artifacts["book.apnx"] = apnx.Data
	}
	type documentInfo struct {
		Filename string         `json:"filename"`
		FID      int            `json:"fid"`
		Bytes    int            `json:"bytes"`
		Changes  []markupChange `json:"changes"`
	}
	var documents []documentInfo
	var trace strings.Builder
	trace.WriteString("Native KF8 lowering (not reader-computed layout)\n")
	if metadata.ASIN == "" && metadata.ContentType == "" {
		trace.WriteString("Kindle identity: local book; catalog ASIN/type records omitted.\n")
	} else {
		fmt.Fprintf(&trace, "Kindle identity: catalog metadata, type=%s identifier=%s.\n", metadata.ContentType, metadata.ASIN)
	}
	trace.WriteString("CSS: source order, duplicate declarations, selectors and at-rules preserved; only resource URLs lowered.\n")
	trace.WriteString("Pagination: statically resolved CSS breaks become data-AmznPageBreak attributes before freezing positions; " +
		"document splitting is unchanged.\n")
	for i, doc := range text.Documents {
		documents = append(documents, documentInfo{Filename: doc.Filename, FID: i, Bytes: len(doc.Data), Changes: doc.Changes})
		artifacts[fmt.Sprintf("azw3-compiled/%04d.xhtml", i)] = doc.Data
		fmt.Fprintf(&trace, "\nDOCUMENT %04d %s (%d bytes)\n", i, doc.Filename, len(doc.Data))
		for _, change := range doc.Changes {
			fmt.Fprintf(&trace, "  %s: <%s> id=%q class=%q selector=%q\n",
				change.Action, change.Tag, change.ID, change.Classes, change.Selector)
		}
	}
	i := 0
	for doc := range pub.Documents() {
		data, err := doc.Doc.WriteToBytes()
		if err != nil {
			return fmt.Errorf("serialize diagnostic publication: %w", err)
		}
		artifacts[fmt.Sprintf("azw3-input/%04d.xhtml", i)] = data
		i++
	}
	type anchorInfo struct {
		Filename string       `json:"filename"`
		ID       string       `json:"id,omitempty"`
		Position textPosition `json:"position"`
		URI      string       `json:"uri"`
	}
	var anchors []anchorInfo
	for target, position := range text.Positions {
		anchors = append(anchors, anchorInfo{Filename: target.Filename, ID: target.ID, Position: position, URI: positionURI(position)})
	}
	slices.SortFunc(anchors, func(a, b anchorInfo) int {
		if result := strings.Compare(a.Filename, b.Filename); result != 0 {
			return result
		}
		return strings.Compare(a.ID, b.ID)
	})
	type resourceInfo struct {
		Filename  string `json:"filename"`
		Kind      string `json:"kind"`
		Number    uint32 `json:"number"`
		MIME      string `json:"mime"`
		InputData string `json:"input_data"`
		SHA256    string `json:"sha256"`
	}
	payloads := make(map[string][]byte)
	for _, image := range pub.Images {
		payloads[image.Filename] = image.Data
	}
	for _, resource := range pub.Stylesheet.Resources {
		payloads[resource.Filename] = resource.Data
	}
	var resources []resourceInfo
	for _, name := range slices.Sorted(maps.Keys(text.References)) {
		reference := text.References[name]
		kind := "image"
		if reference.Kind == resourceFont {
			kind = "font"
		} else if reference.Kind == resourceSVG {
			kind = "svg-flow"
		}
		inputData := fmt.Sprintf("azw3-resources/%s-%04d.bin", kind, reference.Number)
		payload := payloads[name]
		artifacts[inputData] = payload
		resources = append(resources, resourceInfo{
			Filename: name, Kind: kind, Number: reference.Number, MIME: reference.MimeType,
			InputData: inputData, SHA256: fmt.Sprintf("%x", sha256.Sum256(payload)),
		})
		fmt.Fprintf(&trace, "RESOURCE %s -> %s %04d (%s)\n", name, kind, reference.Number, reference.MimeType)
	}
	type flowInfo struct {
		Number int    `json:"number"`
		Start  uint32 `json:"start"`
		End    uint32 `json:"end"`
		SHA256 string `json:"sha256"`
	}
	var flows []flowInfo
	for i := range int(text.FlowCount) {
		start, end := binary.BigEndian.Uint32(text.FDST[12+i*8:]), binary.BigEndian.Uint32(text.FDST[16+i*8:])
		flows = append(flows, flowInfo{Number: i, Start: start, End: end, SHA256: fmt.Sprintf("%x", sha256.Sum256(text.Raw[start:end]))})
		if i == 1 {
			artifacts["book.stylesheet.css"] = text.Raw[start:end]
		}
	}
	type recordInfo struct {
		Number    int    `json:"number"`
		Bytes     int    `json:"bytes"`
		PrefixHex string `json:"prefix_hex"`
		SHA256    string `json:"sha256"`
	}
	var records []recordInfo
	for i, record := range book.Records {
		records = append(records, recordInfo{
			Number: i, Bytes: len(record), PrefixHex: fmt.Sprintf("%x", record[:min(4, len(record))]),
			SHA256: fmt.Sprintf("%x", sha256.Sum256(record)),
		})
	}
	coverFilename := ""
	coverMode := "none"
	if cover := pub.Images[pub.CoverID]; cover != nil {
		coverFilename = cover.Filename
		coverMode = "resource"
		if _, exists := text.Positions[linkTarget{Filename: "cover.xhtml"}]; exists {
			coverMode = "document-fallback"
		}
	}
	fmt.Fprintf(&trace, "\nCOVER %s: %s\n", coverMode, coverFilename)
	artifacts["book.azw3-style-trace.txt"] = []byte(trace.String())
	values := map[string]any{
		"book.azw3-documents.json": documents,
		"book.azw3-anchors.json":   anchors,
		"book.azw3-resources.json": resources,
		"book.azw3-flows.json":     flows,
		"book.azw3-records.json":   records,
		"book.azw3-navigation.json": map[string]any{
			"toc": pub.Navigation, "landmarks": pub.Landmarks, "pages": pub.Pages,
			"start": navigation.Start, "title": metadata.Title, "asin": metadata.ASIN, "content_type": metadata.ContentType,
			"database_name": metadata.Database,
			"language":      c.Book.Description.TitleInfo.Lang.String(), "identifier": c.Book.Description.DocumentInfo.ID,
			"cover_filename": coverFilename, "cover_mode": coverMode,
		},
	}
	for name, value := range values {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return fmt.Errorf("encode %s: %w", name, err)
		}
		artifacts[name] = append(data, '\n')
	}
	var failures []error
	for _, name := range slices.Sorted(maps.Keys(artifacts)) {
		path := filepath.Join(c.WorkDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			failures = append(failures, fmt.Errorf("create diagnostic directory %s: %w", name, err))
			continue
		}
		if err := os.WriteFile(path, artifacts[name], 0644); err != nil {
			failures = append(failures, fmt.Errorf("write diagnostic %s: %w", name, err))
		}
	}
	return errors.Join(failures...)
}
