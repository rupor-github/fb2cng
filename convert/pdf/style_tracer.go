package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"fbc/css"
)

type pdfStyleTracer struct {
	enabled  bool
	workDir  string
	entries  []pdfStyleTraceEntry
	sections map[string]int
}

type pdfStyleTraceEntry struct {
	Operation string
	StyleName string
	Details   string
}

func newPDFStyleTracer(workDir string) *pdfStyleTracer {
	return &pdfStyleTracer{
		enabled:  workDir != "",
		workDir:  workDir,
		sections: make(map[string]int),
	}
}

func (t *pdfStyleTracer) isEnabled() bool {
	return t != nil && t.enabled
}

func (t *pdfStyleTracer) traceDefault(name string, style pdfBlockResolvedStyle) {
	if !t.isEnabled() {
		return
	}
	t.append("DEFAULT", name, pdfTraceFormatResolvedStyle(style))
	t.sections["default"]++
}

func (t *pdfStyleTracer) traceMedia(query string, matched bool, rules int) {
	if !t.isEnabled() {
		return
	}
	state := "skipped"
	section := "media_skipped"
	if matched {
		state = "matched"
		section = "media_matched"
	}
	t.append("MEDIA", query, fmt.Sprintf("%s, rules=%d", state, rules))
	t.sections[section]++
}

func (t *pdfStyleTracer) traceRule(selector css.Selector, properties map[string]css.Value, mapped []string) {
	if !t.isEnabled() {
		return
	}
	details := fmt.Sprintf("maps to: %s\n  properties: %s", strings.Join(mapped, ", "), pdfTraceFormatCSSProperties(properties))
	t.append("CSS", selector.Raw, details)
	t.sections["css_rule"]++
}

func (t *pdfStyleTracer) traceIgnoredRule(selector css.Selector, properties map[string]css.Value) {
	if !t.isEnabled() {
		return
	}
	details := "no PDF style mapping\n  properties: " + pdfTraceFormatCSSProperties(properties)
	t.append("IGNORE", selector.Raw, details)
	t.sections["ignored_rule"]++
}

func (t *pdfStyleTracer) traceStyleUpdate(name string, before, after pdfBlockResolvedStyle) {
	if !t.isEnabled() {
		return
	}
	details := pdfTraceStyleDiff(before, after)
	if details == "" {
		details = "no effective change"
	}
	t.append("UPDATE", name, details)
	t.sections["style_update"]++
}

func (t *pdfStyleTracer) traceDropcapPattern(selector string, parent string, fontSize css.Value, chars int, lines int) {
	if !t.isEnabled() {
		return
	}
	details := fmt.Sprintf("parent=%s, font-size=%g%s, chars=%d, lines=%d", parent, fontSize.Value, fontSize.Unit, chars, lines)
	t.append("DROPCAP CSS", selector, details)
	t.sections["dropcap_css"]++
}

func (t *pdfStyleTracer) traceDropcapResolved(blockIndex int, block pdfTextBlock, layout pdfDropcapLayout, base paragraphStyle) {
	if !t.isEnabled() {
		return
	}
	details := fmt.Sprintf("block #%d %s", blockIndex, block.Kind.String())
	if block.ID != "" {
		details += fmt.Sprintf(", id=%q", block.ID)
	}
	details += fmt.Sprintf("\n  char=%q, classes=%q, context=%q, chars=%d, lines=%d", layout.Run.Text, layout.Run.StyleClasses, layout.Run.ContextClasses, pdfDropcapDefaultChars, layout.Lines)
	details += fmt.Sprintf("\n  base: font-family=%s, font-size=%gpt, line-height=%gpt", base.FontFamily, base.FontSize, pdfEffectiveParagraphLineHeight(base))
	details += fmt.Sprintf("\n  dropcap: font-family=%s, font-size=%gpt, line-height=%gpt, bold=%t, italic=%t, color=%s", layout.Style.FontFamily, layout.Style.FontSize, layout.Style.LineHeight, layout.Style.Bold, layout.Style.Italic, layout.Style.Color.String())
	details += fmt.Sprintf("\n  geometry: padding-right=%gpt, glyph-width=%gpt, exclusion-width=%gpt, reserved-height=%gpt", layout.PaddingRight, layout.Fragment.Width, layout.ExclusionWidth, layout.ReservedHeight)
	t.append("DROPCAP", fmt.Sprintf("#%d", blockIndex), details)
	t.sections["dropcap_resolved"]++
}

