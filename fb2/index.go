package fb2

import (
	"net/url"
	"strings"

	"go.uber.org/zap"
)

// Index building functions for FictionBook - footnotes, IDs, and links.

func (fb *FictionBook) buildFootnotesIndex(log *zap.Logger) FootnoteRefs {
	index := make(FootnoteRefs)

	for i := range fb.Bodies {
		if !fb.Bodies[i].Footnotes() {
			continue
		}

		for j := range fb.Bodies[i].Sections {
			if fb.Bodies[i].Sections[j].ID == "" {
				// Skip sections without ID - they're not valid footnotes
				// NOTE: These should have been removed by normalization step already, so this should not happen
				log.Debug("Footnote section without ID during index building, skipping")
				continue
			}
			if _, exists := index[fb.Bodies[i].Sections[j].ID]; exists {
				// Skip sections with duplicated ID
				// NOTE: These should have been removed by normalization step already, so this should not happen
				log.Debug("Duplicate footnote ID detected during index building, skipping", zap.String("id", fb.Bodies[i].Sections[j].ID))
				continue
			}
			index[fb.Bodies[i].Sections[j].ID] = FootnoteRef{
				BodyIdx:    i,
				SectionIdx: j,
			}
		}
	}

	return index
}

// buildIDIndex walks the entire FictionBook and builds an index of all elements with IDs
// Any ID could be a target for linking from other parts of the book.
func (fb *FictionBook) buildIDIndex(_ *zap.Logger) IDIndex {
	index := make(IDIndex)

	// Index binaries
	for i := range fb.Binaries {
		if fb.Binaries[i].ID != "" {
			index[fb.Binaries[i].ID] = ElementRef{
				Type: "binary",
				Path: []any{&fb.Binaries[i]},
			}
		}
	}

	// Index title-info annotation
	if fb.Description.TitleInfo.Annotation != nil {
		fb.indexFlowIDs(index, fb.Description.TitleInfo.Annotation, []any{&fb.Description, &fb.Description.TitleInfo, fb.Description.TitleInfo.Annotation})
	}

	// Index description history
	if fb.Description.DocumentInfo.History != nil {
		fb.indexFlowIDs(index, fb.Description.DocumentInfo.History, []any{&fb.Description, &fb.Description.DocumentInfo, fb.Description.DocumentInfo.History})
	}

	// Index authors
	for i := range fb.Description.TitleInfo.Authors {
		if fb.Description.TitleInfo.Authors[i].ID != "" {
			index[fb.Description.TitleInfo.Authors[i].ID] = ElementRef{
				Type: "author",
				Path: []any{&fb.Description, &fb.Description.TitleInfo, &fb.Description.TitleInfo.Authors[i]},
			}
		}
	}
	for i := range fb.Description.DocumentInfo.Authors {
		if fb.Description.DocumentInfo.Authors[i].ID != "" {
			index[fb.Description.DocumentInfo.Authors[i].ID] = ElementRef{
				Type: "author",
				Path: []any{&fb.Description, &fb.Description.DocumentInfo, &fb.Description.DocumentInfo.Authors[i]},
			}
		}
	}

	// Index bodies
	for i := range fb.Bodies {
		bodyPath := []any{&fb.Bodies[i]}

		// Index epigraphs
		for j := range fb.Bodies[i].Epigraphs {
			epiPath := append(append([]any{}, bodyPath...), &fb.Bodies[i].Epigraphs[j])
			fb.indexFlowIDs(index, &fb.Bodies[i].Epigraphs[j].Flow, append(epiPath, &fb.Bodies[i].Epigraphs[j].Flow))
		}

		// Index image
		if fb.Bodies[i].Image != nil && fb.Bodies[i].Image.ID != "" {
			index[fb.Bodies[i].Image.ID] = ElementRef{
				Type: "image",
				Path: append(append([]any{}, bodyPath...), fb.Bodies[i].Image),
			}
		}

		// Index sections
		for j := range fb.Bodies[i].Sections {
			fb.indexSectionIDs(index, &fb.Bodies[i].Sections[j], append(append([]any{}, bodyPath...), &fb.Bodies[i].Sections[j]))
		}
	}

	return index
}

