package state

import (
	"fmt"
	"testing"

	imgutil "fbc/utils/images"
)

func TestDefaultVignettesRasterize(t *testing.T) {
	env := newLocalEnv()
	for pos, svg := range env.DefaultVignettes {
		name := fmt.Sprintf("%v", pos)
		t.Run(name, func(t *testing.T) {
			img, err := imgutil.RasterizeSVGToImage(svg, 0, 0)
			if err != nil {
				t.Fatalf("rasterize vignette %s: %v", name, err)
			}
			if img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
				t.Fatalf("unexpected bounds: %v", img.Bounds())
			}
		})
	}
}
