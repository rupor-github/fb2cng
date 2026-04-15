package pdf

import (
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/layout"

	"fbc/config"
)

const (
	// defaultDeviceDPI is the fallback screen resolution when config does not
	// specify one.  300 PPI is correct for all modern e-ink readers.
	defaultDeviceDPI = 300.0

	// cssDPI is the standard CSS reference DPI (1 CSS-px = 1/96 inch).
	// Used only for CSS unit conversions (px→pt), NOT for page geometry.
	cssDPI = 96.0
)

// Geometry describes page size and margins for PDF output.
type Geometry struct {
	PageSize document.PageSize
	Margins  layout.Margins
}

// PxToPt converts device pixels to PDF points using the given device DPI.
// If dpi is zero or negative, defaultDeviceDPI is used.
func PxToPt(px int, dpi float64) float64 {
	if dpi <= 0 {
		dpi = defaultDeviceDPI
	}
	return float64(px) * 72.0 / dpi
}

// CSSPxToPt converts CSS pixels to PDF points (1 CSS-px = 1/96 inch).
func CSSPxToPt(value float64) float64 {
	return value * 72.0 / cssDPI
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

	dpi := float64(cfg.Images.Screen.DPI)
	page := document.PageSize{
		Width:  PxToPt(cfg.Images.Screen.Width, dpi),
		Height: PxToPt(cfg.Images.Screen.Height, dpi),
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
