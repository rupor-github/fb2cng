package pdf

import (
	"slices"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"fbc/css"
)

func (r *pdfStyleResolver) contextDescendantStyleNames(block pdfTextBlock) []string {
	ancestors := []string{pdfStyleHTML, pdfStyleBody}
	ancestors = append(ancestors, strings.Fields(block.ContextClasses)...)
	ancestors = slices.Compact(ancestors)
	candidates := pdfSelectorCandidatesForBlock(block)
	var names []string
	for _, ancestor := range ancestors {
		for _, candidate := range candidates {
			name := ancestor + "--" + candidate
			if _, ok := r.styles[name]; ok {
				names = append(names, name)
			}
		}
	}
	return slices.Compact(names)
}

func pdfSelectorCandidatesForBlock(block pdfTextBlock) []string {
	var candidates []string
	if tag := pdfElementTagForBlock(block); tag != "" {
		candidates = append(candidates, tag)
		for class := range strings.FieldsSeq(block.StyleClasses) {
			candidates = append(candidates, class)
			candidates = append(candidates, tag+"."+class)
		}
		return slices.Compact(candidates)
	}
	candidates = append(candidates, strings.Fields(block.StyleClasses)...)
	return slices.Compact(candidates)
}

func pdfElementClassStyleNames(block pdfTextBlock) []string {
	if tag := pdfElementTagForBlock(block); tag != "" {
		names := make([]string, 0, len(strings.Fields(block.StyleClasses)))
		for class := range strings.FieldsSeq(block.StyleClasses) {
			names = append(names, tag+"."+class)
		}
		return slices.Compact(names)
	}
	return nil
}

func pdfTagStyleNameForBlock(block pdfTextBlock) string {
	switch pdfElementTagForBlock(block) {
	case "p":
		return pdfStyleParagraph
	case "h1":
		return pdfStyleChapterTitleHeader
	case "h2", "h3", "h4", "h5", "h6":
		return pdfStyleSectionTitleHeader
	case "img":
		return pdfStyleImage
	case "table":
		return pdfStyleTable
	case "td":
		return pdfStyleTableCell
	case "th":
		return pdfStyleTableHeaderCell
	case "code":
		return pdfStyleCode
	default:
		return pdfStyleNameForBlock(block)
	}
}

func pdfStyleNameForBlock(block pdfTextBlock) string {
	if block.StyleName != "" {
		return block.StyleName
	}
	if block.Kind == pdfBlockHeading {
		return pdfHeadingStyleName(block.Depth)
	}
	return pdfStyleNameForKind(block.Kind)
}

func pdfStyleNameForKind(kind pdfBlockKind) string {
	switch kind {
	case pdfBlockParagraph:
		return pdfStyleParagraph
	case pdfBlockHeading:
		return pdfStyleChapterTitleHeader
	case pdfBlockSubtitle:
		return pdfStyleSubtitle
	case pdfBlockPoem:
		return pdfStyleVerse
	case pdfBlockTextAuthor:
		return pdfStyleTextAuthor
	case pdfBlockImage:
		return pdfStyleImage
	case pdfBlockTable:
		return pdfStyleTable
	case pdfBlockTOCEntry:
		return pdfStyleTOCItem
	case pdfBlockEmptyLine:
		return pdfStyleEmptyLine
	default:
		return pdfStyleParagraph
	}
}

func pdfElementTagForBlock(block pdfTextBlock) string {
	switch block.Kind {
	case pdfBlockParagraph, pdfBlockSubtitle, pdfBlockPoem, pdfBlockTextAuthor, pdfBlockTOCEntry:
		return "p"
	case pdfBlockHeading:
		level := min(max(block.Depth, 1), 6)
		return "h" + strconv.Itoa(level)
	case pdfBlockImage:
		if block.StyleName != "" && block.StyleName != pdfStyleImage {
			return "p"
		}
		return "img"
	case pdfBlockTable:
		return "table"
	case pdfBlockTableCell:
		if block.StyleName == pdfStyleTableHeaderCell {
			return "th"
		}
		return "td"
	default:
		return ""
	}
}

func pdfHeadingStyleName(depth int) string {
	if depth <= 1 {
		return pdfStyleChapterTitleHeader
	}
	return pdfStyleSectionTitleHeader
}

type pdfStylesheetStats struct {
	Rules  int
	Styles int
}

