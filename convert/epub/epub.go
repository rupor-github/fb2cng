package epub

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/beevik/etree"
	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
	"fbc/fb2"
)

const (
	mimetypeContent = "application/epub+zip"
	oebpsDir        = "OEBPS"
	imagesDir       = "OEBPS/images"
)

type chapterData struct {
	ID       string
	Filename string
	Title    string
	Doc      *etree.Document
}

// Generate creates the EPUB output file.
// It handles epub2, epub3, and kepub variants based on content.OutputFormat.
func Generate(ctx context.Context, c *content.Content, outputPath string, cfg *config.DocumentConfig, log *zap.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	log.Info("Generating EPUB", zap.Stringer("format", c.OutputFormat), zap.String("output", outputPath))

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("unable to create output directory: %w", err)
	}

	f, err := os.Create(outputPath)
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

	chapters, err := convertToXHTML(ctx, c, log)
	if err != nil {
		return fmt.Errorf("unable to convert content: %w", err)
	}

	for _, chapter := range chapters {
		if err := writeXHTMLChapter(zw, &chapter, log); err != nil {
			return fmt.Errorf("unable to write chapter %s: %w", chapter.ID, err)
		}
	}

	if err := writeImages(zw, c.ImagesIndex, log); err != nil {
		return fmt.Errorf("unable to write images: %w", err)
	}

	if err := writeStylesheet(zw, c); err != nil {
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
	rootfile.CreateAttr("full-path", "OEBPS/content.opf")
	rootfile.CreateAttr("media-type", "application/oebps-package+xml")

	return writeXMLToZip(zw, "META-INF/container.xml", doc)
}

func convertToXHTML(ctx context.Context, c *content.Content, log *zap.Logger) ([]chapterData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var chapters []chapterData
	chapterNum := 0

	for _, body := range c.Book.Bodies {
		if !body.Main() {
			continue
		}

		for _, section := range body.Sections {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			collectChapters(&section, &chapters, &chapterNum, c, log)
		}
	}

	return chapters, nil
}

func collectChapters(section *fb2.Section, chapters *[]chapterData, chapterNum *int, c *content.Content, log *zap.Logger) {
	if section.Title != nil {
		*chapterNum++
		chapterID := fmt.Sprintf("index%05d", *chapterNum)
		filename := fmt.Sprintf("%s.xhtml", chapterID)
		title := extractTitle(section)

		doc, err := sectionToXHTML(c, section, title, log)
		if err != nil {
			log.Error("Unable to convert section", zap.Error(err))
			return
		}

		*chapters = append(*chapters, chapterData{
			ID:       chapterID,
			Filename: filename,
			Title:    title,
			Doc:      doc,
		})
	}

	for _, item := range section.Content {
		if item.Kind == fb2.FlowSection && item.Section != nil {
			collectChapters(item.Section, chapters, chapterNum, c, log)
		}
	}
}

func extractTitle(section *fb2.Section) string {
	if section.Title != nil {
		for _, item := range section.Title.Items {
			if item.Paragraph != nil {
				return paragraphToPlainText(item.Paragraph)
			}
		}
	}
	return "Chapter"
}

func paragraphToPlainText(p *fb2.Paragraph) string {
	var buf strings.Builder
	for _, seg := range p.Text {
		buf.WriteString(inlineSegmentToText(&seg))
	}
	return strings.TrimSpace(buf.String())
}

func inlineSegmentToText(seg *fb2.InlineSegment) string {
	var buf strings.Builder
	buf.WriteString(seg.Text)
	for _, child := range seg.Children {
		buf.WriteString(inlineSegmentToText(&child))
	}
	return buf.String()
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

	if err := writeSectionContent(body, c, section, false, log); err != nil {
		return nil, err
	}

	return doc, nil
}

