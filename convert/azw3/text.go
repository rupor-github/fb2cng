package azw3

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/beevik/etree"

	"fbc/convert/publication"
	"fbc/css"
)

type linkTarget struct {
	Filename string
	ID       string
}

// textPosition identifies a byte coordinate in the reassembled main text flow,
// not a rune count or the physical fragment payload location after its skeleton.
type textPosition struct {
	FID      uint32
	Offset   uint32
	Position uint32
}

func resolveMainPosition(text *textLayout, target linkTarget) (textPosition, error) {
	position, found := text.Positions[target]
	if !found || position.Position >= text.MainLength || uint64(position.FID) >= uint64(len(text.Documents)) ||
		text.Documents[position.FID].Filename != target.Filename {
		return textPosition{}, fmt.Errorf("destination %s#%s has no frozen main-text position", target.Filename, target.ID)
	}
	top, found := text.Positions[linkTarget{Filename: target.Filename}]
	if !found || top.FID != position.FID || top.Offset != 0 || uint64(top.Position)+uint64(position.Offset) != uint64(position.Position) {
		return textPosition{}, fmt.Errorf("destination %s#%s has inconsistent fragment coordinates", target.Filename, target.ID)
	}
	return position, nil
}

type textDocument struct {
	Filename string
	Data     []byte
}

// textLayout is one compilation pass, not a complete ebook. It allocates resources
// and lowers their variable-length URIs before freezing text positions; later passes
// must not patch its finalized raw stream and retain stale positions.
type textLayout struct {
	Raw        []byte
	Documents  []textDocument
	Positions  map[linkTarget]textPosition
	Skeletons  [][]byte
	Fragments  [][]byte
	FDST       []byte
	FlowCount  uint32
	MainLength uint32
	Resources  [][]byte
	References map[string]resourceReference
}

type linkFixup struct {
	Element *etree.Element
	Key     string
	Target  linkTarget
}

type pendingDocument struct {
	Filename string
	Doc      *etree.Document
	Data     []byte
	Scan     documentScan
	Fixups   []linkFixup
}

type documentScan struct {
	BodyTag   uint32
	BodyStart uint32
	BodyEnd   uint32
	IDs       map[string]uint32
}

