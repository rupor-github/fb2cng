package epub

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/beevik/etree"
	"go.uber.org/zap"
	"golang.org/x/text/cases"

	"fbc/common"
	"fbc/content"
	"fbc/content/text"
	"fbc/fb2"
	"fbc/state"
)

type chapterData struct {
	ID           string
	Filename     string
	Title        string
	Doc          *etree.Document
	Section      *fb2.Section // Reference to source section for TOC hierarchy
	AnchorID     string       // ID to use as anchor in links (if different from automatic determination)
	IncludeInTOC bool         // Whether to include this chapter in navigation/TOC
}

// idToFileMap maps element IDs to the chapter filename containing them
type idToFileMap map[string]string

// splitResult signals that a nested section needs its own XHTML file.
// When appendFlowItems encounters a FlowSection at a depth where
// SectionNeedsBreak returns true, it stops rendering and returns the
// section plus any remaining sibling items.
type splitResult struct {
	section   *fb2.Section   // the section that triggered the split
	depth     int            // the depth this section lives at
	remaining []fb2.FlowItem // siblings after this section (always FlowSection per FB2 spec)
}

func convertToXHTML(ctx context.Context, c *content.Content, log *zap.Logger) ([]chapterData, idToFileMap, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	env := state.EnvFromContext(ctx)

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

		// Enable page tracking for main and other bodies
		c.PageTrackingEnabled = c.PageSize > 0

		// Process main and other bodies (not footnotes)
		// If body has title, create a chapter for body intro content
		if body.Title != nil {
			chapterNum++
			baseID := fmt.Sprintf("index%05d", chapterNum)
			chapterID, filename := generateUniqueID(baseID, c.IDsIndex)

			title := body.Title.AsTOCText("Untitled")

			// Set current filename for footnote reference tracking
			c.CurrentFilename = filename

			// Start new page at chapter boundary
			c.StartNewPageAtChapter()

			doc, err := bodyIntroToXHTML(c, body, title, chapterID, log)
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
		for j := range body.Sections {
			section := &body.Sections[j]
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}

			// This is a workaround to make EPubCheck happy
			// Check if we need to generate invisible link to nav.xhtml
			addHiddenNavLink := env.Cfg.Document.TOCPage.Placement != common.TOCPagePlacementNone && body.Main() && j == 0

			chapterNum++
			baseID := fmt.Sprintf("index%05d", chapterNum)
			chapterID, filename := generateUniqueID(baseID, c.IDsIndex)

			title := section.AsTitleText("")
			includeInTOC := section.HasTitle()
			if title == "" && env.Cfg.Document.TOCPage.ChaptersWithoutTitle {
				title = fb2.NoTitleText
				includeInTOC = true
			}

			// Set current filename for footnote reference tracking
			c.CurrentFilename = filename

			// Start new page at chapter boundary
			c.StartNewPageAtChapter()

			doc, splits, err := bodyToXHTML(c, body, section, title, addHiddenNavLink, log)
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
				IncludeInTOC: includeInTOC,
			})
			collectIDsFromSection(section, filename, idToFile)

			// Process split sections — nested sections that need their own files
			for len(splits) > 0 {
				split := splits[0]
				splits = splits[1:]

				chapterNum++
				splitBaseID := fmt.Sprintf("index%05d", chapterNum)
				splitChapterID, splitFilename := generateUniqueID(splitBaseID, c.IDsIndex)

				splitTitle := split.section.AsTitleText("")

				// Set current filename and start new page for the split section
				c.CurrentFilename = splitFilename
				c.StartNewPageAtChapter()

				splitDoc, innerSplits, err := renderSplitSection(c, body, split.section, split.depth, log)
				if err != nil {
					log.Error("Unable to convert split section", zap.Error(err))
					continue
				}

				chapters = append(chapters, chapterData{
					ID:           splitChapterID,
					Filename:     splitFilename,
					Title:        splitTitle,
					Doc:          splitDoc,
					IncludeInTOC: false, // TOC entry comes from parent's Section traversal
				})
				// Overwrite ID mappings: split section's IDs now point to new file
				collectIDsFromSection(split.section, splitFilename, idToFile)

				// Queue inner splits (depth-first) then remaining siblings
				var next []splitResult
				next = append(next, innerSplits...)
				for _, rem := range split.remaining {
					if rem.Kind == fb2.FlowSection && rem.Section != nil {
						next = append(next, splitResult{
							section:   rem.Section,
							depth:     split.depth,
							remaining: nil,
						})
					}
				}
				splits = append(next, splits...)
			}
		}
	}

	if len(footnoteBodies) == 0 {
		return chapters, idToFile, nil
	}

	// Process all footnote bodies - each body becomes a separate top-level chapter
	chapterNum++
	// Include footnotes in page calculations regardless of float mode
	// (content is visible in both modes, just displayed differently)
	c.PageTrackingEnabled = c.PageSize > 0

	// Start new page at footnotes chapter boundary
	c.StartNewPageAtChapter()

	footnotesChapters, err := processFootnoteBodies(c, footnoteBodies, idToFile, log)
	if err != nil {
		log.Error("Unable to convert footnotes", zap.Error(err))
		return chapters, idToFile, nil
	}

	chapters = append(chapters, footnotesChapters...)
	return chapters, idToFile, nil
}