// indexHref processes an href and adds it to the index with the appropriate type
func indexHref(index ReverseLinkIndex, href, linkType string, path []any, log *zap.Logger) {
	if targetID, ok := strings.CutPrefix(href, "#"); ok {
		// Internal link
		index[targetID] = append(index[targetID], ElementRef{
			Type: linkType,
			Path: path,
		})
	} else if href == "" {
		// Empty href - collect under special key
		index["links/empty_href"] = append(index["links/empty_href"], ElementRef{
			Type: "empty-href-link",
			Path: path,
		})
	} else {
		// External link
		if _, err := url.Parse(href); err != nil {
			log.Warn("Invalid external link", zap.String("href", href), zap.Error(err))
			// Broken link - collect under the actual name, so it could be reported later
			index[targetID] = append(index[targetID], ElementRef{
				Type: "broken-link",
				Path: path,
			})

		} else {
			index[href] = append(index[href], ElementRef{
				Type: "external-link",
				Path: path,
			})
		}
	}
}

// buildReverseLinkIndex walks the entire FictionBook and builds an index of all links
func (fb *FictionBook) buildReverseLinkIndex(log *zap.Logger) ReverseLinkIndex {
	index := make(ReverseLinkIndex)

	// Index coverpage links
	for i := range fb.Description.TitleInfo.Coverpage {
		href := fb.Description.TitleInfo.Coverpage[i].Href
		indexHref(index, href, "coverpage",
			[]any{&fb.Description, &fb.Description.TitleInfo, &fb.Description.TitleInfo.Coverpage[i]},
			log)
	}

	// Index title-info annotation
	if fb.Description.TitleInfo.Annotation != nil {
		fb.indexFlowLinks(index, fb.Description.TitleInfo.Annotation, []any{&fb.Description, &fb.Description.TitleInfo, fb.Description.TitleInfo.Annotation}, log)
	}

	// Index description history
	if fb.Description.DocumentInfo.History != nil {
		fb.indexFlowLinks(index, fb.Description.DocumentInfo.History, []any{&fb.Description, &fb.Description.DocumentInfo, fb.Description.DocumentInfo.History}, log)
	}

	// Index bodies
	for i := range fb.Bodies {
		bodyPath := []any{&fb.Bodies[i]}

		// Index epigraphs
		for j := range fb.Bodies[i].Epigraphs {
			epiPath := append(append([]any{}, bodyPath...), &fb.Bodies[i].Epigraphs[j])
			fb.indexFlowLinks(index, &fb.Bodies[i].Epigraphs[j].Flow, append(epiPath, &fb.Bodies[i].Epigraphs[j].Flow), log)
		}

		// Index sections
		for j := range fb.Bodies[i].Sections {
			fb.indexSectionLinks(index, &fb.Bodies[i].Sections[j], append(append([]any{}, bodyPath...), &fb.Bodies[i].Sections[j]), log)
		}
	}

	return index
}

func (fb *FictionBook) indexSectionIDs(index IDIndex, s *Section, path []any) {
	if s.ID != "" {
		index[s.ID] = ElementRef{Type: "section", Path: path}
	}

	// Title paragraphs don't contain IDs themselves, only references to IDs
	// So we don't need to index them for IDs

	for i := range s.Epigraphs {
		epiPath := append(append([]any{}, path...), &s.Epigraphs[i])
		fb.indexFlowIDs(index, &s.Epigraphs[i].Flow, append(epiPath, &s.Epigraphs[i].Flow))
	}

	if s.Image != nil && s.Image.ID != "" {
		index[s.Image.ID] = ElementRef{Type: "image", Path: append(append([]any{}, path...), s.Image)}
	}

	if s.Annotation != nil {
		fb.indexFlowIDs(index, s.Annotation, append(append([]any{}, path...), s.Annotation))
	}

	for i := range s.Content {
		fb.indexFlowItemIDs(index, &s.Content[i], append(append([]any{}, path...), &s.Content[i]))
	}
}