// compileText copies an already-rendered publication, freezes byte positions with
// fixed-width link placeholders, then serializes one body fragment per document.
// Multiple adapters can consume the original publication without rerendering it.
func compileText(ctx context.Context, pub *publication.Publication) (*textLayout, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if pub == nil {
		return nil, errors.New("publication is required")
	}
	if !utf8.Valid(pub.Stylesheet.Data) {
		return nil, errors.New("publication CSS must be UTF-8")
	}
	resources, err := compileResources(ctx, pub)
	if err != nil {
		return nil, fmt.Errorf("compile KF8 resources: %w", err)
	}
	stylesheet, err := lowerStylesheetResources(pub.Stylesheet.Data, resources)
	if err != nil {
		return nil, fmt.Errorf("lower KF8 stylesheet resources: %w", err)
	}
	var documents []pendingDocument
	files := make(map[string]bool)
	for chapter := range pub.Documents() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		filename := chapter.Filename
		if filename == "" || path.Clean(filename) != filename || strings.ContainsAny(filename, "\\#?") ||
			path.IsAbs(filename) || filename == ".." || strings.HasPrefix(filename, "../") {
			return nil, fmt.Errorf("invalid publication document filename %q", filename)
		}
		if len(documents) >= 1<<20 {
			return nil, errors.New("too many KF8 fragments for four-digit FIDs")
		}
		files[filename] = true
		documents = append(documents, pendingDocument{Filename: filename, Doc: chapter.Doc.Copy()})
	}
	if len(documents) == 0 {
		return nil, errors.New("publication has no reading-order documents")
	}

	result := &textLayout{
		Positions:  make(map[linkTarget]textPosition),
		Resources:  resources.Records,
		References: resources.References,
	}
	cssNeeded := len(stylesheet) > 0 || len(resources.Flows) > 0
	var textLength uint64
	for i := range documents {
		doc := &documents[i]
		needed, err := prepareTextDocument(ctx, doc, uint32(i), files, resources)
		if err != nil {
			return nil, fmt.Errorf("prepare %s: %w", doc.Filename, err)
		}
		cssNeeded = cssNeeded || needed
		doc.Data, err = doc.Doc.WriteToBytes()
		if err != nil {
			return nil, fmt.Errorf("serialize %s: %w", doc.Filename, err)
		}
		if textLength+uint64(len(doc.Data)) > math.MaxUint32 {
			return nil, errors.New("KF8 main text exceeds 32-bit byte offsets")
		}
		doc.Scan, err = scanDocument(doc.Data)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", doc.Filename, err)
		}
		top := textPosition{FID: uint32(i), Position: uint32(textLength) + doc.Scan.BodyStart}
		result.Positions[linkTarget{Filename: doc.Filename}] = top
		for id, offset := range doc.Scan.IDs {
			if offset < doc.Scan.BodyTag || offset >= doc.Scan.BodyEnd {
				continue // Head/root IDs do not name readable fragment positions.
			}
			position := top
			if offset > doc.Scan.BodyStart {
				position.Offset = offset - doc.Scan.BodyStart
				position.Position += position.Offset
			}
			result.Positions[linkTarget{Filename: doc.Filename, ID: id}] = position
		}
		textLength += uint64(len(doc.Data))
	}

	var skeletonEntries, fragmentEntries []indexEntry
	var fragmentStrings []string
	var start uint32
	for i := range documents {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		doc := &documents[i]
		for _, fixup := range doc.Fixups {
			position, exists := result.Positions[fixup.Target]
			if !exists {
				return nil, fmt.Errorf("unresolved reading target %s#%s in %s", fixup.Target.Filename, fixup.Target.ID, doc.Filename)
			}
			fixup.Element.CreateAttr(fixup.Key, positionURI(position))
		}
		final, err := doc.Doc.WriteToBytes()
		if err != nil {
			return nil, fmt.Errorf("serialize fixed links in %s: %w", doc.Filename, err)
		}
		check, err := scanDocument(final)
		if err != nil {
			return nil, fmt.Errorf("check fixed links in %s: %w", doc.Filename, err)
		}
		if len(final) != len(doc.Data) || check.BodyTag != doc.Scan.BodyTag || check.BodyStart != doc.Scan.BodyStart ||
			check.BodyEnd != doc.Scan.BodyEnd || !maps.Equal(check.IDs, doc.Scan.IDs) {
			return nil, fmt.Errorf("link fixups changed the frozen byte layout of %s", doc.Filename)
		}
		body := final[check.BodyStart:check.BodyEnd]
		skeleton := append([]byte(nil), final[:check.BodyStart]...)
		skeleton = append(skeleton, final[check.BodyEnd:]...)
		result.Raw = append(result.Raw, skeleton...)
		result.Raw = append(result.Raw, body...)
		result.Documents = append(result.Documents, textDocument{Filename: doc.Filename, Data: final})
		skeletonEntries = append(skeletonEntries, indexEntry{
			Key: fmt.Sprintf("SKEL%010d", i), Values: map[byte][]uint32{1: {1}, 6: {start, uint32(len(skeleton))}},
		})
		fragmentEntries = append(fragmentEntries, indexEntry{
			Key:    fmt.Sprintf("%010d", start+check.BodyStart),
			Values: map[byte][]uint32{3: {uint32(i)}, 4: {uint32(i)}, 6: {0, uint32(len(body))}},
		})
		fragmentStrings = append(fragmentStrings, "P-//*[@aid='"+base32(uint32(i), 4)+"']")
		start += uint32(len(final))
	}
	stringsData, stringOffsets, err := encodeStrings(fragmentStrings)
	if err != nil {
		return nil, fmt.Errorf("encode KF8 fragment strings: %w", err)
	}
	for i := range fragmentEntries {
		fragmentEntries[i].Values[2] = []uint32{stringOffsets[i]}
	}
	result.Fragments, err = encodeIndex([]indexTag{{2, 1}, {3, 1}, {4, 1}, {6, 2}}, fragmentEntries, uint32(len(stringsData)))
	if err != nil {
		return nil, fmt.Errorf("encode KF8 fragment index: %w", err)
	}
	result.Fragments = append(result.Fragments, stringsData...)
	result.Skeletons, err = encodeIndex([]indexTag{{1, 1}, {6, 2}}, skeletonEntries, 0)
	if err != nil {
		return nil, fmt.Errorf("encode KF8 skeleton index: %w", err)
	}
	result.MainLength = uint32(len(result.Raw))
	lengths := []uint32{result.MainLength}
	if cssNeeded {
		css := stylesheet
		if len(css) == 0 {
			css = []byte("\n")
		}
		if uint64(len(result.Raw))+uint64(len(css)) > math.MaxUint32 {
			return nil, errors.New("KF8 flows exceed 32-bit byte offsets")
		}
		lengths = append(lengths, uint32(len(css)))
		result.Raw = append(result.Raw, css...)
	}
	for _, flow := range resources.Flows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if uint64(len(result.Raw))+uint64(len(flow.Data)) > math.MaxUint32 {
			return nil, errors.New("KF8 flows exceed 32-bit byte offsets")
		}
		lengths = append(lengths, uint32(len(flow.Data)))
		result.Raw = append(result.Raw, flow.Data...)
	}
	result.FDST, err = encodeFDST(lengths)
	if err != nil {
		return nil, fmt.Errorf("encode KF8 flows: %w", err)
	}
	result.FlowCount = uint32(len(lengths))
	return result, nil
}

