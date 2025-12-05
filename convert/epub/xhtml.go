package epub

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/beevik/etree"
	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
	"fbc/content/text"
	"fbc/fb2"
)

type chapterData struct {
	ID           string
	Filename     string
	Title        string
	Doc          *etree.Document
	Section      *fb2.Section // Reference to source section for TOC hierarchy
	IncludeInTOC bool         // Whether to include this chapter in navigation/TOC
}

const backlinkSym = "<<" // String for additional link backs in the footnote bodies

// idToFileMap maps element IDs to the chapter filename containing them
type idToFileMap map[string]string

func convertToXHTML(ctx context.Context, c *content.Content, log *zap.Logger) ([]chapterData, idToFileMap, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	var chapters []chapterData
	chapterNum := 0
	idToFile := make(idToFileMap)
	var footnoteBodies []*fb2.Body

	for i := range c.Book.Bodies {
		body := &c.Book.Bodies[i]
		if body.Footnotes() {
			// Collect footnote bodies for later processing
			footnoteBodies = append(footnoteBodies, body)
			continue
		}

		// Process main and other bodies (not footnotes)
		// If body has title, create a chapter for body intro content
		if body.Title != nil {
			chapterNum++
			chapterID := fmt.Sprintf("index%05d", chapterNum)
			filename := fmt.Sprintf("%s.xhtml", chapterID)

			var title string
			for _, item := range body.Title.Items {
				if item.Paragraph != nil {
					title = item.Paragraph.AsPlainText()
					break
				}
			}
			if title == "" {
				title = "Untitled"
			}

			// Set current filename for footnote reference tracking
			c.CurrentFilename = filename

			doc, err := bodyIntroToXHTML(c, body, title, log)
			if err != nil {
				log.Error("Unable to convert body intro", zap.Error(err))
			} else {
				chapters = append(chapters, chapterData{
					ID:           chapterID,
					Filename:     filename,
					Title:        title,
					Doc:          doc,
					IncludeInTOC: true,
				})
				collectIDsFromBody(body, filename, idToFile)
			}
		}

		// Process top-level sections as chapters
		for i := range body.Sections {
			section := &body.Sections[i]
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}

			chapterNum++
			chapterID := fmt.Sprintf("index%05d", chapterNum)
			filename := fmt.Sprintf("%s.xhtml", chapterID)
			title := section.AsTitleText(fmt.Sprintf("chapter-section-%d", chapterNum))

			// Set current filename for footnote reference tracking
			c.CurrentFilename = filename

			doc, err := sectionToXHTML(c, section, title, log)
			if err != nil {
				log.Error("Unable to convert section", zap.Error(err))
				continue
			}

			chapters = append(chapters, chapterData{
				ID:           chapterID,
				Filename:     filename,
				Title:        title,
				Doc:          doc,
				Section:      section,
				IncludeInTOC: true,
			})
			collectIDsFromSection(section, filename, idToFile)
		}
	}

	if len(footnoteBodies) == 0 {
		return chapters, idToFile, nil
	}

	// Process all footnote bodies - each body becomes a separate top-level chapter
	chapterNum++
	footnotesChapters, err := processFootnoteBodies(c, footnoteBodies, chapters, idToFile, log)
	if err != nil {
		log.Error("Unable to convert footnotes", zap.Error(err))
		return chapters, idToFile, nil
	}

	chapters = append(chapters, footnotesChapters...)
	return chapters, idToFile, nil
}

// processFootnoteBodies converts all footnote bodies to XHTML and creates chapter entries
func processFootnoteBodies(c *content.Content, footnoteBodies []*fb2.Body, existingChapters []chapterData, idToFile idToFileMap, log *zap.Logger) ([]chapterData, error) {
	// Build map of existing IDs to check for collisions
	existingIDs := make(map[string]bool, len(existingChapters))
	for _, ch := range existingChapters {
		existingIDs[ch.ID] = true
	}

	// Find a unique ID and filename that doesn't collide with existing chapters
	baseID := "footnotes"
	chapterID := baseID
	filename := baseID + ".xhtml"

	counter := 0
	for existingIDs[chapterID] {
		counter++
		chapterID = fmt.Sprintf("%s-%d", baseID, counter)
		filename = chapterID + ".xhtml"
	}

	docTitle := footnoteBodies[0].AsTitleText("Footnotes")

	doc, root := createXHTMLDocument(c, docTitle)

	// Set current filename for footnote tracking
	c.CurrentFilename = filename

	// Process all footnote bodies - build XHTML and chapter metadata in single loop
	var chapters []chapterData
	for bodyIdx, body := range footnoteBodies {
		bodyID := generateFootnoteBodyID(body, bodyIdx)
		bodyTitle := body.AsTitleText(bodyID)

		// Create XHTML wrapper div for this body
		bodyDiv := root.CreateElement("div")
		bodyDiv.CreateAttr("class", "footnote-body")
		bodyDiv.CreateAttr("id", bodyID)

		// Write body intro content if it has a title
		if body.Title != nil {
			if err := appendBodyIntroContent(bodyDiv, c, body, 1, log); err != nil {
				return nil, err
			}
		}

		// Write all sections from this body and collect IDs
		for i := range body.Sections {
			section := &body.Sections[i]
			collectIDsFromSection(section, filename, idToFile)

			// Choose appropriate element type based on mode and format
			if c.FootnotesMode == config.FootnotesModeFloat && c.OutputFormat == config.OutputFmtEpub3 {
				if err := appendEpub3FloatFootnoteSectionContent(bodyDiv, c, section, log); err != nil {
					return nil, err
				}
			} else if c.FootnotesMode == config.FootnotesModeFloat && (c.OutputFormat == config.OutputFmtEpub2 || c.OutputFormat == config.OutputFmtKepub) {
				if err := appendEpub2FloatFootnoteSectionContent(bodyDiv, c, section, log); err != nil {
					return nil, err
				}
			} else {
				if err := appendDefaultFootnoteSectionContent(bodyDiv, c, section, log); err != nil {
					return nil, err
				}
			}
		}

		// Create chapter entry for TOC - all bodies use anchor reference
		// Find unique ID that doesn't collide
		var baseChapterID string
		if bodyIdx == 0 {
			baseChapterID = "footnotes"
		} else {
			baseChapterID = fmt.Sprintf("footnotes-%d", bodyIdx)
		}
		tocChapterID := baseChapterID
		counter := 0
		for existingIDs[tocChapterID] {
			counter++
			tocChapterID = fmt.Sprintf("%s-dup%d", baseChapterID, counter)
		}
		existingIDs[tocChapterID] = true
		var chapterDoc *etree.Document
		if bodyIdx == 0 {
			// First chapter owns the document for writing to file
			chapterDoc = doc
		}

		chapters = append(chapters, chapterData{
			ID:           tocChapterID,
			Filename:     filename + "#" + bodyID,
			Title:        bodyTitle,
			Doc:          chapterDoc,
			IncludeInTOC: true,
		})
	}

	return chapters, nil
}

