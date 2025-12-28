package images

import "testing"

func TestRasterizeSVGToImage(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><rect width="100" height="100"/></svg>`)
	img, err := RasterizeSVGToImage(svg, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
		t.Fatalf("unexpected bounds: %v", img.Bounds())
	}
}