func writeSectionContent(parent *etree.Element, c *content.Content, section *fb2.Section, skipTitle bool, log *zap.Logger) error {
	if section.Title != nil && !skipTitle {
		titleDiv := parent.CreateElement("div")
		titleDiv.CreateAttr("class", "title")
		firstParagraph := true
		for _, item := range section.Title.Items {
			if item.Paragraph != nil {
				p := titleDiv.CreateElement("p")
				if item.Paragraph.ID != "" {
					p.CreateAttr("id", item.Paragraph.ID)
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
				p.CreateAttr("class", class)
				writeParagraphInline(p, c, item.Paragraph)
			} else if item.EmptyLine {
				br := titleDiv.CreateElement("br")
				br.CreateAttr("class", "title")
			}
		}
	}

	for _, epigraph := range section.Epigraphs {
		div := parent.CreateElement("div")
		div.CreateAttr("class", "epigraph")
		if err := writeFlowContent(div, c, &epigraph.Flow, log); err != nil {
			return err
		}
		for _, ta := range epigraph.TextAuthors {
			p := div.CreateElement("p")
			p.CreateAttr("class", "text-author")
			writeParagraphInline(p, c, &ta)
		}
	}

	if section.Image != nil {
		writeImageElement(parent, section.Image)
	}

	if section.Annotation != nil {
		div := parent.CreateElement("div")
		div.CreateAttr("class", "annotation")
		if err := writeFlowContent(div, c, section.Annotation, log); err != nil {
			return err
		}
	}

	if err := writeFlowItems(parent, c, section.Content, true, log); err != nil {
		return err
	}

	return nil
}

func writeFlowContent(parent *etree.Element, c *content.Content, flow *fb2.Flow, log *zap.Logger) error {
	return writeFlowItems(parent, c, flow.Items, false, log)
}

func writeFlowItems(parent *etree.Element, c *content.Content, items []fb2.FlowItem, skipNestedChapters bool, log *zap.Logger) error {
	for _, item := range items {
		switch item.Kind {
		case fb2.FlowParagraph:
			if item.Paragraph != nil {
				p := parent.CreateElement("p")
				if item.Paragraph.ID != "" {
					p.CreateAttr("id", item.Paragraph.ID)
				}
				if item.Paragraph.Style != "" {
					p.CreateAttr("class", item.Paragraph.Style)
				}
				writeParagraphInline(p, c, item.Paragraph)
			}
		case fb2.FlowImage:
			if item.Image != nil {
				writeImageElement(parent, item.Image)
			}
		case fb2.FlowEmptyLine:
			parent.CreateElement("br")
		case fb2.FlowSubtitle:
			if item.Subtitle != nil {
				p := parent.CreateElement("p")
				if item.Subtitle.ID != "" {
					p.CreateAttr("id", item.Subtitle.ID)
				}
				class := "sub-title"
				if item.Subtitle.Style != "" {
					class = class + " " + item.Subtitle.Style
				}
				p.CreateAttr("class", class)
				writeParagraphInline(p, c, item.Subtitle)
			}
		case fb2.FlowPoem:
			if item.Poem != nil {
				writePoemElement(parent, c, item.Poem, log)
			}
		case fb2.FlowCite:
			if item.Cite != nil {
				writeCiteElement(parent, c, item.Cite, log)
			}
		case fb2.FlowTable:
			if item.Table != nil {
				writeTableElement(parent, c, item.Table)
			}
		case fb2.FlowSection:
			if item.Section != nil {
				if skipNestedChapters && item.Section.Title != nil {
					continue
				}
				div := parent.CreateElement("div")
				div.CreateAttr("class", "section")
				if err := writeSectionContent(div, c, item.Section, false, log); err != nil {
					return err
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
		}
		if seg.LinkType != "" {
			a.CreateAttr("type", seg.LinkType)
		}
		for _, child := range seg.Children {
			writeInlineSegment(a, c, &child)
		}
	case fb2.InlineImageSegment:
		if seg.Image != nil {
			img := parent.CreateElement("img")
			imgID := strings.TrimPrefix(seg.Image.Href, "#")
			img.CreateAttr("src", "images/"+imgID)
			if seg.Image.Alt != "" {
				img.CreateAttr("alt", seg.Image.Alt)
			}
		}
	}
}

func writeImageElement(parent *etree.Element, img *fb2.Image) {
	div := parent.CreateElement("div")
	div.CreateAttr("class", "image")
	if img.ID != "" {
		div.CreateAttr("id", img.ID)
	}

	imgElem := div.CreateElement("img")
	imgID := strings.TrimPrefix(img.Href, "#")
	imgElem.CreateAttr("src", "images/"+imgID)
	if img.Alt != "" {
		imgElem.CreateAttr("alt", img.Alt)
	}
	if img.Title != "" {
		imgElem.CreateAttr("title", img.Title)
	}
}

func writePoemElement(parent *etree.Element, c *content.Content, poem *fb2.Poem, log *zap.Logger) {
	div := parent.CreateElement("div")
	div.CreateAttr("class", "poem")
	if poem.ID != "" {
		div.CreateAttr("id", poem.ID)
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

	for _, stanza := range poem.Stanzas {
		stanzaDiv := div.CreateElement("div")
		stanzaDiv.CreateAttr("class", "stanza")
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
}

func writeCiteElement(parent *etree.Element, c *content.Content, cite *fb2.Cite, log *zap.Logger) {
	blockquote := parent.CreateElement("blockquote")
	if cite.ID != "" {
		blockquote.CreateAttr("id", cite.ID)
	}
	blockquote.CreateAttr("class", "cite")

	if err := writeFlowItems(blockquote, c, cite.Items, false, log); err != nil {
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

func writeXHTMLChapter(zw *zip.Writer, chapter *chapterData, log *zap.Logger) error {
	chapter.Doc.Indent(2)
	return writeXMLToZip(zw, oebpsDir+"/"+chapter.Filename, chapter.Doc)
}

func writeImages(zw *zip.Writer, images fb2.BookImages, log *zap.Logger) error {
	for id, img := range images {
		filename := fmt.Sprintf("%s/%s", imagesDir, id)

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
		svgImage.CreateAttr("xlink:href", "images/"+c.CoverID)
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
		svgImage.CreateAttr("xlink:href", "images/"+c.CoverID)
	}

	doc.Indent(2)
	return writeXMLToZip(zw, oebpsDir+"/cover.xhtml", doc)
}

//.title { text-align: center; margin: 1em 0 2em 0; font-weight: bold; }

func writeStylesheet(zw *zip.Writer, c *content.Content) error {
	css := `p { text-indent: 1em; text-align: justify; margin: 0 0 0.3em 0; }
.title { margin: 2em 0 1em 0; page-break-before: always; }
br.title { display: block; margin: 0.5em 0; }
.title-first, .title-next { text-align: center; font-size: 120%; font-weight: bold; text-indent: 0; margin: 0; }
.sub-title { text-align: center; font-weight: bold; text-indent: 0; margin: 1em 0; page-break-after: avoid; }
.poem-title { margin: 1em 0; }
.poem-title-first, .poem-title-next { text-align: center; text-indent: 0; margin: 0; }
.image { text-align: center; text-indent: 0; }
.image img { max-width: 100%; height: auto; }
.epigraph { text-align: right; font-style: italic; margin: 0.4em 0 0.2em 4em; }
.annotation { font-size: 80%; text-align: center; margin: 2em 1em 1em 1em; }
.poem { text-indent: 0; font-style: italic; margin: 0 0 0 3em; }
.stanza { margin: 0.5em 0; }
.verse { margin: 0.25em 0 0.25em 2em; text-indent: 0; }
.cite, blockquote.cite { margin: 1em 2em; }
.text-author { text-align: right; font-style: italic; text-indent: 0; }
.section { margin: 1em 0; }
table { border-collapse: collapse; margin: 1em auto; }
td, th { border: 1px solid #ccc; padding: 0.5em; }
`

	for _, style := range c.Book.Stylesheets {
		if style.Type == "text/css" {
			css += "\n/* FB2 embedded stylesheet */\n" + style.Data + "\n"
		}
	}

	return writeDataToZip(zw, oebpsDir+"/stylesheet.css", []byte(css))
}

func writeOPF(zw *zip.Writer, c *content.Content, chapters []chapterData, log *zap.Logger) error {
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

	if c.OutputFormat == config.OutputFmtEpub2 || c.OutputFormat == config.OutputFmtKepub {
		item := manifest.CreateElement("item")
		item.CreateAttr("id", "ncx")
		item.CreateAttr("href", "toc.ncx")
		item.CreateAttr("media-type", "application/x-dtbncx+xml")
	} else if c.OutputFormat == config.OutputFmtEpub3 {
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
			item.CreateAttr("href", "images/"+id)
			item.CreateAttr("media-type", img.MimeType)
			item.CreateAttr("properties", "cover-image")
		} else {
			item.CreateAttr("id", "img-"+id)
			item.CreateAttr("href", "images/"+id)
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

	doc.Indent(2)
	return writeXMLToZip(zw, oebpsDir+"/content.opf", doc)
}

func writeNCX(zw *zip.Writer, c *content.Content, chapters []chapterData, log *zap.Logger) error {
	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)

	ncx := doc.CreateElement("ncx")
	ncx.CreateAttr("xmlns", "http://www.daisy.org/z3986/2005/ncx/")
	ncx.CreateAttr("version", "2005-1")

	head := ncx.CreateElement("head")

	metaUID := head.CreateElement("meta")
	metaUID.CreateAttr("name", "dtb:uid")
	metaUID.CreateAttr("content", c.Book.Description.DocumentInfo.ID)

	metaDepth := head.CreateElement("meta")
	metaDepth.CreateAttr("name", "dtb:depth")
	metaDepth.CreateAttr("content", "1")

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

	for i, chapter := range chapters {
		navPoint := navMap.CreateElement("navPoint")
		navPoint.CreateAttr("id", chapter.ID)
		navPoint.CreateAttr("playOrder", fmt.Sprintf("%d", i+1))

		navLabel := navPoint.CreateElement("navLabel")
		labelText := navLabel.CreateElement("text")
		labelText.SetText(chapter.Title)

		navContent := navPoint.CreateElement("content")
		navContent.CreateAttr("src", chapter.Filename)
	}

	doc.Indent(2)
	return writeXMLToZip(zw, oebpsDir+"/toc.ncx", doc)
}

func writeNav(zw *zip.Writer, c *content.Content, chapters []chapterData, log *zap.Logger) error {
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
	}

	doc.Indent(2)
	return writeXMLToZip(zw, oebpsDir+"/nav.xhtml", doc)
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