// generateFootnoteBodyID generates a unique ID for a footnote body
func generateFootnoteBodyID(body *fb2.Body, index int) string {
	if body.Name != "" {
		return fmt.Sprintf("%s-%d", body.Name, index)
	}
	return fmt.Sprintf("footnote-body-%d", index)
}

// createXHTMLDocument creates a standard XHTML document structure with head elements
func createXHTMLDocument(c *content.Content, title string) (*etree.Document, *etree.Element) {
	c.KoboSpanSet(0, 0)

	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)

	html := doc.CreateElement("html")
	html.CreateAttr("xmlns", "http://www.w3.org/1999/xhtml")
	html.CreateAttr("xmlns:epub", "http://www.idpf.org/2007/ops")

	head := html.CreateElement("head")

	meta := head.CreateElement("meta")
	meta.CreateAttr("http-equiv", "Content-Type")
	meta.CreateAttr("content", "text/html; charset=utf-8")

	link := head.CreateElement("link")
	link.CreateAttr("rel", "stylesheet")
	link.CreateAttr("type", "text/css")
	link.CreateAttr("href", "stylesheet.css")

	titleElem := head.CreateElement("title")
	titleElem.SetText(title)

	body := html.CreateElement("body")

	var root *etree.Element
	if c.OutputFormat == config.OutputFmtKepub {
		bookColumnsDiv := body.CreateElement("div")
		bookColumnsDiv.CreateAttr("id", "book-columns")
		inner := bookColumnsDiv.CreateElement("div")
		inner.CreateAttr("id", "book-inner")
		root = inner
	} else {
		root = body
	}

	return doc, root
}

// appendTitleAsHeading appends a title as a heading element (h1-h6) with span children for each paragraph
// Used for body and section titles that need semantic heading markup
func appendTitleAsHeading(parent *etree.Element, c *content.Content, title *fb2.Title, depth int, classPrefix string) {
	c.KoboSpanNextParagraph()

	headingLevel := min(depth, 6)
	headingTag := fmt.Sprintf("h%d", headingLevel)

	titleElem := parent.CreateElement(headingTag)
	titleElem.CreateAttr("class", classPrefix)
	firstParagraph := true
	prevWasEmptyLine := false

	for i, item := range title.Items {
		if item.Paragraph != nil {
			// Add <br> before non-first paragraphs to separate them, but not if previous was empty line
			if i > 0 && !prevWasEmptyLine {
				br := titleElem.CreateElement("br")
				br.CreateAttr("class", classPrefix+"-paragraph")
			}
			span := titleElem.CreateElement("span")
			if item.Paragraph.ID != "" {
				span.CreateAttr("id", item.Paragraph.ID)
			}
			var class string
			if firstParagraph {
				class = classPrefix + "-first"
				firstParagraph = false
			} else {
				class = classPrefix + "-next"
			}
			if item.Paragraph.Style != "" {
				class = class + " " + item.Paragraph.Style
			}
			span.CreateAttr("class", class)
			appendParagraphInline(span, c, item.Paragraph)
			prevWasEmptyLine = false
		} else if item.EmptyLine {
			br := titleElem.CreateElement("br")
			br.CreateAttr("class", classPrefix+"-emptyline")
			prevWasEmptyLine = true
		}
	}
}

