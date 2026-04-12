package pdf

import (
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/layout"

	"fbc/config"
)

const defaultScreenDPI = 96.0

// Geometry describes page size and margins for PDF output.
type Geometry struct {
	PageSize document.PageSize
	Margins  layout.Margins
}

// PxToPt converts CSS-like pixels to PDF points using the default screen DPI.
func PxToPt(px int, scale float64) float64 {
	if scale <= 0 {
		scale = 1
	}
	return float64(px) * 72.0 / defaultScreenDPI * scale
}

// DefaultMargins returns conservative content margins in PDF points.
func DefaultMargins() layout.Margins {
	return layout.Margins{Top: 48, Right: 36, Bottom: 48, Left: 36}
}

// GeometryFromConfig derives PDF page geometry from configured screen size.
func GeometryFromConfig(cfg *config.DocumentConfig) Geometry {
	page := document.PageSize{
		Width:  PxToPt(cfg.Images.Screen.Width, 1),
		Height: PxToPt(cfg.Images.Screen.Height, 1),
	}

	return Geometry{
		PageSize: page,
		Margins:  DefaultMargins(),
	}
}
