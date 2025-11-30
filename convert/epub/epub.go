package epub

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/beevik/etree"
	fixzip "github.com/hidez8891/zip"
	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
	"fbc/fb2"
	"fbc/state"
)

const (
	mimetypeContent = "application/epub+zip"
	oebpsDir        = "OEBPS"
	imagesDir       = "images"
)

type chapterData struct {
	ID             string
	Filename       string
	Title          string
	Doc            *etree.Document
	Section        *fb2.Section // Reference to source section for TOC hierarchy
	FootnoteBodies []*fb2.Body  // Footnote bodies for TOC (each body gets separate entry)
}

// idToFileMap maps element IDs to the chapter filename containing them
type idToFileMap map[string]string

// Generate creates the EPUB output file.
// It handles epub2, epub3, and kepub variants based on content.OutputFormat.
func Generate(ctx context.Context, c *content.Content, outputPath string, cfg *config.DocumentConfig, log *zap.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	env := state.EnvFromContext(ctx)

	if _, err := os.Stat(outputPath); err == nil {
		if !env.Overwrite {
			return fmt.Errorf("output file already exists: %s", outputPath)
		}
		log.Warn("Overwriting existing file", zap.String("file", outputPath))
		if err = os.Remove(outputPath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	} else if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("unable to create output directory: %w", err)
	}

	log.Info("Generating EPUB", zap.Stringer("format", c.OutputFormat), zap.String("output", outputPath))

	_, tmpName := filepath.Split(outputPath)
	tmpName = filepath.Join(c.WorkDir, tmpName)

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("unable to create output directory: %w", err)
	}

	f, err := os.Create(tmpName)
	if err != nil {
		return fmt.Errorf("unable to create output file: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	if err := writeMimetype(zw); err != nil {
		return fmt.Errorf("unable to write mimetype: %w", err)
	}

	if err := writeContainer(zw); err != nil {
		return fmt.Errorf("unable to write container: %w", err)
	}

	chapters, idToFile, err := convertToXHTML(ctx, c, log)
	if err != nil {
		return fmt.Errorf("unable to convert content: %w", err)
	}

	// Fix internal links to include chapter filenames
	fixInternalLinks(chapters, idToFile, log)

	for _, chapter := range chapters {
		if chapter.Doc == nil {
			continue // Skip chapters without documents (e.g., additional footnote body TOC entries)
		}
		if err := writeXHTMLChapter(zw, &chapter, log); err != nil {
			return fmt.Errorf("unable to write chapter %s: %w", chapter.ID, err)
		}
	}

	if err := writeImages(zw, c.ImagesIndex, log); err != nil {
		return fmt.Errorf("unable to write images: %w", err)
	}

	if err := writeStylesheet(zw, c, env.DefaultStyle); err != nil {
		return fmt.Errorf("unable to write stylesheet: %w", err)
	}

	if c.CoverID != "" {
		if err := writeCoverPage(zw, c, cfg, log); err != nil {
			return fmt.Errorf("unable to write cover page: %w", err)
		}
	}

	if err := writeOPF(zw, c, chapters, log); err != nil {
		return fmt.Errorf("unable to write OPF: %w", err)
	}

	switch c.OutputFormat {
	case config.OutputFmtEpub2, config.OutputFmtKepub:
		if err := writeNCX(zw, c, chapters, log); err != nil {
			return fmt.Errorf("unable to write NCX: %w", err)
		}
	case config.OutputFmtEpub3:
		if err := writeNav(zw, c, chapters, log); err != nil {
			return fmt.Errorf("unable to write NAV: %w", err)
		}
	}

	// make sure buffers are flushed before continuing
	if err := zw.Close(); err != nil {
		return fmt.Errorf("unable to close output archive: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("unable to finalize output file: %w", err)
	}
	// clean temporary file
	defer os.Remove(tmpName)

	if cfg.FixZip {
		return copyZipWithoutDataDescriptors(tmpName, outputPath)
	}
	return copyFile(tmpName, outputPath)
}

func copyZipWithoutDataDescriptors(from, to string) error {

	out, err := os.Create(to)
	if err != nil {
		return fmt.Errorf("unable to create target file (%s): %w", to, err)
	}
	defer out.Close()

	r, err := fixzip.OpenReader(from)
	if err != nil {
		return fmt.Errorf("unable to read archive file (%s): %w", from, err)
	}
	defer r.Close()

	w := fixzip.NewWriter(out)
	defer w.Close()

	for _, file := range r.File {
		// unset data descriptor flag.
		file.Flags &= ^fixzip.FlagDataDescriptor

		// copy zip entry
		if err := w.CopyFile(file); err != nil {
			return fmt.Errorf("unable to write target file (%s): %w", to, err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {

	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destinationFile.Close()

	if _, err = io.Copy(destinationFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	if err = destinationFile.Close(); err != nil {
		return fmt.Errorf("failed to close destination file: %w", err)
	}
	return nil
}

func writeMimetype(zw *zip.Writer) error {
	w, err := zw.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	})
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, mimetypeContent)
	return err
}

func writeContainer(zw *zip.Writer) error {
	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)

	container := doc.CreateElement("container")
	container.CreateAttr("version", "1.0")
	container.CreateAttr("xmlns", "urn:oasis:names:tc:opendocument:xmlns:container")

	rootfiles := container.CreateElement("rootfiles")
	rootfile := rootfiles.CreateElement("rootfile")
	rootfile.CreateAttr("full-path", path.Join(oebpsDir, "content.opf"))
	rootfile.CreateAttr("media-type", "application/oebps-package+xml")

	return writeXMLToZip(zw, "META-INF/container.xml", doc)
}

func convertToXHTML(ctx context.Context, c *content.Content, log *zap.Logger) ([]chapterData, idToFileMap, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	// First pass: assign sequential IDs to all sections without IDs
	assignSectionIDs(c)
	// Second pass: assign sequential IDs to all chapter-level subtitles without IDs
	assignSubtitleIDs(c)

	var chapters []chapterData
	chapterNum := 0
	idToFile := make(idToFileMap)
	var footnoteBodies []*fb2.Body

	for i := range c.Book.Bodies {
		body := &c.Book.Bodies[i]
		// Process main and other bodies (not footnotes)
		if !body.Footnotes() {
			// If body has title, create a chapter for body intro content
			if body.Title != nil {
				chapterNum++
				chapterID := fmt.Sprintf("index%05d", chapterNum)
				filename := fmt.Sprintf("%s.xhtml", chapterID)

				var title string
				for _, item := range body.Title.Items {
					if item.Paragraph != nil {
						title = paragraphToPlainText(item.Paragraph)
						break
					}
				}
				if title == "" {
					title = "Untitled"
				}

				doc, err := bodyIntroToXHTML(c, body, title, log)
				if err != nil {
					log.Error("Unable to convert body intro", zap.Error(err))
				} else {
					chapters = append(chapters, chapterData{
						ID:       chapterID,
						Filename: filename,
						Title:    title,
						Doc:      doc,
					})
					collectIDsFromBody(body, filename, idToFile)
				}
			}

			// Process only top-level sections as chapters
			// Unwrap sections without titles (grouping sections)
			topSections := collectTopSections(body.Sections)
			for _, section := range topSections {
				if err := ctx.Err(); err != nil {
					return nil, nil, err
				}

				chapterNum++
				chapterID := fmt.Sprintf("index%05d", chapterNum)
				filename := fmt.Sprintf("%s.xhtml", chapterID)
				title := extractTitleText(section)

				doc, err := sectionToXHTML(c, section, title, log)
				if err != nil {
					log.Error("Unable to convert section", zap.Error(err))
					continue
				}

				chapters = append(chapters, chapterData{
					ID:       chapterID,
					Filename: filename,
					Title:    title,
					Doc:      doc,
					Section:  section,
				})
				collectIDsFromSection(section, filename, idToFile)
			}
		} else {
			// Collect footnote bodies for later processing
			footnoteBodies = append(footnoteBodies, body)
		}
	}

	// Process all footnote bodies - each body becomes a separate top-level chapter
	if len(footnoteBodies) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}

		chapterNum++
		chapterID := fmt.Sprintf("index%05d", chapterNum)
		filename := "footnotes.xhtml"

		// Extract title from first footnote body for the document
		var docTitle string
		if footnoteBodies[0].Title != nil {
			for _, item := range footnoteBodies[0].Title.Items {
				if item.Paragraph != nil {
					docTitle = paragraphToPlainText(item.Paragraph)
					break
				}
			}
		}
		if docTitle == "" {
			docTitle = "Notes"
		}

		doc, err := footnotesBodiesToXHTML(c, footnoteBodies, docTitle, log)
		if err != nil {
			log.Error("Unable to convert footnotes", zap.Error(err))
		} else {
			// Collect IDs from all footnote bodies
			for _, body := range footnoteBodies {
				for i := range body.Sections {
					collectIDsFromSection(&body.Sections[i], filename, idToFile)
				}
			}

			// Create separate chapter entries for each footnote body for TOC
			for bodyIdx, body := range footnoteBodies {
				chapterNum++
				bodyChapterID := fmt.Sprintf("index%05d", chapterNum)

				// Extract title from body
				var bodyTitle string
				if body.Title != nil {
					for _, item := range body.Title.Items {
						if item.Paragraph != nil {
							bodyTitle = paragraphToPlainText(item.Paragraph)
							break
						}
					}
				}
				if bodyTitle == "" {
					bodyTitle = "Notes"
				}

				// Generate body ID matching what we used in HTML
				bodyID := fmt.Sprintf("footnote-body-%d", bodyIdx)
				if body.Name != "" {
					bodyID = body.Name
				}

				chapters = append(chapters, chapterData{
					ID:       bodyChapterID,
					Filename: filename + "#" + bodyID,
					Title:    bodyTitle,
					Doc:      nil, // Only first entry has the doc
				})
			}

			// Store the doc in the first footnote body chapter
			if len(chapters) > 0 {
				// Find the first footnote chapter we just added
				chapters[len(chapters)-len(footnoteBodies)].Doc = doc
				chapters[len(chapters)-len(footnoteBodies)].ID = chapterID
				chapters[len(chapters)-len(footnoteBodies)].Filename = filename
			}
		}
	}

	return chapters, idToFile, nil
}

