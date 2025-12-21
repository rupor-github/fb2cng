package kfx

import (
	"strings"

	"fbc/content"
	"fbc/fb2"
)

type textNode struct {
	EID        int64
	BlockIndex int64
}

type linearText struct {
	Blocks []string
	Nodes  []textNode
}

func linearizeMainText(c *content.Content, eidBase int64) linearText {
	if c == nil || c.Book == nil {
		return linearText{}
	}

	var out linearText
	blocks := make([]string, 0, 128)
	nodes := make([]textNode, 0, 128)

	addParagraph := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		idx := int64(len(blocks))
		blocks = append(blocks, s)
		nodes = append(nodes, textNode{EID: eidBase + idx, BlockIndex: idx})
	}

	// Put book title first to match common Kindle layout expectations.
	addParagraph(c.Book.Description.TitleInfo.BookTitle.Value)

	for bi := range c.Book.Bodies {
		body := &c.Book.Bodies[bi]
		if !body.Main() {
			continue
		}
		for si := range body.Sections {
			addSectionText(&body.Sections[si], addParagraph)
		}
	}

	out.Blocks = blocks
	out.Nodes = nodes
	return out
}

func addSectionText(s *fb2.Section, addParagraph func(string)) {
	if s == nil {
		return
	}

	if s.Title != nil {
		addParagraph(s.Title.AsTOCText(""))
	}

	for i := range s.Content {
		item := &s.Content[i]
		switch item.Kind {
		case fb2.FlowParagraph:
			if item.Paragraph != nil {
				addParagraph(item.Paragraph.AsPlainText())
			}
		case fb2.FlowSubtitle:
			if item.Subtitle != nil {
				addParagraph(item.Subtitle.AsPlainText())
			}
		case fb2.FlowSection:
			if item.Section != nil {
				addSectionText(item.Section, addParagraph)
			}
		default:
			// TODO: poems, cites, tables, images.
			if t := item.AsPlainText(); t != "" {
				addParagraph(t)
			}
		}
	}
}