func prepareTextDocument(
	ctx context.Context,
	doc *pendingDocument,
	fid uint32,
	files map[string]bool,
	resources *resourceLayout,
) (bool, error) {
	root := doc.Doc.Root()
	if root == nil || root.Tag != "html" {
		return false, errors.New("expected an XHTML html root")
	}
	bodies := root.SelectElements("body")
	if len(bodies) != 1 {
		return false, errors.New("expected one XHTML body")
	}
	body := bodies[0]
	body.CreateAttr("aid", base32(fid, 4))
	if len(body.Child) == 0 {
		body.CreateText("") // Force an explicit end tag for an empty fragment.
	}
	cssNeeded := false
	stack := []*etree.Element{root}
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		elem := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		stack = append(stack, elem.ChildElements()...)
		if elem.Tag == "span" && len(elem.Child) == 0 && slices.Contains(strings.Fields(elem.SelectAttrValue("class", "")), "page-marker") {
			// A span is not an HTML void element. Keep page anchors explicitly
			// closed before freezing byte positions, so readers and APNX mapping
			// cannot treat a self-closing marker as the preceding named anchor.
			elem.CreateText("")
		}
		if elem.Tag == "img" || elem.Tag == "image" {
			src := elem.SelectAttr("src")
			if elem.Tag == "image" {
				src = elem.SelectAttr("xlink:href")
				if src == nil {
					src = elem.SelectAttr("href")
				}
			}
			if src == nil {
				return false, errors.New("XHTML image is missing its source URI")
			}
			lowered, local, err := lowerResourceURI(doc.Filename, src.Value, resources, resourceImage)
			if err != nil {
				return false, err
			}
			if local {
				elem.CreateAttr(src.FullKey(), lowered)
			}
		}
		if style := elem.SelectAttr("style"); style != nil && strings.Contains(strings.ToLower(style.Value), "url") {
			var rewriteErr error
			lowered := css.RewriteValueURLsUnquoted(style.Value, func(original string) string {
				if rewriteErr != nil {
					return original
				}
				uri, local, err := lowerResourceURI(doc.Filename, original, resources, resourceAny)
				if err != nil {
					rewriteErr = err
					return original
				}
				if !local {
					return original
				}
				return uri
			})
			if rewriteErr != nil {
				return false, rewriteErr
			}
			elem.CreateAttr(style.FullKey(), lowered)
		}
		for _, attr := range elem.Attr {
			if attr.Key != "href" || (elem.Tag != "a" && elem.Tag != "link") {
				continue
			}
			if elem.Tag == "link" && elem.SelectAttrValue("rel", "") != "stylesheet" {
				continue
			}
			target, local, err := resolveTextTarget(doc.Filename, attr.Value)
			if err != nil {
				return false, err
			}
			if !local {
				continue
			}
			if elem.Tag == "link" {
				if target.Filename != "stylesheet.css" || target.ID != "" {
					return false, fmt.Errorf("unsupported local stylesheet %q", attr.Value)
				}
				elem.CreateAttr(attr.FullKey(), "kindle:flow:0001?mime=text/css")
				cssNeeded = true
				continue
			}
			if !files[target.Filename] {
				return false, fmt.Errorf("reading target %q is not a publication document", attr.Value)
			}
			doc.Fixups = append(doc.Fixups, linkFixup{Element: elem, Key: attr.FullKey(), Target: target})
			elem.CreateAttr(attr.FullKey(), positionURI(textPosition{}))
		}
	}
	return cssNeeded, nil
}