// assignSectionIDs assigns sequential IDs to all sections that don't have IDs
func assignSectionIDs(c *content.Content) {
	counter := 0
	for i := range c.Book.Bodies {
		assignBodySectionIDs(&c.Book.Bodies[i], c.GeneratedIDs, &counter)
	}
}

// assignBodySectionIDs recursively assigns IDs to sections in a body
func assignBodySectionIDs(body *fb2.Body, idMap map[*fb2.Section]string, counter *int) {
	for i := range body.Sections {
		assignSectionIDRecursive(&body.Sections[i], idMap, counter)
	}
}

// assignSectionIDRecursive recursively assigns IDs to a section and its children
func assignSectionIDRecursive(section *fb2.Section, idMap map[*fb2.Section]string, counter *int) {
	// Only assign ID if section doesn't have one
	if section.ID == "" {
		*counter++
		idMap[section] = fmt.Sprintf("sect_%d", *counter)
	}

	// Process child sections
	for i := range section.Content {
		if section.Content[i].Kind == fb2.FlowSection && section.Content[i].Section != nil {
			assignSectionIDRecursive(section.Content[i].Section, idMap, counter)
		}
	}
}

// assignSubtitleIDs assigns sequential IDs to chapter-level subtitles that don't have IDs
func assignSubtitleIDs(c *content.Content) {
	counter := 0
	for i := range c.Book.Bodies {
		assignBodySubtitleIDs(&c.Book.Bodies[i], c.GeneratedSubtitles, &counter)
	}
}

// assignBodySubtitleIDs recursively assigns IDs to subtitles in a body
func assignBodySubtitleIDs(body *fb2.Body, idMap map[*fb2.Paragraph]string, counter *int) {
	for i := range body.Sections {
		assignSectionSubtitleIDs(&body.Sections[i], idMap, counter)
	}
}

// assignSectionSubtitleIDs recursively assigns IDs to subtitles at section level (not in poems/cites/etc.)
func assignSectionSubtitleIDs(section *fb2.Section, idMap map[*fb2.Paragraph]string, counter *int) {
	// Process content items
	for i := range section.Content {
		if section.Content[i].Kind == fb2.FlowSubtitle && section.Content[i].Subtitle != nil && section.Content[i].Subtitle.ID == "" {
			// Subtitle at section level without ID - assign one
			*counter++
			idMap[section.Content[i].Subtitle] = fmt.Sprintf("subtitle_%d", *counter)
		} else if section.Content[i].Kind == fb2.FlowSection && section.Content[i].Section != nil {
			// Recurse into nested sections
			assignSectionSubtitleIDs(section.Content[i].Section, idMap, counter)
		}
	}
}

// collectTopSections recursively unwraps sections without titles (grouping sections)
// and returns only sections with titles that should become chapters
func collectTopSections(sections []fb2.Section) []*fb2.Section {
	var result []*fb2.Section
	for i := range sections {
		section := &sections[i]
		if section.Title != nil {
			// This section has a title, it's a real chapter
			result = append(result, section)
		} else {
			// This section has no title, it's a grouping section
			// Look for nested sections inside it
			nestedSections := extractNestedSections(section)
			result = append(result, nestedSections...)
		}
	}
	return result
}

// extractNestedSections extracts sections from within a section's content
func extractNestedSections(section *fb2.Section) []*fb2.Section {
	var result []*fb2.Section
	for i := range section.Content {
		if section.Content[i].Kind == fb2.FlowSection && section.Content[i].Section != nil {
			nested := section.Content[i].Section
			if nested.Title != nil {
				// Found a section with title
				result = append(result, nested)
			} else {
				// Recursively unwrap this section too
				result = append(result, extractNestedSections(nested)...)
			}
		}
	}
	return result
}

func extractTitleText(section *fb2.Section) string {
	if section.Title != nil {
		text := extractTOCText(section.Title.Items)
		if text != "" {
			return text
		}
	}
	// Fallback to ID with tildes
	if section.ID != "" {
		return "~ " + section.ID + " ~"
	}
	return "Chapter"
}

