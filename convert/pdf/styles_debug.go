package pdf

import (
	"slices"
	"strconv"
)

func (r *pdfStyleResolver) traceDefaults() {
	if r == nil || r.tracer == nil {
		return
	}
	names := make([]string, 0, len(r.styles))
	for name := range r.styles {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		r.tracer.traceDefault(name, r.styles[name])
	}
}

func pdfHyphenationString(mode paragraphHyphenation) string {
	switch mode {
	case paragraphHyphenationNone:
		return "none"
	case paragraphHyphenationManual:
		return "manual"
	case paragraphHyphenationAuto:
		return "auto"
	default:
		return "auto"
	}
}

func pdfCSSFontWeightString(bold bool) string {
	if bold {
		return "bold"
	}
	return "normal"
}

func pdfCSSFontStyleString(italic bool) string {
	if italic {
		return "italic"
	}
	return "normal"
}

func pdfBlockLengthString(length pdfBlockLength, ok bool) string {
	if !ok {
		return ""
	}
	if length.Percent {
		return strconv.FormatFloat(length.Value, 'f', -1, 64) + "%"
	}
	return strconv.FormatFloat(length.Value, 'f', -1, 64) + "pt"
}

func pdfDebugBackgroundColor(style pdfBlockResolvedStyle) string {
	if !style.HasBackground {
		return ""
	}
	return style.BackgroundColor.String()
}

func pdfDebugBorderColor(style pdfBlockResolvedStyle) string {
	if !style.HasBorder || style.BorderWidth <= 0 {
		return ""
	}
	return style.BorderColor.String()
}

func (r *pdfStyleResolver) debugStyles() []pdfDebugResolvedStyle {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.styles))
	for name := range r.styles {
		names = append(names, name)
	}
	slices.Sort(names)
	out := make([]pdfDebugResolvedStyle, 0, len(names))
	for _, name := range names {
		style := r.styles[name]
		out = append(out, pdfDebugResolvedStyle{
			Name:              name,
			FontFamily:        normalizedPDFFontFamily(style.Paragraph.FontFamily),
			FontWeight:        pdfCSSFontWeightString(style.Paragraph.Bold),
			FontStyle:         pdfCSSFontStyleString(style.Paragraph.Italic),
			FontSize:          style.Paragraph.FontSize,
			LineHeight:        style.Paragraph.LineHeight,
			LetterSpacing:     style.Paragraph.LetterSpacing,
			FirstLineIndent:   style.Paragraph.FirstLineIndent,
			TextAlign:         style.Paragraph.Align.String(),
			VerticalAlign:     style.Paragraph.VerticalAlign.String(),
			Color:             style.Paragraph.Color.String(),
			Underline:         style.Paragraph.Underline,
			Strikethrough:     style.Paragraph.Strikethrough,
			PreserveSpace:     style.Paragraph.PreserveSpace,
			NoWrap:            style.Paragraph.NoWrap,
			Hyphenation:       pdfHyphenationString(style.Paragraph.Hyphenation),
			SpaceBefore:       style.SpaceBefore,
			SpaceAfter:        style.SpaceAfter,
			MarginLeft:        style.MarginLeft,
			MarginRight:       style.MarginRight,
			PaddingTop:        style.PaddingTop,
			PaddingRight:      style.PaddingRight,
			PaddingBottom:     style.PaddingBottom,
			PaddingLeft:       style.PaddingLeft,
			Width:             pdfBlockLengthString(style.Width, style.HasWidth),
			MinWidth:          pdfBlockLengthString(style.MinWidth, style.HasMinWidth),
			MaxWidth:          pdfBlockLengthString(style.MaxWidth, style.HasMaxWidth),
			BackgroundColor:   pdfDebugBackgroundColor(style),
			BorderWidth:       style.BorderWidth,
			BorderColor:       pdfDebugBorderColor(style),
			KeepTogether:      style.KeepTogether,
			KeepWithNextLines: style.KeepWithNextLines,
			PageBreakBefore:   style.PageBreakBefore,
			PageBreakAfter:    style.PageBreakAfter,
			Hidden:            style.Hidden,
			Orphans:           style.Orphans,
			Widows:            style.Widows,
		})
	}
	return out
}