func resolveTextTarget(filename, href string) (linkTarget, bool, error) {
	u, err := url.Parse(href)
	if err != nil {
		return linkTarget{}, false, fmt.Errorf("invalid publication URI %q: %w", href, err)
	}
	if u.Scheme == "kindle" {
		return linkTarget{}, false, fmt.Errorf("unexpected precompiled URI %q", href)
	}
	if u.IsAbs() || u.Host != "" {
		return linkTarget{}, false, nil
	}
	if u.RawQuery != "" || u.ForceQuery || path.IsAbs(u.Path) || strings.Contains(u.Path, "\\") {
		return linkTarget{}, false, fmt.Errorf("unsupported local publication URI %q", href)
	}
	resolved := filename
	if u.Path != "" {
		resolved = path.Join(path.Dir(filename), u.Path)
	}
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return linkTarget{}, false, fmt.Errorf("publication URI escapes the book: %q", href)
	}
	return linkTarget{Filename: resolved, ID: u.Fragment}, true, nil
}

func positionURI(position textPosition) string {
	return "kindle:pos:fid:" + base32(position.FID, 4) + ":off:" + base32(position.Offset, 10)
}

func base32(value uint32, width int) string {
	encoded := strings.ToUpper(strconv.FormatUint(uint64(value), 32))
	return strings.Repeat("0", max(0, width-len(encoded))) + encoded
}

func scanDocument(data []byte) (documentScan, error) {
	if uint64(len(data)) > math.MaxUint32 {
		return documentScan{}, errors.New("XHTML document exceeds 32-bit byte offsets")
	}
	result := documentScan{IDs: make(map[string]uint32)}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	depth, bodies := 0, 0
	for {
		before := decoder.InputOffset()
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return documentScan{}, err
		}
		switch element := token.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 && element.Name.Local == "body" {
				bodies++
				result.BodyTag, result.BodyStart = uint32(before), uint32(decoder.InputOffset())
				if bytes.HasSuffix(data[result.BodyTag:result.BodyStart], []byte("/>")) {
					return documentScan{}, errors.New("XHTML body must have an explicit end tag")
				}
			}
			for _, attr := range element.Attr {
				if attr.Value == "" || attr.Name.Space != "" ||
					(attr.Name.Local != "id" && (element.Name.Local != "a" || attr.Name.Local != "name")) {
					continue
				}
				if previous, exists := result.IDs[attr.Value]; exists && previous != uint32(before) {
					return documentScan{}, fmt.Errorf("duplicate reading anchor %q", attr.Value)
				}
				result.IDs[attr.Value] = uint32(before)
			}
		case xml.EndElement:
			if depth == 2 && element.Name.Local == "body" {
				result.BodyEnd = uint32(before)
			}
			depth--
		}
	}
	if bodies != 1 || result.BodyEnd < result.BodyStart || !bytes.HasPrefix(data[result.BodyEnd:], []byte("</")) {
		return documentScan{}, errors.New("expected one explicitly closed XHTML body")
	}
	return result, nil
}
