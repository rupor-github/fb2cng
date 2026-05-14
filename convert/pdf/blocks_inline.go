package pdf

import (
	"strings"

	"fbc/fb2"
)

func paragraphTextAndLinks(paragraph *fb2.Paragraph) (string, []pdfTextLink) {
	if paragraph == nil {
		return "", nil
	}
	return inlineSegmentsTextAndLinks(paragraph.Text)
}

func inlineRunsRenderable(runs []pdfInlineRun) bool {
	for _, run := range runs {
		if run.Text != "" || run.ImageID != "" {
			return true
		}
	}
	return false
}

func paragraphInlineRuns(paragraph *fb2.Paragraph) []pdfInlineRun {
	if paragraph == nil {
		return nil
	}
	var runs []pdfInlineRun
	for i := range paragraph.Text {
		appendInlineSegmentRun(&runs, &paragraph.Text[i], pdfInlineRun{})
	}
	if paragraphIsCodeBlock(paragraph) {
		return trimCodeBlockInlineRuns(runs)
	}
	return trimInlineRuns(runs)
}

func paragraphImageOnly(paragraph *fb2.Paragraph) (string, string, bool) {
	if paragraph == nil || len(paragraph.Text) == 0 {
		return "", "", false
	}
	var imageID string
	var alt string
	for i := range paragraph.Text {
		if !inlineSegmentImageOnly(&paragraph.Text[i], &imageID, &alt) {
			return "", "", false
		}
	}
	return imageID, alt, imageID != ""
}

func inlineSegmentImageOnly(seg *fb2.InlineSegment, imageID *string, alt *string) bool {
	if seg == nil {
		return true
	}
	if seg.Text != "" && strings.TrimSpace(seg.Text) != "" {
		return false
	}
	if seg.Kind == fb2.InlineImageSegment {
		if seg.Image == nil {
			return true
		}
		id := imageRefID(seg.Image.Href)
		if id == "" {
			return true
		}
		if *imageID != "" {
			return false
		}
		*imageID = id
		*alt = strings.TrimSpace(seg.Image.Alt)
	}
	for i := range seg.Children {
		if !inlineSegmentImageOnly(&seg.Children[i], imageID, alt) {
			return false
		}
	}
	return true
}

func paragraphIsCodeBlock(paragraph *fb2.Paragraph) bool {
	if paragraph == nil || len(paragraph.Text) == 0 {
		return false
	}
	seenCode := false
	for i := range paragraph.Text {
		if !inlineSegmentIsCodeBlockContent(&paragraph.Text[i], false, &seenCode) {
			return false
		}
	}
	return seenCode
}

func inlineSegmentIsCodeBlockContent(seg *fb2.InlineSegment, inCode bool, seenCode *bool) bool {
	if seg == nil {
		return true
	}
	if seg.Kind == fb2.InlineCode {
		inCode = true
		*seenCode = true
	}
	if seg.Text != "" && !inCode && strings.TrimSpace(seg.Text) != "" {
		return false
	}
	if seg.Kind == fb2.InlineImageSegment && !inCode {
		return false
	}
	for i := range seg.Children {
		if !inlineSegmentIsCodeBlockContent(&seg.Children[i], inCode, seenCode) {
			return false
		}
	}
	return true
}

func inlineSegmentsTextAndLinks(segments []fb2.InlineSegment) (string, []pdfTextLink) {
	var b strings.Builder
	var links []pdfTextLink
	for i := range segments {
		appendInlineSegmentText(&b, &links, &segments[i])
	}
	return normalizeInlineTextAndLinks(b.String(), links)
}

func imageRefID(href string) string {
	return strings.TrimPrefix(strings.TrimSpace(href), "#")
}

func normalizeInlineTextAndLinks(raw string, links []pdfTextLink) (string, []pdfTextLink) {
	runes := []rune(raw)
	boundary := make([]int, len(runes)+1)
	var b strings.Builder
	normalizedLen := 0
	pendingSpace := false
	for i, r := range runes {
		if isBreakableSpace(r) {
			boundary[i] = normalizedLen
			if normalizedLen > 0 {
				pendingSpace = true
			}
			continue
		}
		if pendingSpace && normalizedLen > 0 {
			b.WriteByte(' ')
			normalizedLen++
		}
		pendingSpace = false
		boundary[i] = normalizedLen
		b.WriteRune(r)
		normalizedLen++
	}
	boundary[len(runes)] = normalizedLen

	normalizedLinks := make([]pdfTextLink, 0, len(links))
	for _, link := range links {
		start, end := trimRawLinkRange(runes, link.Start, link.End)
		if start >= end || strings.TrimSpace(link.Href) == "" {
			continue
		}
		normalizedStart := boundary[start]
		normalizedEnd := boundary[end]
		if normalizedStart >= normalizedEnd {
			continue
		}
		normalizedLinks = append(normalizedLinks, pdfTextLink{Start: normalizedStart, End: normalizedEnd, Href: link.Href})
	}
	return b.String(), normalizedLinks
}

func trimRawLinkRange(runes []rune, start int, end int) (int, int) {
	start = min(max(start, 0), len(runes))
	end = min(max(end, start), len(runes))
	for start < end && isBreakableSpace(runes[start]) {
		start++
	}
	for end > start && isBreakableSpace(runes[end-1]) {
		end--
	}
	return start, end
}