func (t *pdfStyleTracer) traceMarginCollapse(previousIndex, currentIndex int, previousBlock, currentBlock pdfTextBlock, previousMargin, currentMargin, collapsed float64) {
	if !t.isEnabled() {
		return
	}
	details := fmt.Sprintf("previous #%d %s margin-bottom=%g\n  current #%d %s margin-top=%g\n  collapsed margin-top=%g",
		previousIndex,
		previousBlock.Kind.String(),
		previousMargin,
		currentIndex,
		currentBlock.Kind.String(),
		currentMargin,
		collapsed)
	t.append("COLLAPSE", fmt.Sprintf("#%d -> #%d", previousIndex, currentIndex), details)
	t.sections["margin_collapsed"]++
}

func (t *pdfStyleTracer) traceAssign(block pdfTextBlock, styleName string, style pdfBlockResolvedStyle) {
	if !t.isEnabled() {
		return
	}
	var details strings.Builder
	details.WriteString(fmt.Sprintf("block kind=%s", block.Kind.String()))
	if block.ID != "" {
		details.WriteString(fmt.Sprintf(", id=%q", block.ID))
	}
	if block.Depth != 0 {
		details.WriteString(fmt.Sprintf(", depth=%d", block.Depth))
	}
	if strings.TrimSpace(block.StyleClasses) != "" {
		details.WriteString(fmt.Sprintf(", classes=%q", strings.TrimSpace(block.StyleClasses)))
	}
	if block.ImageID != "" {
		details.WriteString(fmt.Sprintf(", image_id=%q", block.ImageID))
	}
	if block.Text != "" {
		details.WriteString(fmt.Sprintf("\n  text: %q", pdfTraceExcerpt(block.Text)))
	}
	details.WriteString("\n  resolved: ")
	details.WriteString(pdfTraceFormatResolvedStyle(style))
	t.append("ASSIGN", styleName, details.String())
	t.sections["assigned"]++
}

func (t *pdfStyleTracer) flush() string {
	if !t.isEnabled() || len(t.entries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("=== PDF Style Trace ===\n\n")
	sb.WriteString("Summary:\n")
	sections := make([]string, 0, len(t.sections))
	for section := range t.sections {
		sections = append(sections, section)
	}
	slices.Sort(sections)
	for _, section := range sections {
		sb.WriteString(fmt.Sprintf("  %s: %d\n", section, t.sections[section]))
	}
	sb.WriteString("\nDetailed Trace:\n")
	sb.WriteString(strings.Repeat("-", 80) + "\n")
	for i, entry := range t.entries {
		sb.WriteString(fmt.Sprintf("[%04d] %s: %s\n", i+1, entry.Operation, entry.StyleName))
		if entry.Details != "" {
			for line := range strings.SplitSeq(entry.Details, "\n") {
				sb.WriteString("       " + line + "\n")
			}
		}
		sb.WriteByte('\n')
	}

	tracePath := filepath.Join(t.workDir, "pdf-style-trace.txt")
	if err := os.WriteFile(tracePath, []byte(sb.String()), 0644); err != nil {
		return ""
	}
	t.entries = nil
	t.sections = make(map[string]int)
	return tracePath
}

func (t *pdfStyleTracer) append(operation, styleName, details string) {
	t.entries = append(t.entries, pdfStyleTraceEntry{Operation: operation, StyleName: styleName, Details: details})
}

func pdfTraceFormatCSSProperties(props map[string]css.Value) string {
	if len(props) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	slices.Sort(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+": "+formatCSSValue(props[name]))
	}
	return strings.Join(parts, "; ")
}

