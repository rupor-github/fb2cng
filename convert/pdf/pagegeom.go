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
	return GeometryFromStyles(cfg, resolvedStyle{})
}

// GeometryFromStyles derives PDF page geometry from configured screen size and
// CSS-resolved body spacing. When CSS does not define body margins or padding,
// it falls back to conservative printable defaults.
func GeometryFromStyles(cfg *config.DocumentConfig, bodyStyle resolvedStyle) Geometry {
	if cfg == nil {
		return Geometry{PageSize: document.PageSize{}, Margins: DefaultMargins()}
	}

	page := document.PageSize{
		Width:  PxToPt(cfg.Images.Screen.Width, 1),
		Height: PxToPt(cfg.Images.Screen.Height, 1),
	}

	return Geometry{
		PageSize: page,
		Margins:  marginsFromBodyStyle(bodyStyle),
	}
}

func marginsFromBodyStyle(style resolvedStyle) layout.Margins {
	margins := layout.Margins{
		Top:    max(style.MarginTop+style.PaddingTop, 0),
		Right:  max(style.MarginRight+style.PaddingRight, 0),
		Bottom: max(style.MarginBottom+style.PaddingBottom, 0),
		Left:   max(style.MarginLeft+style.PaddingLeft, 0),
	}

	if margins.Top == 0 && margins.Right == 0 && margins.Bottom == 0 && margins.Left == 0 {
		return DefaultMargins()
	}

	return margins
}
