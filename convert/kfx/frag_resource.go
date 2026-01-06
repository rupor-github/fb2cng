package kfx

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"fbc/fb2"
)

func buildImageResourceFragments(images fb2.BookImages) ([]*Fragment, []*Fragment, imageResourceInfoByID) {
	if len(images) == 0 {
		return nil, nil, nil
	}

	ids := make([]string, 0, len(images))
	for id := range images {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	external := make([]*Fragment, 0, len(images))
	raw := make([]*Fragment, 0, len(images))
	resourceInfoByImageID := make(imageResourceInfoByID, len(images))

	idx := 0
	for _, id := range ids {
		img := images[id]
		if img == nil || len(img.Data) == 0 {
			continue
		}

		format := imageFormatSymbol(img.MimeType)
		if format < 0 {
			continue
		}

		idx++
		location := makeResourceLocation(idx)
		resourceName := makeResourceName(idx)

		external = append(external, &Fragment{
			FType:   SymExtResource,
			FIDName: resourceName,
			Value: NewExternalResource(location, format, int64(img.Dim.Width), int64(img.Dim.Height)).
				SetString(SymResourceName, resourceName), // $175 = resource_name as string
		})

		raw = append(raw, &Fragment{
			FType:   SymRawMedia,
			FIDName: location, // fid == location
			Value:   RawValue(img.Data),
		})

		resourceInfoByImageID[id] = imageResourceInfo{
			ResourceName: resourceName,
			Width:        img.Dim.Width,
			Height:       img.Dim.Height,
		}
	}

	return external, raw, resourceInfoByImageID
}

func makeResourceLocation(idx int) string {
	// Matches reference KFX pattern: "resource/rsrcXYZ" where XYZ is base36-encoded index.
	// Base36 encoding is used to keep resource location strings compact while maintaining
	// compatibility with Amazon's KFX format conventions.
	return fmt.Sprintf("resource/rsrc%s", toBase36(idx))
}

func makeResourceName(idx int) string {
	// Matches reference KFX pattern: short local symbols like "e40G".
	// Uses base36 encoding for compact representation, consistent with resource locations.
	// These are the fragment IDs (FIDName) for external resource fragments.
	return fmt.Sprintf("e%s", toBase36(idx))
}

// toBase36 converts an integer to a base36 string (0-9a-z).
// Used for resource naming to match reference KFX format conventions.
// Returns "0" for values <= 0.
func toBase36(v int) string {
	if v <= 0 {
		return "0"
	}
	return strings.ToUpper(strconv.FormatInt(int64(v), 36))
}

func imageFormatSymbol(mimeType string) KFXSymbol {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	switch m {
	case "image/jpeg", "image/jpg":
		return SymFormatJPG
	case "image/png":
		return SymFormatPNG
	case "image/gif":
		return SymFormatGIF
	default:
		return -1
	}
}
