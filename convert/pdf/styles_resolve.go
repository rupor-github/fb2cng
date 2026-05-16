package pdf

import "strings"

func (r *pdfStyleResolver) styleForBlock(block pdfTextBlock) pdfBlockResolvedStyle {
	name := pdfStyleNameForBlock(block)
	defaultStyle := r.defaultStyle(name)
	tagStyle := r.tagStyleForBlock(block)
	style := r.applyRootInheritedParagraphDefaults(defaultStyle)
	style = mergePDFStyleOverrides(style, r.namedStyle(name), defaultStyle)
	style = r.applyContextInheritedBlockDefaults(style, tagStyle, block)
	classFallback := r.namedStyle(pdfStyleParagraph)
	for _, class := range strings.Fields(block.StyleClasses) {
		classStyle, ok := r.styles[class]
		if !ok {
			continue
		}
		if contextStyleClassShouldSkipInheritedAndHorizontalMargins(block, class) {
			style = mergePDFContextClassStyleOverrides(style, classStyle, classFallback)
			continue
		}
		if pdfContainerStyleClass(class) {
			style = mergePDFContainerClassStyleOverrides(style, classStyle, classFallback)
			continue
		}
		style = mergePDFStyleOverrides(style, classStyle, classFallback)
	}
	for _, selectorStyleName := range pdfElementClassStyleNames(block) {
		selectorStyle, ok := r.styles[selectorStyleName]
		if !ok {
			continue
		}
		style = mergePDFStyleOverrides(style, selectorStyle, classFallback)
	}
	for _, descStyleName := range r.contextDescendantStyleNames(block) {
		descStyle, ok := r.styles[descStyleName]
		if !ok {
			continue
		}
		style = mergePDFStyleOverrides(style, descStyle, classFallback)
	}
	if block.Kind == pdfBlockTOCEntry {
		style.Paragraph.FirstLineIndent = max(float64(block.Depth-1)*pdfTOCIndentPerDepth, 0)
	}
	return injectPDFImplicitLineHeight(style, defaultStyle)
}

func injectPDFImplicitLineHeight(style, fallback pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	if style.Paragraph.LineHeightExplicit || fallback.Paragraph.FontSize <= 0 || fallback.Paragraph.LineHeight <= 0 || style.Paragraph.FontSize <= 0 {
		return style
	}
	if style.Paragraph.LineHeight != fallback.Paragraph.LineHeight || style.Paragraph.FontSize == fallback.Paragraph.FontSize {
		return style
	}
	style.Paragraph.LineHeight = fallback.Paragraph.LineHeight * style.Paragraph.FontSize / fallback.Paragraph.FontSize
	return style
}

