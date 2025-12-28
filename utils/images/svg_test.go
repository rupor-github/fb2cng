package images

import "testing"

func TestRasterizeSVGToImage(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"><rect width="100" height="50"/></svg>`)

	t.Run("intrinsic", func(t *testing.T) {
		img, err := RasterizeSVGToImage(svg, 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if img.Bounds().Dx() != 100 || img.Bounds().Dy() != 50 {
			t.Fatalf("unexpected bounds: %v", img.Bounds())
		}
	})

	t.Run("scale_by_width", func(t *testing.T) {
		img, err := RasterizeSVGToImage(svg, 200, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if img.Bounds().Dx() != 200 || img.Bounds().Dy() != 100 {
			t.Fatalf("unexpected bounds: %v", img.Bounds())
		}
	})

	t.Run("scale_by_height", func(t *testing.T) {
		img, err := RasterizeSVGToImage(svg, 0, 200)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if img.Bounds().Dx() != 400 || img.Bounds().Dy() != 200 {
			t.Fatalf("unexpected bounds: %v", img.Bounds())
		}
	})

	t.Run("fit_box", func(t *testing.T) {
		img, err := RasterizeSVGToImage(svg, 150, 150)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if img.Bounds().Dx() != 150 || img.Bounds().Dy() != 75 {
			t.Fatalf("unexpected bounds: %v", img.Bounds())
		}
	})
}
