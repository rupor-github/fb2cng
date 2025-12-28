package convert

import (
	"testing"

	imgutil "fbc/utils/images"
)

func TestDefaultCoverSVGRasterizes(t *testing.T) {
	img, err := imgutil.RasterizeSVGToImage(defaultCoverSVG, 0, 0)
	if err != nil {
		t.Fatalf("rasterize default cover svg: %v", err)
	}
	if img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
		t.Fatalf("unexpected bounds: %v", img.Bounds())
	}
}