func mergePDFStyleOverrides(base, override, fallback pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	if override.Paragraph.FontFamily != fallback.Paragraph.FontFamily {
		base.Paragraph.FontFamily = override.Paragraph.FontFamily
	}
	if override.Paragraph.Bold != fallback.Paragraph.Bold {
		base.Paragraph.Bold = override.Paragraph.Bold
	}
	if override.Paragraph.Italic != fallback.Paragraph.Italic {
		base.Paragraph.Italic = override.Paragraph.Italic
	}
	if override.Paragraph.FontSize != fallback.Paragraph.FontSize {
		base.Paragraph.FontSize = override.Paragraph.FontSize
	}
	base.Paragraph = mergePDFLineHeightOverride(base.Paragraph, override.Paragraph, fallback.Paragraph)
	if override.Paragraph.LetterSpacing != fallback.Paragraph.LetterSpacing {
		base.Paragraph.LetterSpacing = override.Paragraph.LetterSpacing
	}
	if override.Paragraph.HasFirstLineIndent || override.Paragraph.FirstLineIndent != fallback.Paragraph.FirstLineIndent {
		base.Paragraph.FirstLineIndent = override.Paragraph.FirstLineIndent
		base.Paragraph.HasFirstLineIndent = override.Paragraph.HasFirstLineIndent
	}
	if override.Paragraph.Align != fallback.Paragraph.Align {
		base.Paragraph.Align = override.Paragraph.Align
	}
	if override.Paragraph.VerticalAlign != fallback.Paragraph.VerticalAlign {
		base.Paragraph.VerticalAlign = override.Paragraph.VerticalAlign
	}
	if override.Paragraph.Color != fallback.Paragraph.Color {
		base.Paragraph.Color = override.Paragraph.Color
	}
	if override.Paragraph.Underline != fallback.Paragraph.Underline {
		base.Paragraph.Underline = override.Paragraph.Underline
	}
	if override.Paragraph.Strikethrough != fallback.Paragraph.Strikethrough {
		base.Paragraph.Strikethrough = override.Paragraph.Strikethrough
	}
	if override.Paragraph.PreserveSpace != fallback.Paragraph.PreserveSpace {
		base.Paragraph.PreserveSpace = override.Paragraph.PreserveSpace
	}
	if override.Paragraph.Hyphenation != fallback.Paragraph.Hyphenation {
		base.Paragraph.Hyphenation = override.Paragraph.Hyphenation
	}
	if override.HasSpaceBefore || override.SpaceBefore != fallback.SpaceBefore {
		base.SpaceBefore = override.SpaceBefore
	}
	if override.HasSpaceAfter || override.SpaceAfter != fallback.SpaceAfter {
		base.SpaceAfter = override.SpaceAfter
	}
	if override.MarginLeft != fallback.MarginLeft {
		base.MarginLeft = override.MarginLeft
	}
	if override.MarginRight != fallback.MarginRight {
		base.MarginRight = override.MarginRight
	}
	if override.PaddingTop != fallback.PaddingTop {
		base.PaddingTop = override.PaddingTop
	}
	if override.PaddingRight != fallback.PaddingRight {
		base.PaddingRight = override.PaddingRight
	}
	if override.PaddingBottom != fallback.PaddingBottom {
		base.PaddingBottom = override.PaddingBottom
	}
	if override.PaddingLeft != fallback.PaddingLeft {
		base.PaddingLeft = override.PaddingLeft
	}
	if override.HasWidth != fallback.HasWidth || override.Width != fallback.Width {
		base.HasWidth = override.HasWidth
		base.Width = override.Width
	}
	if override.HasMinWidth != fallback.HasMinWidth || override.MinWidth != fallback.MinWidth {
		base.HasMinWidth = override.HasMinWidth
		base.MinWidth = override.MinWidth
	}
	if override.HasMaxWidth != fallback.HasMaxWidth || override.MaxWidth != fallback.MaxWidth {
		base.HasMaxWidth = override.HasMaxWidth
		base.MaxWidth = override.MaxWidth
	}
	if override.HasBackground != fallback.HasBackground {
		base.HasBackground = override.HasBackground
		base.BackgroundColor = override.BackgroundColor
	}
	if override.HasBorder != fallback.HasBorder || override.BorderWidth != fallback.BorderWidth || override.BorderColor != fallback.BorderColor {
		base.HasBorder = override.HasBorder
		base.BorderWidth = override.BorderWidth
		base.BorderColor = override.BorderColor
	}
	if override.KeepTogether != fallback.KeepTogether {
		base.KeepTogether = override.KeepTogether
	}
	if override.KeepWithNextLines != fallback.KeepWithNextLines {
		base.KeepWithNextLines = override.KeepWithNextLines
	}
	if override.PageBreakBefore != fallback.PageBreakBefore {
		base.PageBreakBefore = override.PageBreakBefore
	}
	if override.PageBreakAfter != fallback.PageBreakAfter {
		base.PageBreakAfter = override.PageBreakAfter
	}
	if override.Hidden != fallback.Hidden {
		base.Hidden = override.Hidden
	}
	if override.Orphans != fallback.Orphans {
		base.Orphans = override.Orphans
	}
	if override.Widows != fallback.Widows {
		base.Widows = override.Widows
	}
	return base
}

func (r *pdfStyleResolver) namedStyle(name string) pdfBlockResolvedStyle {
	if r != nil {
		if style, ok := r.styles[name]; ok {
			return style
		}
	}
	if r != nil {
		if style, ok := r.styles[pdfStyleParagraph]; ok {
			return style
		}
	}
	return defaultPDFStyles()[pdfStyleParagraph]
}

func (r *pdfStyleResolver) defaultStyle(name string) pdfBlockResolvedStyle {
	if r != nil && r.defaults != nil {
		if style, ok := r.defaults[name]; ok {
			return style
		}
		if style, ok := r.defaults[pdfStyleParagraph]; ok {
			return style
		}
	}
	return defaultPDFStyles()[pdfStyleParagraph]
}

