package pdf

import (
	"slices"
	"strconv"
	"strings"

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
		for _, class := range strings.Fields(block.StyleClasses) {
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
		for _, class := range strings.Fields(block.StyleClasses) {
			names = append(names, tag+"."+class)
		}
		return slices.Compact(names)
	}
	return nil
}

func pdfStyleForBlock(block pdfTextBlock) pdfBlockResolvedStyle {
	return newPDFStyleResolver(nil, nil).styleForBlock(block)
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

func (r *pdfStyleResolver) applyStylesheet(sheet *css.Stylesheet) {
	if sheet == nil {
		return
	}
	for _, item := range sheet.Items {
		switch {
		case item.Rule != nil:
			r.applyRule(*item.Rule)
		case item.MediaBlock != nil:
			matched := item.MediaBlock.Query.Evaluate(true, true)
			r.tracer.traceMedia(item.MediaBlock.Query.Raw, matched, len(item.MediaBlock.Rules))
			if !matched {
				continue
			}
			for _, rule := range item.MediaBlock.Rules {
				r.applyRule(rule)
			}
		}
	}
}

func (r *pdfStyleResolver) applyRule(rule css.Rule) {
	mapped := pdfSelectorStyleNames(rule.Selector)
	if len(mapped) == 0 {
		r.tracer.traceIgnoredRule(rule.Selector, rule.Properties)
		return
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
}

func pdfStyleSeedWithoutExplicitFlags(style pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	style.Paragraph.HasBold = false
	style.Paragraph.HasItalic = false
	style.Paragraph.HasFirstLineIndent = false
	style.Paragraph.HasAlign = false
	style.Paragraph.HasVerticalAlign = false
	style.Paragraph.HasUnderline = false
	style.Paragraph.HasStrikethrough = false
	style.Paragraph.HasPreserveSpace = false
	style.Paragraph.HasHyphenation = false
	style.HasSpaceBefore = false
	style.HasSpaceAfter = false
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
	case "html", "body", "p", "h1", "h2", "h3", "h4", "h5", "h6", "img", "table", "code", "a", "span", "div":
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
		case "p", "h1", "h2", "h3", "h4", "h5", "h6", "img", "table", "code":
			return []string{strings.ToLower(sel.Element) + "." + sel.Class}
		}
		return nil
	}
	var targets []string
	if sel.Element != "" {
		switch strings.ToLower(sel.Element) {
		case "p", "h1", "h2", "h3", "h4", "h5", "h6", "img", "table", "code":
			targets = append(targets, strings.ToLower(sel.Element))
		}
	}
	if sel.Class != "" {
		targets = append(targets, sel.Class)
	}
	return slices.Compact(targets)
}