// extractTOCText extracts text from title items for TOC display
// Priority: 1) plain text, 2) image alt attributes, 3) empty
func extractTOCText(items []fb2.TitleItem) string {
	var buf strings.Builder
	var imageAltBuf strings.Builder
	hasText := false

	for _, item := range items {
		if item.Paragraph != nil {
			text := paragraphToPlainText(item.Paragraph)
			if text != "" {
				if buf.Len() > 0 {
					buf.WriteString(" ")
				}
				buf.WriteString(text)
				hasText = true
			}

			// Also collect image alt text as fallback
			imageAlt := extractImageAltFromParagraph(item.Paragraph)
			if imageAlt != "" {
				if imageAltBuf.Len() > 0 {
					imageAltBuf.WriteString(" ")
				}
				imageAltBuf.WriteString(imageAlt)
			}
		}
	}

	if hasText {
		return strings.TrimSpace(buf.String())
	}

	// Fallback to image alt text if no plain text
	imageAltText := strings.TrimSpace(imageAltBuf.String())
	if imageAltText != "" {
		return imageAltText
	}

	return ""
}

// extractImageAltFromParagraph extracts alt text from all images in a paragraph
func extractImageAltFromParagraph(p *fb2.Paragraph) string {
	var buf strings.Builder
	for _, seg := range p.Text {
		alt := extractImageAltFromSegment(&seg)
		if alt != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(alt)
		}
	}
	return strings.TrimSpace(buf.String())
}

// extractImageAltFromSegment recursively extracts alt text from inline images
func extractImageAltFromSegment(seg *fb2.InlineSegment) string {
	if seg.Kind == fb2.InlineImageSegment && seg.Image != nil && seg.Image.Alt != "" {
		return seg.Image.Alt
	}

	// Recurse into children
	var buf strings.Builder
	for _, child := range seg.Children {
		alt := extractImageAltFromSegment(&child)
		if alt != "" {
			if buf.Len() > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(alt)
		}
	}
	return strings.TrimSpace(buf.String())
}

// extractSubtitleText extracts text from a subtitle paragraph for TOC display
// Priority: 1) plain text, 2) image alt attributes, 3) ID with tildes
func extractSubtitleText(subtitle *fb2.Paragraph, subtitleID string) string {
	// Try plain text first
	text := paragraphToPlainText(subtitle)
	if text != "" {
		return text
	}

	// Try image alt text as fallback
	imageAlt := extractImageAltFromParagraph(subtitle)
	if imageAlt != "" {
		return imageAlt
	}

	// Last resort: use ID with tildes
	return "~ " + subtitleID + " ~"
}

func paragraphToPlainText(p *fb2.Paragraph) string {
	var buf strings.Builder
	for _, seg := range p.Text {
		buf.WriteString(inlineSegmentToText(&seg))
	}
	return strings.TrimSpace(buf.String())
}

func inlineSegmentToText(seg *fb2.InlineSegment) string {
	// Skip link elements completely for TOC text
	if seg.Kind == fb2.InlineLink {
		return ""
	}

	var buf strings.Builder
	buf.WriteString(seg.Text)
	for _, child := range seg.Children {
		buf.WriteString(inlineSegmentToText(&child))
	}
	return buf.String()
}

func bodyIntroToXHTML(c *content.Content, body *fb2.Body, title string, log *zap.Logger) (*etree.Document, error) {
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

	bodyElem := html.CreateElement("body")

	if err := writeBodyIntroContent(bodyElem, c, body, 1, log); err != nil {
		return nil, err
	}

	return doc, nil
}

func sectionToXHTML(c *content.Content, section *fb2.Section, title string, log *zap.Logger) (*etree.Document, error) {
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
	// Add section ID to body element so it can be a link target
	if section.ID != "" {
		body.CreateAttr("id", section.ID)
	}

	if err := writeSectionContent(body, c, section, false, 1, log); err != nil {
		return nil, err
	}

	return doc, nil
}

func footnotesBodiesToXHTML(c *content.Content, bodies []*fb2.Body, title string, log *zap.Logger) (*etree.Document, error) {
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

	bodyElem := html.CreateElement("body")

	// Process all footnote bodies
	for bodyIdx, body := range bodies {
		// Create a wrapper div for each body with an ID
		bodyDiv := bodyElem.CreateElement("div")
		bodyDiv.CreateAttr("class", "footnote-body")
		// Generate an ID for this body for TOC linking
		bodyID := fmt.Sprintf("footnote-body-%d", bodyIdx)
		if body.Name != "" {
			bodyID = body.Name
		}
		bodyDiv.CreateAttr("id", bodyID)

		// Write body intro content if it has a title
		if body.Title != nil {
			if err := writeBodyIntroContent(bodyDiv, c, body, 1, log); err != nil {
				return nil, err
			}
		}

		// Write all sections from this body
		for i := range body.Sections {
			section := &body.Sections[i]
			// Create a div wrapper for each footnote section to preserve structure
			div := bodyDiv.CreateElement("div")
			div.CreateAttr("class", "section")
			if section.ID != "" {
				div.CreateAttr("id", section.ID)
			}
			if section.Lang != "" {
				div.CreateAttr("xml:lang", section.Lang)
			}

			if err := writeFootnoteSectionContent(div, c, section, false, log); err != nil {
				return nil, err
			}
		}
	}

	return doc, nil
}

func writeBodyIntroContent(parent *etree.Element, c *content.Content, body *fb2.Body, depth int, log *zap.Logger) error {
	if body.Title != nil {
		headingLevel := min(depth, 6)
		headingTag := fmt.Sprintf("h%d", headingLevel)

		titleElem := parent.CreateElement(headingTag)
		titleElem.CreateAttr("class", "title")
		firstParagraph := true
		prevWasEmptyLine := false
		for i, item := range body.Title.Items {
			if item.Paragraph != nil {
				// Add <br> before non-first paragraphs to separate them, but not if previous was empty line
				if i > 0 && !prevWasEmptyLine {
					br := titleElem.CreateElement("br")
					br.CreateAttr("class", "title-paragraph")
				}
				span := titleElem.CreateElement("span")
				if item.Paragraph.ID != "" {
					span.CreateAttr("id", item.Paragraph.ID)
				}
				var class string
				if firstParagraph {
					class = "title-first"
					firstParagraph = false
				} else {
					class = "title-next"
				}
				if item.Paragraph.Style != "" {
					class = class + " " + item.Paragraph.Style
				}
				span.CreateAttr("class", class)
				writeParagraphInline(span, c, item.Paragraph)
				prevWasEmptyLine = false
			} else if item.EmptyLine {
				br := titleElem.CreateElement("br")
				br.CreateAttr("class", "title-emptyline")
				prevWasEmptyLine = true
			}
		}
	}

	for _, epigraph := range body.Epigraphs {
		div := parent.CreateElement("div")
		div.CreateAttr("class", "epigraph")
		if err := writeFlowContent(div, c, &epigraph.Flow, depth, log); err != nil {
			return err
		}
		for _, ta := range epigraph.TextAuthors {
			p := div.CreateElement("p")
			p.CreateAttr("class", "text-author")
			writeParagraphInline(p, c, &ta)
		}
	}

	if body.Image != nil {
		writeImageElement(parent, c, body.Image)
	}

	return nil
}

