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
	return fmt.Sprintf("font-size=%gpt, line-height=%gpt, text-indent=%gpt, align=%s, hyphens=%s, margins=%g/%g/%g/%g, keep-together=%t, keep-next=%d, page-break-before=%t, page-break-after=%t, hidden=%t, orphans=%d, widows=%d",
		style.Paragraph.FontSize,
		style.Paragraph.LineHeight,
		style.Paragraph.FirstLineIndent,
		style.Paragraph.Align.String(),
		pdfHyphenationString(style.Paragraph.Hyphenation),
		style.SpaceBefore,
		style.MarginRight,
		style.SpaceAfter,
		style.MarginLeft,
		style.KeepTogether,
		style.KeepWithNextLines,
		style.PageBreakBefore,
		style.PageBreakAfter,
		style.Hidden,
		style.Orphans,
		style.Widows)
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
	if before.Paragraph.Align != after.Paragraph.Align {
		changes = append(changes, fmt.Sprintf("text-align: %s -> %s", before.Paragraph.Align, after.Paragraph.Align))
	}
	if before.Paragraph.Hyphenation != after.Paragraph.Hyphenation {
		changes = append(changes, fmt.Sprintf("hyphens: %s -> %s", pdfHyphenationString(before.Paragraph.Hyphenation), pdfHyphenationString(after.Paragraph.Hyphenation)))
	}
	appendFloatChange("font-size", before.Paragraph.FontSize, after.Paragraph.FontSize)
	appendFloatChange("line-height", before.Paragraph.LineHeight, after.Paragraph.LineHeight)
	appendFloatChange("text-indent", before.Paragraph.FirstLineIndent, after.Paragraph.FirstLineIndent)
	appendFloatChange("margin-top", before.SpaceBefore, after.SpaceBefore)
	appendFloatChange("margin-bottom", before.SpaceAfter, after.SpaceAfter)
	appendFloatChange("margin-left", before.MarginLeft, after.MarginLeft)
	appendFloatChange("margin-right", before.MarginRight, after.MarginRight)
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