// processFootnoteBodies converts all footnote bodies to XHTML and creates chapter entries
func processFootnoteBodies(c *content.Content, footnoteBodies []*fb2.Body, idToFile idToFileMap, log *zap.Logger) ([]chapterData, error) {
	_, filename := generateUniqueID("footnotes", c.IDsIndex)

	docTitle := footnoteBodies[0].AsTitleText("Footnotes")

	doc, root := createXHTMLDocument(c, docTitle)

	// Set current filename for footnote tracking
	c.CurrentFilename = filename

	// Check if we're in float mode (footnote sections should NOT appear in TOC in float mode)
	isFloatMode := c.FootnotesMode.IsFloat()

	// Process all footnote bodies - build XHTML and chapter metadata in single loop
	var chapters []chapterData
	for bodyIdx, body := range footnoteBodies {
		baseBodyID := fmt.Sprintf("%s%05d", body.Name, bodyIdx)
		bodyID, _ := generateUniqueID(baseBodyID, c.IDsIndex)
		bodyTitle := body.AsTitleText(cases.Title(c.Book.Description.TitleInfo.Lang, cases.NoLower).String(body.Name))

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
			if c.FootnotesMode.IsFloat() && c.OutputFormat == common.OutputFmtEpub3 {
				if err := appendFloatFootnoteSectionContentEpub3(bodyDiv, c, section, log); err != nil {
					return nil, err
				}
			} else if c.FootnotesMode.IsFloat() && (c.OutputFormat == common.OutputFmtEpub2 || c.OutputFormat == common.OutputFmtKepub) {
				if err := appendFloatFootnoteSectionContentEpub2(bodyDiv, c, section, log); err != nil {
					return nil, err
				}
			} else {
				if err := appendFootnoteSectionContent(bodyDiv, c, section, log); err != nil {
					return nil, err
				}
			}
		}

		// Create chapter entry for TOC - reuse bodyID as base for uniqueness
		tocChapterID, _ := generateUniqueID(bodyID, c.IDsIndex)
		var chapterDoc *etree.Document
		if bodyIdx == 0 {
			// First chapter owns the document for writing to file
			chapterDoc = doc
		}

		// In default (non-float) mode, create a wrapper section that contains all footnote
		// sections as nested content. This enables the TOC generation machinery to include
		// individual footnote sections in the TOC.
		var wrapperSection *fb2.Section
		if !isFloatMode && len(body.Sections) > 0 {
			wrapperSection = createFootnoteWrapperSection(body)
		}

		chapters = append(chapters, chapterData{
			ID:           tocChapterID,
			Filename:     filename,
			AnchorID:     bodyID,
			Title:        bodyTitle,
			Doc:          chapterDoc,
			Section:      wrapperSection,
			IncludeInTOC: bodyTitle != "",
		})
	}

	return chapters, nil
}

// createFootnoteWrapperSection creates a virtual section that wraps all footnote sections
// from a body as nested FlowSection items. This enables the TOC generation to traverse
// and include individual footnote sections.
func createFootnoteWrapperSection(body *fb2.Body) *fb2.Section {
	// Create flow items that reference the actual footnote sections
	content := make([]fb2.FlowItem, 0, len(body.Sections))
	for i := range body.Sections {
		content = append(content, fb2.FlowItem{
			Kind:    fb2.FlowSection,
			Section: &body.Sections[i],
		})
	}

	return &fb2.Section{
		Title:   body.Title,
		Content: content,
	}
}