func writeFootnoteSectionContent(parent *etree.Element, c *content.Content, section *fb2.Section, skipTitle bool, log *zap.Logger) error {
	if section.Title != nil && !skipTitle {
		titleDiv := parent.CreateElement("div")
		titleDiv.CreateAttr("class", "footnote-title")
		firstParagraph := true
		for i, item := range section.Title.Items {
			if item.Paragraph != nil {
				p := titleDiv.CreateElement("p")
				if item.Paragraph.ID != "" {
					p.CreateAttr("id", item.Paragraph.ID)
				}
				var class string
				if firstParagraph {
					class = "footnote-title-first"
					firstParagraph = false
				} else {
					class = "footnote-title-next"
				}
				if item.Paragraph.Style != "" {
					class = class + " " + item.Paragraph.Style
				}
				p.CreateAttr("class", class)
				writeParagraphInline(p, c, item.Paragraph)
			} else if item.EmptyLine {
				if i > 0 && i < len(section.Title.Items)-1 {
					p := titleDiv.CreateElement("p")
					p.CreateAttr("class", "footnote-title-emptyline")
				}
			}
		}
	}

	for _, epigraph := range section.Epigraphs {
		div := parent.CreateElement("div")
		div.CreateAttr("class", "epigraph")
		if err := writeFlowContentInEpigraph(div, c, &epigraph.Flow, 1, log); err != nil {
			return err
		}
		for _, ta := range epigraph.TextAuthors {
			p := div.CreateElement("p")
			p.CreateAttr("class", "text-author")
			writeParagraphInline(p, c, &ta)
		}
	}

	if section.Image != nil {
		writeImageElement(parent, c, section.Image)
	}

	if section.Annotation != nil {
		div := parent.CreateElement("div")
		div.CreateAttr("class", "annotation")
		if err := writeFlowContentInAnnotation(div, c, section.Annotation, 1, log); err != nil {
			return err
		}
	}

	if err := writeFlowItems(parent, c, section.Content, 1, log); err != nil {
		return err
	}

	return nil
}

func writeSectionContent(parent *etree.Element, c *content.Content, section *fb2.Section, skipTitle bool, depth int, log *zap.Logger) error {
	if section.Title != nil && !skipTitle {
		headingLevel := min(depth, 6)
		headingTag := fmt.Sprintf("h%d", headingLevel)

		titleElem := parent.CreateElement(headingTag)
		titleElem.CreateAttr("class", "title")
		firstParagraph := true
		prevWasEmptyLine := false
		for i, item := range section.Title.Items {
			if item.Paragraph != nil {
				// Add <br> before non-first paragraphs to separate them, but not if previous was empty line
				if i > 0 && !prevWasEmptyLine {
					br := titleElem.CreateElement("br")
					br.CreateAttr("class", "title-paragraph")
				}
				span := titleElem.CreateElement("span")
				if item.Paragraph.ID != "" {
					span.CreateAttr("id", item.Paragraph.ID)
				}
				var class string
				if firstParagraph {
					class = "title-first"
					firstParagraph = false
				} else {
					class = "title-next"
				}
				if item.Paragraph.Style != "" {
					class = class + " " + item.Paragraph.Style
				}
				span.CreateAttr("class", class)
				writeParagraphInline(span, c, item.Paragraph)
				prevWasEmptyLine = false
			} else if item.EmptyLine {
				br := titleElem.CreateElement("br")
				br.CreateAttr("class", "title-emptyline")
				prevWasEmptyLine = true
			}
		}
	}

	for _, epigraph := range section.Epigraphs {
		div := parent.CreateElement("div")
		div.CreateAttr("class", "epigraph")
		if err := writeFlowContentInEpigraph(div, c, &epigraph.Flow, depth, log); err != nil {
			return err
		}
		for _, ta := range epigraph.TextAuthors {
			p := div.CreateElement("p")
			p.CreateAttr("class", "text-author")
			writeParagraphInline(p, c, &ta)
		}
	}

	if section.Image != nil {
		writeImageElement(parent, c, section.Image)
	}

	if section.Annotation != nil {
		div := parent.CreateElement("div")
		div.CreateAttr("class", "annotation")
		if err := writeFlowContentInAnnotation(div, c, section.Annotation, depth, log); err != nil {
			return err
		}
	}

	if err := writeFlowItems(parent, c, section.Content, depth, log); err != nil {
		return err
	}

	return nil
}

func writeFlowContent(parent *etree.Element, c *content.Content, flow *fb2.Flow, depth int, log *zap.Logger) error {
	return writeFlowItems(parent, c, flow.Items, depth, log)
}

func writeFlowContentInEpigraph(parent *etree.Element, c *content.Content, flow *fb2.Flow, depth int, log *zap.Logger) error {
	return writeFlowItemsWithContext(parent, c, flow.Items, depth, "epigraph", log)
}

func writeFlowContentInAnnotation(parent *etree.Element, c *content.Content, flow *fb2.Flow, depth int, log *zap.Logger) error {
	return writeFlowItemsWithContext(parent, c, flow.Items, depth, "annotation", log)
}

func writeFlowContentInCite(parent *etree.Element, c *content.Content, items []fb2.FlowItem, depth int, log *zap.Logger) error {
	return writeFlowItemsWithContext(parent, c, items, depth, "cite", log)
}

// getSectionID returns the ID for a section, using the generated ID if the section doesn't have one
func getSectionID(section *fb2.Section, generatedIDs map[*fb2.Section]string) string {
	if section.ID != "" {
		return section.ID
	}
	if id, exists := generatedIDs[section]; exists {
		return id
	}
	// Should not happen if assignSectionIDs was called, but provide fallback
	return "section-unknown"
}

// getSubtitleID returns the ID for a subtitle, using the generated ID if the subtitle doesn't have one
func getSubtitleID(subtitle *fb2.Paragraph, generatedIDs map[*fb2.Paragraph]string) string {
	if subtitle.ID != "" {
		return subtitle.ID
	}
	if id, exists := generatedIDs[subtitle]; exists {
		return id
	}
	// Should not happen if assignSubtitleIDs was called, but provide fallback
	return "subtitle-unknown"
}

// isGroupingSection checks if a section is a pure grouping container
// (no content except nested sections)
func isGroupingSection(section *fb2.Section) bool {
	if section.Title != nil || section.Image != nil || section.Annotation != nil || len(section.Epigraphs) > 0 {
		return false
	}
	// Must have at least one section child
	if len(section.Content) == 0 {
		return false
	}
	// Check if all content items are sections
	for _, item := range section.Content {
		if item.Kind != fb2.FlowSection {
			return false
		}
	}
	return true
}

func writeFlowItems(parent *etree.Element, c *content.Content, items []fb2.FlowItem, depth int, log *zap.Logger) error {
	return writeFlowItemsWithContext(parent, c, items, depth, "", log)
}

