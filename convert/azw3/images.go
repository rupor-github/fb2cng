package azw3

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/beevik/etree"
	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/convert/publication"
	"fbc/fb2"
)

// preparePublicationImages makes a private resource view before resource numbering
// and text layout. Body rasters have already undergone common image preparation:
// do not resize or recompress them a second time. SVG uses native flows unless
// explicit raster policy or cover compatibility requires JPEG. CSS-only images
// have not undergone image preparation.
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
	policy := fb2.ImagePolicyFor(common.OutputFmtAzw3, cfg)
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
		binary := fb2.BinaryObject{
			ID: id, ContentType: imageMediaType(source.MimeType), Data: source.Data, BuiltinVignette: source.BuiltinVignette,
		}
		image := binary.PrepareImage(policy, id == pub.CoverID, cfg, log)
		if err := sizeNativeSVG(image); err != nil {
			return nil, fmt.Errorf("size SVG image %q: %w", id, err)
		}
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
			if err := sizeNativeSVG(image); err != nil {
				return nil, fmt.Errorf("size CSS SVG image %q: %w", resource.Filename, err)
			}
		}
		resource.MimeType, resource.Data = image.MimeType, image.Data
	}
	return &result, nil
}

// Explicit intrinsic dimensions preserve the common screen/scale policy for
// both img references and CSS backgrounds without discarding vector geometry.
// Damaged sources kept by use_broken are never repaired or reserialized here.
func sizeNativeSVG(image *fb2.BookImage) error {
	if imageMediaType(image.MimeType) != "image/svg+xml" || image.Dim.Width <= 0 || image.Dim.Height <= 0 {
		return nil
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(image.Data); err != nil || doc.Root() == nil || doc.Root().Tag != "svg" {
		return nil
	}
	doc.Root().CreateAttr("width", fmt.Sprintf("%d", image.Dim.Width))
	doc.Root().CreateAttr("height", fmt.Sprintf("%d", image.Dim.Height))
	data, err := doc.WriteToBytes()
	if err != nil {
		return err
	}
	image.Data = data
	return nil
}

func imageMediaType(value string) string {
	mediaType, _, _ := strings.Cut(value, ";")
	return strings.ToLower(strings.TrimSpace(mediaType))
}
