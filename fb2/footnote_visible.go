package fb2

// CountFootnoteVisibleElements counts visible footnote body elements after the
// footnote title/backlink area. It treats paragraph-like items and tables as
// one visible element each, and images as one visible element each.
//
// This matches the popup truncation heuristic used by the converters: if the
// footnote contains more than one visible element, readers may only render the
// first one and should therefore show the "more" indicator.
func CountFootnoteVisibleElements(section *Section) int {
	return countFootnoteSectionVisibleElements(section)
}

// FootnoteStartsWithBlock reports whether the first visible element in the footnote body
// is a non-paragraph block that cannot host the marker inline today. At the moment that
// means block images and tables.
func FootnoteStartsWithBlock(section *Section) bool {
	kind := firstFootnoteVisibleKind(section)
	return kind == footnoteVisibleBlockImage || kind == footnoteVisibleTable
}

type footnoteVisibleKind uint8

const (
	footnoteVisibleNone footnoteVisibleKind = iota
	footnoteVisibleParagraph
	footnoteVisibleBlockImage
	footnoteVisibleTable
)

func countFootnoteSectionVisibleElements(section *Section) int {
	if section == nil {
		return 0
	}

	count := 0
	for i := range section.Epigraphs {
		count += countFootnoteEpigraphVisibleElements(&section.Epigraphs[i])
	}
	if section.Image != nil {
		count++
	}
	if section.Annotation != nil {
		count += countFootnoteFlowVisibleElements(section.Annotation.Items)
	}
	count += countFootnoteFlowVisibleElements(section.Content)
	return count
}

func firstFootnoteVisibleKind(section *Section) footnoteVisibleKind {
	if section == nil {
		return footnoteVisibleNone
	}

	for i := range section.Epigraphs {
		if kind := firstFootnoteVisibleKindInEpigraph(&section.Epigraphs[i]); kind != footnoteVisibleNone {
			return kind
		}
	}
	if section.Image != nil {
		return footnoteVisibleBlockImage
	}
	if section.Annotation != nil {
		if kind := firstFootnoteVisibleKindInFlow(section.Annotation.Items); kind != footnoteVisibleNone {
			return kind
		}
	}
	return firstFootnoteVisibleKindInFlow(section.Content)
}

func firstFootnoteVisibleKindInEpigraph(epigraph *Epigraph) footnoteVisibleKind {
	if epigraph == nil {
		return footnoteVisibleNone
	}

	if kind := firstFootnoteVisibleKindInFlow(epigraph.Flow.Items); kind != footnoteVisibleNone {
		return kind
	}
	if len(epigraph.TextAuthors) > 0 {
		return footnoteVisibleParagraph
	}
	return footnoteVisibleNone
}

func firstFootnoteVisibleKindInFlow(items []FlowItem) footnoteVisibleKind {
	for i := range items {
		if kind := firstFootnoteVisibleKindInFlowItem(&items[i]); kind != footnoteVisibleNone {
			return kind
		}
	}
	return footnoteVisibleNone
}

func firstFootnoteVisibleKindInFlowItem(item *FlowItem) footnoteVisibleKind {
	if item == nil {
		return footnoteVisibleNone
	}

	switch item.Kind {
	case FlowParagraph, FlowSubtitle:
		if item.Paragraph != nil || item.Subtitle != nil {
			return footnoteVisibleParagraph
		}
	case FlowImage:
		if item.Image != nil {
			return footnoteVisibleBlockImage
		}
	case FlowPoem:
		return firstFootnoteVisibleKindInPoem(item.Poem)
	case FlowCite:
		return firstFootnoteVisibleKindInCite(item.Cite)
	case FlowTable:
		if item.Table != nil {
			return footnoteVisibleTable
		}
	}

	return footnoteVisibleNone
}