func (r *pdfStyleResolver) applyStylesheet(sheet *css.Stylesheet) pdfStylesheetStats {
	if sheet == nil {
		return pdfStylesheetStats{}
	}
	r.detectDropcapPatterns(sheet)
	var stats pdfStylesheetStats
	for _, item := range sheet.Items {
		switch {
		case item.Rule != nil:
			stats.Rules++
			stats.Styles += r.applyRule(*item.Rule)
		case item.MediaBlock != nil:
			matched := item.MediaBlock.Query.EvaluateContext(css.MediaContext{KF8: true, ET: true, FBCPDF: true})
			r.tracer.traceMedia(item.MediaBlock.Query.Raw, matched, len(item.MediaBlock.Rules))
			if !matched {
				continue
			}
			for _, rule := range item.MediaBlock.Rules {
				stats.Rules++
				stats.Styles += r.applyRule(rule)
			}
		}
	}
	return stats
}

func (r *pdfStyleResolver) detectDropcapPatterns(sheet *css.Stylesheet) {
	if r == nil || sheet == nil {
		return
	}
	for _, rule := range pdfStylesheetRules(sheet) {
		if rule.Selector.Ancestor == nil || rule.Selector.DescendantBaseName() != pdfStyleDropcap {
			continue
		}
		parents := pdfSelectorStyleNames(*rule.Selector.Ancestor)
		if len(parents) == 0 {
			continue
		}
		chars := pdfDropcapDefaultChars
		lines := pdfDropcapDefaultLines
		fontSize := css.Value{}
		if value, ok := rule.GetProperty("font-size"); ok {
			fontSize = value
			if value.Value > 0 {
				lines = clampPDFDropcapLines(int(value.Value + 0.5))
			}
		}
		for _, parent := range parents {
			r.dropcaps[parent] = pdfDropcapCSSConfig{Chars: chars, Lines: lines}
			if r.log != nil {
				r.log.Debug("Detected drop cap pattern",
					zap.String("parent", parent),
					zap.Float64("font-size", fontSize.Value),
					zap.String("unit", fontSize.Unit),
					zap.Int("lines", lines))
			}
			r.tracer.traceDropcapPattern(rule.Selector.Raw, parent, fontSize, chars, lines)
		}
	}
}

func pdfStylesheetRules(sheet *css.Stylesheet) []css.Rule {
	if sheet == nil {
		return nil
	}
	var rules []css.Rule
	for _, item := range sheet.Items {
		switch {
		case item.Rule != nil:
			rules = append(rules, *item.Rule)
		case item.MediaBlock != nil && item.MediaBlock.Query.EvaluateContext(css.MediaContext{KF8: true, ET: true, FBCPDF: true}):
			rules = append(rules, item.MediaBlock.Rules...)
		}
	}
	return rules
}

func (r *pdfStyleResolver) applyRule(rule css.Rule) int {
	if rule.Selector.Pseudo != css.PseudoNone {
		r.tracer.traceIgnoredRule(rule.Selector, rule.Properties)
		return 0
	}
	mapped := pdfSelectorStyleNames(rule.Selector)
	if len(mapped) == 0 {
		r.tracer.traceIgnoredRule(rule.Selector, rule.Properties)
		return 0
	}
	r.tracer.traceRule(rule.Selector, rule.Properties, mapped)
	for _, name := range mapped {
		style, ok := r.styles[name]
		if !ok {
			style = pdfStyleSeedWithoutExplicitFlags(r.namedStyle(pdfStyleParagraph))
		}
		before := style
		applyPDFStyleProperties(&style, rule.Properties)
		r.tracer.traceStyleUpdate(name, before, style)
		r.styles[name] = style
	}
	return len(mapped)
}

