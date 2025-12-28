package fb2

import (
	"testing"

	imgutil "fbc/utils/images"
)

func TestEmbeddedSVGPlaceholdersRasterize(t *testing.T) {
	cases := map[string][]byte{
		"brokenImage":   brokenImage,
		"notFoundImage": notFoundImage,
	}

	for name, svg := range cases {
		t.Run(name, func(t *testing.T) {
			img, err := imgutil.RasterizeSVGToImage(svg, 0, 0)
			if err != nil {
				t.Fatalf("rasterize %s: %v", name, err)
			}
			if img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
				t.Fatalf("unexpected bounds: %v", img.Bounds())
			}
		})
	}
}