func (r *pdfStyleResolver) rootParagraphStyle() paragraphStyle {
	base := r.defaultStyle(pdfStyleHTML).Paragraph
	rootDefault := r.defaultStyle(pdfStyleBody).Paragraph
	base = mergePDFInheritedParagraphStyle(base, r.namedStyle(pdfStyleHTML).Paragraph, r.defaultStyle(pdfStyleHTML).Paragraph)
	base = mergePDFInheritedParagraphStyle(base, r.namedStyle(pdfStyleBody).Paragraph, rootDefault)
	return base
}

func (r *pdfStyleResolver) tagStyleForBlock(block pdfTextBlock) pdfBlockResolvedStyle {
	name := pdfTagStyleNameForBlock(block)
	style := r.applyRootInheritedParagraphDefaults(r.defaultStyle(name))
	return mergePDFStyleOverrides(style, r.namedStyle(name), r.defaultStyle(name))
}

func (r *pdfStyleResolver) contextInheritedBlockStyle(tagStyle pdfBlockResolvedStyle, block pdfTextBlock) pdfBlockResolvedStyle {
	style := tagStyle
	var (
		left     float64
		right    float64
		hasLeft  bool
		hasRight bool
	)
	for _, class := range strings.Fields(block.ContextClasses) {
		if class == pdfStyleNameForBlock(block) {
			continue
		}
		contextStyle, ok := r.styles[class]
		if !ok {
			continue
		}
		fallback := r.defaultStyle(class)
		style.Paragraph = mergePDFInheritedParagraphStyle(style.Paragraph, contextStyle.Paragraph, fallback.Paragraph)
		marginFallback := r.defaultStyle(pdfStyleParagraph)
		if contextStyle.MarginLeft != marginFallback.MarginLeft {
			left += contextStyle.MarginLeft
			hasLeft = true
		}
		if contextStyle.MarginRight != marginFallback.MarginRight {
			right += contextStyle.MarginRight
			hasRight = true
		}
	}
	if hasLeft {
		style.MarginLeft = left
	}
	if hasRight {
		style.MarginRight = right
	}
	return style
}

func (r *pdfStyleResolver) applyContextInheritedBlockDefaults(style, tagStyle pdfBlockResolvedStyle, block pdfTextBlock) pdfBlockResolvedStyle {
	contextStyle := r.contextInheritedBlockStyle(tagStyle, block)
	if style.Paragraph.FontFamily == tagStyle.Paragraph.FontFamily {
		style.Paragraph.FontFamily = contextStyle.Paragraph.FontFamily
	}
	if style.Paragraph.Bold == tagStyle.Paragraph.Bold {
		style.Paragraph.Bold = contextStyle.Paragraph.Bold
	}
	if style.Paragraph.Italic == tagStyle.Paragraph.Italic {
		style.Paragraph.Italic = contextStyle.Paragraph.Italic
	}
	if style.Paragraph.FontSize == tagStyle.Paragraph.FontSize {
		style.Paragraph.FontSize = contextStyle.Paragraph.FontSize
	}
	if !style.Paragraph.LineHeightExplicit && style.Paragraph.LineHeight == tagStyle.Paragraph.LineHeight {
		style.Paragraph.LineHeight = contextStyle.Paragraph.LineHeight
		style.Paragraph.LineHeightExplicit = contextStyle.Paragraph.LineHeightExplicit
	}
	if style.Paragraph.LetterSpacing == tagStyle.Paragraph.LetterSpacing {
		style.Paragraph.LetterSpacing = contextStyle.Paragraph.LetterSpacing
	}
	if style.Paragraph.FirstLineIndent == tagStyle.Paragraph.FirstLineIndent {
		style.Paragraph.FirstLineIndent = contextStyle.Paragraph.FirstLineIndent
		style.Paragraph.HasFirstLineIndent = contextStyle.Paragraph.HasFirstLineIndent
	}
	if style.Paragraph.Align == tagStyle.Paragraph.Align {
		style.Paragraph.Align = contextStyle.Paragraph.Align
	}
	if style.Paragraph.Color == tagStyle.Paragraph.Color {
		style.Paragraph.Color = contextStyle.Paragraph.Color
	}
	if style.Paragraph.PreserveSpace == tagStyle.Paragraph.PreserveSpace {
		style.Paragraph.PreserveSpace = contextStyle.Paragraph.PreserveSpace
	}
	if style.Paragraph.Hyphenation == tagStyle.Paragraph.Hyphenation {
		style.Paragraph.Hyphenation = contextStyle.Paragraph.Hyphenation
	}
	if style.MarginLeft == tagStyle.MarginLeft {
		style.MarginLeft = contextStyle.MarginLeft
	} else if contextStyle.MarginLeft != tagStyle.MarginLeft {
		style.MarginLeft += contextStyle.MarginLeft
	}
	if style.MarginRight == tagStyle.MarginRight {
		style.MarginRight = contextStyle.MarginRight
	} else if contextStyle.MarginRight != tagStyle.MarginRight {
		style.MarginRight += contextStyle.MarginRight
	}
	return style
}