// createXHTMLDocument creates a standard XHTML document structure with head elements
func createXHTMLDocument(c *content.Content, title string) (*etree.Document, *etree.Element) {
	c.KoboSpanSet(0, 0)

	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)

	// Add DOCTYPE declaration based on output format to make Sigil happy
	if c.OutputFormat == common.OutputFmtEpub3 {
		doc.CreateDirective("DOCTYPE html")
	} else {
		doc.CreateDirective(`DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd"`)
	}

	html := doc.CreateElement("html")
	html.CreateAttr("xmlns", "http://www.w3.org/1999/xhtml")
	html.CreateAttr("xmlns:epub", "http://www.idpf.org/2007/ops")

	head := html.CreateElement("head")

	link := head.CreateElement("link")
	link.CreateAttr("rel", "stylesheet")
	link.CreateAttr("type", "text/css")
	link.CreateAttr("href", "stylesheet.css")

	if title == "" {
		title = "Untitled"
	}
	titleElem := head.CreateElement("title")
	titleElem.SetText(title)

	body := html.CreateElement("body")

	var root *etree.Element
	if c.OutputFormat == common.OutputFmtKepub {
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

func bodyIntroToXHTML(c *content.Content, body *fb2.Body, title string, chapterID string, log *zap.Logger) (*etree.Document, error) {
	doc, root := createXHTMLDocument(c, title)

	// Create wrapper div with class based on body type
	var bodyClass string
	if body.Main() {
		bodyClass = "main-body"
	} else {
		bodyClass = "other-body"
	}

	bodyDiv := root.CreateElement("div")
	bodyDiv.CreateAttr("class", bodyClass)

	// Add unique ID to div wrapper
	bodyDiv.CreateAttr("id", chapterID)

	if err := appendBodyIntroContent(bodyDiv, c, body, 1, log); err != nil {
		return nil, err
	}

	return doc, nil
}

func bodyToXHTML(c *content.Content, body *fb2.Body, section *fb2.Section, title string, addHiddenNav bool, log *zap.Logger) (*etree.Document, []splitResult, error) {
	doc, root := createXHTMLDocument(c, title)

	// Create wrapper div with class based on body type
	var bodyClass string
	if body.Main() {
		bodyClass = "main-body"
	} else {
		bodyClass = "other-body"
	}

	bodyDiv := root.CreateElement("div")
	bodyDiv.CreateAttr("class", bodyClass)

	// Add section ID to div wrapper so it can be a link target
	if section.ID != "" {
		bodyDiv.CreateAttr("id", section.ID)
	}

	splits, err := appendSectionContent(bodyDiv, c, section, 1, log)
	if err != nil {
		return nil, nil, err
	}

	// EPUB3: Add hidden navigation link at the end of the first main body section
	if addHiddenNav && c.OutputFormat == common.OutputFmtEpub3 {
		hiddenP := bodyDiv.CreateElement("p")
		hiddenP.CreateAttr("style", "display: none; visibility: hidden")
		navLink := hiddenP.CreateElement("a")
		navLink.CreateAttr("href", "nav.xhtml")
		navLink.SetText("Navigation")
	}

	return doc, splits, nil
}

// renderSplitSection creates a new XHTML document for a section that was split out
// of its parent due to CSS page-break-before rules. It renders the section content
// and returns any further nested splits.
func renderSplitSection(c *content.Content, body *fb2.Body, section *fb2.Section, depth int, log *zap.Logger) (*etree.Document, []splitResult, error) {
	title := section.AsTitleText("")
	doc, root := createXHTMLDocument(c, title)

	// Create wrapper div with class based on body type
	var bodyClass string
	if body.Main() {
		bodyClass = "main-body"
	} else {
		bodyClass = "other-body"
	}

	bodyDiv := root.CreateElement("div")
	bodyDiv.CreateAttr("class", bodyClass)

	// Add section ID to div wrapper so it can be a link target
	if section.ID != "" {
		bodyDiv.CreateAttr("id", section.ID)
	}

	splits, err := appendSectionContent(bodyDiv, c, section, depth, log)
	if err != nil {
		return nil, nil, err
	}

	return doc, splits, nil
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
				br.CreateAttr("class", classPrefix+"-break")
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
		if _, err := appendFlowItems(div, c, epigraph.Flow.Items, depth, "epigraph", log); err != nil {
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

func appendBodyIntroContent(parent *etree.Element, c *content.Content, body *fb2.Body, depth int, log *zap.Logger) error {
	// Per FictionBook.xsd bodyType sequence: image -> title -> epigraph -> section

	if body.Image != nil {
		appendImageElement(parent, c, body.Image)
	}

	if body.Title != nil {
		// Always create wrapper div for body title
		titleWrapper := parent.CreateElement("div")
		titleWrapper.CreateAttr("class", "body-title")

		// Insert top vignette if needed
		if body.Main() && c.Book.IsVignetteEnabled(common.VignettePosBookTitleTop) {
			appendVignetteImage(titleWrapper, c, common.VignettePosBookTitleTop)
		}

		// Append the title with header class
		appendTitleAsHeading(titleWrapper, c, body.Title, depth, "body-title-header")

		// Insert bottom vignette if needed
		if body.Main() && c.Book.IsVignetteEnabled(common.VignettePosBookTitleBottom) {
			appendVignetteImage(titleWrapper, c, common.VignettePosBookTitleBottom)
		}
	}

	if err := appendEpigraphs(parent, c, body.Epigraphs, depth, log); err != nil {
		return err
	}

	return nil
}

// hasImageChild checks if an element contains an img child
func hasImageChild(elem *etree.Element) bool {
	for _, child := range elem.ChildElements() {
		if child.Tag == "img" {
			return true
		}
	}
	return false
}

// appendFloatFootnoteSectionContentEpub2 appends footnote section content in
// EPUB2 float mode uses <p class="footnote"> and simplified rendering to fit
// everything in a single paragraph keeping as much formatting as possible.
func appendFloatFootnoteSectionContentEpub2(parent *etree.Element, c *content.Content, section *fb2.Section, _ *zap.Logger) error {
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
					appendPlainText(sectionElem, c, text.NBSP)
				}
				backLink := sectionElem.CreateElement("a")
				href := ref.Filename + "#" + ref.RefID
				backLink.CreateAttr("href", href)
				backLink.CreateAttr("class", "link-backlink")

				textParent := backLink
				if c.OutputFormat == common.OutputFmtKepub {
					paragraph, sentence := c.KoboSpanNextSentence()
					span := backLink.CreateElement("span")
					span.CreateAttr("class", "koboSpan")
					span.CreateAttr("id", fmt.Sprintf("kobo.%d.%d", paragraph, sentence))
					textParent = span
				}
				// Use title as link text if available, otherwise use ↩
				if section.Title != nil && i == 0 {
					appendPlainText(textParent, c, section.Title.AsTOCText(c.BacklinkStr))
				} else {
					appendPlainText(textParent, c, c.BacklinkStr)
				}
			}
			appendPlainText(sectionElem, c, text.NBSP)
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
				if c.OutputFormat == common.OutputFmtKepub {
					paragraph, sentence := c.KoboSpanNextSentence()
					span := sectionElem.CreateElement("span")
					span.CreateAttr("class", "koboSpan")
					span.CreateAttr("id", fmt.Sprintf("kobo.%d.%d", paragraph, sentence))
					imgsectionElem = span
				} else {
					imgsectionElem = sectionElem
				}
				img := imgsectionElem.CreateElement("img")
				img.CreateAttr("class", "image-inline")
				if item.Image.ID != "" {
					img.CreateAttr("id", item.Image.ID)
				}
				imgID := strings.TrimPrefix(item.Image.Href, "#")
				c.TrackImageUsage(imgID)
				if imgData, ok := c.ImagesIndex[imgID]; ok {
					img.CreateAttr("src", imgData.Filename)
				} else {
					img.CreateAttr("src", path.Join(fb2.ImagesDir, imgID))
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
				appendPlainText(span, c, item.Poem.AsPlainText())
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
				appendPlainText(span, c, item.Cite.AsPlainText())
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
				appendPlainText(span, c, item.Table.AsPlainText())
				sectionElem.CreateElement("br")
			}
		}
	}

	// If there's more than one text span, add indicator to the first one
	textSpans := make([]*etree.Element, 0)
	for _, child := range sectionElem.ChildElements() {
		if child.Tag == "span" {
			// Exclude koboSpan and image-containing spans
			if class := child.SelectAttrValue("class", ""); class != "koboSpan" && !hasImageChild(child) {
				textSpans = append(textSpans, child)
			}
		}
	}

	if len(textSpans) > 1 {
		firstSpan := textSpans[0]

		// Create the "more" indicator span
		moreSpan := etree.NewElement("span")
		moreSpan.CreateAttr("class", "footnote-more")
		appendPlainText(moreSpan, c, c.MoreParaStr)

		// Insert at the beginning of first span
		firstSpan.InsertChildAt(0, moreSpan)
	}

	return nil
}