func pdfStyleSeedWithoutExplicitFlags(style pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	style.Paragraph.FontSizeSpec = pdfCSSLengthSpec{}
	style.Paragraph.HasBold = false
	style.Paragraph.HasItalic = false
	style.Paragraph.LineHeightSpec = pdfCSSLengthSpec{}
	style.Paragraph.LetterSpacingSpec = pdfCSSLengthSpec{}
	style.Paragraph.FirstLineIndentSpec = pdfCSSLengthSpec{}
	style.Paragraph.HasFirstLineIndent = false
	style.Paragraph.HasAlign = false
	style.Paragraph.HasVerticalAlign = false
	style.Paragraph.HasUnderline = false
	style.Paragraph.HasStrikethrough = false
	style.Paragraph.HasPreserveSpace = false
	style.Paragraph.HasNoWrap = false
	style.Paragraph.HasHyphenation = false
	style.SpaceBeforeSpec = pdfCSSLengthSpec{}
	style.HasSpaceBefore = false
	style.SpaceAfterSpec = pdfCSSLengthSpec{}
	style.HasSpaceAfter = false
	style.MarginLeftSpec = pdfCSSLengthSpec{}
	style.MarginRightSpec = pdfCSSLengthSpec{}
	style.PaddingTopSpec = pdfCSSLengthSpec{}
	style.PaddingRightSpec = pdfCSSLengthSpec{}
	style.PaddingBottomSpec = pdfCSSLengthSpec{}
	style.PaddingLeftSpec = pdfCSSLengthSpec{}
	style.HasKeepTogether = false
	style.HasPageBreakBefore = false
	style.HasPageBreakAfter = false
	style.HasHidden = false
	return style
}

func pdfSelectorStyleNames(sel css.Selector) []string {
	if sel.Ancestor != nil {
		return pdfDescendantSelectorStyleNames(sel)
	}
	if sel.Element != "" && sel.Class != "" {
		if strings.EqualFold(sel.Element, "img") {
			return []string{strings.ToLower(sel.Element) + "." + sel.Class}
		}
		return []string{sel.Class}
	}
	if sel.Class != "" {
		return []string{pdfClassSelectorStyleName(sel.Class)}
	}
	switch strings.ToLower(sel.Element) {
	case "html":
		return []string{pdfStyleHTML}
	case "body":
		return []string{pdfStyleBody}
	case "p":
		return []string{pdfStyleParagraph}
	case "td":
		return []string{pdfStyleTableCell}
	case "th":
		return []string{pdfStyleTableHeaderCell}
	case "h1":
		return []string{pdfStyleBodyTitleHeader, pdfStyleChapterTitleHeader, pdfStyleTOCTitle}
	case "h2", "h3", "h4", "h5", "h6":
		return []string{pdfStyleSectionTitleHeader}
	case "img":
		return []string{pdfStyleImage}
	case "table":
		return []string{pdfStyleTable}
	case "code":
		return []string{pdfStyleCode}
	default:
		return nil
	}
}

func pdfClassSelectorStyleName(class string) string {
	if pdfSelectorClassCollidesWithElement(class) {
		return "." + class
	}
	return class
}

func pdfSelectorClassCollidesWithElement(class string) bool {
	switch strings.ToLower(class) {
	case "html", "body", "p", "h1", "h2", "h3", "h4", "h5", "h6", "img", "table", "td", "th", "code", "a", "span", "div":
		return true
	default:
		return false
	}
}

func pdfDescendantSelectorStyleNames(sel css.Selector) []string {
	if sel.Ancestor == nil {
		return nil
	}
	ancestorNames := pdfSelectorStyleNames(*sel.Ancestor)
	if len(ancestorNames) == 0 {
		return nil
	}
	rightNames := pdfDescendantSelectorTargets(sel)
	if len(rightNames) == 0 {
		return nil
	}
	mapped := make([]string, 0, len(ancestorNames)*len(rightNames))
	for _, ancestor := range ancestorNames {
		for _, rightName := range rightNames {
			mapped = append(mapped, ancestor+"--"+rightName)
		}
	}
	return slices.Compact(mapped)
}

func pdfDescendantSelectorTargets(sel css.Selector) []string {
	if sel.Element != "" && sel.Class != "" {
		switch strings.ToLower(sel.Element) {
		case "p", "h1", "h2", "h3", "h4", "h5", "h6", "img", "table", "td", "th", "code":
			return []string{strings.ToLower(sel.Element) + "." + sel.Class}
		}
		return nil
	}
	var targets []string
	if sel.Element != "" {
		switch strings.ToLower(sel.Element) {
		case "p", "h1", "h2", "h3", "h4", "h5", "h6", "img", "table", "td", "th", "code":
			targets = append(targets, strings.ToLower(sel.Element))
		}
	}
	if sel.Class != "" {
		targets = append(targets, sel.Class)
	}
	return slices.Compact(targets)
}