func pdfTraceFormatResolvedStyle(style pdfBlockResolvedStyle) string {
	return fmt.Sprintf("font-family=%s, font-weight=%s, font-style=%s, color=%s, background=%s, border=%gpt %s, underline=%t, strikethrough=%t, font-size=%gpt, line-height=%gpt, letter-spacing=%gpt, text-indent=%gpt, align=%s, vertical-align=%s, hyphens=%s, margins=%g/%g/%g/%g, padding=%g/%g/%g/%g, width=%s, min-width=%s, max-width=%s, keep-together=%t, keep-next=%d, page-break-before=%t, page-break-after=%t, hidden=%t, orphans=%d, widows=%d",
		normalizedPDFFontFamily(style.Paragraph.FontFamily),
		pdfCSSFontWeightString(style.Paragraph.Bold),
		pdfCSSFontStyleString(style.Paragraph.Italic),
		style.Paragraph.Color.String(),
		pdfTraceBackgroundColor(style),
		style.BorderWidth,
		pdfTraceBorderColor(style),
		style.Paragraph.Underline,
		style.Paragraph.Strikethrough,
		style.Paragraph.FontSize,
		style.Paragraph.LineHeight,
		style.Paragraph.LetterSpacing,
		style.Paragraph.FirstLineIndent,
		style.Paragraph.Align.String(),
		style.Paragraph.VerticalAlign.String(),
		pdfHyphenationString(style.Paragraph.Hyphenation),
		style.SpaceBefore,
		style.MarginRight,
		style.SpaceAfter,
		style.MarginLeft,
		style.PaddingTop,
		style.PaddingRight,
		style.PaddingBottom,
		style.PaddingLeft,
		pdfTraceBlockLength(style.Width, style.HasWidth),
		pdfTraceBlockLength(style.MinWidth, style.HasMinWidth),
		pdfTraceBlockLength(style.MaxWidth, style.HasMaxWidth),
		style.KeepTogether,
		style.KeepWithNextLines,
		style.PageBreakBefore,
		style.PageBreakAfter,
		style.Hidden,
		style.Orphans,
		style.Widows)
}

func pdfTraceBlockLength(length pdfBlockLength, ok bool) string {
	if !ok {
		return "auto"
	}
	return pdfBlockLengthString(length, true)
}

func pdfTraceBackgroundColor(style pdfBlockResolvedStyle) string {
	if !style.HasBackground {
		return "none"
	}
	return style.BackgroundColor.String()
}

func pdfTraceBorderColor(style pdfBlockResolvedStyle) string {
	if !style.HasBorder || style.BorderWidth <= 0 {
		return "none"
	}
	return style.BorderColor.String()
}