func contextStyleClassShouldSkipInheritedAndHorizontalMargins(block pdfTextBlock, class string) bool {
	if class == "" || !pdfContainerStyleClass(class) {
		return false
	}
	for _, contextClass := range strings.Fields(block.ContextClasses) {
		if contextClass == class {
			return true
		}
	}
	return false
}

func pdfContainerStyleClass(class string) bool {
	switch class {
	case pdfStyleBodyTitle, pdfStyleChapterTitle, pdfStyleSectionTitle, pdfStyleAnnotation, pdfStyleFootnote, pdfStylePoem, pdfStyleStanza, pdfStyleEpigraph, pdfStyleCite:
		return true
	default:
		return false
	}
}

func mergePDFContainerClassStyleOverrides(base, override, fallback pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	base = mergePDFContextClassStyleOverrides(base, override, fallback)
	if override.MarginLeft != fallback.MarginLeft {
		base.MarginLeft = override.MarginLeft
	}
	if override.MarginRight != fallback.MarginRight {
		base.MarginRight = override.MarginRight
	}
	return base
}

func mergePDFContextClassStyleOverrides(base, override, fallback pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	if override.HasSpaceBefore || override.SpaceBefore != fallback.SpaceBefore {
		base.SpaceBefore = override.SpaceBefore
	}
	if override.HasSpaceAfter || override.SpaceAfter != fallback.SpaceAfter {
		base.SpaceAfter = override.SpaceAfter
	}
	if override.PaddingTop != fallback.PaddingTop {
		base.PaddingTop = override.PaddingTop
	}
	if override.PaddingRight != fallback.PaddingRight {
		base.PaddingRight = override.PaddingRight
	}
	if override.PaddingBottom != fallback.PaddingBottom {
		base.PaddingBottom = override.PaddingBottom
	}
	if override.PaddingLeft != fallback.PaddingLeft {
		base.PaddingLeft = override.PaddingLeft
	}
	if override.HasWidth != fallback.HasWidth || override.Width != fallback.Width {
		base.HasWidth = override.HasWidth
		base.Width = override.Width
	}
	if override.HasMinWidth != fallback.HasMinWidth || override.MinWidth != fallback.MinWidth {
		base.HasMinWidth = override.HasMinWidth
		base.MinWidth = override.MinWidth
	}
	if override.HasMaxWidth != fallback.HasMaxWidth || override.MaxWidth != fallback.MaxWidth {
		base.HasMaxWidth = override.HasMaxWidth
		base.MaxWidth = override.MaxWidth
	}
	if override.HasBackground != fallback.HasBackground {
		base.HasBackground = override.HasBackground
		base.BackgroundColor = override.BackgroundColor
	}
	if override.HasBorder != fallback.HasBorder || override.BorderWidth != fallback.BorderWidth || override.BorderColor != fallback.BorderColor {
		base.HasBorder = override.HasBorder
		base.BorderWidth = override.BorderWidth
		base.BorderColor = override.BorderColor
	}
	if override.KeepTogether != fallback.KeepTogether {
		base.KeepTogether = override.KeepTogether
	}
	if override.KeepWithNextLines != fallback.KeepWithNextLines {
		base.KeepWithNextLines = override.KeepWithNextLines
	}
	if override.PageBreakBefore != fallback.PageBreakBefore {
		base.PageBreakBefore = override.PageBreakBefore
	}
	if override.PageBreakAfter != fallback.PageBreakAfter {
		base.PageBreakAfter = override.PageBreakAfter
	}
	if override.Hidden != fallback.Hidden {
		base.Hidden = override.Hidden
	}
	if override.Orphans != fallback.Orphans {
		base.Orphans = override.Orphans
	}
	if override.Widows != fallback.Widows {
		base.Widows = override.Widows
	}
	return base
}

