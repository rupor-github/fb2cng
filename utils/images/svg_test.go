package images

import (
	"strings"
	"testing"
)

func TestRasterizeSVGToImage(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"><rect width="100" height="50"/></svg>`)

	t.Run("intrinsic", func(t *testing.T) {
		img, err := RasterizeSVGToImage(svg, 0, 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if img.Bounds().Dx() != 100 || img.Bounds().Dy() != 50 {
			t.Fatalf("unexpected bounds: %v", img.Bounds())
		}
	})

	t.Run("scale_by_width", func(t *testing.T) {
		img, err := RasterizeSVGToImage(svg, 200, 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if img.Bounds().Dx() != 200 || img.Bounds().Dy() != 100 {
			t.Fatalf("unexpected bounds: %v", img.Bounds())
		}
	})

	t.Run("scale_by_height", func(t *testing.T) {
		img, err := RasterizeSVGToImage(svg, 0, 200, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if img.Bounds().Dx() != 400 || img.Bounds().Dy() != 200 {
			t.Fatalf("unexpected bounds: %v", img.Bounds())
		}
	})

	t.Run("fit_box", func(t *testing.T) {
		img, err := RasterizeSVGToImage(svg, 150, 150, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if img.Bounds().Dx() != 150 || img.Bounds().Dy() != 75 {
			t.Fatalf("unexpected bounds: %v", img.Bounds())
		}
	})
}

func TestScaleSVGStrokeWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		factor   float64
		expected string
	}{
		{
			name:     "attribute with quotes",
			input:    `<path stroke-width="1"/>`,
			factor:   5.0,
			expected: `<path stroke-width="5"/>`,
		},
		{
			name:     "attribute with single quotes",
			input:    `<path stroke-width='1.2'/>`,
			factor:   5.0,
			expected: `<path stroke-width='6'/>`,
		},
		{
			name:     "multiple stroke-widths",
			input:    `<path stroke-width="1"/><line stroke-width="2"/>`,
			factor:   3.0,
			expected: `<path stroke-width="3"/><line stroke-width="6"/>`,
		},
		{
			name:     "CSS style property",
			input:    `<path style="stroke-width: 1.5"/>`,
			factor:   4.0,
			expected: `<path style="stroke-width: 6"/>`,
		},
		{
			name:     "factor of 1 returns unchanged",
			input:    `<path stroke-width="1"/>`,
			factor:   1.0,
			expected: `<path stroke-width="1"/>`,
		},
		{
			name:     "factor of 0 returns unchanged",
			input:    `<path stroke-width="1"/>`,
			factor:   0,
			expected: `<path stroke-width="1"/>`,
		},
		{
			name:     "negative factor returns unchanged",
			input:    `<path stroke-width="1"/>`,
			factor:   -1.0,
			expected: `<path stroke-width="1"/>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ScaleSVGStrokeWidth([]byte(tt.input), tt.factor)
			if string(result) != tt.expected {
				t.Errorf("ScaleSVGStrokeWidth() = %q, want %q", string(result), tt.expected)
			}
		})
	}
}

func TestScaleSVGStrokeWidthWithVignette(t *testing.T) {
	// Test with actual vignette-like SVG
	svg := `<svg viewBox="0 0 300 20" xmlns="http://www.w3.org/2000/svg">
  <path d="M10 10 Q40 0 70 10" stroke="black" fill="none" stroke-width="1"/>
  <line x1="70" y1="10" x2="230" y2="10" stroke="black" stroke-width="1"/>
</svg>`

	result := ScaleSVGStrokeWidth([]byte(svg), 5.0)

	if !strings.Contains(string(result), `stroke-width="5"`) {
		t.Errorf("expected stroke-width to be scaled to 5, got: %s", result)
	}
}
