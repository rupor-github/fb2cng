package publication

import (
	"github.com/beevik/etree"
	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
	"fbc/convert/tocnav"
	"fbc/fb2"
)

func generateAnnotation(c *renderState, cfg *config.AnnotationConfig, log *zap.Logger) Chapter {
	doc, root := createXHTMLDocument(c, cfg.Title)

	id, filename := generateUniqueID("annotation-page", c.IDsIndex)

	oldFilename := c.CurrentFilename
	c.CurrentFilename = filename
	defer func() {
		c.CurrentFilename = oldFilename
	}()

	// Create wrapper div with annotation-body class and proper chapter ID
	annotationBodyDiv := root.CreateElement("div")
	annotationBodyDiv.CreateAttr("class", "annotation-body")
	annotationBodyDiv.CreateAttr("id", id)

	c.KoboSpanNextParagraph()
	h1 := annotationBodyDiv.CreateElement("h1")
	h1.CreateAttr("class", "annotation-title")
	appendInlineText(h1, c, cfg.Title, false)

	annotationDiv := annotationBodyDiv.CreateElement("div")
	annotationDiv.CreateAttr("class", "annotation")

	if _, err := appendFlowItems(annotationDiv, c, c.Book.Description.TitleInfo.Annotation.Items, 1, 1, "annotation", log); err != nil {
		log.Warn("Unable to convert annotation content", zap.Error(err))
	}

	return Chapter{
		ID:           id,
		Filename:     filename,
		Title:        cfg.Title,
		Doc:          doc,
		IncludeInTOC: cfg.InTOC,
	}
}

// generateTOCPage creates a TOC chapter as an XHTML page
func generateTOCPage(c *renderState, chapters []Chapter, cfg *config.TOCPageConfig, idToFile idToFileMap, log *zap.Logger) Chapter {
	title := "Table of Contents" // Default title, not actually visible
	doc, root := createXHTMLDocument(c, title)

	id, filename := generateUniqueID("toc-page", c.IDsIndex)

	oldFilename := c.CurrentFilename
	c.CurrentFilename = filename
	defer func() {
		c.CurrentFilename = oldFilename
	}()

	// Create wrapper div with toc-body class and proper chapter ID
	tocBodyDiv := root.CreateElement("div")
	tocBodyDiv.CreateAttr("class", "toc-body")
	tocBodyDiv.CreateAttr("id", id)

	// Build TOC content structure
	buildTOCContent(tocBodyDiv, c, chapters, cfg, idToFile, log)

	return Chapter{
		ID:           id,
		Filename:     filename,
		Title:        title,
		Doc:          doc,
		IncludeInTOC: false,
	}
}

func filterUnusedImages(c *content.Content) {
	if c == nil || len(c.ImagesIndex) == 0 {
		return
	}
	if len(c.UsedImageIDs) == 0 {
		return
	}
	filtered := make(fb2.BookImages, len(c.UsedImageIDs))
	for id := range c.UsedImageIDs {
		if img, ok := c.ImagesIndex[id]; ok {
			filtered[id] = img
		}
	}
	c.ImagesIndex = filtered
}

// CollectNavigation collects the logical TOC before device-specific hierarchy shaping.
// Resolved rendered IDs determine destinations for sections split into separate files.
func CollectNavigation(chapters []Chapter, idToFile map[string]string) []tocnav.Item {
	var items []tocnav.Item
	for _, chapter := range chapters {
		if !chapter.IncludeInTOC {
			// Untitled wrapper chapter: promote its titled children to the current level.
			if chapter.Section != nil {
				items = append(items, collectSectionNavItems(chapter.Section, chapter.Filename, 1, idToFile)...)
			}
			continue
		}

		items = append(items, tocnav.Item{
			ID:    chapter.ID,
			Title: chapter.Title,
			Href:  chapter.Filename + chapter.AnchorSuffix(),
			Level: 1,
		})
		if chapter.Section != nil {
			items = append(items, collectSectionNavItems(chapter.Section, chapter.Filename, 2, idToFile)...)
		}
	}
	return items
}