// appendFloatFootnoteSectionContentEpub3 appends footnote section content in EPUB3 float mode
// EPUB3 Float mode uses <aside epub:type="footnote"> with class="footnote" on each paragraph
func appendFloatFootnoteSectionContentEpub3(parent *etree.Element, c *content.Content, section *fb2.Section, log *zap.Logger) error {
	if section.Title != nil {
		appendTitleAsDiv(parent, c, section.Title, "footnote-title")
	}

	asideElem := parent.CreateElement("aside")
	asideElem.CreateAttr("epub:type", "footnote")
	if section.ID != "" {
		asideElem.CreateAttr("id", section.ID)
	}
	if section.Lang != "" {
		asideElem.CreateAttr("xml:lang", section.Lang)
	}

	if err := appendEpigraphs(asideElem, c, section.Epigraphs, 1, log); err != nil {
		return err
	}

	if section.Image != nil {
		appendImageElement(asideElem, c, section.Image)
	}

	if section.Annotation != nil {
		div := asideElem.CreateElement("div")
		div.CreateAttr("class", "annotation")
		if _, err := appendFlowItems(div, c, section.Annotation.Items, 1, "annotation", log); err != nil {
			return err
		}
	}

	if _, err := appendFlowItems(asideElem, c, section.Content, 1, "section", log); err != nil {
		return err
	}

	// Add class="footnote" to all paragraphs (including those inside epigraph/annotation divs)
	// and collect them for "more" indicator placement. Use recursive traversal to find <p> elements
	// inside wrapper divs like <div class="epigraph"> and <div class="annotation">.
	paragraphs := collectAndClassifyFootnoteParagraphs(asideElem)

	if len(paragraphs) > 1 {
		firstPara := paragraphs[0]

		// Create the "more" indicator span
		moreSpan := etree.NewElement("span")
		moreSpan.CreateAttr("class", "footnote-more")
		appendPlainText(moreSpan, c, c.MoreParaStr)

		// Insert at the beginning of first paragraph
		firstPara.InsertChildAt(0, moreSpan)
	}

	// Add back-references for EPUB3 float mode - OUTSIDE the aside element
	// This ensures backlinks are not part of the footnote popup content
	if section.ID != "" {
		if refs, exists := c.BackLinkIndex[section.ID]; exists && len(refs) > 0 {
			// Add back-reference links paragraph (without footnote class)
			backPara := parent.CreateElement("p")

			for i, ref := range refs {
				if i > 0 {
					appendPlainText(backPara, c, text.NBSP)
				}
				backLink := backPara.CreateElement("a")
				backLink.CreateAttr("class", "link-backlink")
				// Include filename in href for cross-file back-references
				href := ref.Filename + "#" + ref.RefID
				backLink.CreateAttr("href", href)
				backLink.CreateAttr("epub:type", "backlink")
				backLink.CreateAttr("role", "doc-backlink")
				appendPlainText(backLink, c, c.BacklinkStr)
			}
		}
	}
	return nil
}

