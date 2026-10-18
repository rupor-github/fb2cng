package fb2

import (
	"image"
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
			for _, policy := range []struct {
				name        string
				transparent bool
			}{
				{"opaque white", false},
				{"transparent", true},
			} {
				t.Run(policy.name, func(t *testing.T) {
					render := imgutil.RasterizeSVGToImage
					if policy.transparent {
						render = imgutil.RasterizeSVGToTransparentImage
					}
					img, err := render(svg, 0, 0, 0)
					if err != nil {
						t.Fatalf("rasterize %s: %v", name, err)
					}
					if img.Bounds() != image.Rect(0, 0, 200, 200) {
						t.Fatalf("unexpected bounds: %v", img.Bounds())
					}
					// Check empty areas outside and inside the border: SVG backgrounds
					// must stay clear, while opaque raster output must flatten to white.
					for _, point := range []image.Point{image.Pt(1, 1), image.Pt(20, 20)} {
						r, g, b, a := img.At(point.X, point.Y).RGBA()
						if policy.transparent {
							if a != 0 {
								t.Errorf("background at %v has alpha %d, want transparent", point, a)
							}
						} else if r != 0xffff || g != 0xffff || b != 0xffff || a != 0xffff {
							t.Errorf("background at %v = %v, want opaque white", point, img.At(point.X, point.Y))
						}
					}
					if _, _, _, a := img.At(6, 100).RGBA(); a != 0xffff {
						t.Errorf("outer border has alpha %d, want opaque", a)
					}
				})
			}
		})
	}
}
