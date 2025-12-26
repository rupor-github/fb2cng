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
				collectInlineImageIDsFromParagraph(it.Paragraph, used)
			}
		}

		for _, epi := range body.Epigraphs {
			collectImageIDsFromFlow(&epi.Flow, used)
			for i := range epi.TextAuthors {
				collectInlineImageIDsFromParagraph(&epi.TextAuthors[i], used)
			}
		}

		for si := range body.Sections {
			collectImageIDsFromSection(&body.Sections[si], used)
		}
	}

	return used
}

func collectImageIDsFromSection(section *fb2.Section, used map[string]bool) {
	if section.Image != nil {
		id := strings.TrimPrefix(section.Image.Href, "#")
		if id != "" {
			used[id] = true
		}
	}

	if section.Title != nil {
		for _, it := range section.Title.Items {
			collectInlineImageIDsFromParagraph(it.Paragraph, used)
		}
	}

	for _, epi := range section.Epigraphs {
		collectImageIDsFromFlow(&epi.Flow, used)
		for i := range epi.TextAuthors {
			collectInlineImageIDsFromParagraph(&epi.TextAuthors[i], used)
		}
	}

	collectImageIDsFromFlow(section.Annotation, used)

	for _, item := range section.Content {
		if item.Kind == fb2.FlowSection && item.Section != nil {
			collectImageIDsFromSection(item.Section, used)
			continue
		}
		collectImageIDsFromFlowItem(&item, used)
	}
}

func collectImageIDsFromFlow(flow *fb2.Flow, used map[string]bool) {
	if flow == nil {
		return
	}
	for i := range flow.Items {
		collectImageIDsFromFlowItem(&flow.Items[i], used)
	}
}

func collectImageIDsFromFlowItem(item *fb2.FlowItem, used map[string]bool) {
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
		collectInlineImageIDsFromParagraph(item.Paragraph, used)
	case fb2.FlowSubtitle:
		collectInlineImageIDsFromParagraph(item.Subtitle, used)
	case fb2.FlowPoem:
		if item.Poem != nil {
			collectInlineImageIDsFromTitle(item.Poem.Title, used)
			for si := range item.Poem.Stanzas {
				st := &item.Poem.Stanzas[si]
				collectInlineImageIDsFromTitle(st.Title, used)
				for i := range st.Verses {
					collectInlineImageIDsFromParagraph(&st.Verses[i], used)
				}
			}
			for i := range item.Poem.TextAuthors {
				collectInlineImageIDsFromParagraph(&item.Poem.TextAuthors[i], used)
			}
		}
	case fb2.FlowCite:
		if item.Cite != nil {
			for i := range item.Cite.Items {
				collectImageIDsFromFlowItem(&item.Cite.Items[i], used)
			}
			for i := range item.Cite.TextAuthors {
				collectInlineImageIDsFromParagraph(&item.Cite.TextAuthors[i], used)
			}
		}
	}
}

func collectInlineImageIDsFromTitle(t *fb2.Title, used map[string]bool) {
	if t == nil {
		return
	}
	for _, it := range t.Items {
		collectInlineImageIDsFromParagraph(it.Paragraph, used)
	}
}

func collectInlineImageIDsFromParagraph(p *fb2.Paragraph, used map[string]bool) {
	if p == nil {
		return
	}
	for i := range p.Text {
		collectInlineImageIDsFromSegment(&p.Text[i], used)
	}
}

func collectInlineImageIDsFromSegment(seg *fb2.InlineSegment, used map[string]bool) {
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
		collectInlineImageIDsFromSegment(&seg.Children[i], used)
	}
}