// collectAndClassifyFootnoteParagraphs finds all <p> elements inside an <aside> element
// (including those nested inside wrapper divs like <div class="epigraph"> and
// <div class="annotation">), adds class="footnote" to each, and returns them in
// document order. This ensures the "more" indicator is placed on the first visible
// paragraph regardless of whether it's in an epigraph, annotation, or body content.
func collectAndClassifyFootnoteParagraphs(aside *etree.Element) []*etree.Element {
	var paragraphs []*etree.Element
	var collect func(parent *etree.Element)
	collect = func(parent *etree.Element) {
		for _, child := range parent.ChildElements() {
			if child.Tag == "p" {
				// Add or append "footnote" class to paragraph
				existingClass := child.SelectAttrValue("class", "")
				if existingClass != "" {
					child.RemoveAttr("class")
					child.CreateAttr("class", existingClass+" footnote")
				} else {
					child.CreateAttr("class", "footnote")
				}
				paragraphs = append(paragraphs, child)
			} else if child.Tag == "div" {
				// Recurse into wrapper divs (epigraph, annotation, etc.)
				collect(child)
			}
		}
	}
	collect(aside)
	return paragraphs
}

// appendFootnoteSectionContent appends footnote section content in
// default mode using <div class="footnote">
func appendFootnoteSectionContent(parent *etree.Element, c *content.Content, section *fb2.Section, log *zap.Logger) error {
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
		if _, err := appendFlowItems(div, c, section.Annotation.Items, 1, "annotation", log); err != nil {
			return err
		}
	}

	if _, err := appendFlowItems(sectionElem, c, section.Content, 1, "section", log); err != nil {
		return err
	}
	return nil
}