// appendTitleAsDiv appends a title as a div container with p elements for each paragraph
// Used for poem, stanza, and footnote titles
func appendTitleAsDiv(parent *etree.Element, c *content.Content, title *fb2.Title, classPrefix string) {
	titleDiv := parent.CreateElement("div")
	titleDiv.CreateAttr("class", classPrefix)
	firstParagraph := true

	for _, item := range title.Items {
		if item.Paragraph != nil {
			c.KoboSpanNextParagraph()
			p := titleDiv.CreateElement("p")
			if item.Paragraph.ID != "" {
				p.CreateAttr("id", item.Paragraph.ID)
			}
			var class string
			if firstParagraph {
				class = classPrefix + "-first"
				firstParagraph = false
			} else {
				class = classPrefix + "-next"
			}
			if item.Paragraph.Style != "" {
				class = class + " " + item.Paragraph.Style
			}
			p.CreateAttr("class", class)
			appendParagraphInline(p, c, item.Paragraph)
		} else if item.EmptyLine {
			titleDiv.CreateElement("br")
		}
	}
}

// appendEpigraphs appends all epigraphs with their flow content and text authors
func appendEpigraphs(parent *etree.Element, c *content.Content, epigraphs []fb2.Epigraph, depth int, log *zap.Logger) error {
	for _, epigraph := range epigraphs {
		div := parent.CreateElement("div")
		div.CreateAttr("class", "epigraph")
		if err := appendFlowItemsWithContext(div, c, epigraph.Flow.Items, depth, "epigraph", log); err != nil {
			return err
		}
		for _, ta := range epigraph.TextAuthors {
			c.KoboSpanNextParagraph()
			p := div.CreateElement("p")
			p.CreateAttr("class", "text-author")
			appendParagraphInline(p, c, &ta)
		}
	}
	return nil
}

func bodyIntroToXHTML(c *content.Content, body *fb2.Body, title string, log *zap.Logger) (*etree.Document, error) {
	doc, root := createXHTMLDocument(c, title)

	if err := appendBodyIntroContent(root, c, body, 1, log); err != nil {
		return nil, err
	}

	return doc, nil
}

func sectionToXHTML(c *content.Content, section *fb2.Section, title string, log *zap.Logger) (*etree.Document, error) {
	doc, root := createXHTMLDocument(c, title)

	// Add section ID to root element so it can be a link target
	if section.ID != "" {
		root.CreateAttr("id", section.ID)
	}

	if err := appendSectionContent(root, c, section, false, 1, log); err != nil {
		return nil, err
	}

	return doc, nil
}

func appendBodyIntroContent(parent *etree.Element, c *content.Content, body *fb2.Body, depth int, log *zap.Logger) error {
	if body.Title != nil {
		appendTitleAsHeading(parent, c, body.Title, depth, "title")
	}

	if err := appendEpigraphs(parent, c, body.Epigraphs, depth, log); err != nil {
		return err
	}

	if body.Image != nil {
		appendImageElement(parent, c, body.Image)
	}

	return nil
}

// appendEpub2FloatFootnoteSectionContent appends footnote section content in
// EPUB2 float mode uses <p class="footnote"> and simplified rendering to fit
// everything in a single paragraph keeping as much formatting as possible.
func appendEpub2FloatFootnoteSectionContent(parent *etree.Element, c *content.Content, section *fb2.Section, _ *zap.Logger) error {
	c.KoboSpanNextParagraph()

	sectionElem := parent.CreateElement("p")
	sectionElem.CreateAttr("class", "footnote")
	if section.ID != "" {
		sectionElem.CreateAttr("id", section.ID)
	}
	if section.Lang != "" {
		sectionElem.CreateAttr("xml:lang", section.Lang)
	}

	// Add back-reference link at the beginning with title
	if section.ID != "" {
		if refs, exists := c.BackLinkIndex[section.ID]; exists && len(refs) > 0 {
			for i, ref := range refs {
				if i > 0 {
					sectionElem.CreateText(text.NBSP)
				}
				backLink := sectionElem.CreateElement("a")
				href := ref.Filename + "#" + ref.RefID
				backLink.CreateAttr("href", href)
				backLink.CreateAttr("class", "footnote-backlink")

				textParent := backLink
				if c.OutputFormat == config.OutputFmtKepub {
					paragraph, sentence := c.KoboSpanNextSentence()
					span := backLink.CreateElement("span")
					span.CreateAttr("class", "koboSpan")
					span.CreateAttr("id", fmt.Sprintf("kobo.%d.%d", paragraph, sentence))
					textParent = span
				}
				// Use title as link text if available, otherwise use ↩
				if section.Title != nil && i == 0 {
					textParent.CreateText(section.Title.AsTOCText(backlinkSym))
				} else {
					textParent.CreateText(backlinkSym)
				}
			}
			sectionElem.CreateText(text.NBSP)
		}
	}

	for _, item := range section.Content {
		switch item.Kind {
		case fb2.FlowParagraph:
			if item.Paragraph != nil {
				span := sectionElem.CreateElement("span")
				if item.Paragraph.ID != "" {
					span.CreateAttr("id", item.Paragraph.ID)
				}
				if item.Paragraph.Style != "" {
					span.CreateAttr("class", item.Paragraph.Style)
				}
				appendParagraphInline(span, c, item.Paragraph)
				sectionElem.CreateElement("br")
			}
		case fb2.FlowImage:
			if item.Image != nil {
				// Render image inline (no div wrapper)
				var imgsectionElem *etree.Element
				if c.OutputFormat == config.OutputFmtKepub {
					paragraph, sentence := c.KoboSpanNextSentence()
					span := sectionElem.CreateElement("span")
					span.CreateAttr("class", "koboSpan")
					span.CreateAttr("id", fmt.Sprintf("kobo.%d.%d", paragraph, sentence))
					imgsectionElem = span
				} else {
					imgsectionElem = sectionElem
				}
				img := imgsectionElem.CreateElement("img")
				img.CreateAttr("class", "inline-image")
				if item.Image.ID != "" {
					img.CreateAttr("id", item.Image.ID)
				}
				imgID := strings.TrimPrefix(item.Image.Href, "#")
				if imgData, ok := c.ImagesIndex[imgID]; ok {
					img.CreateAttr("src", path.Join(imagesDir, imgData.Filename))
				} else {
					img.CreateAttr("src", path.Join(imagesDir, imgID))
				}
				img.CreateAttr("alt", item.Image.Alt)
				if item.Image.Title != "" {
					img.CreateAttr("title", item.Image.Title)
				}
			}
		case fb2.FlowPoem:
			if item.Poem != nil {
				span := sectionElem.CreateElement("span")
				span.CreateAttr("class", "poem")
				if item.Poem.ID != "" {
					span.CreateAttr("id", item.Poem.ID)
				}
				span.CreateText(item.Poem.AsPlainText())
				sectionElem.CreateElement("br")
			}
		case fb2.FlowSubtitle:
			if item.Subtitle != nil {
				span := sectionElem.CreateElement("span")
				if item.Subtitle.ID != "" {
					span.CreateAttr("id", item.Subtitle.ID)
				}
				span.CreateAttr("class", "subtitle")
				appendParagraphInline(span, c, item.Subtitle)
				sectionElem.CreateElement("br")
			}
		case fb2.FlowCite:
			if item.Cite != nil {
				span := sectionElem.CreateElement("span")
				span.CreateAttr("class", "cite")
				if item.Cite.ID != "" {
					span.CreateAttr("id", item.Cite.ID)
				}
				span.CreateText(item.Cite.AsPlainText())
				sectionElem.CreateElement("br")
			}
		case fb2.FlowEmptyLine:
			sectionElem.CreateElement("br")
		case fb2.FlowTable:
			if item.Table != nil {
				span := sectionElem.CreateElement("span")
				if item.Table.ID != "" {
					span.CreateAttr("id", item.Table.ID)
				}
				span.CreateText(item.Table.AsPlainText())
				sectionElem.CreateElement("br")
			}
		}
	}

	return nil
}