func firstFootnoteVisibleKindInPoem(poem *Poem) footnoteVisibleKind {
	if poem == nil {
		return footnoteVisibleNone
	}

	if kind := firstFootnoteVisibleKindInTitle(poem.Title); kind != footnoteVisibleNone {
		return kind
	}
	for i := range poem.Epigraphs {
		if kind := firstFootnoteVisibleKindInEpigraph(&poem.Epigraphs[i]); kind != footnoteVisibleNone {
			return kind
		}
	}
	if len(poem.Subtitles) > 0 {
		return footnoteVisibleParagraph
	}
	for i := range poem.Stanzas {
		if kind := firstFootnoteVisibleKindInTitle(poem.Stanzas[i].Title); kind != footnoteVisibleNone {
			return kind
		}
		if poem.Stanzas[i].Subtitle != nil || len(poem.Stanzas[i].Verses) > 0 {
			return footnoteVisibleParagraph
		}
	}
	if len(poem.TextAuthors) > 0 || poem.Date != nil {
		return footnoteVisibleParagraph
	}
	return footnoteVisibleNone
}

func firstFootnoteVisibleKindInCite(cite *Cite) footnoteVisibleKind {
	if cite == nil {
		return footnoteVisibleNone
	}

	if kind := firstFootnoteVisibleKindInFlow(cite.Items); kind != footnoteVisibleNone {
		return kind
	}
	if len(cite.TextAuthors) > 0 {
		return footnoteVisibleParagraph
	}
	return footnoteVisibleNone
}

func firstFootnoteVisibleKindInTitle(title *Title) footnoteVisibleKind {
	if title == nil {
		return footnoteVisibleNone
	}

	for i := range title.Items {
		if title.Items[i].Paragraph != nil {
			return footnoteVisibleParagraph
		}
	}
	return footnoteVisibleNone
}

func countFootnoteEpigraphVisibleElements(epigraph *Epigraph) int {
	if epigraph == nil {
		return 0
	}

	count := countFootnoteFlowVisibleElements(epigraph.Flow.Items)
	count += len(epigraph.TextAuthors)
	return count
}

func countFootnoteFlowVisibleElements(items []FlowItem) int {
	count := 0
	for i := range items {
		count += countFootnoteFlowItemVisibleElements(&items[i])
	}
	return count
}

func countFootnoteFlowItemVisibleElements(item *FlowItem) int {
	if item == nil {
		return 0
	}

	switch item.Kind {
	case FlowParagraph, FlowSubtitle:
		if item.Paragraph != nil || item.Subtitle != nil {
			return 1
		}
	case FlowImage:
		if item.Image != nil {
			return 1
		}
	case FlowPoem:
		return countFootnotePoemVisibleElements(item.Poem)
	case FlowCite:
		return countFootnoteCiteVisibleElements(item.Cite)
	case FlowTable:
		if item.Table != nil {
			return 1
		}
	}

	return 0
}

func countFootnoteTitleVisibleElements(title *Title) int {
	if title == nil {
		return 0
	}

	count := 0
	for i := range title.Items {
		if title.Items[i].Paragraph != nil {
			count++
		}
	}
	return count
}

func countFootnotePoemVisibleElements(poem *Poem) int {
	if poem == nil {
		return 0
	}

	count := countFootnoteTitleVisibleElements(poem.Title)
	for i := range poem.Epigraphs {
		count += countFootnoteEpigraphVisibleElements(&poem.Epigraphs[i])
	}
	count += len(poem.Subtitles)
	for i := range poem.Stanzas {
		count += countFootnoteTitleVisibleElements(poem.Stanzas[i].Title)
		if poem.Stanzas[i].Subtitle != nil {
			count++
		}
		count += len(poem.Stanzas[i].Verses)
	}
	count += len(poem.TextAuthors)
	if poem.Date != nil {
		count++
	}
	return count
}

func countFootnoteCiteVisibleElements(cite *Cite) int {
	if cite == nil {
		return 0
	}

	count := countFootnoteFlowVisibleElements(cite.Items)
	count += len(cite.TextAuthors)
	return count
}
