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

// imageNode tracks an image placement in the document.
type imageNode struct {
	EID     int64  // EID for this image block
	ImageID string // FB2 image ID (href without #)
}

type linearText struct {
	Blocks []string
	Nodes  []textNode
	Images []imageNode // Images in document order
}

type tocNode struct {
	Title    string
	EID      int64
	Children []*tocNode
}

type topSectionSpan struct {
	Title      string
	AnchorEID  int64
	StartBlock int
	EndBlock   int
}

func linearizeMainTextWithTOC(c *content.Content, eidBase int64) (linearText, []*tocNode, []topSectionSpan) {
	if c == nil || c.Book == nil {
		return linearText{}, nil, nil
	}

	var out linearText
	blocks := make([]string, 0, 128)
	nodes := make([]textNode, 0, 128)
	images := make([]imageNode, 0, 64)
	toc := make([]*tocNode, 0, 64)
	spans := make([]topSectionSpan, 0, 64)

	// EID counter for images (separate from text blocks).
	// Use a high base to avoid collision with text block EIDs.
	imageEIDBase := int64(100000)
	nextImageEID := imageEIDBase

	addParagraph := func(s string) int64 {
		s = strings.TrimSpace(s)
		if s == "" {
			return -1
		}
		idx := int64(len(blocks))
		blocks = append(blocks, s)
		eid := eidBase + idx
		nodes = append(nodes, textNode{EID: eid, BlockIndex: idx})
		return eid
	}

	addImage := func(href string) {
		if href == "" {
			return
		}
		// Strip leading # from href if present.
		imageID := strings.TrimPrefix(href, "#")
		if imageID == "" {
			return
		}
		eid := nextImageEID
		nextImageEID++
		images = append(images, imageNode{EID: eid, ImageID: imageID})
	}

	// Put book title first to match common Kindle layout expectations.
	addParagraph(c.Book.Description.TitleInfo.BookTitle.Value)

	var buildTOC func(s *fb2.Section) *tocNode
	buildTOC = func(s *fb2.Section) *tocNode {
		if s == nil {
			return nil
		}

		title := ""
		if s.Title != nil {
			title = s.Title.AsTOCText("")
		}

		anchorEID := int64(-1)
		if title != "" {
			anchorEID = addParagraph(title)
		}

		kids := make([]*tocNode, 0, 8)
		for i := range s.Content {
			item := &s.Content[i]
			if item.Kind == fb2.FlowSection && item.Section != nil {
				if item.Section.Title != nil {
					if child := buildTOC(item.Section); child != nil {
						kids = append(kids, child)
					}
				} else {
					addSectionText(item.Section, addParagraph, addImage)
				}
				continue
			}

			switch item.Kind {
			case fb2.FlowParagraph:
				if item.Paragraph != nil {
					addParagraph(item.Paragraph.AsPlainText())
				}
			case fb2.FlowSubtitle:
				if item.Subtitle != nil {
					addParagraph(item.Subtitle.AsPlainText())
				}
			case fb2.FlowImage:
				if item.Image != nil {
					addImage(item.Image.Href)
				}
			default:
				if t := item.AsPlainText(); t != "" {
					addParagraph(t)
				}
			}
		}

		if title == "" {
			return nil
		}
		return &tocNode{Title: title, EID: anchorEID, Children: kids}
	}

	for bi := range c.Book.Bodies {
		body := &c.Book.Bodies[bi]
		if !body.Main() {
			continue
		}
		for si := range body.Sections {
			s := &body.Sections[si]
			if s.Title == nil {
				addSectionText(s, addParagraph, addImage)
				continue
			}

			start := len(blocks)
			n := buildTOC(s)
			end := len(blocks)

			if n != nil {
				toc = append(toc, n)
				spans = append(spans, topSectionSpan{Title: n.Title, AnchorEID: n.EID, StartBlock: start, EndBlock: end})
			}
		}
	}

	out.Blocks = blocks
	out.Nodes = nodes
	out.Images = images
	return out, toc, spans
}

func addSectionText(s *fb2.Section, addParagraph func(string) int64, addImage func(string)) {
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
		case fb2.FlowImage:
			if item.Image != nil {
				addImage(item.Image.Href)
			}
		case fb2.FlowSection:
			if item.Section != nil {
				addSectionText(item.Section, addParagraph, addImage)
			}
		default:
			// TODO: poems, cites, tables.
			if t := item.AsPlainText(); t != "" {
				addParagraph(t)
			}
		}
	}
}