func collectSectionNavItems(section *fb2.Section, filename string, level int, idToFile idToFileMap) []tocnav.Item {
	var (
		items          []tocnav.Item
		lastTitleLevel int
		hasLastTitle   bool
	)

	for _, item := range section.Content {
		if item.Kind != fb2.FlowSection || item.Section == nil {
			continue
		}

		titleText := ""
		if item.Section.Title != nil {
			titleText = item.Section.Title.AsTOCText("")
		}
		if titleText != "" {
			sectionID := item.Section.ID
			sectionFilename := filename
			if mapped, ok := idToFile[sectionID]; ok {
				sectionFilename = mapped
			}
			items = append(items, tocnav.Item{
				ID:    "navpoint-" + sectionID,
				Title: titleText,
				Href:  sectionFilename + "#" + sectionID,
				Level: level,
			})
			items = append(items, collectSectionNavItems(item.Section, sectionFilename, level+1, idToFile)...)
			lastTitleLevel = level
			hasLastTitle = true
			continue
		}

		// Untitled section: nest children inside the last titled sibling if one exists;
		// otherwise promote them to the current level. This preserves the previous
		// recursive NCX behavior before toc_type shaping is applied.
		childLevel := level
		if hasLastTitle {
			childLevel = lastTitleLevel + 1
		}
		items = append(items, collectSectionNavItems(item.Section, filename, childLevel, idToFile)...)
	}
	return items
}

// buildTOCContent creates the TOC title, authors, and list structure
// parentContainer is the element to add content to (e.g., nav element or div)
// cfg is the TOC configuration
func buildTOCContent(
	parentContainer *etree.Element,
	c *renderState,
	chapters []Chapter,
	cfg *config.TOCPageConfig,
	idToFile idToFileMap,
	log *zap.Logger,
) {
	// Add book title
	c.KoboSpanNextParagraph()
	h1 := parentContainer.CreateElement("h1")
	h1.CreateAttr("class", "toc-title")

	// First span for book title
	titleSpan := h1.CreateElement("span")
	titleSpan.CreateAttr("class", "toc-title-first")
	appendInlineText(titleSpan, c, c.Book.Description.TitleInfo.BookTitle.Value, false)

	// Add authors if template is provided
	if cfg.AuthorsTemplate != "" {
		expanded, err := c.Book.ExpandTemplateMetainfo(config.AuthorsTemplateFieldName, cfg.AuthorsTemplate, c.SrcName, c.OutputFormat)
		if err != nil {
			log.Warn("Unable to prepare list of authors for TOC", zap.Error(err))
		} else {
			// Add break before authors
			br := h1.CreateElement("br")
			br.CreateAttr("class", "toc-title-break")

			// Add authors span
			authorsSpan := h1.CreateElement("span")
			authorsSpan.CreateAttr("class", "toc-title-next")
			appendInlineText(authorsSpan, c, expanded, false)
		}
	}

	// Build TOC list
	c.KoboSpanNextParagraph()
	ol := parentContainer.CreateElement("ol")
	ol.CreateAttr("class", "toc-list")

	for _, chapter := range chapters {
		if !chapter.IncludeInTOC {
			// Untitled wrapper chapter: promote its children to the top-level list.
			// This handles FB2 structures where a top-level <section> has no title
			// but contains titled subsections that should appear in the TOC.
			if chapter.Section != nil {
				buildTOCPageOLItems(ol, chapter.Section, chapter.Filename, c, cfg.ChaptersWithoutTitle, nil, idToFile)
			}
			continue
		}
		li := ol.CreateElement("li")
		li.CreateAttr("class", "toc-item")
		a := li.CreateElement("a")
		a.CreateAttr("class", "link-toc")
		a.CreateAttr("href", chapter.Filename+chapter.AnchorSuffix())
		appendInlineText(a, c, chapter.Title, false)

		// Add nested sections to TOC
		if chapter.Section != nil {
			buildTOCPageOL(li, chapter.Section, chapter.Filename, c, cfg.ChaptersWithoutTitle, idToFile)
		}
	}
}