func appendInlineSegmentText(b *strings.Builder, links *[]pdfTextLink, seg *fb2.InlineSegment) {
	if seg == nil {
		return
	}
	if seg.Kind == fb2.InlineImageSegment {
		return
	}
	start := runeLenString(b.String())
	b.WriteString(seg.Text)
	for i := range seg.Children {
		appendInlineSegmentText(b, links, &seg.Children[i])
	}
	if seg.Kind == fb2.InlineLink && seg.Href != "" {
		end := runeLenString(b.String())
		if end > start {
			*links = append(*links, pdfTextLink{Start: start, End: end, Href: seg.Href})
		}
	}
}

func appendInlineSegmentRun(runs *[]pdfInlineRun, seg *fb2.InlineSegment, inherited pdfInlineRun) {
	if seg == nil {
		return
	}
	style := inherited
	style.Text = ""
	style = applyInlineSegmentStyle(style, seg)
	if seg.Kind == fb2.InlineImageSegment {
		if seg.Image != nil {
			style.ImageID = imageRefID(seg.Image.Href)
			appendInlineRun(runs, style)
		}
		return
	}
	if seg.Text != "" {
		style.Text = seg.Text
		appendInlineRun(runs, style)
	}
	style.Text = ""
	for i := range seg.Children {
		appendInlineSegmentRun(runs, &seg.Children[i], style)
	}
}

func pdfLinkStyleClass(seg *fb2.InlineSegment) string {
	if seg == nil || strings.TrimSpace(seg.Href) == "" {
		return ""
	}
	if strings.EqualFold(seg.LinkType, "note") {
		return pdfStyleLinkFootnote
	}
	if strings.HasPrefix(strings.TrimSpace(seg.Href), "#") {
		return pdfStyleLinkInternal
	}
	return pdfStyleLinkExternal
}

func applyInlineSegmentStyle(style pdfInlineRun, seg *fb2.InlineSegment) pdfInlineRun {
	switch seg.Kind {
	case fb2.InlineStrong:
		style.Bold = true
	case fb2.InlineEmphasis:
		style.Italic = true
	case fb2.InlineNamedStyle:
		style.StyleClasses = joinStyleClasses(style.StyleClasses, seg.Style)
	case fb2.InlineStrikethrough:
		style.Strikethrough = true
	case fb2.InlineSub:
		style.Subscript = true
		style.Superscript = false
	case fb2.InlineSup:
		style.Superscript = true
		style.Subscript = false
	case fb2.InlineCode:
		style.Code = true
		style.StyleClasses = joinStyleClasses(style.StyleClasses, pdfStyleCode)
	case fb2.InlineLink:
		style.StyleClasses = joinStyleClasses(style.StyleClasses, pdfLinkStyleClass(seg))
		style.LinkHref = strings.TrimSpace(seg.Href)
	}
	return style
}

func appendInlineRun(runs *[]pdfInlineRun, run pdfInlineRun) {
	if run.Superscript || run.Subscript {
		run.Text = strings.TrimSpace(run.Text)
	}
	if run.Text == "" && run.ImageID == "" {
		return
	}
	last := len(*runs) - 1
	if last >= 0 && sameInlineStyle((*runs)[last], run) {
		(*runs)[last].Text += run.Text
		return
	}
	*runs = append(*runs, run)
}

func sameInlineStyle(a, b pdfInlineRun) bool {
	return a.StyleClasses == b.StyleClasses && a.LinkHref == b.LinkHref && a.ImageID == b.ImageID && a.Bold == b.Bold && a.Italic == b.Italic && a.Underline == b.Underline && a.Strikethrough == b.Strikethrough && a.Subscript == b.Subscript && a.Superscript == b.Superscript && a.Code == b.Code
}

func trimCodeBlockInlineRuns(runs []pdfInlineRun) []pdfInlineRun {
	for len(runs) > 0 {
		trimmed := strings.TrimLeft(runs[0].Text, "\r\n")
		if trimmed != "" || runs[0].ImageID != "" {
			runs[0].Text = trimmed
			break
		}
		runs = runs[1:]
	}
	for len(runs) > 0 {
		last := len(runs) - 1
		trimmed := strings.TrimRight(runs[last].Text, " \t\n\r")
		if trimmed != "" || runs[last].ImageID != "" {
			runs[last].Text = trimmed
			break
		}
		runs = runs[:last]
	}
	return runs
}

func trimInlineRuns(runs []pdfInlineRun) []pdfInlineRun {
	for len(runs) > 0 {
		trimmed := strings.TrimLeft(runs[0].Text, " \t\n\r")
		if trimmed != "" || runs[0].ImageID != "" {
			runs[0].Text = trimmed
			break
		}
		runs = runs[1:]
	}
	for len(runs) > 0 {
		last := len(runs) - 1
		trimmed := strings.TrimRight(runs[last].Text, " \t\n\r")
		if trimmed != "" || runs[last].ImageID != "" {
			runs[last].Text = trimmed
			break
		}
		runs = runs[:last]
	}
	return runs
}

func runeLenString(s string) int {
	return len([]rune(s))
}
