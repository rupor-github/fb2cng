package azw3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"fbc/convert/publication"
	"fbc/css"
)

type resourceKind byte

const (
	resourceAny resourceKind = iota
	resourceImage
	resourceFont
	resourceSVG
)

type resourceReference struct {
	Number   uint32
	Kind     resourceKind
	MimeType string
}

type resourceLayout struct {
	Records    [][]byte
	Flows      []resourceFlow
	References map[string]resourceReference
}

// SVG flows use the text-flow namespace, not the physical embed namespace.
// Main text is flow 0 and the combined CSS (or an empty placeholder) is flow 1.
type resourceFlow struct {
	Filename string
	Data     []byte
}

type resourceCandidate struct {
	Filename string
	MimeType string
	Data     []byte
	Kind     resourceKind
}

type resourceContentKey struct {
	Kind   resourceKind
	Digest [sha256.Size]byte
	Scope  string // Relative SVG dependencies are interpreted from this directory.
}

// compileResources allocates stable, separate embed and SVG-flow namespaces.
// Equal payloads share a physical record/flow (SVG aliases also share a relative
// base directory), while every publication filename remains a valid URI alias.
func compileResources(ctx context.Context, pub *publication.Publication) (*resourceLayout, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if pub == nil {
		return nil, errors.New("publication is required")
	}

	candidates := make([]resourceCandidate, 0, len(pub.Images)+len(pub.Stylesheet.Resources))
	for _, image := range pub.Images {
		if image == nil {
			return nil, errors.New("publication contains a nil image")
		}
		candidates = append(candidates, resourceCandidate{
			Filename: image.Filename,
			MimeType: image.MimeType,
			Data:     image.Data,
			Kind:     resourceImage,
		})
	}
	for _, resource := range pub.Stylesheet.Resources {
		kind, mimeType, err := classifyResource(resource.MimeType, resource.Data)
		if err != nil {
			return nil, fmt.Errorf("stylesheet resource %q: %w", resource.Filename, err)
		}
		candidates = append(candidates, resourceCandidate{
			Filename: resource.Filename,
			MimeType: mimeType,
			Data:     resource.Data,
			Kind:     kind,
		})
	}

	for i := range candidates {
		candidate := &candidates[i]
		if err := validateResourceFilename(candidate.Filename); err != nil {
			return nil, err
		}
		if len(candidate.Data) == 0 {
			return nil, fmt.Errorf("publication resource %q is empty", candidate.Filename)
		}
		if candidate.Kind == resourceImage && imageMediaType(candidate.MimeType) == "image/svg+xml" {
			candidate.Kind, candidate.MimeType = resourceSVG, "image/svg+xml"
		}
		if candidate.Kind == resourceSVG {
			if !utf8.Valid(candidate.Data) {
				return nil, fmt.Errorf("SVG resource %q must be UTF-8", candidate.Filename)
			}
			continue
		}
		if candidate.Kind == resourceImage {
			mimeType, err := normalizeRasterMIME(candidate.MimeType)
			if err != nil {
				return nil, fmt.Errorf("publication image %q: %w", candidate.Filename, err)
			}
			candidate.MimeType = mimeType
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Filename != candidates[j].Filename {
			return candidates[i].Filename < candidates[j].Filename
		}
		if candidates[i].Kind != candidates[j].Kind {
			return candidates[i].Kind < candidates[j].Kind
		}
		return candidates[i].MimeType < candidates[j].MimeType
	})

	result := &resourceLayout{References: make(map[string]resourceReference, len(candidates))}
	contents := make(map[resourceContentKey]resourceReference)
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if previous, exists := result.References[candidate.Filename]; exists {
			key := candidate.contentKey()
			if previous.Kind != candidate.Kind || previous.MimeType != candidate.MimeType || contents[key].Number != previous.Number {
				return nil, fmt.Errorf("conflicting publication resources use filename %q", candidate.Filename)
			}
			continue
		}

		key := candidate.contentKey()
		reference, exists := contents[key]
		if !exists {
			reference = resourceReference{Kind: candidate.Kind, MimeType: candidate.MimeType}
			if candidate.Kind == resourceSVG {
				if len(result.Flows) >= (1<<20)-2 {
					return nil, errors.New("too many KF8 SVG flows for four-digit flow references")
				}
				reference.Number = uint32(len(result.Flows) + 2)
				result.Flows = append(result.Flows, resourceFlow{Filename: candidate.Filename, Data: bytes.Clone(candidate.Data)})
			} else {
				if len(result.Records) >= (1<<20)-1 {
					return nil, errors.New("too many KF8 resources for four-digit resource references")
				}
				record, err := encodeResource(candidate)
				if err != nil {
					return nil, fmt.Errorf("encode publication resource %q: %w", candidate.Filename, err)
				}
				reference.Number = uint32(len(result.Records) + 1)
				result.Records = append(result.Records, record)
			}
			contents[key] = reference
		} else if reference.MimeType != candidate.MimeType {
			return nil, fmt.Errorf("identical publication resource %q has conflicting MIME type %q", candidate.Filename, candidate.MimeType)
		}
		result.References[candidate.Filename] = reference
	}
	for i := range result.Flows {
		flow := &result.Flows[i]
		data, err := lowerSVGResources(ctx, flow.Filename, flow.Data, result)
		if err != nil {
			return nil, fmt.Errorf("lower SVG resource %q: %w", flow.Filename, err)
		}
		flow.Data = data
	}
	return result, nil
}

func (candidate resourceCandidate) contentKey() resourceContentKey {
	key := resourceContentKey{Kind: candidate.Kind, Digest: sha256.Sum256(candidate.Data)}
	if candidate.Kind == resourceSVG {
		key.Scope = path.Dir(candidate.Filename)
	}
	return key
}

