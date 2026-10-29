package config

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// PageFramePoints converts the PDF page outline thickness to points using the
// configured screen DPI for px. The zero-value config also disables the frame.
func (p PDFConfig) PageFramePoints(screen ScreenConfig) (float64, error) {
	value := strings.TrimSpace(p.PageFrame)
	if p.PageFrame == "" || value == "0" {
		return 0, nil
	}
	unit := ""
	for _, suffix := range []string{"pt", "px"} {
		if strings.HasSuffix(value, suffix) {
			unit = suffix
			break
		}
	}
	if unit == "" {
		return 0, fmt.Errorf("document.pdf.page_frame: expected 0 or a nonnegative thickness in pt or px, got %q", p.PageFrame)
	}
	width, err := strconv.ParseFloat(strings.TrimSuffix(value, unit), 64)
	if err != nil || math.IsNaN(width) || math.IsInf(width, 0) || width < 0 {
		return 0, fmt.Errorf("document.pdf.page_frame: invalid thickness %q", p.PageFrame)
	}
	if width == 0 {
		return 0, nil
	}
	if screen.DPI <= 0 || screen.Width <= 0 || screen.Height <= 0 {
		return 0, errors.New("document.pdf.page_frame: positive screen dimensions and DPI are required")
	}
	if unit == "px" {
		width *= 72 / float64(screen.DPI)
	}
	if width >= float64(min(screen.Width, screen.Height))*72/float64(screen.DPI) {
		return 0, fmt.Errorf("document.pdf.page_frame: thickness %q must be smaller than both page dimensions", p.PageFrame)
	}
	return width, nil
}