func pdfTraceStyleDiff(before, after pdfBlockResolvedStyle) string {
	var changes []string
	appendFloatChange := func(name string, old, new float64) {
		if old != new {
			changes = append(changes, fmt.Sprintf("%s: %g -> %g", name, old, new))
		}
	}
	appendIntChange := func(name string, old, new int) {
		if old != new {
			changes = append(changes, fmt.Sprintf("%s: %d -> %d", name, old, new))
		}
	}
	appendBoolChange := func(name string, old, new bool) {
		if old != new {
			changes = append(changes, fmt.Sprintf("%s: %t -> %t", name, old, new))
		}
	}
	if before.Paragraph.FontFamily != after.Paragraph.FontFamily {
		changes = append(changes, fmt.Sprintf("font-family: %s -> %s", normalizedPDFFontFamily(before.Paragraph.FontFamily), normalizedPDFFontFamily(after.Paragraph.FontFamily)))
	}
	if before.Paragraph.Bold != after.Paragraph.Bold {
		changes = append(changes, fmt.Sprintf("font-weight: %s -> %s", pdfCSSFontWeightString(before.Paragraph.Bold), pdfCSSFontWeightString(after.Paragraph.Bold)))
	}
	if before.Paragraph.Italic != after.Paragraph.Italic {
		changes = append(changes, fmt.Sprintf("font-style: %s -> %s", pdfCSSFontStyleString(before.Paragraph.Italic), pdfCSSFontStyleString(after.Paragraph.Italic)))
	}
	if before.Paragraph.Align != after.Paragraph.Align {
		changes = append(changes, fmt.Sprintf("text-align: %s -> %s", before.Paragraph.Align, after.Paragraph.Align))
	}
	if before.Paragraph.VerticalAlign != after.Paragraph.VerticalAlign {
		changes = append(changes, fmt.Sprintf("vertical-align: %s -> %s", before.Paragraph.VerticalAlign, after.Paragraph.VerticalAlign))
	}
	if before.Paragraph.Color != after.Paragraph.Color {
		changes = append(changes, fmt.Sprintf("color: %s -> %s", before.Paragraph.Color, after.Paragraph.Color))
	}
	appendBoolChange("underline", before.Paragraph.Underline, after.Paragraph.Underline)
	appendBoolChange("strikethrough", before.Paragraph.Strikethrough, after.Paragraph.Strikethrough)
	if before.Paragraph.Hyphenation != after.Paragraph.Hyphenation {
		changes = append(changes, fmt.Sprintf("hyphens: %s -> %s", pdfHyphenationString(before.Paragraph.Hyphenation), pdfHyphenationString(after.Paragraph.Hyphenation)))
	}
	appendFloatChange("font-size", before.Paragraph.FontSize, after.Paragraph.FontSize)
	appendFloatChange("line-height", before.Paragraph.LineHeight, after.Paragraph.LineHeight)
	appendFloatChange("letter-spacing", before.Paragraph.LetterSpacing, after.Paragraph.LetterSpacing)
	appendFloatChange("text-indent", before.Paragraph.FirstLineIndent, after.Paragraph.FirstLineIndent)
	appendFloatChange("margin-top", before.SpaceBefore, after.SpaceBefore)
	appendFloatChange("margin-bottom", before.SpaceAfter, after.SpaceAfter)
	appendFloatChange("margin-left", before.MarginLeft, after.MarginLeft)
	appendFloatChange("margin-right", before.MarginRight, after.MarginRight)
	appendFloatChange("padding-top", before.PaddingTop, after.PaddingTop)
	appendFloatChange("padding-right", before.PaddingRight, after.PaddingRight)
	appendFloatChange("padding-bottom", before.PaddingBottom, after.PaddingBottom)
	appendFloatChange("padding-left", before.PaddingLeft, after.PaddingLeft)
	appendBlockLengthChange := func(name string, beforeLength pdfBlockLength, beforeOK bool, afterLength pdfBlockLength, afterOK bool) {
		if beforeOK != afterOK || beforeLength != afterLength {
			changes = append(changes, fmt.Sprintf("%s: %s -> %s", name, pdfTraceBlockLength(beforeLength, beforeOK), pdfTraceBlockLength(afterLength, afterOK)))
		}
	}
	appendBlockLengthChange("width", before.Width, before.HasWidth, after.Width, after.HasWidth)
	appendBlockLengthChange("min-width", before.MinWidth, before.HasMinWidth, after.MinWidth, after.HasMinWidth)
	appendBlockLengthChange("max-width", before.MaxWidth, before.HasMaxWidth, after.MaxWidth, after.HasMaxWidth)
	if before.HasBackground != after.HasBackground || before.BackgroundColor != after.BackgroundColor {
		changes = append(changes, fmt.Sprintf("background-color: %s -> %s", pdfTraceBackgroundColor(before), pdfTraceBackgroundColor(after)))
	}
	if before.HasBorder != after.HasBorder || before.BorderWidth != after.BorderWidth || before.BorderColor != after.BorderColor {
		changes = append(changes, fmt.Sprintf("border: %gpt %s -> %gpt %s", before.BorderWidth, pdfTraceBorderColor(before), after.BorderWidth, pdfTraceBorderColor(after)))
	}
	appendBoolChange("keep-together", before.KeepTogether, after.KeepTogether)
	appendIntChange("keep-next", before.KeepWithNextLines, after.KeepWithNextLines)
	appendBoolChange("page-break-before", before.PageBreakBefore, after.PageBreakBefore)
	appendBoolChange("page-break-after", before.PageBreakAfter, after.PageBreakAfter)
	appendBoolChange("hidden", before.Hidden, after.Hidden)
	appendIntChange("orphans", before.Orphans, after.Orphans)
	appendIntChange("widows", before.Widows, after.Widows)
	return strings.Join(changes, "\n  ")
}

func pdfTraceExcerpt(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= 120 {
		return text
	}
	return string(runes[:120]) + "…"
}
