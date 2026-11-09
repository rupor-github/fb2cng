package azw3

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.uber.org/zap"

	"fbc/config"
	"fbc/convert/publication"
	"fbc/fb2"
)

// preparePublicationImages makes a private resource view before resource numbering
// and text layout. Body rasters have already undergone common image preparation:
// do not resize or recompress them a second time. Retained SVG artwork still needs
// the raster backend fallback. CSS-only images have not undergone image preparation.
// Publication filenames remain URI aliases, even when their encoding changes.
func preparePublicationImages(
	ctx context.Context, pub *publication.Publication, cfg *config.ImagesConfig, log *zap.Logger,
) (*publication.Publication, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if pub == nil || cfg == nil {
		return nil, errors.New("publication and image configuration are required")
	}
	policy := fb2.ImagePolicy{RequireJPEG: true, KindleSVGStrokes: true}
	result := *pub
	result.Images = maps.Clone(pub.Images)
	result.Stylesheet.Resources = slices.Clone(pub.Stylesheet.Resources)
	for _, id := range slices.Sorted(maps.Keys(pub.Images)) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		source := pub.Images[id]
		if source == nil {
			return nil, fmt.Errorf("publication image %q is nil", id)
		}
		if imageMediaType(source.MimeType) != "image/svg+xml" {
			continue
		}
		log.Debug("Falling back to SVG rasterization for AZW3 output",
			zap.String("id", id), zap.String("filename", source.Filename))
		binary := fb2.BinaryObject{
			ID: id, ContentType: imageMediaType(source.MimeType), Data: source.Data, BuiltinVignette: source.BuiltinVignette,
		}
		image := binary.RasterizeSVG(policy, id == pub.CoverID, cfg, log)
		image.Filename = source.Filename
		result.Images[id] = image
	}
	for i := range result.Stylesheet.Resources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		resource := &result.Stylesheet.Resources[i]
		if !strings.HasPrefix(imageMediaType(resource.MimeType), "image/") {
			continue
		}
		// A CSS reference to a body/cover binary must reuse its prepared bytes,
		// not apply scaling and cover processing to the same image twice.
		image := result.Images[resource.ResolvedID]
		if resource.ResolvedID == "" || image == nil {
			binary := fb2.BinaryObject{ID: resource.ResolvedID, ContentType: imageMediaType(resource.MimeType), Data: resource.Data}
			image = binary.PrepareImage(policy, false, cfg, log)
		}
		resource.MimeType, resource.Data = image.MimeType, image.Data
	}
	return &result, nil
}

func imageMediaType(value string) string {
	mediaType, _, _ := strings.Cut(value, ";")
	return strings.ToLower(strings.TrimSpace(mediaType))
}