func classifyResource(mimeType string, data []byte) (resourceKind, string, error) {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "font/ttf", "font/otf", "application/x-font-ttf", "application/x-font-otf", "application/x-font-opentype",
		"application/font-sfnt", "application/vnd.ms-opentype":
		if isTrueType(data) {
			return resourceFont, "font/ttf", nil
		}
		if len(data) >= 4 && bytes.Equal(data[:4], []byte("OTTO")) {
			return resourceFont, "font/otf", nil
		}
		return 0, "", errors.New("font resource does not contain a supported TTF or OTF sfnt")
	case "font/woff", "font/woff2", "application/font-woff", "application/font-woff2", "application/vnd.ms-fontobject":
		return 0, "", fmt.Errorf("unsupported KF8 font format %q; use TTF or OTF", mimeType)
	default:
		if imageMediaType(mimeType) == "image/svg+xml" {
			return resourceSVG, "image/svg+xml", nil
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "font/") ||
			strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "application/x-font-") {
			return 0, "", fmt.Errorf("unsupported KF8 font format %q; use TTF or OTF", mimeType)
		}
		mimeType, err := normalizeRasterMIME(mimeType)
		if err != nil {
			return 0, "", err
		}
		return resourceImage, mimeType, nil
	}
}

func normalizeRasterMIME(mimeType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "image/jpeg", "image/jpg":
		return "image/jpeg", nil
	case "image/png":
		return "image/png", nil
	case "image/gif":
		return "image/gif", nil
	default:
		return "", fmt.Errorf("unsupported KF8 raster MIME type %q", mimeType)
	}
}

func isTrueType(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return bytes.Equal(data[:4], []byte{0, 1, 0, 0}) || bytes.Equal(data[:4], []byte("true")) || bytes.Equal(data[:4], []byte("ttcf"))
}

func validateResourceFilename(filename string) error {
	if filename == "" || path.Clean(filename) != filename || path.IsAbs(filename) || filename == ".." ||
		strings.HasPrefix(filename, "../") || strings.ContainsAny(filename, "\\#?") {
		return fmt.Errorf("invalid publication resource filename %q", filename)
	}
	return nil
}

func encodeResource(candidate resourceCandidate) ([]byte, error) {
	if candidate.Kind == resourceImage {
		return bytes.Clone(candidate.Data), nil
	}
	if uint64(len(candidate.Data)) > math.MaxUint32 || uint64(len(candidate.Data)) > uint64(math.MaxInt-24) {
		return nil, errors.New("font exceeds 32-bit length")
	}
	data := make([]byte, 24, 24+len(candidate.Data))
	copy(data, "FONT")
	binary.BigEndian.PutUint32(data[4:], uint32(len(candidate.Data)))
	binary.BigEndian.PutUint32(data[12:], 24)
	return append(data, candidate.Data...), nil
}

func lowerStylesheetResources(data []byte, resources *resourceLayout) ([]byte, error) {
	if len(data) == 0 || (len(resources.References) == 0 &&
		!strings.Contains(strings.ToLower(string(data)), "url(") && !strings.Contains(strings.ToLower(string(data)), "@import")) {
		return bytes.Clone(data), nil
	}
	sheet := css.NewParser(nil).Parse(data, "")
	var rewriteErr error
	sheet.RewriteURLs(func(original string) string {
		if rewriteErr != nil {
			return original
		}
		lowered, local, err := lowerResourceURI("", original, resources, resourceAny)
		if err != nil {
			rewriteErr = err
			return original
		}
		if !local {
			return original
		}
		return lowered
	})
	if rewriteErr != nil {
		return nil, rewriteErr
	}
	var output bytes.Buffer
	if _, err := sheet.WriteTo(&output); err != nil {
		return nil, fmt.Errorf("serialize lowered stylesheet: %w", err)
	}
	return output.Bytes(), nil
}

func lowerResourceURI(filename, rawURI string, resources *resourceLayout, requiredKind resourceKind) (string, bool, error) {
	u, err := url.Parse(rawURI)
	if err != nil {
		return "", false, fmt.Errorf("invalid publication resource URI %q: %w", rawURI, err)
	}
	if u.Scheme == "kindle" {
		return "", false, fmt.Errorf("unexpected precompiled resource URI %q", rawURI)
	}
	if u.IsAbs() || u.Host != "" {
		return rawURI, false, nil
	}
	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || path.IsAbs(u.Path) || strings.Contains(u.Path, "\\") {
		return "", false, fmt.Errorf("unsupported local publication resource URI %q", rawURI)
	}
	resourcePath := path.Clean(u.Path)
	reference, exists := resources.References[resourcePath]
	if !exists && filename != "" {
		resourcePath = path.Join(path.Dir(filename), u.Path)
		reference, exists = resources.References[resourcePath]
	}
	if !exists {
		return "", false, fmt.Errorf("unresolved publication resource %q", rawURI)
	}
	if requiredKind != resourceAny && reference.Kind != requiredKind &&
		!(requiredKind == resourceImage && reference.Kind == resourceSVG) {
		return "", false, fmt.Errorf("image URI %q refers to a font", rawURI)
	}
	if reference.Kind == resourceSVG {
		return "kindle:flow:" + base32(reference.Number, 4) + "?mime=image/svg+xml", true, nil
	}
	uri := "kindle:embed:" + base32(reference.Number, 4)
	if reference.Kind == resourceImage {
		uri += "?mime=" + reference.MimeType
	}
	return uri, true, nil
}
