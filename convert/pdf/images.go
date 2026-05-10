package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"sort"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"fbc/convert/pdf/docwriter"
	"fbc/fb2"
	imgutil "fbc/utils/images"
)

type pdfImageResource struct {
	ObjectID int
	Name     string
	Dict     docwriter.Dict
	Data     []byte
}

func fitPDFImageInBox(doc skeletonDocument, img *fb2.BookImage, x, y, maxWidth, maxHeight float64) (pdfRect, bool) {
	width, height, ok := fitPDFImageSize(doc, img, maxWidth, maxHeight)
	if !ok {
		return pdfRect{}, false
	}
	return pdfRect{
		X1: x + max((maxWidth-width)/2, 0),
		Y1: y + max((maxHeight-height)/2, 0),
		X2: x + max((maxWidth-width)/2, 0) + width,
		Y2: y + max((maxHeight-height)/2, 0) + height,
	}, true
}

func fitPDFImageSize(doc skeletonDocument, img *fb2.BookImage, maxWidth, maxHeight float64) (float64, float64, bool) {
	width, height := naturalPDFImageSize(doc, img)
	if width <= 0 || height <= 0 || maxWidth <= 0 || maxHeight <= 0 {
		return 0, 0, false
	}
	scale := min(maxWidth/width, maxHeight/height)
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return 0, 0, false
	}
	if scale > 1 {
		scale = 1
	}
	width *= scale
	height *= scale
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func naturalPDFImageSize(doc skeletonDocument, img *fb2.BookImage) (float64, float64) {
	widthPx, heightPx := pdfImagePixelSize(img)
	if widthPx <= 0 || heightPx <= 0 {
		return 0, 0
	}
	if doc.ScreenWidthPx > 0 && doc.ScreenHeightPx > 0 && doc.PageWidth > 0 && doc.PageHeight > 0 {
		return float64(widthPx) * doc.PageWidth / float64(doc.ScreenWidthPx),
			float64(heightPx) * doc.PageHeight / float64(doc.ScreenHeightPx)
	}
	return float64(widthPx) * 72 / defaultDPI, float64(heightPx) * 72 / defaultDPI
}