func (fb *FictionBook) indexFlowIDs(index IDIndex, flow *Flow, path []any) {
	if flow.ID != "" {
		index[flow.ID] = ElementRef{Type: "flow", Path: path}
	}
	for i := range flow.Items {
		fb.indexFlowItemIDs(index, &flow.Items[i], append(append([]any{}, path...), &flow.Items[i]))
	}
}

func (fb *FictionBook) indexFlowItemIDs(index IDIndex, item *FlowItem, path []any) {
	switch item.Kind {
	case FlowParagraph:
		if item.Paragraph != nil && item.Paragraph.ID != "" {
			index[item.Paragraph.ID] = ElementRef{Type: "paragraph", Path: append(append([]any{}, path...), item.Paragraph)}
		}
	case FlowSubtitle:
		if item.Subtitle != nil && item.Subtitle.ID != "" {
			index[item.Subtitle.ID] = ElementRef{Type: "subtitle", Path: append(append([]any{}, path...), item.Subtitle)}
		}
	case FlowPoem:
		if item.Poem != nil {
			poemPath := append(append([]any{}, path...), item.Poem)
			if item.Poem.ID != "" {
				index[item.Poem.ID] = ElementRef{Type: "poem", Path: poemPath}
			}
			for i := range item.Poem.Epigraphs {
				epiPath := append(append([]any{}, poemPath...), &item.Poem.Epigraphs[i])
				fb.indexFlowIDs(index, &item.Poem.Epigraphs[i].Flow, append(epiPath, &item.Poem.Epigraphs[i].Flow))
			}
		}
	case FlowCite:
		if item.Cite != nil {
			citePath := append(append([]any{}, path...), item.Cite)
			if item.Cite.ID != "" {
				index[item.Cite.ID] = ElementRef{Type: "cite", Path: citePath}
			}
			for i := range item.Cite.Items {
				fb.indexFlowItemIDs(index, &item.Cite.Items[i], append(append([]any{}, citePath...), &item.Cite.Items[i]))
			}
		}
	case FlowTable:
		if item.Table != nil {
			tablePath := append(append([]any{}, path...), item.Table)
			if item.Table.ID != "" {
				index[item.Table.ID] = ElementRef{Type: "table", Path: tablePath}
			}
			// Index cell IDs
			for i := range item.Table.Rows {
				for j := range item.Table.Rows[i].Cells {
					if item.Table.Rows[i].Cells[j].ID != "" {
						cellPath := append(append([]any{}, tablePath...), &item.Table.Rows[i], &item.Table.Rows[i].Cells[j])
						index[item.Table.Rows[i].Cells[j].ID] = ElementRef{Type: "table-cell", Path: cellPath}
					}
				}
			}
		}
	case FlowImage:
		if item.Image != nil && item.Image.ID != "" {
			index[item.Image.ID] = ElementRef{Type: "image", Path: append(append([]any{}, path...), item.Image)}
		}
	case FlowSection:
		if item.Section != nil {
			fb.indexSectionIDs(index, item.Section, append(append([]any{}, path...), item.Section))
		}
	}
}

func (fb *FictionBook) indexSectionLinks(index ReverseLinkIndex, s *Section, path []any, log *zap.Logger) {
	// Index title paragraphs
	if s.Title != nil {
		titlePath := append(append([]any{}, path...), s.Title)
		for i := range s.Title.Items {
			if s.Title.Items[i].Paragraph != nil {
				fb.indexInlineLinks(index, s.Title.Items[i].Paragraph.Text, append(append([]any{}, titlePath...), &s.Title.Items[i], s.Title.Items[i].Paragraph), log)
			}
		}
	}

	for i := range s.Epigraphs {
		epiPath := append(append([]any{}, path...), &s.Epigraphs[i])
		fb.indexFlowLinks(index, &s.Epigraphs[i].Flow, append(epiPath, &s.Epigraphs[i].Flow), log)
	}

	if s.Annotation != nil {
		fb.indexFlowLinks(index, s.Annotation, append(append([]any{}, path...), s.Annotation), log)
	}

	for i := range s.Content {
		fb.indexFlowItemLinks(index, &s.Content[i], append(append([]any{}, path...), &s.Content[i]), log)
	}
}