// appendEpub3FloatFootnoteSectionContent appends footnote section content in EPUB3 float mode
// EPUB3 Float mode uses <aside epub:type="footnote">
func appendEpub3FloatFootnoteSectionContent(parent *etree.Element, c *content.Content, section *fb2.Section, log *zap.Logger) error {
	if section.Title != nil {
		appendTitleAsDiv(parent, c, section.Title, "footnote-title")
	}

	sectionElem := parent.CreateElement("aside")
	sectionElem.CreateAttr("epub:type", "footnote")
	if section.ID != "" {
		sectionElem.CreateAttr("id", section.ID)
	}
	if section.Lang != "" {
		sectionElem.CreateAttr("xml:lang", section.Lang)
	}

	if err := appendEpigraphs(sectionElem, c, section.Epigraphs, 1, log); err != nil {
		return err
	}

	if section.Image != nil {
		appendImageElement(sectionElem, c, section.Image)
	}

	if section.Annotation != nil {
		div := parent.CreateElement("div")
		div.CreateAttr("class", "annotation")
		if err := appendFlowItemsWithContext(div, c, section.Annotation.Items, 1, "annotation", log); err != nil {
			return err
		}
	}

	if err := appendFlowItemsWithContext(sectionElem, c, section.Content, 1, "", log); err != nil {
		return err
	}

	// Add back-references for EPUB3 float mode
	if section.ID != "" {
		if refs, exists := c.BackLinkIndex[section.ID]; exists && len(refs) > 0 {
			// Add back-reference links
			backDiv := parent.CreateElement("p")
			backDiv.CreateAttr("class", "footnote-backlink")

			for i, ref := range refs {
				if i == 0 {
					backDiv.CreateText("Back:" + text.NBSP)
				} else {
					backDiv.CreateText(text.NBSP)
				}
				backLink := backDiv.CreateElement("a")
				// Include filename in href for cross-file back-references
				href := ref.Filename + "#" + ref.RefID
				backLink.CreateAttr("href", href)
				backLink.CreateAttr("epub:type", "backlink")
				backLink.CreateText(backlinkSym)
			}
		}
	}
	return nil
}

// appendDefaultFootnoteSectionContent appends footnote section content in
// default mode using <div class="footnote">
func appendDefaultFootnoteSectionContent(parent *etree.Element, c *content.Content, section *fb2.Section, log *zap.Logger) error {
	sectionElem := parent.CreateElement("div")
	sectionElem.CreateAttr("class", "footnote")
	if section.ID != "" {
		sectionElem.CreateAttr("id", section.ID)
	}
	if section.Lang != "" {
		sectionElem.CreateAttr("xml:lang", section.Lang)
	}

	if section.Title != nil {
		appendTitleAsDiv(sectionElem, c, section.Title, "footnote-title")
	}

	if err := appendEpigraphs(sectionElem, c, section.Epigraphs, 1, log); err != nil {
		return err
	}

	if section.Image != nil {
		appendImageElement(sectionElem, c, section.Image)
	}

	if section.Annotation != nil {
		div := sectionElem.CreateElement("div")
		div.CreateAttr("class", "annotation")
		if err := appendFlowItemsWithContext(div, c, section.Annotation.Items, 1, "annotation", log); err != nil {
			return err
		}
	}

	if err := appendFlowItemsWithContext(sectionElem, c, section.Content, 1, "", log); err != nil {
		return err
	}
	return nil
}

