package pdf

import (
	"math"
	"testing"

	"fbc/config"
)

func TestPxToPt(t *testing.T) {
	got := PxToPt(1264, 1)
	want := 948.0
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("PxToPt(1264, 1) = %.6f, want %.6f", got, want)
	}
}

func TestGeometryFromConfig(t *testing.T) {
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Document.Images.Screen.Width = 1264
	cfg.Document.Images.Screen.Height = 1680

	geom := GeometryFromConfig(&cfg.Document)

	if got, want := geom.PageSize.Width, 948.0; math.Abs(got-want) > 0.001 {
		t.Fatalf("page width = %.6f, want %.6f", got, want)
	}
	if got, want := geom.PageSize.Height, 1260.0; math.Abs(got-want) > 0.001 {
		t.Fatalf("page height = %.6f, want %.6f", got, want)
	}
	if geom.Margins.Top != 48 || geom.Margins.Right != 36 || geom.Margins.Bottom != 48 || geom.Margins.Left != 36 {
		t.Fatalf("unexpected default margins: %#v", geom.Margins)
	}
}