// buildTOCPageOL recursively builds nested TOC structure for the TOC page
func buildTOCPageOL(parent *etree.Element, section *fb2.Section, filename string, c *renderState, includeUntitled bool, idToFile idToFileMap) {
	var oldParagraphs, oldSentences int
	nestedOL := parent.SelectElement("ol")
	if nestedOL == nil {
		oldParagraphs, oldSentences = c.KoboSpanNextParagraph()
		nestedOL = parent.CreateElement("ol")
		nestedOL.CreateAttr("class", "toc-list toc-nested")
	}

	hadItems := len(nestedOL.ChildElements()) > 0
	buildTOCPageOLItems(nestedOL, section, filename, c, includeUntitled, nil, idToFile)

	if !hadItems && len(nestedOL.ChildElements()) == 0 {
		parent.RemoveChild(nestedOL)
		c.KoboSpanSet(oldParagraphs, oldSentences)
	}
}

// buildTOCPageOLItems adds TOC entries for subsections to the ordered list
func buildTOCPageOLItems(
	parentOL *etree.Element,
	section *fb2.Section,
	filename string,
	c *renderState,
	includeUntitled bool,
	lastLI *etree.Element,
	idToFile idToFileMap,
) {
	for _, item := range section.Content {
		if item.Kind == fb2.FlowSection && item.Section != nil {
			// IMPORTANT: for nested TOC nodes we only consider explicit, non-empty titles.
			// Wrapper sections with no effective title must not produce TOC entries.
			titleText := ""
			if item.Section.Title != nil {
				titleText = item.Section.Title.AsTOCText("")
			}
			if titleText != "" {
				li := parentOL.CreateElement("li")
				li.CreateAttr("class", "toc-item toc-section")
				a := li.CreateElement("a")
				a.CreateAttr("class", "link-toc")
				sectionID := item.Section.ID

				// Use idToFile to resolve the correct filename for split sections
				sectionFilename := filename
				if mapped, ok := idToFile[sectionID]; ok {
					sectionFilename = mapped
				}

				a.CreateAttr("href", sectionFilename+"#"+sectionID)
				appendInlineText(a, c, titleText, false)

				buildTOCPageOL(li, item.Section, sectionFilename, c, includeUntitled, idToFile)
				lastLI = li
			} else {
				// Untitled section: nest children inside the last titled sibling if one exists
				if lastLI != nil {
					buildTOCPageOLIntoLI(lastLI, item.Section, filename, c, includeUntitled, idToFile)
				} else {
					// No preceding titled sibling, children go to parent level
					buildTOCPageOLItems(parentOL, item.Section, filename, c, includeUntitled, nil, idToFile)
				}
			}
		}
	}
}

// buildTOCPageOLIntoLI creates nested OL inside an LI for untitled section's children
func buildTOCPageOLIntoLI(li *etree.Element, section *fb2.Section, filename string, c *renderState, includeUntitled bool, idToFile idToFileMap) {
	// Check if there are titled children to add
	hasTitledChildren := false
	for _, item := range section.Content {
		if item.Kind == fb2.FlowSection && item.Section != nil {
			titleText := ""
			if item.Section.Title != nil {
				titleText = item.Section.Title.AsTOCText("")
			}
			if titleText != "" {
				hasTitledChildren = true
				break
			}
		}
	}
	if !hasTitledChildren {
		return
	}

	// Find or create nested OL
	var nestedOL *etree.Element
	for _, child := range li.ChildElements() {
		if child.Tag == "ol" {
			nestedOL = child
			break
		}
	}
	if nestedOL == nil {
		nestedOL = li.CreateElement("ol")
		nestedOL.CreateAttr("class", "toc-list toc-nested")
	}

	buildTOCPageOLItems(nestedOL, section, filename, c, includeUntitled, nil, idToFile)
}

// AnchorSuffix returns the anchor suffix (including #) for chapter links in TOC/Nav/NCX.
// Only footnote bodies need anchors since multiple bodies share one file.
// Regular chapters start at the top of their own files, so no anchor is needed.
func (chapter Chapter) AnchorSuffix() string {
	if chapter.AnchorID != "" {
		return "#" + chapter.AnchorID
	}
	return ""
}