func appendSectionContent(parent *etree.Element, c *content.Content, section *fb2.Section, skipTitle bool, depth int, log *zap.Logger) error {
	if section.Title != nil && !skipTitle {
		appendTitleAsHeading(parent, c, section.Title, depth, "title")
	}

	if err := appendEpigraphs(parent, c, section.Epigraphs, depth, log); err != nil {
		return err
	}

	if section.Image != nil {
		appendImageElement(parent, c, section.Image)
	}

	if section.Annotation != nil {
		div := parent.CreateElement("div")
		div.CreateAttr("class", "annotation")
		if err := appendFlowItemsWithContext(div, c, section.Annotation.Items, depth, "annotation", log); err != nil {
			return err
		}
	}

	if err := appendFlowItemsWithContext(parent, c, section.Content, depth, "", log); err != nil {
		return err
	}

	return nil
}

func appendFlowItemsWithContext(parent *etree.Element, c *content.Content, items []fb2.FlowItem, depth int, context string, log *zap.Logger) error {
	for _, item := range items {
		switch item.Kind {
		case fb2.FlowParagraph:
			if item.Paragraph != nil {
				c.KoboSpanNextParagraph()
				p := parent.CreateElement("p")
				if item.Paragraph.ID != "" {
					p.CreateAttr("id", item.Paragraph.ID)
				}
				if item.Paragraph.Lang != "" {
					p.CreateAttr("xml:lang", item.Paragraph.Lang)
				}
				if item.Paragraph.Style != "" {
					p.CreateAttr("class", item.Paragraph.Style)
				}
				appendParagraphInline(p, c, item.Paragraph)
			}
		case fb2.FlowImage:
			if item.Image != nil {
				appendImageElement(parent, c, item.Image)
			}
		case fb2.FlowEmptyLine:
			div := parent.CreateElement("div")
			div.CreateAttr("class", "emptyline")
		case fb2.FlowSubtitle:
			c.KoboSpanNextParagraph()
			if item.Subtitle != nil {
				var class string
				if context != "" {
					// Inside poem/cite/epigraph/annotation - use paragraph with context-specific class
					class = context + "-subtitle"
					if item.Subtitle.Style != "" {
						class = class + " " + item.Subtitle.Style
					}
					p := parent.CreateElement("p")
					if item.Subtitle.ID != "" {
						p.CreateAttr("id", item.Subtitle.ID)
					}
					if item.Subtitle.Lang != "" {
						p.CreateAttr("xml:lang", item.Subtitle.Lang)
					}
					p.CreateAttr("class", class)
					appendParagraphInline(p, c, item.Subtitle)
				} else {
					// In section - use header (one level more than enclosing title)
					headingLevel := min(depth+1, 6)
					headingTag := fmt.Sprintf("h%d", headingLevel)
					h := parent.CreateElement(headingTag)
					// Always set ID (use generated if original doesn't exist)
					subtitleID := item.Subtitle.ID
					h.CreateAttr("id", subtitleID)
					if item.Subtitle.Lang != "" {
						h.CreateAttr("xml:lang", item.Subtitle.Lang)
					}
					class = "subtitle"
					if item.Subtitle.Style != "" {
						class = class + " " + item.Subtitle.Style
					}
					h.CreateAttr("class", class)
					appendParagraphInline(h, c, item.Subtitle)
				}
			}
		case fb2.FlowPoem:
			if item.Poem != nil {
				if err := appendPoemElement(parent, c, item.Poem, depth, log); err != nil {
					return err
				}
			}
		case fb2.FlowCite:
			if item.Cite != nil {
				if err := appendCiteElement(parent, c, item.Cite, depth, log); err != nil {
					return err
				}
			}
		case fb2.FlowTable:
			if item.Table != nil {
				appendTableElement(parent, c, item.Table)
			}
		case fb2.FlowSection:
			if item.Section != nil {
				div := parent.CreateElement("div")
				div.CreateAttr("class", "section")
				div.CreateAttr("id", item.Section.ID)
				if item.Section.Lang != "" {
					div.CreateAttr("xml:lang", item.Section.Lang)
				}
				if err := appendSectionContent(div, c, item.Section, false, depth+1, log); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func appendParagraphInline(parent *etree.Element, c *content.Content, p *fb2.Paragraph) {
	hyphenate := !p.Special && c.Hyphen != nil
	for _, seg := range p.Text {
		appendInlineSegment(parent, c, &seg, hyphenate)
	}
}

func appendInlineText(parent *etree.Element, c *content.Content, text string, hyphenate bool) {
	if hyphenate {
		text = c.Hyphen.Hyphenate(text)
	}
	if c.OutputFormat == config.OutputFmtKepub && strings.TrimSpace(text) != "" {
		// Kobo mode: wrap text in span with unique ID
		for s := range c.Splitter.Sentences(text) {
			paragraph, sentence := c.KoboSpanNextSentence()
			span := parent.CreateElement("span")
			span.CreateAttr("class", "koboSpan")
			span.CreateAttr("id", fmt.Sprintf("kobo.%d.%d", paragraph, sentence))
			span.SetText(s)
		}
	} else {
		// Standard mode: use original tail-based approach
		if parent.ChildElements() == nil || len(parent.ChildElements()) == 0 {
			parent.SetText(text)
		} else {
			lastChild := parent.ChildElements()[len(parent.ChildElements())-1]
			lastChild.SetTail(lastChild.Tail() + text)
		}
	}
}

func appendInlineSegment(parent *etree.Element, c *content.Content, seg *fb2.InlineSegment, hyphenate bool) {
	switch seg.Kind {
	case fb2.InlineText:
		appendInlineText(parent, c, seg.Text, hyphenate)
	case fb2.InlineStrong:
		strong := parent.CreateElement("strong")
		for _, child := range seg.Children {
			appendInlineSegment(strong, c, &child, hyphenate)
		}
	case fb2.InlineEmphasis:
		em := parent.CreateElement("em")
		for _, child := range seg.Children {
			appendInlineSegment(em, c, &child, hyphenate)
		}
	case fb2.InlineStrikethrough:
		del := parent.CreateElement("del")
		for _, child := range seg.Children {
			appendInlineSegment(del, c, &child, hyphenate)
		}
	case fb2.InlineSub:
		sub := parent.CreateElement("sub")
		for _, child := range seg.Children {
			appendInlineSegment(sub, c, &child, hyphenate)
		}
	case fb2.InlineSup:
		sup := parent.CreateElement("sup")
		for _, child := range seg.Children {
			appendInlineSegment(sup, c, &child, hyphenate)
		}
	case fb2.InlineCode:
		code := parent.CreateElement("code")
		for _, child := range seg.Children {
			appendInlineSegment(code, c, &child, hyphenate)
		}
	case fb2.InlineNamedStyle:
		span := parent.CreateElement("span")
		if seg.Style != "" {
			span.CreateAttr("class", seg.Style)
		}
		for _, child := range seg.Children {
			appendInlineSegment(span, c, &child, hyphenate)
		}
	case fb2.InlineLink:
		a := parent.CreateElement("a")
		if seg.Href != "" {
			a.CreateAttr("href", seg.Href)

			// Determine link type and apply appropriate class
			var linkClass string
			if linkID, internalLink := strings.CutPrefix(seg.Href, "#"); internalLink {
				if _, isFootnote := c.FootnotesIndex[linkID]; isFootnote {
					linkClass = "footnote-link"
					// Handle float mode footnote references
					if c.FootnotesMode == config.FootnotesModeFloat {
						ref := c.AddFootnoteBackLinkRef(linkID)
						// Add reference ID for EPUB2 bidirectional linking
						if c.OutputFormat == config.OutputFmtEpub2 {
							a.CreateAttr("id", ref.RefID)
						}
						// Add epub:type="noteref" for EPUB3
						if c.OutputFormat == config.OutputFmtEpub3 {
							a.CreateAttr("epub:type", "noteref")
							a.CreateAttr("id", ref.RefID)
						}
					}
				} else {
					linkClass = "internal-link"
				}
			} else {
				// External link
				linkClass = "external-link"
			}
			a.CreateAttr("class", linkClass)
		}
		for _, child := range seg.Children {
			appendInlineSegment(a, c, &child, hyphenate)
		}
	case fb2.InlineImageSegment:
		if seg.Image != nil {
			var imgParent *etree.Element
			if c.OutputFormat == config.OutputFmtKepub {
				paragraph, sentence := c.KoboSpanNextSentence()
				span := parent.CreateElement("span")
				span.CreateAttr("class", "koboSpan")
				span.CreateAttr("id", fmt.Sprintf("kobo.%d.%d", paragraph, sentence))
				imgParent = span
			} else {
				imgParent = parent
			}
			img := imgParent.CreateElement("img")
			img.CreateAttr("class", "inline-image")
			imgID := strings.TrimPrefix(seg.Image.Href, "#")
			if imgData, ok := c.ImagesIndex[imgID]; ok {
				img.CreateAttr("src", path.Join(imagesDir, imgData.Filename))
			} else {
				img.CreateAttr("src", path.Join(imagesDir, imgID))
			}
			img.CreateAttr("alt", seg.Image.Alt)
		}
	}
}

func appendImageElement(parent *etree.Element, c *content.Content, img *fb2.Image) {
	div := parent.CreateElement("div")
	div.CreateAttr("class", "image")
	if img.ID != "" {
		div.CreateAttr("id", img.ID)
	}

	c.KoboSpanNextParagraph()
	var imgParent *etree.Element
	if c.OutputFormat == config.OutputFmtKepub {
		paragraph, sentence := c.KoboSpanNextSentence()
		span := div.CreateElement("span")
		span.CreateAttr("class", "koboSpan")
		span.CreateAttr("id", fmt.Sprintf("kobo.%d.%d", paragraph, sentence))
		imgParent = span
	} else {
		imgParent = div
	}

	imgElem := imgParent.CreateElement("img")
	imgElem.CreateAttr("class", "block-image")
	imgID := strings.TrimPrefix(img.Href, "#")
	if imgData, ok := c.ImagesIndex[imgID]; ok {
		imgElem.CreateAttr("src", path.Join(imagesDir, imgData.Filename))
	} else {
		imgElem.CreateAttr("src", path.Join(imagesDir, imgID))
	}
	imgElem.CreateAttr("alt", img.Alt)
	if img.Title != "" {
		imgElem.CreateAttr("title", img.Title)
	}
}

func appendPoemElement(parent *etree.Element, c *content.Content, poem *fb2.Poem, depth int, log *zap.Logger) error {
	div := parent.CreateElement("div")
	div.CreateAttr("class", "poem")
	if poem.ID != "" {
		div.CreateAttr("id", poem.ID)
	}
	if poem.Lang != "" {
		div.CreateAttr("xml:lang", poem.Lang)
	}

	if poem.Title != nil {
		appendTitleAsDiv(div, c, poem.Title, "poem-title")
	}

	if err := appendEpigraphs(div, c, poem.Epigraphs, depth, log); err != nil {
		return err
	}

	for _, subtitle := range poem.Subtitles {
		c.KoboSpanNextParagraph()
		p := div.CreateElement("p")
		p.CreateAttr("class", "poem-subtitle")
		if subtitle.ID != "" {
			p.CreateAttr("id", subtitle.ID)
		}
		appendParagraphInline(p, c, &subtitle)
	}

	for _, stanza := range poem.Stanzas {
		stanzaDiv := div.CreateElement("div")
		stanzaDiv.CreateAttr("class", "stanza")
		if stanza.Lang != "" {
			stanzaDiv.CreateAttr("xml:lang", stanza.Lang)
		}

		if stanza.Title != nil {
			appendTitleAsDiv(stanzaDiv, c, stanza.Title, "stanza-title")
		}

		if stanza.Subtitle != nil {
			c.KoboSpanNextParagraph()
			p := stanzaDiv.CreateElement("p")
			p.CreateAttr("class", "stanza-subtitle")
			if stanza.Subtitle.ID != "" {
				p.CreateAttr("id", stanza.Subtitle.ID)
			}
			appendParagraphInline(p, c, stanza.Subtitle)
		}

		for _, verse := range stanza.Verses {
			c.KoboSpanNextParagraph()
			p := stanzaDiv.CreateElement("p")
			p.CreateAttr("class", "verse")
			appendParagraphInline(p, c, &verse)
		}
	}

	for _, ta := range poem.TextAuthors {
		c.KoboSpanNextParagraph()
		p := div.CreateElement("p")
		p.CreateAttr("class", "text-author")
		appendParagraphInline(p, c, &ta)
	}

	if poem.Date != nil {
		c.KoboSpanNextParagraph()
		p := div.CreateElement("p")
		p.CreateAttr("class", "date")
		var dateText string
		if poem.Date.Display != "" {
			dateText = poem.Date.Display
		} else if !poem.Date.Value.IsZero() {
			dateText = poem.Date.Value.Format("2006-01-02")
		}
		appendInlineText(p, c, dateText, false)
	}
	return nil
}

func appendCiteElement(parent *etree.Element, c *content.Content, cite *fb2.Cite, depth int, log *zap.Logger) error {
	blockquote := parent.CreateElement("blockquote")
	if cite.ID != "" {
		blockquote.CreateAttr("id", cite.ID)
	}
	if cite.Lang != "" {
		blockquote.CreateAttr("xml:lang", cite.Lang)
	}
	blockquote.CreateAttr("class", "cite")

	if err := appendFlowItemsWithContext(blockquote, c, cite.Items, depth, "cite", log); err != nil {
		return err
	}

	for _, ta := range cite.TextAuthors {
		c.KoboSpanNextParagraph()
		p := blockquote.CreateElement("p")
		p.CreateAttr("class", "text-author")
		appendParagraphInline(p, c, &ta)
	}
	return nil
}

// buildStyleAttr builds a CSS style attribute from base style and alignment properties
func buildStyleAttr(baseStyle, align, vAlign string) string {
	style := baseStyle

	if align != "" {
		if style != "" && !strings.HasSuffix(style, ";") {
			style += ";"
		}
		if style != "" {
			style += " "
		}
		style += fmt.Sprintf("text-align: %s;", align)
	}

	if vAlign != "" {
		if style != "" && !strings.HasSuffix(style, ";") {
			style += ";"
		}
		if style != "" {
			style += " "
		}
		style += fmt.Sprintf("vertical-align: %s;", vAlign)
	}

	return style
}

func appendTableElement(parent *etree.Element, c *content.Content, table *fb2.Table) {
	c.KoboSpanNextParagraph()
	tableElem := parent.CreateElement("table")
	if table.ID != "" {
		tableElem.CreateAttr("id", table.ID)
	}
	if table.Style != "" {
		tableElem.CreateAttr("class", table.Style)
	}

	for _, row := range table.Rows {
		tr := tableElem.CreateElement("tr")
		if row.Style != "" || row.Align != "" {
			tr.CreateAttr("style", buildStyleAttr(row.Style, row.Align, ""))
		}

		for _, cell := range row.Cells {
			var td *etree.Element
			if cell.Header {
				td = tr.CreateElement("th")
			} else {
				td = tr.CreateElement("td")
			}

			if cell.ID != "" {
				td.CreateAttr("id", cell.ID)
			}
			if cell.ColSpan > 1 {
				td.CreateAttr("colspan", fmt.Sprintf("%d", cell.ColSpan))
			}
			if cell.RowSpan > 1 {
				td.CreateAttr("rowspan", fmt.Sprintf("%d", cell.RowSpan))
			}
			if cell.Style != "" || cell.Align != "" || cell.VAlign != "" {
				td.CreateAttr("style", buildStyleAttr(cell.Style, cell.Align, cell.VAlign))
			}

			for _, seg := range cell.Content {
				appendInlineSegment(td, c, &seg, false)
			}
		}
	}
}

func writeXHTMLChapter(zw *zip.Writer, chapter *chapterData, _ *zap.Logger) error {
	// Extract base filename without anchor for file writing
	filename := chapter.Filename
	if idx := strings.Index(filename, "#"); idx != -1 {
		filename = filename[:idx]
	}
	return writeXMLToZip(zw, filepath.Join(oebpsDir, filename), chapter.Doc)
}

// collectIDsFromBody collects all IDs from a body and maps them to the given filename
func collectIDsFromBody(body *fb2.Body, filename string, idToFile idToFileMap) {
	if body.Image != nil && body.Image.ID != "" {
		idToFile[body.Image.ID] = filename
	}
	for i := range body.Sections {
		collectIDsFromSection(&body.Sections[i], filename, idToFile)
	}
}

// collectIDsFromSection recursively collects all IDs from a section and its content
func collectIDsFromSection(section *fb2.Section, filename string, idToFile idToFileMap) {
	if section.ID != "" {
		idToFile[section.ID] = filename
	}
	if section.Image != nil && section.Image.ID != "" {
		idToFile[section.Image.ID] = filename
	}
	if section.Title != nil {
		for i := range section.Title.Items {
			if section.Title.Items[i].Paragraph != nil && section.Title.Items[i].Paragraph.ID != "" {
				idToFile[section.Title.Items[i].Paragraph.ID] = filename
			}
		}
	}
	for i := range section.Content {
		collectIDsFromFlowItem(&section.Content[i], filename, idToFile)
	}
}

// collectIDsFromFlowItem recursively collects IDs from flow items
func collectIDsFromFlowItem(item *fb2.FlowItem, filename string, idToFile idToFileMap) {
	switch item.Kind {
	case fb2.FlowParagraph:
		if item.Paragraph != nil && item.Paragraph.ID != "" {
			idToFile[item.Paragraph.ID] = filename
		}
	case fb2.FlowSubtitle:
		if item.Subtitle != nil && item.Subtitle.ID != "" {
			idToFile[item.Subtitle.ID] = filename
		}
	case fb2.FlowImage:
		if item.Image != nil && item.Image.ID != "" {
			idToFile[item.Image.ID] = filename
		}
	case fb2.FlowPoem:
		if item.Poem != nil && item.Poem.ID != "" {
			idToFile[item.Poem.ID] = filename
		}
	case fb2.FlowCite:
		if item.Cite != nil {
			if item.Cite.ID != "" {
				idToFile[item.Cite.ID] = filename
			}
			for i := range item.Cite.Items {
				collectIDsFromFlowItem(&item.Cite.Items[i], filename, idToFile)
			}
		}
	case fb2.FlowTable:
		if item.Table != nil {
			if item.Table.ID != "" {
				idToFile[item.Table.ID] = filename
			}
			for i := range item.Table.Rows {
				for j := range item.Table.Rows[i].Cells {
					if item.Table.Rows[i].Cells[j].ID != "" {
						idToFile[item.Table.Rows[i].Cells[j].ID] = filename
					}
				}
			}
		}
	case fb2.FlowSection:
		if item.Section != nil {
			collectIDsFromSection(item.Section, filename, idToFile)
		}
	}
}

// fixInternalLinks updates internal links to include chapter filenames when needed
func fixInternalLinks(chapters []chapterData, idToFile idToFileMap, log *zap.Logger) {
	for i := range chapters {
		if chapters[i].Doc == nil {
			continue
		}
		currentFile := chapters[i].Filename
		fixLinksInDocument(chapters[i].Doc, currentFile, idToFile, log)
	}
}

// fixLinksInDocument traverses the XHTML document and fixes internal links
func fixLinksInDocument(doc *etree.Document, currentFile string, idToFile idToFileMap, log *zap.Logger) {
	root := doc.Root()
	if root == nil {
		return
	}
	fixLinksInElement(root, currentFile, idToFile, log)
}

// fixLinksInElement recursively fixes links in an element and its children
func fixLinksInElement(elem *etree.Element, currentFile string, idToFile idToFileMap, log *zap.Logger) {
	// Fix links in this element
	if elem.Tag == "a" {
		href := elem.SelectAttrValue("href", "")
		if href != "" && strings.HasPrefix(href, "#") {
			targetID := strings.TrimPrefix(href, "#")
			if targetFile, exists := idToFile[targetID]; exists && targetFile != currentFile {
				// Link points to different file, update href
				newHref := targetFile + "#" + targetID
				elem.RemoveAttr("href")
				elem.CreateAttr("href", newHref)
			}
		}
	}

	// Recursively process children
	for _, child := range elem.ChildElements() {
		fixLinksInElement(child, currentFile, idToFile, log)
	}
}

func writeXMLToZip(zw *zip.Writer, name string, doc *etree.Document) error {
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		return err
	}
	return writeDataToZip(zw, name, buf.Bytes())
}