func pdfImagePixelSize(img *fb2.BookImage) (int, int) {
	if img == nil {
		return 0, 0
	}
	if img.Dim.Width > 0 && img.Dim.Height > 0 {
		return img.Dim.Width, img.Dim.Height
	}
	if strings.HasSuffix(strings.ToLower(img.MimeType), "svg+xml") {
		decoded, err := imgutil.RasterizeSVGToImage(img.Data, 0, 0, 0)
		if err != nil {
			return 0, 0
		}
		return decoded.Bounds().Dx(), decoded.Bounds().Dy()
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(img.Data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func preparePDFImageResources(images fb2.BookImages, pages []pdfPage, nextObjectID *int) (map[string]pdfImageResource, error) {
	ids := usedPDFImageIDs(pages)
	resources := make(map[string]pdfImageResource, len(ids))
	for i, id := range ids {
		img := images[id]
		if img == nil {
			continue
		}
		resource, err := makePDFImageResource(img)
		if err != nil {
			return nil, fmt.Errorf("prepare pdf image %q: %w", id, err)
		}
		resource.ObjectID = *nextObjectID
		resource.Name = fmt.Sprintf("Im%d", i+1)
		(*nextObjectID)++
		resources[id] = resource
	}
	return resources, nil
}

func usedPDFImageIDs(pages []pdfPage) []string {
	seen := make(map[string]bool)
	for _, page := range pages {
		for _, img := range page.Images {
			if img.ImageID != "" {
				seen[img.ImageID] = true
			}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func assignPDFImageResourceNames(pages []pdfPage, resources map[string]pdfImageResource) {
	for pageIndex := range pages {
		for imageIndex := range pages[pageIndex].Images {
			resource, ok := resources[pages[pageIndex].Images[imageIndex].ImageID]
			if !ok {
				continue
			}
			pages[pageIndex].Images[imageIndex].Name = resource.Name
		}
	}
}

func pageImageXObjects(page pdfPage, resources map[string]pdfImageResource) docwriter.Dict {
	if len(page.Images) == 0 {
		return nil
	}
	xobjects := docwriter.Dict{}
	for _, img := range page.Images {
		resource, ok := resources[img.ImageID]
		if !ok || resource.Name == "" {
			continue
		}
		xobjects[resource.Name] = docwriter.Ref{ObjectNumber: resource.ObjectID}
	}
	if len(xobjects) == 0 {
		return nil
	}
	return xobjects
}

func writePDFImageObjects(writer *docwriter.Writer, resources map[string]pdfImageResource) error {
	if len(resources) == 0 {
		return nil
	}
	ids := make([]string, 0, len(resources))
	for id := range resources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		resource := resources[id]
		if err := writer.StreamObject(resource.ObjectID, resource.Dict, resource.Data); err != nil {
			return fmt.Errorf("write pdf image %q: %w", id, err)
		}
	}
	return nil
}

func makePDFImageResource(img *fb2.BookImage) (pdfImageResource, error) {
	if resource, ok := makeDirectJPEGImageResource(img); ok {
		return resource, nil
	}
	decoded, err := decodePDFImage(img)
	if err != nil {
		return pdfImageResource{}, err
	}
	rgb := flattenImageToRGB(decoded)
	data, err := flateImageData(rgb)
	if err != nil {
		return pdfImageResource{}, err
	}
	bounds := decoded.Bounds()
	return pdfImageResource{
		Dict: docwriter.Dict{
			"BitsPerComponent": docwriter.Integer(8),
			"ColorSpace":       docwriter.Name("DeviceRGB"),
			"Filter":           docwriter.Name("FlateDecode"),
			"Height":           docwriter.Integer(bounds.Dy()),
			"Subtype":          docwriter.Name("Image"),
			"Type":             docwriter.Name("XObject"),
			"Width":            docwriter.Integer(bounds.Dx()),
		},
		Data: data,
	}, nil
}

func makeDirectJPEGImageResource(img *fb2.BookImage) (pdfImageResource, bool) {
	if img == nil || len(img.Data) == 0 {
		return pdfImageResource{}, false
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(img.Data))
	if err != nil || format != "jpeg" || cfg.Width <= 0 || cfg.Height <= 0 {
		return pdfImageResource{}, false
	}
	colorSpace := docwriter.Name("DeviceRGB")
	if cfg.ColorModel == color.GrayModel {
		colorSpace = docwriter.Name("DeviceGray")
	} else if cfg.ColorModel == color.CMYKModel {
		return pdfImageResource{}, false
	}
	return pdfImageResource{
		Dict: docwriter.Dict{
			"BitsPerComponent": docwriter.Integer(8),
			"ColorSpace":       colorSpace,
			"Filter":           docwriter.Name("DCTDecode"),
			"Height":           docwriter.Integer(cfg.Height),
			"Subtype":          docwriter.Name("Image"),
			"Type":             docwriter.Name("XObject"),
			"Width":            docwriter.Integer(cfg.Width),
		},
		Data: img.Data,
	}, true
}

func decodePDFImage(img *fb2.BookImage) (image.Image, error) {
	if img == nil || len(img.Data) == 0 {
		return nil, fmt.Errorf("empty image data")
	}
	if strings.HasSuffix(strings.ToLower(img.MimeType), "svg+xml") {
		return imgutil.RasterizeSVGToImage(img.Data, img.Dim.Width, img.Dim.Height, 0)
	}
	decoded, _, err := image.Decode(bytes.NewReader(img.Data))
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func flattenImageToRGB(img image.Image) []byte {
	bounds := img.Bounds()
	out := make([]byte, 0, bounds.Dx()*bounds.Dy()*3)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			out = append(out,
				uint8((r+0xffff-a)>>8),
				uint8((g+0xffff-a)>>8),
				uint8((b+0xffff-a)>>8),
			)
		}
	}
	return out
}

func flateImageData(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		return nil, fmt.Errorf("compress image data: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finish image data compression: %w", err)
	}
	return buf.Bytes(), nil
}
