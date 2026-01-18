package kfx

import (
	"strings"

	"fbc/fb2"
)

func collectUsedImageIDs(book *fb2.FictionBook) map[string]bool {
	used := make(map[string]bool)

	// Cover
	if len(book.Description.TitleInfo.Coverpage) > 0 {
		id := strings.TrimPrefix(book.Description.TitleInfo.Coverpage[0].Href, "#")
		if id != "" {
			used[id] = true
		}
	}

	// Vignettes
	for _, id := range book.VignetteIDs {
		if id != "" {
			used[id] = true
		}
	}

	for i := range book.Bodies {
		body := &book.Bodies[i]
		if body.Footnotes() {
			continue
		}

		if body.Image != nil {
			id := strings.TrimPrefix(body.Image.Href, "#")
			if id != "" {
				used[id] = true
			}
		}

		if body.Title != nil {
			for _, it := range body.Title.Items {
				collectInlineImageIDsParagraph(it.Paragraph, used)
			}
		}

		for _, epi := range body.Epigraphs {
			collectImageIDsFlow(&epi.Flow, used)
			for i := range epi.TextAuthors {
				collectInlineImageIDsParagraph(&epi.TextAuthors[i], used)
			}
		}

		for si := range body.Sections {
			collectImageIDsSection(&body.Sections[si], used)
		}
	}

	return used
}

func collectImageIDsSection(section *fb2.Section, used map[string]bool) {
	if section.Image != nil {
		id := strings.TrimPrefix(section.Image.Href, "#")
		if id != "" {
			used[id] = true
		}
	}

	if section.Title != nil {
		for _, it := range section.Title.Items {
			collectInlineImageIDsParagraph(it.Paragraph, used)
		}
	}

	for _, epi := range section.Epigraphs {
		collectImageIDsFlow(&epi.Flow, used)
		for i := range epi.TextAuthors {
			collectInlineImageIDsParagraph(&epi.TextAuthors[i], used)
		}
	}

	collectImageIDsFlow(section.Annotation, used)

	for _, item := range section.Content {
		if item.Kind == fb2.FlowSection && item.Section != nil {
			collectImageIDsSection(item.Section, used)
			continue
		}
		collectImageIDsFlowItem(&item, used)
	}
}

func collectImageIDsFlow(flow *fb2.Flow, used map[string]bool) {
	if flow == nil {
		return
	}
	for i := range flow.Items {
		collectImageIDsFlowItem(&flow.Items[i], used)
	}
}

func collectImageIDsFlowItem(item *fb2.FlowItem, used map[string]bool) {
	if item == nil {
		return
	}

	switch item.Kind {
	case fb2.FlowImage:
		if item.Image != nil {
			id := strings.TrimPrefix(item.Image.Href, "#")
			if id != "" {
				used[id] = true
			}
		}
	case fb2.FlowParagraph:
		collectInlineImageIDsParagraph(item.Paragraph, used)
	case fb2.FlowSubtitle:
		collectInlineImageIDsParagraph(item.Subtitle, used)
	case fb2.FlowPoem:
		if item.Poem != nil {
			collectInlineImageIDsTitle(item.Poem.Title, used)
			for si := range item.Poem.Stanzas {
				st := &item.Poem.Stanzas[si]
				collectInlineImageIDsTitle(st.Title, used)
				for i := range st.Verses {
					collectInlineImageIDsParagraph(&st.Verses[i], used)
				}
			}
			for i := range item.Poem.TextAuthors {
				collectInlineImageIDsParagraph(&item.Poem.TextAuthors[i], used)
			}
		}
	case fb2.FlowCite:
		if item.Cite != nil {
			for i := range item.Cite.Items {
				collectImageIDsFlowItem(&item.Cite.Items[i], used)
			}
			for i := range item.Cite.TextAuthors {
				collectInlineImageIDsParagraph(&item.Cite.TextAuthors[i], used)
			}
		}
	case fb2.FlowTable:
		if item.Table != nil {
			for _, row := range item.Table.Rows {
				for _, cell := range row.Cells {
					for i := range cell.Content {
						collectInlineImageIDsSegment(&cell.Content[i], used)
					}
				}
			}
		}
	}
}

func collectInlineImageIDsTitle(t *fb2.Title, used map[string]bool) {
	if t == nil {
		return
	}
	for _, it := range t.Items {
		collectInlineImageIDsParagraph(it.Paragraph, used)
	}
}

func collectInlineImageIDsParagraph(p *fb2.Paragraph, used map[string]bool) {
	if p == nil {
		return
	}
	for i := range p.Text {
		collectInlineImageIDsSegment(&p.Text[i], used)
	}
}

func collectInlineImageIDsSegment(seg *fb2.InlineSegment, used map[string]bool) {
	if seg == nil {
		return
	}
	if seg.Kind == fb2.InlineImageSegment && seg.Image != nil {
		id := strings.TrimPrefix(seg.Image.Href, "#")
		if id != "" {
			used[id] = true
		}
	}
	for i := range seg.Children {
		collectInlineImageIDsSegment(&seg.Children[i], used)
	}
}