func appendSectionContent(parent *etree.Element, c *content.Content, section *fb2.Section, depth int, log *zap.Logger) ([]splitResult, error) {
	if section.Title != nil {
		// Always create wrapper div for section title
		titleWrapper := parent.CreateElement("div")
		var wrapperClass, headerClass string
		if depth == 1 {
			wrapperClass = "chapter-title"
			headerClass = "chapter-title-header"

			// Insert top vignette for chapters
			if c.Book.IsVignetteEnabled(common.VignettePosChapterTitleTop) {
				appendVignetteImage(titleWrapper, c, common.VignettePosChapterTitleTop)
			}
		} else {
			// Include heading level in wrapper class for proper page-break control
			// e.g., "section-title section-title-h2" for depth 2
			headingLevel := min(depth, 6)
			wrapperClass = fmt.Sprintf("section-title section-title-h%d", headingLevel)
			headerClass = "section-title-header"

			// Insert top vignette for sections
			if c.Book.IsVignetteEnabled(common.VignettePosSectionTitleTop) {
				appendVignetteImage(titleWrapper, c, common.VignettePosSectionTitleTop)
			}
		}
		titleWrapper.CreateAttr("class", wrapperClass)

		// Append the title with appropriate header class
		appendTitleAsHeading(titleWrapper, c, section.Title, depth, headerClass)

		// Insert bottom vignette
		if depth == 1 {
			if c.Book.IsVignetteEnabled(common.VignettePosChapterTitleBottom) {
				appendVignetteImage(titleWrapper, c, common.VignettePosChapterTitleBottom)
			}
		} else {
			if c.Book.IsVignetteEnabled(common.VignettePosSectionTitleBottom) {
				appendVignetteImage(titleWrapper, c, common.VignettePosSectionTitleBottom)
			}
		}
	}

	if err := appendEpigraphs(parent, c, section.Epigraphs, depth, log); err != nil {
		return nil, err
	}

	if section.Image != nil {
		appendImageElement(parent, c, section.Image)
	}

	if section.Annotation != nil {
		div := parent.CreateElement("div")
		div.CreateAttr("class", "annotation")
		if _, err := appendFlowItems(div, c, section.Annotation.Items, depth, "annotation", log); err != nil {
			return nil, err
		}
	}

	splits, err := appendFlowItems(parent, c, section.Content, depth, "section", log)
	if err != nil {
		return nil, err
	}
	if len(splits) > 0 {
		// Content was split — end vignette will be on the last split section's file,
		// not this one. Skip it here.
		return splits, nil
	}

	// Insert end vignette
	if section.Title != nil {
		if depth == 1 && c.Book.IsVignetteEnabled(common.VignettePosChapterEnd) {
			appendVignetteImage(parent, c, common.VignettePosChapterEnd)
		} else if depth > 1 && c.Book.IsVignetteEnabled(common.VignettePosSectionEnd) {
			appendVignetteImage(parent, c, common.VignettePosSectionEnd)
		}
	}

	return nil, nil
}

func appendFlowItems(parent *etree.Element, c *content.Content, items []fb2.FlowItem, depth int, context string, log *zap.Logger) ([]splitResult, error) {
	for i, item := range items {
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
			if item.Subtitle != nil {
				c.KoboSpanNextParagraph()
				p := parent.CreateElement("p")
				if item.Subtitle.ID != "" {
					p.CreateAttr("id", item.Subtitle.ID)
				}
				if item.Subtitle.Lang != "" {
					p.CreateAttr("xml:lang", item.Subtitle.Lang)
				}
				class := context + "-subtitle"
				if item.Subtitle.Style != "" {
					class = class + " " + item.Subtitle.Style
				}
				p.CreateAttr("class", class)
				appendParagraphInline(p, c, item.Subtitle)
			}
		case fb2.FlowPoem:
			if item.Poem != nil {
				if err := appendPoemElement(parent, c, item.Poem, depth, log); err != nil {
					return nil, err
				}
			}
		case fb2.FlowCite:
			if item.Cite != nil {
				if err := appendCiteElement(parent, c, item.Cite, depth, log); err != nil {
					return nil, err
				}
			}
		case fb2.FlowTable:
			if item.Table != nil {
				appendTableElement(parent, c, item.Table)
			}
		case fb2.FlowSection:
			if item.Section != nil {
				sectionDepth := depth + 1
				if c.Book.SectionNeedsBreak(sectionDepth) {
					// This section needs its own file. Return it plus remaining siblings.
					return []splitResult{{
						section:   item.Section,
						depth:     sectionDepth,
						remaining: items[i+1:],
					}}, nil
				}
				div := parent.CreateElement("div")
				div.CreateAttr("class", "section")
				div.CreateAttr("id", item.Section.ID)
				if item.Section.Lang != "" {
					div.CreateAttr("xml:lang", item.Section.Lang)
				}
				splits, err := appendSectionContent(div, c, item.Section, sectionDepth, log)
				if err != nil {
					return nil, err
				}
				if len(splits) > 0 {
					// Bubble up: append remaining siblings to last split's remaining list.
					lastSplit := &splits[len(splits)-1]
					lastSplit.remaining = append(lastSplit.remaining, items[i+1:]...)
					return splits, nil
				}
			}
		}
	}
	return nil, nil
}