func writeFlowItemsWithContext(parent *etree.Element, c *content.Content, items []fb2.FlowItem, depth int, context string, log *zap.Logger) error {
	for _, item := range items {
		switch item.Kind {
		case fb2.FlowParagraph:
			if item.Paragraph != nil {
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
				writeParagraphInline(p, c, item.Paragraph)
			}
		case fb2.FlowImage:
			if item.Image != nil {
				writeImageElement(parent, c, item.Image)
			}
		case fb2.FlowEmptyLine:
			parent.CreateElement("br")
		case fb2.FlowSubtitle:
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
					writeParagraphInline(p, c, item.Subtitle)
				} else {
					// In section - use header (one level more than enclosing title)
					headingLevel := min(depth+1, 6)
					headingTag := fmt.Sprintf("h%d", headingLevel)
					h := parent.CreateElement(headingTag)
					// Always set ID (use generated if original doesn't exist)
					subtitleID := getSubtitleID(item.Subtitle, c.GeneratedSubtitles)
					h.CreateAttr("id", subtitleID)
					if item.Subtitle.Lang != "" {
						h.CreateAttr("xml:lang", item.Subtitle.Lang)
					}
					class = "subtitle"
					if item.Subtitle.Style != "" {
						class = class + " " + item.Subtitle.Style
					}
					h.CreateAttr("class", class)
					writeParagraphInline(h, c, item.Subtitle)
				}
			}
		case fb2.FlowPoem:
			if item.Poem != nil {
				writePoemElement(parent, c, item.Poem, depth, log)
			}
		case fb2.FlowCite:
			if item.Cite != nil {
				writeCiteElement(parent, c, item.Cite, depth, log)
			}
		case fb2.FlowTable:
			if item.Table != nil {
				writeTableElement(parent, c, item.Table)
			}
		case fb2.FlowSection:
			if item.Section != nil {
				// Check if this is a pure grouping section (no title, no content except nested sections)
				if item.Section.Title == nil && isGroupingSection(item.Section) {
					// Transparent grouping - write children directly without wrapper
					if err := writeFlowItems(parent, c, item.Section.Content, depth, log); err != nil {
						return err
					}
				} else {
					// Regular section - create wrapper div
					div := parent.CreateElement("div")
					div.CreateAttr("class", "section")
					// Always add an ID for sections so TOC links work
					sectionID := getSectionID(item.Section, c.GeneratedIDs)
					div.CreateAttr("id", sectionID)
					if item.Section.Lang != "" {
						div.CreateAttr("xml:lang", item.Section.Lang)
					}
					if err := writeSectionContent(div, c, item.Section, false, depth+1, log); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func writeParagraphInline(parent *etree.Element, c *content.Content, p *fb2.Paragraph) {
	for _, seg := range p.Text {
		writeInlineSegment(parent, c, &seg)
	}
}

func writeInlineSegment(parent *etree.Element, c *content.Content, seg *fb2.InlineSegment) {
	switch seg.Kind {
	case fb2.InlineText:
		if parent.ChildElements() == nil || len(parent.ChildElements()) == 0 {
			parent.SetText(seg.Text)
		} else {
			lastChild := parent.ChildElements()[len(parent.ChildElements())-1]
			lastChild.SetTail(lastChild.Tail() + seg.Text)
		}
	case fb2.InlineStrong:
		strong := parent.CreateElement("strong")
		for _, child := range seg.Children {
			writeInlineSegment(strong, c, &child)
		}
	case fb2.InlineEmphasis:
		em := parent.CreateElement("em")
		for _, child := range seg.Children {
			writeInlineSegment(em, c, &child)
		}
	case fb2.InlineStrikethrough:
		del := parent.CreateElement("del")
		for _, child := range seg.Children {
			writeInlineSegment(del, c, &child)
		}
	case fb2.InlineSub:
		sub := parent.CreateElement("sub")
		for _, child := range seg.Children {
			writeInlineSegment(sub, c, &child)
		}
	case fb2.InlineSup:
		sup := parent.CreateElement("sup")
		for _, child := range seg.Children {
			writeInlineSegment(sup, c, &child)
		}
	case fb2.InlineCode:
		code := parent.CreateElement("code")
		for _, child := range seg.Children {
			writeInlineSegment(code, c, &child)
		}
	case fb2.InlineNamedStyle:
		span := parent.CreateElement("span")
		if seg.Style != "" {
			span.CreateAttr("class", seg.Style)
		}
		for _, child := range seg.Children {
			writeInlineSegment(span, c, &child)
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
			writeInlineSegment(a, c, &child)
		}
	case fb2.InlineImageSegment:
		if seg.Image != nil {
			img := parent.CreateElement("img")
			img.CreateAttr("class", "inline-image")
			imgID := strings.TrimPrefix(seg.Image.Href, "#")
			if imgData, ok := c.ImagesIndex[imgID]; ok {
				img.CreateAttr("src", path.Join(imagesDir, imgData.Filename))
			} else {
				img.CreateAttr("src", path.Join(imagesDir, imgID))
			}
			if seg.Image.Alt != "" {
				img.CreateAttr("alt", seg.Image.Alt)
			}
		}
	}
}

func writeImageElement(parent *etree.Element, c *content.Content, img *fb2.Image) {
	div := parent.CreateElement("div")
	div.CreateAttr("class", "image")
	if img.ID != "" {
		div.CreateAttr("id", img.ID)
	}

	imgElem := div.CreateElement("img")
	imgElem.CreateAttr("class", "block-image")
	imgID := strings.TrimPrefix(img.Href, "#")
	if imgData, ok := c.ImagesIndex[imgID]; ok {
		imgElem.CreateAttr("src", path.Join(imagesDir, imgData.Filename))
	} else {
		imgElem.CreateAttr("src", path.Join(imagesDir, imgID))
	}
	if img.Alt != "" {
		imgElem.CreateAttr("alt", img.Alt)
	}
	if img.Title != "" {
		imgElem.CreateAttr("title", img.Title)
	}
}

func writePoemElement(parent *etree.Element, c *content.Content, poem *fb2.Poem, depth int, log *zap.Logger) {
	div := parent.CreateElement("div")
	div.CreateAttr("class", "poem")
	if poem.ID != "" {
		div.CreateAttr("id", poem.ID)
	}
	if poem.Lang != "" {
		div.CreateAttr("xml:lang", poem.Lang)
	}

	if poem.Title != nil {
		titleDiv := div.CreateElement("div")
		titleDiv.CreateAttr("class", "poem-title")
		firstParagraph := true
		for _, item := range poem.Title.Items {
			if item.Paragraph != nil {
				p := titleDiv.CreateElement("p")
				if item.Paragraph.ID != "" {
					p.CreateAttr("id", item.Paragraph.ID)
				}
				var class string
				if firstParagraph {
					class = "poem-title-first"
					firstParagraph = false
				} else {
					class = "poem-title-next"
				}
				if item.Paragraph.Style != "" {
					class = class + " " + item.Paragraph.Style
				}
				p.CreateAttr("class", class)
				writeParagraphInline(p, c, item.Paragraph)
			} else if item.EmptyLine {
				titleDiv.CreateElement("br")
			}
		}
	}

	for _, epigraph := range poem.Epigraphs {
		epigraphDiv := div.CreateElement("div")
		epigraphDiv.CreateAttr("class", "epigraph")
		if err := writeFlowContentInEpigraph(epigraphDiv, c, &epigraph.Flow, depth, log); err != nil {
			log.Warn("Error writing poem epigraph content", zap.Error(err))
		}
		for _, ta := range epigraph.TextAuthors {
			p := epigraphDiv.CreateElement("p")
			p.CreateAttr("class", "text-author")
			writeParagraphInline(p, c, &ta)
		}
	}

	for _, subtitle := range poem.Subtitles {
		p := div.CreateElement("p")
		p.CreateAttr("class", "poem-subtitle")
		if subtitle.ID != "" {
			p.CreateAttr("id", subtitle.ID)
		}
		writeParagraphInline(p, c, &subtitle)
	}

	for _, stanza := range poem.Stanzas {
		stanzaDiv := div.CreateElement("div")
		stanzaDiv.CreateAttr("class", "stanza")
		if stanza.Lang != "" {
			stanzaDiv.CreateAttr("xml:lang", stanza.Lang)
		}

		if stanza.Title != nil {
			stanzaTitleDiv := stanzaDiv.CreateElement("div")
			stanzaTitleDiv.CreateAttr("class", "stanza-title")
			firstParagraph := true
			for _, item := range stanza.Title.Items {
				if item.Paragraph != nil {
					p := stanzaTitleDiv.CreateElement("p")
					if item.Paragraph.ID != "" {
						p.CreateAttr("id", item.Paragraph.ID)
					}
					var class string
					if firstParagraph {
						class = "stanza-title-first"
						firstParagraph = false
					} else {
						class = "stanza-title-next"
					}
					if item.Paragraph.Style != "" {
						class = class + " " + item.Paragraph.Style
					}
					p.CreateAttr("class", class)
					writeParagraphInline(p, c, item.Paragraph)
				} else if item.EmptyLine {
					stanzaTitleDiv.CreateElement("br")
				}
			}
		}

		if stanza.Subtitle != nil {
			p := stanzaDiv.CreateElement("p")
			p.CreateAttr("class", "stanza-subtitle")
			if stanza.Subtitle.ID != "" {
				p.CreateAttr("id", stanza.Subtitle.ID)
			}
			writeParagraphInline(p, c, stanza.Subtitle)
		}

		for _, verse := range stanza.Verses {
			p := stanzaDiv.CreateElement("p")
			p.CreateAttr("class", "verse")
			writeParagraphInline(p, c, &verse)
		}
	}

	for _, ta := range poem.TextAuthors {
		p := div.CreateElement("p")
		p.CreateAttr("class", "text-author")
		writeParagraphInline(p, c, &ta)
	}

	if poem.Date != nil {
		p := div.CreateElement("p")
		p.CreateAttr("class", "date")
		if poem.Date.Display != "" {
			p.SetText(poem.Date.Display)
		} else if !poem.Date.Value.IsZero() {
			p.SetText(poem.Date.Value.Format("2006-01-02"))
		}
	}
}

func writeCiteElement(parent *etree.Element, c *content.Content, cite *fb2.Cite, depth int, log *zap.Logger) {
	blockquote := parent.CreateElement("blockquote")
	if cite.ID != "" {
		blockquote.CreateAttr("id", cite.ID)
	}
	if cite.Lang != "" {
		blockquote.CreateAttr("xml:lang", cite.Lang)
	}
	blockquote.CreateAttr("class", "cite")

	if err := writeFlowContentInCite(blockquote, c, cite.Items, depth, log); err != nil {
		log.Warn("Error writing cite content", zap.Error(err))
	}

	for _, ta := range cite.TextAuthors {
		p := blockquote.CreateElement("p")
		p.CreateAttr("class", "text-author")
		writeParagraphInline(p, c, &ta)
	}
}

func writeTableElement(parent *etree.Element, c *content.Content, table *fb2.Table) {
	tableElem := parent.CreateElement("table")
	if table.ID != "" {
		tableElem.CreateAttr("id", table.ID)
	}
	if table.Style != "" {
		tableElem.CreateAttr("class", table.Style)
	}

	for _, row := range table.Rows {
		tr := tableElem.CreateElement("tr")
		if row.Align != "" {
			tr.CreateAttr("align", row.Align)
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
			if cell.Style != "" {
				td.CreateAttr("class", cell.Style)
			}
			if cell.ColSpan > 1 {
				td.CreateAttr("colspan", fmt.Sprintf("%d", cell.ColSpan))
			}
			if cell.RowSpan > 1 {
				td.CreateAttr("rowspan", fmt.Sprintf("%d", cell.RowSpan))
			}
			if cell.Align != "" {
				td.CreateAttr("align", cell.Align)
			}
			if cell.VAlign != "" {
				td.CreateAttr("valign", cell.VAlign)
			}

			for _, seg := range cell.Content {
				writeInlineSegment(td, c, &seg)
			}
		}
	}
}

func writeXHTMLChapter(zw *zip.Writer, chapter *chapterData, _ *zap.Logger) error {
	return writeXMLToZip(zw, filepath.Join(oebpsDir, chapter.Filename), chapter.Doc)
}

func writeImages(zw *zip.Writer, images fb2.BookImages, log *zap.Logger) error {
	for id, img := range images {
		filename := filepath.Join(oebpsDir, imagesDir, img.Filename)

		if err := writeDataToZip(zw, filename, img.Data); err != nil {
			return fmt.Errorf("unable to write image %s: %w", id, err)
		}

		log.Debug("Wrote image", zap.String("id", id), zap.String("file", filename))
	}
	return nil
}

func writeCoverPage(zw *zip.Writer, c *content.Content, cfg *config.DocumentConfig, log *zap.Logger) error {
	if c.CoverID == "" {
		return nil
	}

	coverImage, ok := c.ImagesIndex[c.CoverID]
	if !ok {
		log.Warn("Cover image not found in images index", zap.String("cover_id", c.CoverID))
		return nil
	}

	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)

	html := doc.CreateElement("html")
	html.CreateAttr("xmlns", "http://www.w3.org/1999/xhtml")

	head := html.CreateElement("head")

	meta := head.CreateElement("meta")
	meta.CreateAttr("http-equiv", "Content-Type")
	meta.CreateAttr("content", "text/html; charset=utf-8")

	style := head.CreateElement("style")
	style.CreateAttr("type", "text/css")

	switch cfg.Images.Cover.Resize {
	case config.ImageResizeModeStretch:
		style.SetText("html, body { margin: 0; padding: 0; width:100%; heignt: 100%; } svg { display: block; width: 100%; height: 100%; }")
	case config.ImageResizeModeKeepAR:
		fallthrough
	default:
		style.SetText("html, body { margin: 0; padding: 0; width:100%; heignt: 100%; } svg { display: block; width: auto; height: 100%; margin: 0 auto }")
	}

	title := head.CreateElement("title")
	title.SetText(c.Book.Description.TitleInfo.BookTitle.Value)

	body := html.CreateElement("body")

	svg := body.CreateElement("svg")
	svg.CreateAttr("version", "1.1")
	svg.CreateAttr("xmlns", "http://www.w3.org/2000/svg")
	svg.CreateAttr("xmlns:xlink", "http://www.w3.org/1999/xlink")

	switch cfg.Images.Cover.Resize {
	case config.ImageResizeModeStretch:
		svg.CreateAttr("viewBox", "0 0 100 100")
		svg.CreateAttr("preserveAspectRatio", "xMidYMid slice")
		svgImage := svg.CreateElement("image")
		svgImage.CreateAttr("x", "0")
		svgImage.CreateAttr("y", "0")
		svgImage.CreateAttr("width", "100")
		svgImage.CreateAttr("height", "100")
		svgImage.CreateAttr("xlink:href", "images/"+coverImage.Filename)
	case config.ImageResizeModeKeepAR:
		fallthrough
	default:
		// Use actual image dimensions for ImageResizeModeNone
		w, h := coverImage.Dim.Width, coverImage.Dim.Height
		// Fallback to config values if dimensions are not set
		if w == 0 || h == 0 {
			w, h = cfg.Images.Cover.Width, cfg.Images.Cover.Height
			log.Debug("Cover image dimensions not available, using config values",
				zap.Int("width", w), zap.Int("height", h))
		}
		svg.CreateAttr("viewBox", fmt.Sprintf("0 0 %d %d", w, h))
		svg.CreateAttr("preserveAspectRatio", "xMidYMid meet")
		svgImage := svg.CreateElement("image")
		svgImage.CreateAttr("x", "0")
		svgImage.CreateAttr("y", "0")
		svgImage.CreateAttr("width", fmt.Sprintf("%d", w))
		svgImage.CreateAttr("height", fmt.Sprintf("%d", h))
		svgImage.CreateAttr("xlink:href", "images/"+coverImage.Filename)
	}

	return writeXMLToZip(zw, filepath.Join(oebpsDir, "cover.xhtml"), doc)
}

func writeStylesheet(zw *zip.Writer, c *content.Content, css []byte) error {
	for _, style := range c.Book.Stylesheets {
		if style.Type == "text/css" {
			css = append(css, "\n/* FB2 embedded stylesheet */\n"+style.Data+"\n"...)
		}
	}

	return writeDataToZip(zw, filepath.Join(oebpsDir, "stylesheet.css"), css)
}

func writeOPF(zw *zip.Writer, c *content.Content, chapters []chapterData, _ *zap.Logger) error {
	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)

	pkg := doc.CreateElement("package")
	pkg.CreateAttr("xmlns", "http://www.idpf.org/2007/opf")
	pkg.CreateAttr("unique-identifier", "BookId")

	switch c.OutputFormat {
	case config.OutputFmtEpub2, config.OutputFmtKepub:
		pkg.CreateAttr("version", "2.0")
	case config.OutputFmtEpub3:
		pkg.CreateAttr("version", "3.0")
	}
	metadata := pkg.CreateElement("metadata")
	metadata.CreateAttr("xmlns:dc", "http://purl.org/dc/elements/1.1/")
	metadata.CreateAttr("xmlns:opf", "http://www.idpf.org/2007/opf")

	dcTitle := metadata.CreateElement("dc:title")
	dcTitle.SetText(c.Book.Description.TitleInfo.BookTitle.Value)

	dcIdentifier := metadata.CreateElement("dc:identifier")
	dcIdentifier.CreateAttr("id", "BookId")
	dcIdentifier.SetText(c.Book.Description.DocumentInfo.ID)

	dcLang := metadata.CreateElement("dc:language")
	dcLang.SetText(c.Book.Description.TitleInfo.Lang.String())

	for _, author := range c.Book.Description.TitleInfo.Authors {
		dcCreator := metadata.CreateElement("dc:creator")
		dcCreator.CreateAttr("opf:role", "aut")
		authorName := strings.TrimSpace(fmt.Sprintf("%s %s %s", author.FirstName, author.MiddleName, author.LastName))
		dcCreator.SetText(authorName)
	}

	if c.CoverID != "" {
		meta := metadata.CreateElement("meta")
		meta.CreateAttr("name", "cover")
		meta.CreateAttr("content", "book-cover-image")
	}

	manifest := pkg.CreateElement("manifest")

	switch c.OutputFormat {
	case config.OutputFmtEpub2, config.OutputFmtKepub:
		item := manifest.CreateElement("item")
		item.CreateAttr("id", "ncx")
		item.CreateAttr("href", "toc.ncx")
		item.CreateAttr("media-type", "application/x-dtbncx+xml")
	case config.OutputFmtEpub3:
		item := manifest.CreateElement("item")
		item.CreateAttr("id", "nav")
		item.CreateAttr("href", "nav.xhtml")
		item.CreateAttr("media-type", "application/xhtml+xml")
		item.CreateAttr("properties", "nav")
	}

	cssItem := manifest.CreateElement("item")
	cssItem.CreateAttr("id", "stylesheet")
	cssItem.CreateAttr("href", "stylesheet.css")
	cssItem.CreateAttr("media-type", "text/css")

	if c.CoverID != "" {
		coverPageItem := manifest.CreateElement("item")
		coverPageItem.CreateAttr("id", "cover-page")
		coverPageItem.CreateAttr("href", "cover.xhtml")
		coverPageItem.CreateAttr("media-type", "application/xhtml+xml")
	}

	for _, chapter := range chapters {
		item := manifest.CreateElement("item")
		item.CreateAttr("id", chapter.ID)
		item.CreateAttr("href", chapter.Filename)
		item.CreateAttr("media-type", "application/xhtml+xml")
	}

	for id, img := range c.ImagesIndex {
		item := manifest.CreateElement("item")
		if id == c.CoverID {
			item.CreateAttr("id", "book-cover-image")
			item.CreateAttr("href", "images/"+img.Filename)
			item.CreateAttr("media-type", img.MimeType)
			item.CreateAttr("properties", "cover-image")
		} else {
			item.CreateAttr("id", "img-"+id)
			item.CreateAttr("href", "images/"+img.Filename)
			item.CreateAttr("media-type", img.MimeType)
		}
	}

	spine := pkg.CreateElement("spine")
	if c.OutputFormat == config.OutputFmtEpub2 || c.OutputFormat == config.OutputFmtKepub {
		spine.CreateAttr("toc", "ncx")
	}

	if c.CoverID != "" {
		coverRef := spine.CreateElement("itemref")
		coverRef.CreateAttr("idref", "cover-page")
	}

	for _, chapter := range chapters {
		itemref := spine.CreateElement("itemref")
		itemref.CreateAttr("idref", chapter.ID)
	}

	return writeXMLToZip(zw, filepath.Join(oebpsDir, "content.opf"), doc)
}

func writeNCX(zw *zip.Writer, c *content.Content, chapters []chapterData, _ *zap.Logger) error {
	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)

	ncx := doc.CreateElement("ncx")
	ncx.CreateAttr("xmlns", "http://www.daisy.org/z3986/2005/ncx/")
	ncx.CreateAttr("version", "2005-1")

	head := ncx.CreateElement("head")

	metaUID := head.CreateElement("meta")
	metaUID.CreateAttr("name", "dtb:uid")
	metaUID.CreateAttr("content", c.Book.Description.DocumentInfo.ID)

	// Calculate TOC depth
	maxDepth := 1
	for _, chapter := range chapters {
		if chapter.Section != nil {
			depth := calculateSectionDepth(chapter.Section, 1)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	}

	metaDepth := head.CreateElement("meta")
	metaDepth.CreateAttr("name", "dtb:depth")
	metaDepth.CreateAttr("content", fmt.Sprintf("%d", maxDepth))

	metaTotal := head.CreateElement("meta")
	metaTotal.CreateAttr("name", "dtb:totalPageCount")
	metaTotal.CreateAttr("content", "0")

	metaMax := head.CreateElement("meta")
	metaMax.CreateAttr("name", "dtb:maxPageNumber")
	metaMax.CreateAttr("content", "0")

	docTitle := ncx.CreateElement("docTitle")
	text := docTitle.CreateElement("text")
	text.SetText(c.Book.Description.TitleInfo.BookTitle.Value)

	navMap := ncx.CreateElement("navMap")

	playOrder := 0
	for _, chapter := range chapters {
		playOrder++
		navPoint := navMap.CreateElement("navPoint")
		navPoint.CreateAttr("id", chapter.ID)
		navPoint.CreateAttr("playOrder", fmt.Sprintf("%d", playOrder))

		navLabel := navPoint.CreateElement("navLabel")
		labelText := navLabel.CreateElement("text")
		labelText.SetText(chapter.Title)

		navContent := navPoint.CreateElement("content")
		navContent.CreateAttr("src", chapter.Filename)

		// Add nested sections to TOC
		if chapter.Section != nil {
			buildNCXNavPoints(navPoint, chapter.Section, chapter.Filename, &playOrder, c)
		}
	}

	return writeXMLToZip(zw, filepath.Join(oebpsDir, "toc.ncx"), doc)
}

func calculateSectionDepth(section *fb2.Section, currentDepth int) int {
	maxDepth := currentDepth
	for _, item := range section.Content {
		if item.Kind == fb2.FlowSubtitle && item.Subtitle != nil {
			// Subtitles count as same depth as sections at this level
			if currentDepth > maxDepth {
				maxDepth = currentDepth
			}
		} else if item.Kind == fb2.FlowSection && item.Section != nil && item.Section.Title != nil {
			depth := calculateSectionDepth(item.Section, currentDepth+1)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	}
	return maxDepth
}

func buildNCXNavPoints(parent *etree.Element, section *fb2.Section, filename string, playOrder *int, c *content.Content) {
	var lastNavPoint *etree.Element

	for _, item := range section.Content {
		if item.Kind == fb2.FlowSubtitle && item.Subtitle != nil {
			// Subtitle in section - add to TOC
			*playOrder++
			subtitleID := getSubtitleID(item.Subtitle, c.GeneratedSubtitles)
			navPoint := parent.CreateElement("navPoint")
			navPoint.CreateAttr("id", fmt.Sprintf("navpoint-%s", subtitleID))
			navPoint.CreateAttr("playOrder", fmt.Sprintf("%d", *playOrder))

			navLabel := navPoint.CreateElement("navLabel")
			labelText := navLabel.CreateElement("text")
			labelText.SetText(extractSubtitleText(item.Subtitle, subtitleID))

			navContent := navPoint.CreateElement("content")
			navContent.CreateAttr("src", filename+"#"+subtitleID)

			lastNavPoint = navPoint
		} else if item.Kind == fb2.FlowSection && item.Section != nil {
			if item.Section.Title != nil {
				// Section with title - add to TOC
				*playOrder++
				// Use section ID or generated ID
				sectionID := getSectionID(item.Section, c.GeneratedIDs)
				navPoint := parent.CreateElement("navPoint")
				navPoint.CreateAttr("id", fmt.Sprintf("navpoint-%s", sectionID))
				navPoint.CreateAttr("playOrder", fmt.Sprintf("%d", *playOrder))

				navLabel := navPoint.CreateElement("navLabel")
				labelText := navLabel.CreateElement("text")
				labelText.SetText(extractTitleText(item.Section))

				navContent := navPoint.CreateElement("content")
				navContent.CreateAttr("src", filename+"#"+sectionID)

				// Recursively process nested sections
				buildNCXNavPoints(navPoint, item.Section, filename, playOrder, c)
				lastNavPoint = navPoint
			} else {
				// Section without title (grouping) - process its children
				// If there's a previous sibling with title, nest under it; otherwise at current level
				if lastNavPoint != nil {
					buildNCXNavPoints(lastNavPoint, item.Section, filename, playOrder, c)
				} else {
					buildNCXNavPoints(parent, item.Section, filename, playOrder, c)
				}
			}
		}
	}
}

func writeNav(zw *zip.Writer, c *content.Content, chapters []chapterData, _ *zap.Logger) error {
	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)

	html := doc.CreateElement("html")
	html.CreateAttr("xmlns", "http://www.w3.org/1999/xhtml")
	html.CreateAttr("xmlns:epub", "http://www.idpf.org/2007/ops")

	head := html.CreateElement("head")
	title := head.CreateElement("title")
	title.SetText("Table of Contents")

	body := html.CreateElement("body")

	nav := body.CreateElement("nav")
	nav.CreateAttr("epub:type", "toc")
	nav.CreateAttr("id", "toc")

	h1 := nav.CreateElement("h1")
	h1.SetText("Table of Contents")

	ol := nav.CreateElement("ol")

	for _, chapter := range chapters {
		li := ol.CreateElement("li")
		a := li.CreateElement("a")
		a.CreateAttr("href", chapter.Filename)
		a.SetText(chapter.Title)

		// Add nested sections to TOC
		if chapter.Section != nil {
			buildNavOL(li, chapter.Section, chapter.Filename, c)
		}
	}

	return writeXMLToZip(zw, filepath.Join(oebpsDir, "nav.xhtml"), doc)
}

func buildNavOL(parent *etree.Element, section *fb2.Section, filename string, c *content.Content) {
	var nestedOL *etree.Element
	var lastLI *etree.Element

	for _, item := range section.Content {
		if item.Kind == fb2.FlowSubtitle && item.Subtitle != nil {
			// Subtitle in section - add to TOC
			if nestedOL == nil {
				nestedOL = parent.CreateElement("ol")
			}
			subtitleID := getSubtitleID(item.Subtitle, c.GeneratedSubtitles)
			li := nestedOL.CreateElement("li")
			a := li.CreateElement("a")
			a.CreateAttr("href", filename+"#"+subtitleID)
			a.SetText(extractSubtitleText(item.Subtitle, subtitleID))
			lastLI = li
		} else if item.Kind == fb2.FlowSection && item.Section != nil {
			if item.Section.Title != nil {
				// Section with title - add to TOC
				if nestedOL == nil {
					nestedOL = parent.CreateElement("ol")
				}
				li := nestedOL.CreateElement("li")
				a := li.CreateElement("a")
				// Use section ID or generated ID
				sectionID := getSectionID(item.Section, c.GeneratedIDs)
				a.CreateAttr("href", filename+"#"+sectionID)
				a.SetText(extractTitleText(item.Section))

				// Recursively process nested sections
				buildNavOL(li, item.Section, filename, c)
				lastLI = li
			} else {
				// Section without title (grouping) - process its children
				// If there's a previous sibling with title, nest under it; otherwise at current level
				if lastLI != nil {
					buildNavOL(lastLI, item.Section, filename, c)
				} else {
					buildNavOL(parent, item.Section, filename, c)
				}
			}
		}
	}
}

func writeXMLToZip(zw *zip.Writer, name string, doc *etree.Document) error {
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		return err
	}
	return writeDataToZip(zw, name, buf.Bytes())
}

func writeDataToZip(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
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
				log.Debug("Fixed internal link", zap.String("from", currentFile), zap.String("to", targetFile), zap.String("id", targetID))
			}
		}
	}

	// Recursively process children
	for _, child := range elem.ChildElements() {
		fixLinksInElement(child, currentFile, idToFile, log)
	}
}
