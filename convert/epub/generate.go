package epub

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/gosimple/slug"
	fixzip "github.com/hidez8891/zip"
	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
	"fbc/fb2"
	"fbc/fb2/fields"
	"fbc/state"
)

const (
	mimetypeContent = "application/epub+zip"
	oebpsDir        = "OEBPS"
	imagesDir       = "images"
)

// Generate creates the EPUB output file.
// It handles epub2, epub3, and kepub variants based on content.OutputFormat.
func Generate(ctx context.Context, c *content.Content, outputPath string, cfg *config.DocumentConfig, log *zap.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	env := state.EnvFromContext(ctx)

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

	// Add Annotation chapter if requested
	if cfg.Annotation.Enable && c.Book.Description.TitleInfo.Annotation != nil {
		annotationChapter := generateAnnotation(c, chapters, &cfg.Annotation, log)
		chapters = append([]chapterData{annotationChapter}, chapters...)
	}

	// Add TOC page if requested
	if cfg.TOCPage.Placement != config.TOCPagePlacementNone {
		tocChapter := generateTOCPage(c, chapters, &cfg.TOCPage, log)
		if cfg.TOCPage.Placement == config.TOCPagePlacementBefore {
			chapters = append([]chapterData{tocChapter}, chapters...)
		} else {
			chapters = append(chapters, tocChapter)
		}
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

	if err := writeOPF(zw, c, cfg, chapters, log); err != nil {
		return fmt.Errorf("unable to write OPF: %w", err)
	}

	switch c.OutputFormat {
	case config.OutputFmtEpub3:
		if err := writeNav(zw, c, chapters, log); err != nil {
			return fmt.Errorf("unable to write NAV: %w", err)
		}
	default:
		if err := writeNCX(zw, c, chapters, log); err != nil {
			return fmt.Errorf("unable to write NCX: %w", err)
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

func writeImages(zw *zip.Writer, images fb2.BookImages, _ *zap.Logger) error {
	for id, img := range images {
		filename := filepath.Join(oebpsDir, imagesDir, img.Filename)

		if err := writeDataToZip(zw, filename, img.Data); err != nil {
			return fmt.Errorf("unable to write image %s: %w", id, err)
		}
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

func writeOPF(zw *zip.Writer, c *content.Content, cfg *config.DocumentConfig, chapters []chapterData, log *zap.Logger) error {
	doc := etree.NewDocument()
	doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)

	pkg := doc.CreateElement("package")
	pkg.CreateAttr("xmlns", "http://www.idpf.org/2007/opf")
	pkg.CreateAttr("unique-identifier", "BookId")

	switch c.OutputFormat {
	case config.OutputFmtEpub3:
		pkg.CreateAttr("version", "3.0")
	default:
		pkg.CreateAttr("version", "2.0")
	}
	metadata := pkg.CreateElement("metadata")
	metadata.CreateAttr("xmlns:dc", "http://purl.org/dc/elements/1.1/")
	metadata.CreateAttr("xmlns:opf", "http://www.idpf.org/2007/opf")

	dcTitle := metadata.CreateElement("dc:title")
	title := c.Book.Description.TitleInfo.BookTitle.Value
	if cfg.Metainformation.TitleTemplate != "" {
		expanded, err := fields.Expand(config.MetaTitleTemplateFieldName, cfg.Metainformation.TitleTemplate, -1, c.Book, c.SrcName, c.OutputFormat)
		if err != nil {
			log.Warn("Unable to prepare title for generated OPF", zap.Error(err))
		} else {
			title = expanded
		}
	}
	if cfg.Metainformation.Transliterate {
		title = slug.Make(title)
	}
	dcTitle.SetText(title)

	dcIdentifier := metadata.CreateElement("dc:identifier")
	dcIdentifier.CreateAttr("id", "BookId")
	dcIdentifier.SetText(c.Book.Description.DocumentInfo.ID)

	dcLang := metadata.CreateElement("dc:language")
	dcLang.SetText(c.Book.Description.TitleInfo.Lang.String())

	for idx, author := range c.Book.Description.TitleInfo.Authors {
		dcCreator := metadata.CreateElement("dc:creator")
		authorName := strings.TrimSpace(fmt.Sprintf("%s %s %s", author.FirstName, author.MiddleName, author.LastName))
		if cfg.Metainformation.CreatorNameTemplate != "" {
			expanded, err := fields.Expand(config.MetaCreatorNameTemplateFieldName, cfg.Metainformation.CreatorNameTemplate, idx, c.Book, c.SrcName, c.OutputFormat)
			if err != nil {
				log.Warn("Unable to prepare author name for generated OPF", zap.Error(err))
			} else {
				authorName = expanded
			}
		}
		if cfg.Metainformation.Transliterate {
			authorName = slug.Make(authorName)
		}
		dcCreator.SetText(authorName)

		// EPUB3 uses <meta property="role"> with refines, EPUB2 uses opf:role attribute
		if c.OutputFormat == config.OutputFmtEpub3 {
			creatorID := fmt.Sprintf("creator%d", idx)
			dcCreator.CreateAttr("id", creatorID)

			roleMeta := metadata.CreateElement("meta")
			roleMeta.CreateAttr("refines", "#"+creatorID)
			roleMeta.CreateAttr("property", "role")
			roleMeta.CreateAttr("scheme", "marc:relators")
			roleMeta.SetText("aut")
		} else {
			dcCreator.CreateAttr("opf:role", "aut")
		}
	}

	for _, genreRef := range c.Book.Description.TitleInfo.Genres {
		meta := metadata.CreateElement("dc:subject")
		meta.SetText(genreRef.Value)
	}

	if c.Book.Description.TitleInfo.Annotation != nil {
		meta := metadata.CreateElement("dc:description")
		meta.SetText(c.Book.Description.TitleInfo.Annotation.AsPlainText())
	}

	if len(c.Book.Description.TitleInfo.Sequences) > 0 {
		// Do not let series metadata to disappear, use calibre meta tags
		meta := metadata.CreateElement("meta")
		meta.CreateAttr("name", "calibre:series")
		meta.CreateAttr("content", c.Book.Description.TitleInfo.Sequences[0].Name)
		if c.Book.Description.TitleInfo.Sequences[0].Number != nil {
			meta = metadata.CreateElement("meta")
			meta.CreateAttr("name", "calibre:series_index")
			meta.CreateAttr("content", strconv.Itoa(*c.Book.Description.TitleInfo.Sequences[0].Number))
		}
	}

	// EPUB2 uses <meta name="cover">, EPUB3 uses properties="cover-image" on manifest item
	if c.CoverID != "" && c.OutputFormat != config.OutputFmtEpub3 {
		meta := metadata.CreateElement("meta")
		meta.CreateAttr("name", "cover")
		meta.CreateAttr("content", "book-cover-image")
	}

	// EPUB3 requires dcterms:modified metadata
	if c.OutputFormat == config.OutputFmtEpub3 {
		modifiedMeta := metadata.CreateElement("meta")
		modifiedMeta.CreateAttr("property", "dcterms:modified")
		modifiedMeta.SetText(time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	}

	manifest := pkg.CreateElement("manifest")

	switch c.OutputFormat {
	case config.OutputFmtEpub3:
		item := manifest.CreateElement("item")
		item.CreateAttr("id", "nav")
		item.CreateAttr("href", "nav.xhtml")
		item.CreateAttr("media-type", "application/xhtml+xml")
		item.CreateAttr("properties", "nav")
	default:
		item := manifest.CreateElement("item")
		item.CreateAttr("id", "ncx")
		item.CreateAttr("href", "toc.ncx")
		item.CreateAttr("media-type", "application/x-dtbncx+xml")
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
		if c.OutputFormat == config.OutputFmtEpub3 {
			coverPageItem.CreateAttr("properties", "svg")
		}
	}

	// Track files added to manifest to avoid duplicates (e.g., footnote files with multiple body fragments)
	// Map filename -> chapter ID of first occurrence
	addedFiles := make(map[string]string)
	for _, chapter := range chapters {
		// Extract base filename without fragment
		filename := chapter.Filename
		if idx := strings.Index(filename, "#"); idx != -1 {
			filename = filename[:idx]
		}

		// Only add each file once to manifest
		if _, exists := addedFiles[filename]; !exists {
			item := manifest.CreateElement("item")
			item.CreateAttr("id", chapter.ID)
			item.CreateAttr("href", filename)
			item.CreateAttr("media-type", "application/xhtml+xml")
			addedFiles[filename] = chapter.ID
		}
	}

	for id, img := range c.ImagesIndex {
		item := manifest.CreateElement("item")
		if id == c.CoverID {
			item.CreateAttr("id", "book-cover-image")
			item.CreateAttr("href", "images/"+img.Filename)
			item.CreateAttr("media-type", img.MimeType)
			if c.OutputFormat == config.OutputFmtEpub3 {
				item.CreateAttr("properties", "cover-image")
			}
		} else {
			item.CreateAttr("id", "img-"+id)
			item.CreateAttr("href", "images/"+img.Filename)
			item.CreateAttr("media-type", img.MimeType)
		}
	}

	spine := pkg.CreateElement("spine")
	if c.OutputFormat != config.OutputFmtEpub3 {
		spine.CreateAttr("toc", "ncx")
	}

	if c.CoverID != "" {
		coverRef := spine.CreateElement("itemref")
		coverRef.CreateAttr("idref", "cover-page")
	}

	// Add chapters to spine, but only reference each file once (use first chapter ID for files with fragments)
	addedToSpine := make(map[string]bool)
	for _, chapter := range chapters {
		// Extract base filename without fragment
		filename := chapter.Filename
		if idx := strings.Index(filename, "#"); idx != -1 {
			filename = filename[:idx]
		}

		// Only add each file once to spine
		if !addedToSpine[filename] {
			itemref := spine.CreateElement("itemref")
			itemref.CreateAttr("idref", addedFiles[filename])
			addedToSpine[filename] = true
		}
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
		if !chapter.IncludeInTOC {
			continue
		}
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
		if !chapter.IncludeInTOC {
			continue
		}
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
			*playOrder++
			subtitleID := item.Subtitle.ID
			navPoint := parent.CreateElement("navPoint")
			navPoint.CreateAttr("id", fmt.Sprintf("navpoint-%s", subtitleID))
			navPoint.CreateAttr("playOrder", fmt.Sprintf("%d", *playOrder))

			navLabel := navPoint.CreateElement("navLabel")
			labelText := navLabel.CreateElement("text")
			labelText.SetText(item.Subtitle.AsTOCText(fb2.FormatIDToTOC(subtitleID)))

			navContent := navPoint.CreateElement("content")
			navContent.CreateAttr("src", filename+"#"+subtitleID)

			lastNavPoint = navPoint
		} else if item.Kind == fb2.FlowSection && item.Section != nil {
			if item.Section.Title != nil {
				*playOrder++
				sectionID := item.Section.ID
				navPoint := parent.CreateElement("navPoint")
				navPoint.CreateAttr("id", fmt.Sprintf("navpoint-%s", sectionID))
				navPoint.CreateAttr("playOrder", fmt.Sprintf("%d", *playOrder))

				navLabel := navPoint.CreateElement("navLabel")
				labelText := navLabel.CreateElement("text")
				labelText.SetText(item.Section.AsTitleText(""))

				navContent := navPoint.CreateElement("content")
				navContent.CreateAttr("src", filename+"#"+sectionID)

				buildNCXNavPoints(navPoint, item.Section, filename, playOrder, c)
				lastNavPoint = navPoint
			} else {
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

	meta := head.CreateElement("meta")
	meta.CreateAttr("charset", "utf-8")

	title := head.CreateElement("title")
	title.SetText("Table of Contents")

	// Add CSS for better presentation
	link := head.CreateElement("link")
	link.CreateAttr("rel", "stylesheet")
	link.CreateAttr("type", "text/css")
	link.CreateAttr("href", "stylesheet.css")

	body := html.CreateElement("body")

	nav := body.CreateElement("nav")
	nav.CreateAttr("epub:type", "toc")
	nav.CreateAttr("id", "toc")
	nav.CreateAttr("role", "doc-toc")

	h1 := nav.CreateElement("h1")
	h1.SetText("Table of Contents")

	ol := nav.CreateElement("ol")

	for _, chapter := range chapters {
		if !chapter.IncludeInTOC {
			continue
		}
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
	nestedOL := parent.SelectElement("ol")
	if nestedOL == nil {
		nestedOL = parent.CreateElement("ol")
	}

	hadItems := len(nestedOL.ChildElements()) > 0
	buildNavOLItems(nestedOL, section, filename, c)

	if !hadItems && len(nestedOL.ChildElements()) == 0 {
		parent.RemoveChild(nestedOL)
	}
}

func buildNavOLItems(parentOL *etree.Element, section *fb2.Section, filename string, c *content.Content) {
	var lastLI *etree.Element

	for _, item := range section.Content {
		if item.Kind == fb2.FlowSubtitle && item.Subtitle != nil {
			subtitleID := item.Subtitle.ID
			li := parentOL.CreateElement("li")
			a := li.CreateElement("a")
			a.CreateAttr("href", filename+"#"+subtitleID)
			a.SetText(item.Subtitle.AsTOCText(fb2.FormatIDToTOC(subtitleID)))
			lastLI = li
		} else if item.Kind == fb2.FlowSection && item.Section != nil {
			if item.Section.Title != nil {
				li := parentOL.CreateElement("li")
				a := li.CreateElement("a")
				sectionID := item.Section.ID
				a.CreateAttr("href", filename+"#"+sectionID)
				a.SetText(item.Section.AsTitleText(""))

				buildNavOL(li, item.Section, filename, c)
				lastLI = li
			} else {
				if lastLI != nil {
					buildNavOL(lastLI, item.Section, filename, c)
				} else {
					buildNavOLItems(parentOL, item.Section, filename, c)
				}
			}
		}
	}
}

func generateAnnotation(c *content.Content, chapters []chapterData, cfg *config.AnnotationConfig, log *zap.Logger) chapterData {
	doc := createXHTMLDocument(cfg.Title)
	body := doc.Root().SelectElement("body")
	body.CreateAttr("class", "annotation-page")

	h1 := body.CreateElement("h1")
	h1.CreateAttr("class", "annotation-title")
	h1.SetText(cfg.Title)

	annotationDiv := body.CreateElement("div")
	annotationDiv.CreateAttr("class", "annotation")

	if err := appendFlowItemsWithContext(annotationDiv, c, c.Book.Description.TitleInfo.Annotation.Items, 1, "annotation", log); err != nil {
		log.Warn("Unable to convert annotation content", zap.Error(err))
	}

	// Find a unique ID and filename that doesn't collide with existing chapters
	baseID := "annotation-page"
	id := baseID
	filename := baseID + ".xhtml"

	existingIDs := make(map[string]bool, len(chapters))
	for _, ch := range chapters {
		existingIDs[ch.ID] = true
	}

	counter := 0
	for existingIDs[id] {
		counter++
		id = fmt.Sprintf("%s-%d", baseID, counter)
		filename = id + ".xhtml"
	}

	return chapterData{
		ID:           id,
		Filename:     filename,
		Title:        cfg.Title,
		Doc:          doc,
		IncludeInTOC: cfg.TOC,
	}
}

// generateTOCPage creates a TOC chapter as an XHTML page
func generateTOCPage(c *content.Content, chapters []chapterData, cfg *config.TOCPageConfig, log *zap.Logger) chapterData {
	doc := createXHTMLDocument(cfg.Title)
	body := doc.Root().SelectElement("body")
	body.CreateAttr("class", "toc-page")

	h1 := body.CreateElement("h1")
	h1.CreateAttr("class", "toc-title")
	h1.SetText(c.Book.Description.TitleInfo.BookTitle.Value)

	if cfg.AuthorsTemplate != "" {
		expanded, err := fields.Expand(config.AuthorsTemplateFieldName, cfg.AuthorsTemplate, -1, c.Book, c.SrcName, c.OutputFormat)
		if err != nil {
			log.Warn("Unable to prepare list of authors for generated TOC", zap.Error(err))
		} else {
			h2 := body.CreateElement("h2")
			h2.CreateAttr("class", "toc-authors")
			h2.SetText(expanded)
		}
	}

	ol := body.CreateElement("ol")
	ol.CreateAttr("class", "toc-list")

	for _, chapter := range chapters {
		if !chapter.IncludeInTOC {
			continue
		}
		li := ol.CreateElement("li")
		li.CreateAttr("class", "toc-item")
		a := li.CreateElement("a")
		a.CreateAttr("class", "toc-link")
		a.CreateAttr("href", chapter.Filename)
		a.SetText(chapter.Title)

		if chapter.Section != nil {
			buildTOCPageOL(li, chapter.Section, chapter.Filename, c)
		}
	}

	// Find a unique ID and filename that doesn't collide with existing chapters
	baseID := "toc-page"
	id := baseID
	filename := baseID + ".xhtml"

	existingIDs := make(map[string]bool, len(chapters))
	for _, ch := range chapters {
		existingIDs[ch.ID] = true
	}

	counter := 0
	for existingIDs[id] {
		counter++
		id = fmt.Sprintf("%s-%d", baseID, counter)
		filename = id + ".xhtml"
	}

	return chapterData{
		ID:           id,
		Filename:     filename,
		Title:        cfg.Title,
		Doc:          doc,
		IncludeInTOC: true,
	}
}

// buildTOCPageOL recursively builds nested TOC structure for the TOC page
func buildTOCPageOL(parent *etree.Element, section *fb2.Section, filename string, c *content.Content) {
	nestedOL := parent.SelectElement("ol")
	if nestedOL == nil {
		nestedOL = parent.CreateElement("ol")
		nestedOL.CreateAttr("class", "toc-list toc-nested")
	}

	hadItems := len(nestedOL.ChildElements()) > 0
	buildTOCPageOLItems(nestedOL, section, filename, c)

	if !hadItems && len(nestedOL.ChildElements()) == 0 {
		parent.RemoveChild(nestedOL)
	}
}

// buildTOCPageOLItems adds TOC entries for subsections to the ordered list
func buildTOCPageOLItems(parentOL *etree.Element, section *fb2.Section, filename string, c *content.Content) {
	var lastLI *etree.Element

	for _, item := range section.Content {
		if item.Kind == fb2.FlowSubtitle && item.Subtitle != nil {
			subtitleID := item.Subtitle.ID
			li := parentOL.CreateElement("li")
			li.CreateAttr("class", "toc-item toc-subtitle")
			a := li.CreateElement("a")
			a.CreateAttr("class", "toc-link")
			a.CreateAttr("href", filename+"#"+subtitleID)
			a.SetText(item.Subtitle.AsTOCText(fb2.FormatIDToTOC(subtitleID)))
			lastLI = li
		} else if item.Kind == fb2.FlowSection && item.Section != nil {
			if item.Section.Title != nil {
				li := parentOL.CreateElement("li")
				li.CreateAttr("class", "toc-item toc-section")
				a := li.CreateElement("a")
				a.CreateAttr("class", "toc-link")
				sectionID := item.Section.ID
				a.CreateAttr("href", filename+"#"+sectionID)
				a.SetText(item.Section.AsTitleText(""))

				buildTOCPageOL(li, item.Section, filename, c)
				lastLI = li
			} else {
				if lastLI != nil {
					buildTOCPageOL(lastLI, item.Section, filename, c)
				} else {
					buildTOCPageOLItems(parentOL, item.Section, filename, c)
				}
			}
		}
	}
}

func writeDataToZip(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