func appendParagraphInline(parent *etree.Element, c *content.Content, p *fb2.Paragraph) {
	hyphenate := !p.Special && c.Hyphen != nil

	// Handle drop cap if paragraph has has-dropcap style
	if hasStyle("has-dropcap", p.Style) && len(p.Text) > 0 {
		// Find first non-empty text segment
		for i := range p.Text {
			seg := &p.Text[i]
			if seg.Kind == fb2.InlineText && seg.Text != "" {
				// Extract first character
				runes := []rune(seg.Text)
				if len(runes) > 0 {
					firstChar := string(runes[0])
					restOfText := string(runes[1:])

					// Create span for drop cap
					dropCapSpan := parent.CreateElement("span")
					dropCapSpan.CreateAttr("class", "dropcap")
					if c.OutputFormat == common.OutputFmtKepub {
						paragraph, sentence := c.KoboSpanNextSentence()
						koboSpan := dropCapSpan.CreateElement("span")
						koboSpan.CreateAttr("class", "koboSpan")
						koboSpan.CreateAttr("id", fmt.Sprintf("kobo.%d.%d", paragraph, sentence))
					}

					chunks, markers := c.SplitTextByPage(firstChar)
					for i, chunk := range chunks {
						if c.OutputFormat == common.OutputFmtKepub {
							koboSpan := dropCapSpan.ChildElements()[0]
							koboSpan.SetText(koboSpan.Text() + chunk)
						} else {
							dropCapSpan.SetText(dropCapSpan.Text() + chunk)
						}
						if markers[i] {
							appendPageMarker(dropCapSpan, c)
						}
					}

					// Render the rest of the text
					if restOfText != "" {
						appendInlineText(parent, c, restOfText, hyphenate)
					}

					// Render remaining segments
					for j := i + 1; j < len(p.Text); j++ {
						appendInlineSegment(parent, c, &p.Text[j], hyphenate)
					}
					return
				}
			}
		}
	}

	// Regular paragraph rendering (no drop cap)
	for _, seg := range p.Text {
		appendInlineSegment(parent, c, &seg, hyphenate)
	}
}

func appendInlineText(parent *etree.Element, c *content.Content, text string, hyphenate bool) {
	if hyphenate {
		text = c.Hyphen.Hyphenate(text)
	}

	chunks, markers := c.SplitTextByPage(text)
	if len(chunks) == 0 {
		return
	}

	if c.OutputFormat == common.OutputFmtKepub && strings.TrimSpace(text) != "" {
		// Kobo mode: wrap text in span with unique ID
		for i, chunk := range chunks {
			for s := range c.Splitter.Sentences(chunk) {
				paragraph, sentence := c.KoboSpanNextSentence()
				span := parent.CreateElement("span")
				span.CreateAttr("class", "koboSpan")
				span.CreateAttr("id", fmt.Sprintf("kobo.%d.%d", paragraph, sentence))
				span.SetText(s)
			}
			if markers[i] {
				appendPageMarker(parent, c)
			}
		}
		return
	}

	// Standard mode: use original tail-based approach
	for i, chunk := range chunks {
		if parent.ChildElements() == nil || len(parent.ChildElements()) == 0 {
			parent.SetText(chunk)
		} else {
			lastChild := parent.ChildElements()[len(parent.ChildElements())-1]
			lastChild.SetTail(lastChild.Tail() + chunk)
		}
		if markers[i] {
			appendPageMarker(parent, c)
		}
	}
}

func appendPlainText(parent *etree.Element, c *content.Content, text string) {
	if text == "" {
		return
	}
	chunks, markers := c.SplitTextByPage(text)
	if len(chunks) == 0 {
		return
	}
	for i, chunk := range chunks {
		parent.CreateText(chunk)
		if markers[i] {
			appendPageMarker(parent, c)
		}
	}
}

