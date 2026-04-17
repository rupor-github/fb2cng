package pdf

import (
	"math"
	"testing"

	"fbc/config"
)

func TestPxToPt(t *testing.T) {
	// 1264 device pixels at 300 DPI → 1264 * 72 / 300 = 303.36 pt
	got := PxToPt(1264, 300)
	want := 303.36
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("PxToPt(1264, 300) = %.6f, want %.6f", got, want)
	}
}

func TestPxToPt_DefaultDPI(t *testing.T) {
	// Zero DPI falls back to defaultDeviceDPI (300).
	got := PxToPt(1264, 0)
	want := 303.36
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("PxToPt(1264, 0) = %.6f, want %.6f", got, want)
	}
}

func TestCSSPxToPt(t *testing.T) {
	// CSS px always uses 96 DPI: 960px * 72/96 = 720 pt
	got := CSSPxToPt(960)
	want := 720.0
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("CSSPxToPt(960) = %.6f, want %.6f", got, want)
	}
}

func TestGeometryFromConfig(t *testing.T) {
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Document.Images.Screen.Width = 1264
	cfg.Document.Images.Screen.Height = 1680
	// DPI comes from config template default (300).

	geom := GeometryFromConfig(&cfg.Document)

	// 1264 * 72 / 300 = 303.36, 1680 * 72 / 300 = 403.2
	if got, want := geom.PageSize.Width, 303.36; math.Abs(got-want) > 0.001 {
		t.Fatalf("page width = %.6f, want %.6f", got, want)
	}
	if got, want := geom.PageSize.Height, 403.2; math.Abs(got-want) > 0.001 {
		t.Fatalf("page height = %.6f, want %.6f", got, want)
	}
	if geom.Margins.Top != 18 || geom.Margins.Right != 18 || geom.Margins.Bottom != 18 || geom.Margins.Left != 18 {
		t.Fatalf("unexpected default margins: %#v", geom.Margins)
	}
}

func TestGeometryFromStyles_UsesBodySpacing(t *testing.T) {
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Document.Images.Screen.Width = 1264
	cfg.Document.Images.Screen.Height = 1680

	geom := GeometryFromStyles(&cfg.Document, resolvedStyle{
		MarginTop:     12,
		MarginRight:   18,
		MarginBottom:  24,
		MarginLeft:    30,
		PaddingTop:    3,
		PaddingRight:  4,
		PaddingBottom: 5,
		PaddingLeft:   6,
	})

	if geom.Margins.Top != 15 || geom.Margins.Right != 22 || geom.Margins.Bottom != 29 || geom.Margins.Left != 36 {
		t.Fatalf("unexpected CSS-derived margins: %#v", geom.Margins)
	}
}