func (r *pdfStyleResolver) applyRootInheritedParagraphDefaults(style pdfBlockResolvedStyle) pdfBlockResolvedStyle {
	root := r.rootParagraphStyle()
	rootDefault := r.defaultStyle(pdfStyleBody).Paragraph
	if style.Paragraph.FontFamily == rootDefault.FontFamily {
		style.Paragraph.FontFamily = root.FontFamily
	}
	if style.Paragraph.Bold == rootDefault.Bold {
		style.Paragraph.Bold = root.Bold
	}
	if style.Paragraph.Italic == rootDefault.Italic {
		style.Paragraph.Italic = root.Italic
	}
	if style.Paragraph.FontSize == rootDefault.FontSize {
		style.Paragraph.FontSize = root.FontSize
	}
	if !style.Paragraph.LineHeightExplicit && style.Paragraph.LineHeight == rootDefault.LineHeight {
		style.Paragraph.LineHeight = root.LineHeight
		style.Paragraph.LineHeightExplicit = root.LineHeightExplicit
	}
	if style.Paragraph.LetterSpacing == rootDefault.LetterSpacing {
		style.Paragraph.LetterSpacing = root.LetterSpacing
	}
	if !style.Paragraph.HasFirstLineIndent && style.Paragraph.FirstLineIndent == rootDefault.FirstLineIndent {
		style.Paragraph.FirstLineIndent = root.FirstLineIndent
		style.Paragraph.HasFirstLineIndent = root.HasFirstLineIndent
	}
	if style.Paragraph.Align == rootDefault.Align {
		style.Paragraph.Align = root.Align
	}
	if style.Paragraph.Color == rootDefault.Color {
		style.Paragraph.Color = root.Color
	}
	if style.Paragraph.PreserveSpace == rootDefault.PreserveSpace {
		style.Paragraph.PreserveSpace = root.PreserveSpace
	}
	if style.Paragraph.Hyphenation == rootDefault.Hyphenation {
		style.Paragraph.Hyphenation = root.Hyphenation
	}
	return style
}

func mergePDFInheritedParagraphStyle(base, override, fallback paragraphStyle) paragraphStyle {
	if override.FontFamily != fallback.FontFamily {
		base.FontFamily = override.FontFamily
	}
	if override.Bold != fallback.Bold {
		base.Bold = override.Bold
	}
	if override.Italic != fallback.Italic {
		base.Italic = override.Italic
	}
	if override.FontSize != fallback.FontSize {
		base.FontSize = override.FontSize
	}
	base = mergePDFLineHeightOverride(base, override, fallback)
	if override.LetterSpacing != fallback.LetterSpacing {
		base.LetterSpacing = override.LetterSpacing
	}
	if override.HasFirstLineIndent || override.FirstLineIndent != fallback.FirstLineIndent {
		base.FirstLineIndent = override.FirstLineIndent
		base.HasFirstLineIndent = override.HasFirstLineIndent
	}
	if override.Align != fallback.Align {
		base.Align = override.Align
	}
	if override.Color != fallback.Color {
		base.Color = override.Color
	}
	if override.PreserveSpace != fallback.PreserveSpace {
		base.PreserveSpace = override.PreserveSpace
	}
	if override.Hyphenation != fallback.Hyphenation {
		base.Hyphenation = override.Hyphenation
	}
	return base
}

func mergePDFLineHeightOverride(base, override, fallback paragraphStyle) paragraphStyle {
	if override.LineHeightExplicit {
		base.LineHeight = override.LineHeight
		base.LineHeightExplicit = true
		return base
	}
	if !base.LineHeightExplicit && override.LineHeight != fallback.LineHeight {
		base.LineHeight = override.LineHeight
	}
	return base
}