func appendPageMarker(parent *etree.Element, c *content.Content) {
	spanID := c.AddPageMapEntry()
	pageMarker := etree.NewElement("span")
	pageMarker.CreateAttr("id", spanID)
	pageMarker.CreateAttr("class", "page-marker")

	// Insert after the last child when possible, otherwise at start.
	if len(parent.Child) > 0 {
		parent.AddChild(pageMarker)
		return
	}
	blockParent := findBlockLevelParent(parent)
	if blockParent != nil {
		blockParent.InsertChildAt(0, pageMarker)
		return
	}
	parent.AddChild(pageMarker)
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
					linkClass = "link-footnote"
					// Handle float mode footnote references
					if c.FootnotesMode.IsFloat() {
						ref := c.AddFootnoteBackLinkRef(linkID)
						// Add reference ID
						a.CreateAttr("id", ref.RefID)
						// Add epub:type="noteref" for EPUB3
						if c.OutputFormat == common.OutputFmtEpub3 {
							a.CreateAttr("epub:type", "noteref")
							a.CreateAttr("role", "doc-noteref")
						}
					}
				} else {
					linkClass = "link-internal"
				}
			} else {
				// External link
				linkClass = "link-external"
			}
			a.CreateAttr("class", linkClass)
		}
		for _, child := range seg.Children {
			appendInlineSegment(a, c, &child, hyphenate)
		}
	case fb2.InlineImageSegment:
		if seg.Image != nil {
			var imgParent *etree.Element
			if c.OutputFormat == common.OutputFmtKepub {
				paragraph, sentence := c.KoboSpanNextSentence()
				span := parent.CreateElement("span")
				span.CreateAttr("class", "koboSpan")
				span.CreateAttr("id", fmt.Sprintf("kobo.%d.%d", paragraph, sentence))
				imgParent = span
			} else {
				imgParent = parent
			}
			img := imgParent.CreateElement("img")
			img.CreateAttr("class", "image-inline")
			imgID := strings.TrimPrefix(seg.Image.Href, "#")
			c.TrackImageUsage(imgID)
			if imgData, ok := c.ImagesIndex[imgID]; ok {
				img.CreateAttr("src", imgData.Filename)
			} else {
				img.CreateAttr("src", path.Join(fb2.ImagesDir, imgID))
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
	if c.OutputFormat == common.OutputFmtKepub {
		paragraph, sentence := c.KoboSpanNextSentence()
		span := div.CreateElement("span")
		span.CreateAttr("class", "koboSpan")
		span.CreateAttr("id", fmt.Sprintf("kobo.%d.%d", paragraph, sentence))
		imgParent = span
	} else {
		imgParent = div
	}

	imgElem := imgParent.CreateElement("img")
	imgElem.CreateAttr("class", "image-block")
	imgID := strings.TrimPrefix(img.Href, "#")
	c.TrackImageUsage(imgID)
	if imgData, ok := c.ImagesIndex[imgID]; ok {
		imgElem.CreateAttr("src", imgData.Filename)
	} else {
		imgElem.CreateAttr("src", path.Join(fb2.ImagesDir, imgID))
	}
	imgElem.CreateAttr("alt", img.Alt)
	if img.Title != "" {
		imgElem.CreateAttr("title", img.Title)
	}
}

func appendVignetteImage(parent *etree.Element, c *content.Content, position common.VignettePos) {
	if !c.Book.IsVignetteEnabled(position) {
		return
	}

	vignetteID := c.Book.VignetteIDs[position]
	c.TrackImageUsage(vignetteID)
	imgData, ok := c.ImagesIndex[vignetteID]
	if !ok {
		return
	}

	div := parent.CreateElement("div")
	div.CreateAttr("class", fmt.Sprintf("vignette vignette-%s", position.String()))

	c.KoboSpanNextParagraph()
	var imgParent *etree.Element
	if c.OutputFormat == common.OutputFmtKepub {
		paragraph, sentence := c.KoboSpanNextSentence()
		span := div.CreateElement("span")
		span.CreateAttr("class", "koboSpan")
		span.CreateAttr("id", fmt.Sprintf("kobo.%d.%d", paragraph, sentence))
		imgParent = span
	} else {
		imgParent = div
	}

	imgElem := imgParent.CreateElement("img")
	imgElem.CreateAttr("class", "image-vignette")
	imgElem.CreateAttr("src", imgData.Filename)
	imgElem.CreateAttr("alt", "")
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

	if _, err := appendFlowItems(blockquote, c, cite.Items, depth, "cite", log); err != nil {
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

// hasStyle checks if the paragraph style contains "has-dropcap"
func hasStyle(style, styles string) bool {
	if styles == "" {
		return false
	}
	if style == "" {
		return true
	}
	for part := range strings.FieldsSeq(styles) {
		if part == style {
			return true
		}
	}
	return false
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

// generateUniqueID returns a unique ID and filename that doesn't collide with FB2 element IDs
func generateUniqueID(baseID string, fbIDs fb2.IDIndex) (id, filename string) {
	id = baseID
	filename = baseID + ".xhtml"
	counter := 0
	_, exists := fbIDs[id]
	for exists {
		counter++
		id = fmt.Sprintf("%s-%d", baseID, counter)
		filename = id + ".xhtml"
		_, exists = fbIDs[id]
	}
	return id, filename
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

// findBlockLevelParent walks up the element tree to find the nearest block-level container
// Returns nil if no suitable parent is found
func findBlockLevelParent(elem *etree.Element) *etree.Element {
	blockTags := map[string]bool{
		"p": true, "div": true, "blockquote": true, "td": true, "th": true,
		"li": true, "dd": true, "dt": true, "section": true, "article": true,
		"aside": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	}

	current := elem
	for current != nil {
		if blockTags[current.Tag] {
			return current
		}
		current = current.Parent()
	}
	return nil
}