func (fb *FictionBook) indexFlowLinks(index ReverseLinkIndex, flow *Flow, path []any, log *zap.Logger) {
	for i := range flow.Items {
		fb.indexFlowItemLinks(index, &flow.Items[i], append(append([]any{}, path...), &flow.Items[i]), log)
	}
}

func (fb *FictionBook) indexFlowItemLinks(index ReverseLinkIndex, item *FlowItem, path []any, log *zap.Logger) {
	switch item.Kind {
	case FlowParagraph:
		if item.Paragraph != nil {
			fb.indexInlineLinks(index, item.Paragraph.Text, append(append([]any{}, path...), item.Paragraph), log)
		}
	case FlowSubtitle:
		if item.Subtitle != nil {
			fb.indexInlineLinks(index, item.Subtitle.Text, append(append([]any{}, path...), item.Subtitle), log)
		}
	case FlowPoem:
		if item.Poem != nil {
			poemPath := append(append([]any{}, path...), item.Poem)
			for i := range item.Poem.Epigraphs {
				epiPath := append(append([]any{}, poemPath...), &item.Poem.Epigraphs[i])
				fb.indexFlowLinks(index, &item.Poem.Epigraphs[i].Flow, append(epiPath, &item.Poem.Epigraphs[i].Flow), log)
			}
			for i := range item.Poem.Subtitles {
				fb.indexInlineLinks(index, item.Poem.Subtitles[i].Text, append(append([]any{}, poemPath...), &item.Poem.Subtitles[i]), log)
			}
			for i := range item.Poem.Stanzas {
				stanzaPath := append(append([]any{}, poemPath...), &item.Poem.Stanzas[i])
				for j := range item.Poem.Stanzas[i].Verses {
					fb.indexInlineLinks(index, item.Poem.Stanzas[i].Verses[j].Text, append(stanzaPath, &item.Poem.Stanzas[i].Verses[j]), log)
				}
			}
		}
	case FlowCite:
		if item.Cite != nil {
			citePath := append(append([]any{}, path...), item.Cite)
			for i := range item.Cite.Items {
				fb.indexFlowItemLinks(index, &item.Cite.Items[i], append(append([]any{}, citePath...), &item.Cite.Items[i]), log)
			}
			for i := range item.Cite.TextAuthors {
				fb.indexInlineLinks(index, item.Cite.TextAuthors[i].Text, append(append([]any{}, citePath...), &item.Cite.TextAuthors[i]), log)
			}
		}
	case FlowTable:
		if item.Table != nil {
			tablePath := append(append([]any{}, path...), item.Table)
			for i := range item.Table.Rows {
				rowPath := append(append([]any{}, tablePath...), &item.Table.Rows[i])
				for j := range item.Table.Rows[i].Cells {
					cellPath := append(append([]any{}, rowPath...), &item.Table.Rows[i].Cells[j])
					fb.indexInlineLinks(index, item.Table.Rows[i].Cells[j].Content, cellPath, log)
				}
			}
		}
	case FlowImage:
		if item.Image != nil {
			href := item.Image.Href
			indexHref(index, href, "block-image",
				append(append([]any{}, path...), item.Image),
				log)
		}
	case FlowSection:
		if item.Section != nil {
			fb.indexSectionLinks(index, item.Section, append(append([]any{}, path...), item.Section), log)
		}
	}
}

func (fb *FictionBook) indexInlineLinks(index ReverseLinkIndex, segments []InlineSegment, path []any, log *zap.Logger) {
	for i := range segments {
		segPath := append(append([]any{}, path...), &segments[i])
		if segments[i].Kind == InlineLink {
			href := segments[i].Href
			indexHref(index, href, "inline-link", segPath, log)
		} else if segments[i].Kind == InlineImageSegment && segments[i].Image != nil {
			href := segments[i].Image.Href
			indexHref(index, href, "inline-image", segPath, log)
		}
		if len(segments[i].Children) > 0 {
			fb.indexInlineLinks(index, segments[i].Children, segPath, log)
		}
	}
}
