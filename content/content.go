package content

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/beevik/etree"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/net/html/charset"

	"fbc/common"
	"fbc/config"
	"fbc/content/text"
	"fbc/fb2"
	"fbc/misc"
	"fbc/state"
)

// BackLinkRef tracks a single reference to a footnote for generating back-links
type BackLinkRef struct {
	RefID    string // unique ID for this reference occurrence (e.g., "ref-note1-1")
	TargetID string // ID of the footnote being referenced
	Filename string // filename containing the reference
}

// PageMapEntry represents a page marker in the document
type PageMapEntry struct {
	PageNum  int    // global page number
	SpanID   string // id="page{num}" to insert
	Filename string // filename containing this page marker
}

// Content encapsulates both the raw FB2 XML document and the structured
// normalized internal representation derived from the official FictionBook 2.0
// schemas. https://github.com/gribuser/fb2.git commit 4d3740e319039911c30d291abb0c8b26ec99703b
type Content struct {
	SrcName       string
	Doc           *etree.Document
	OutputFormat  common.OutputFmt     // config: output format
	KindleEbook   bool                 // cli: kindle ebook (EBOK) metadata
	ASIN          string               // cli: ASIN override for Kindle formats
	FootnotesMode common.FootnotesMode // config: footnotes handling mode
	PageSize      int                  // config: runes per page, 0 if disabled
	AdobeDE       bool                 // config: Adobe DE page markers are being generated instead of NCX pageList
	BacklinkStr   string               // config: backlink indicator
	MoreParaStr   string               // config: more paragraphs indicator
	ScreenWidth   int                  // config: target screen width for image sizing
	ScreenHeight  int                  // config: target screen height for image sizing

	Book           *fb2.FictionBook
	CoverID        string
	FootnotesIndex fb2.FootnoteRefs
	ImagesIndex    fb2.BookImages
	UsedImageIDs   map[string]bool
	IDsIndex       fb2.IDIndex
	LinksRevIndex  fb2.ReverseLinkIndex

	Splitter *text.Splitter
	Hyphen   *text.Hyphenator
	WorkDir  string
	Debug    bool

	// Footnote back-link tracking
	BackLinkIndex   map[string][]BackLinkRef // targetID -> list of references to it
	CurrentFilename string                   // current file being processed

	// Kobo span tracking
	koboSpanParagraphs int
	koboSpanSentences  int

	// Page map tracking
	PageTrackingEnabled bool                      // whether to track pages in current context
	pageRuneCounter     int                       // current rune count
	TotalPages          int                       // current page number
	PageMapIndex        map[string][]PageMapEntry // filename -> page entries in that file
}

// Prepare reads, parses, and prepares FB2 content for conversion.
// It is used for all output formats.
func Prepare(ctx context.Context, r io.Reader, srcName string, outputFormat common.OutputFmt, log *zap.Logger) (_ *Content, retErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	env := state.EnvFromContext(ctx)

	doc := etree.NewDocument()

	// Respect as many HTML named character references as possible, old FB2s
	// offten do not properly follow XML standard
	entities, err := prepareHTMLNamedEntities()
	if err != nil {
		return nil, fmt.Errorf("unable to write prepare HTML named entities: %w", err)
	}
	doc.WriteSettings = etree.WriteSettings{
		CanonicalText:    true,
		CanonicalAttrVal: true,
	}
	doc.ReadSettings = etree.ReadSettings{
		CharsetReader: makeCharsetReader(log),
		Entity:        entities,
		ValidateInput: false,
		Permissive:    true,
	}

	// Read and parse fb2
	if _, err := doc.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("unable to read FB2: %w", err)
	}

	book, err := fb2.ParseBookXML(doc, env.Cfg.Document.Footnotes.BodyNames, log)
	if err != nil {
		return nil, fmt.Errorf("unable to parse FB2: %w", err)
	}

	// Make sure book ID is not empty and is valid UUID
	var refID uuid.UUID
	if _, err := uuid.Parse(book.Description.DocumentInfo.ID); err != nil {
		if refID, err = uuid.NewV7(); err != nil {
			return nil, fmt.Errorf("unable to generate new book UUID: %w", err)
		}
		log.Warn("Book has invalid ID, correcting", zap.String("old_id", book.Description.DocumentInfo.ID), zap.Stringer("new_id", refID))
	}
	if refID != uuid.Nil {
		book.Description.DocumentInfo.ID = refID.String()
	}

	tmpDir, err := os.MkdirTemp("", misc.GetAppName()+"-")
	if err != nil {
		return nil, fmt.Errorf("unable to create temporary directory: %w", err)
	}
	defer func() {
		// On error, clean up the temp directory immediately. The reporter
		// (if active) tolerates absent paths in finalize(), so this is safe.
		if retErr != nil {
			os.RemoveAll(tmpDir)
		}
	}()
	env.Rpt.Store(fmt.Sprintf("%s-%s", misc.GetAppName(), book.Description.DocumentInfo.ID), tmpDir)

	baseSrcName := filepath.Base(srcName)

	// Save parsed document to file for debugging
	if env.Rpt != nil {
		if err := doc.WriteToFile(filepath.Join(tmpDir, baseSrcName)); err != nil {
			return nil, fmt.Errorf("unable to write input doc for debugging: %w", err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, baseSrcName+"_pristine"), []byte(book.String()), 0644); err != nil {
			return nil, fmt.Errorf("unable to write parsed doc for debugging: %w", err)
		}
	}

	// Handle cover image before normalization
	var coverID string

	// If cover image is specified, remember it
	if len(book.Description.TitleInfo.Coverpage) > 0 {
		// NOTE: we only support single cover image - first one. FB2 format
		// allows multiple covers and for some reason Libruseq's fb2 files
		// sometimes have several covers.
		coverID = strings.TrimPrefix(book.Description.TitleInfo.Coverpage[0].Href, "#")
	}

	// If no cover image is specified, and default cover generation is
	// requested, add default cover image
	if len(coverID) == 0 && env.Cfg.Document.Images.Cover.Generate {
		// Find an unused cover ID
		existingIDs := make(map[string]bool)
		for i := range book.Binaries {
			existingIDs[book.Binaries[i].ID] = true
		}

		for i := 0; ; i++ {
			coverID = fmt.Sprintf("cover_%d", i)
			if !existingIDs[coverID] {
				break
			}
		}

		log.Info("Adding default cover image", zap.String("id", coverID))

		ref := fb2.InlineImage{Href: "#" + coverID}
		book.Binaries = append(book.Binaries, fb2.BinaryObject{
			ID:          coverID,
			ContentType: "image/jpeg",
			Data:        env.DefaultCover,
		})
		book.Description.TitleInfo.Coverpage = append([]fb2.InlineImage{}, ref)
	}

	// Process vignettes configuration
	vignettes, err := prepareVignettes(&env.Cfg.Document.Vignettes, env.DefaultVignettes)
	if err != nil {
		return nil, err
	}

	// Order of calls is important here!

	// Normalize footnote bodies and build footnote index
	book, footnotes := book.NormalizeFootnoteBodies(log)
	// For floatRenumbered mode, renumber footnotes and update labels
	if env.Cfg.Document.Footnotes.Mode == common.FootnotesModeFloatRenumbered {
		book, footnotes = book.NormalizeFootnoteLabels(footnotes, env.Cfg.Document.Footnotes.LabelTemplate, log)
	}
	// Build id and link indexes replacing/removing broken links (may add not-found image binary and vignette binaries)
	book, ids, links := book.NormalizeLinks(vignettes, log)
	// Normalize stylesheets and resolve external resources (may load files from disk and add to binaries)
	// Pass default stylesheet so it's processed uniformly with FB2 embedded stylesheets
	book = book.NormalizeStylesheets(srcName, env.DefaultStyle, log)
	// Assign sequential IDs to all sections without IDs
	// (avoiding collisions with existing IDs) - we will need it for ToC. This
	// also updates the ID index with generated IDs marked as "section-generated"
	book, ids = book.NormalizeIDs(ids, log)
	// Apply text transformations to regular content paragraphs
	book = book.TransformText(&env.Cfg.Document.Transformations)
	// Mark first paragraphs in sections with drop-cap style for rendering
	book = book.MarkDropcaps(&env.Cfg.Document.Dropcaps)

	// Process all binary objects creating actual images and reference index
	// This happens after NormalizeLinks so the not-found image binary is included
	allImages := book.PrepareImages(outputFormat.ForKindle(), &env.Cfg.Document.Images, log)

	// Filter images to only include those that are actually referenced
	imagesIndex := book.FilterReferencedImages(allImages, links, coverID, log)

	c := &Content{
		SrcName:        srcName,
		Doc:            doc,
		OutputFormat:   outputFormat,
		KindleEbook:    env.KindleEbook && outputFormat.ForKindle(),
		ASIN:           env.KindleASIN,
		FootnotesMode:  env.Cfg.Document.Footnotes.Mode,
		BacklinkStr:    string(env.Cfg.Document.Footnotes.Backlinks),
		MoreParaStr:    string(env.Cfg.Document.Footnotes.MoreParagraphs),
		ScreenWidth:    env.Cfg.Document.Images.Screen.Width,
		ScreenHeight:   env.Cfg.Document.Images.Screen.Height,
		Book:           book,
		CoverID:        coverID,
		FootnotesIndex: footnotes,
		ImagesIndex:    imagesIndex,
		UsedImageIDs:   make(map[string]bool),
		IDsIndex:       ids,
		LinksRevIndex:  links,
		WorkDir:        tmpDir,
		Debug:          env.Rpt != nil,
		BackLinkIndex:  make(map[string][]BackLinkRef),
		PageMapIndex:   make(map[string][]PageMapEntry),
	}

	// Initialize page map settings
	if env.Cfg.Document.PageMap.Enable {
		c.PageSize = env.Cfg.Document.PageMap.Size
		if outputFormat == common.OutputFmtEpub2 || outputFormat == common.OutputFmtKepub {
			c.AdobeDE = env.Cfg.Document.PageMap.AdobeDE
		}
	}

	if env.Cfg.Document.InsertSoftHyphen {
		c.Hyphen = text.NewHyphenator(book.Description.TitleInfo.Lang, log)
	}

	// We only need sentences tokenizer for kepub
	if outputFormat == common.OutputFmtKepub {
		c.Splitter = text.NewSplitter(book.Description.TitleInfo.Lang, log)
	}

	// Save prepared document to file for debugging
	if env.Rpt != nil {
		if err := os.WriteFile(filepath.Join(tmpDir, baseSrcName+"_prepared"), []byte(c.String()), 0644); err != nil {
			return nil, fmt.Errorf("unable to write prepared doc for debugging: %w", err)
		}
	}

	return c, nil
}

// prepareVignettes creates a map of vignette binaries from configuration
// Returns an initialized but empty map if no vignettes are defined
func prepareVignettes(vigCfg *config.VignettesConfig, defaultVignettes map[common.VignettePos][]byte) (map[common.VignettePos]*fb2.BinaryObject, error) {
	vignettes := make(map[common.VignettePos]*fb2.BinaryObject)

	vignetteChecks := []struct {
		configValue string
		position    common.VignettePos
	}{
		{vigCfg.Book.TitleTop, common.VignettePosBookTitleTop},
		{vigCfg.Book.TitleBottom, common.VignettePosBookTitleBottom},
		{vigCfg.Chapter.TitleTop, common.VignettePosChapterTitleTop},
		{vigCfg.Chapter.TitleBottom, common.VignettePosChapterTitleBottom},
		{vigCfg.Chapter.End, common.VignettePosChapterEnd},
		{vigCfg.Section.TitleTop, common.VignettePosSectionTitleTop},
		{vigCfg.Section.TitleBottom, common.VignettePosSectionTitleBottom},
		{vigCfg.Section.End, common.VignettePosSectionEnd},
	}

	for _, check := range vignetteChecks {
		if check.configValue == "" {
			continue
		}

		var data []byte
		var contentType string
		var builtinVignette bool
		if check.configValue == "builtin" {
			var ok bool
			data, ok = defaultVignettes[check.position]
			if !ok {
				continue
			}
			contentType = "image/svg+xml"
			builtinVignette = true
		} else {
			fileData, err := os.ReadFile(check.configValue)
			if err != nil {
				return nil, fmt.Errorf("failed to read vignette file %q: %w", check.configValue, err)
			}
			data = fileData
			contentType = http.DetectContentType(data)

			// SVG files are detected as text/plain or text/xml, so check for SVG content
			if strings.HasPrefix(contentType, "text/") && bytes.Contains(data, []byte("<svg")) {
				contentType = "image/svg+xml"
			}

			if !strings.HasPrefix(contentType, "image/") {
				return nil, fmt.Errorf("vignette file %q has unsupported content type %q (only image/* types are supported)", check.configValue, contentType)
			}
		}

		vignettes[check.position] = &fb2.BinaryObject{
			ContentType:     contentType,
			Data:            data,
			BuiltinVignette: builtinVignette,
		}
	}

	return vignettes, nil
}

// KoboSpanNextSentence increments sentence and returns the current Kobo span
func (c *Content) KoboSpanNextSentence() (int, int) {
	c.koboSpanSentences++
	return c.koboSpanParagraphs, c.koboSpanSentences
}

// KoboSpanNextParagraph increments paragraph, resets sentence, and returns previous Kobo span
func (c *Content) KoboSpanNextParagraph() (int, int) {
	oldParagraphs, oldSentences := c.koboSpanParagraphs, c.koboSpanSentences
	c.koboSpanParagraphs++
	c.koboSpanSentences = 0
	return oldParagraphs, oldSentences
}

func (c *Content) KoboSpanSet(paragraphs, sentences int) {
	c.koboSpanParagraphs, c.koboSpanSentences = paragraphs, sentences
}

// UpdatePageRuneCount adds rune count and checks if a new page boundary is reached
func (c *Content) UpdatePageRuneCount(text string) {
	if c.PageSize == 0 || !c.PageTrackingEnabled {
		return
	}
	c.pageRuneCounter += utf8.RuneCountInString(text)
}

// SplitTextByPage splits text into chunks by page boundaries at word boundaries.
// It returns chunks and a parallel slice indicating whether a page marker
// should be inserted after each chunk.
// Page breaks occur at the last word boundary before the page size is reached,
// ensuring words are not split across pages.
func (c *Content) SplitTextByPage(text string) ([]string, []bool) {
	if text == "" {
		return nil, nil
	}
	if c.PageSize == 0 || !c.PageTrackingEnabled {
		return []string{text}, []bool{false}
	}

	runes := []rune(text)
	var chunks []string
	var markers []bool

	chunkStart := 0
	lastWordBoundary := -1 // Position of last word boundary in current chunk
	pos := 0

	for pos < len(runes) {
		// Track word boundaries
		if IsWordBoundary(runes[pos]) {
			lastWordBoundary = pos
		}

		c.pageRuneCounter++
		pos++

		if c.pageRuneCounter >= c.PageSize {
			// Page boundary reached - split at last word boundary if available
			splitPos := pos // Default: split after current rune

			if lastWordBoundary > chunkStart {
				// Backtrack to word boundary
				runesAfterBoundary := pos - lastWordBoundary - 1
				c.pageRuneCounter = runesAfterBoundary
				splitPos = lastWordBoundary + 1
			} else {
				c.pageRuneCounter = 0
			}

			chunk := string(runes[chunkStart:splitPos])
			if chunk != "" {
				chunks = append(chunks, chunk)
				markers = append(markers, true)
			}

			chunkStart = splitPos
			lastWordBoundary = -1
		}
	}

	// Remaining text
	if chunkStart < len(runes) {
		chunks = append(chunks, string(runes[chunkStart:]))
		markers = append(markers, false)
	}

	return chunks, markers
}

// IsWordBoundary returns true if the rune is a word boundary character
// (whitespace or common punctuation that typically allows line breaks).
func IsWordBoundary(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	switch r {
	case '-', '–', '—', // hyphens and dashes
		',', '.', ';', ':', '!', '?', // punctuation
		')', ']', '}', '»', '"', '\'', // closing brackets/quotes
		'/': // slash
		return true
	}
	return false
}

// CheckPageBoundary checks if we've crossed a page boundary and returns true if a page marker should be inserted
func (c *Content) CheckPageBoundary() bool {
	if c.PageSize == 0 || !c.PageTrackingEnabled {
		return false
	}
	if c.pageRuneCounter < c.PageSize {
		return false
	}
	c.pageRuneCounter -= c.PageSize
	return true
}

// AddPageMapEntry records a page marker in the current file
func (c *Content) AddPageMapEntry() string {
	c.TotalPages++
	spanID := fmt.Sprintf("page%d", c.TotalPages)
	entry := PageMapEntry{
		PageNum:  c.TotalPages,
		SpanID:   spanID,
		Filename: c.CurrentFilename,
	}
	c.PageMapIndex[c.CurrentFilename] = append(c.PageMapIndex[c.CurrentFilename], entry)
	return spanID
}

func (c *Content) ResetPageMap() {
	c.pageRuneCounter = 0
	c.TotalPages = 0
	c.PageMapIndex = make(map[string][]PageMapEntry)
}

// StartNewPageAtChapter resets the rune counter so the next content starts on a new page.
// Call this at chapter/section boundaries to ensure chapters start on fresh pages.
// Unlike ForceNewPage, this doesn't record a synthetic page entry - the first content
// in the new chapter will naturally continue page numbering.
func (c *Content) StartNewPageAtChapter() {
	if c.PageSize == 0 || !c.PageTrackingEnabled {
		return
	}
	c.pageRuneCounter = 0
}

// ForceNewPage records a synthetic page for a file.
// Used for spine items where we don't (or can't) inject in-document page markers (e.g., cover, TOC page).
func (c *Content) ForceNewPage(filename string) {
	if c.PageSize == 0 {
		return
	}
	c.TotalPages++
	c.pageRuneCounter = 0
	entry := PageMapEntry{
		PageNum:  c.TotalPages,
		SpanID:   "",
		Filename: filename,
	}
	c.PageMapIndex[filename] = append(c.PageMapIndex[filename], entry)
}

// GetAllPagesSeq returns an iterator over all page entries sorted by page number
func (c *Content) GetAllPagesSeq() iter.Seq[PageMapEntry] {
	return func(yield func(PageMapEntry) bool) {
		var pages []PageMapEntry
		for _, entries := range c.PageMapIndex {
			pages = append(pages, entries...)
		}
		slices.SortFunc(pages, func(a, b PageMapEntry) int {
			return a.PageNum - b.PageNum
		})
		for _, page := range pages {
			if !yield(page) {
				return
			}
		}
	}
}

// TrackImageUsage records that an image ID was referenced during generation.
func (c *Content) TrackImageUsage(id string) {
	if id == "" {
		return
	}
	if c.UsedImageIDs == nil {
		c.UsedImageIDs = make(map[string]bool)
	}
	c.UsedImageIDs[id] = true
}

// AddFootnoteBackLinkRef adds a footnote reference and returns the BackLinkRef for generating links
func (c *Content) AddFootnoteBackLinkRef(targetID string) BackLinkRef {
	refs := c.BackLinkIndex[targetID]
	refNum := len(refs) + 1
	ref := BackLinkRef{
		RefID:    fmt.Sprintf("ref-%s-%d", targetID, refNum),
		TargetID: targetID,
		Filename: c.CurrentFilename,
	}
	c.BackLinkIndex[targetID] = append(refs, ref)
	return ref
}

// makeCharsetReader returns a CharsetReader function that detects when an XML
// document declares a non-UTF-8 encoding (e.g. "Windows-1251") but its content
// is actually valid UTF-8. This is a common problem with FB2 files from various
// sources. When a mismatch is detected, the reader bypasses charset conversion
// to avoid producing mojibake. For genuinely non-UTF-8 content, it falls
// through to charset.NewReaderLabel for proper transcoding.
func makeCharsetReader(log *zap.Logger) func(string, io.Reader) (io.Reader, error) {
	return func(label string, input io.Reader) (io.Reader, error) {
		// Read a sample from the stream to check the actual encoding.
		// We need enough bytes to see multi-byte UTF-8 sequences (Cyrillic,
		// CJK, etc.) that would be mangled by a wrong charset decoder.
		const peekSize = 2048 // 2KB should be enough for detection in most cases
		buf, err := io.ReadAll(io.LimitReader(input, peekSize))
		if err != nil {
			return nil, fmt.Errorf("unable to peek at XML content: %w", err)
		}

		// Reconstruct the full reader: peeked bytes + remainder.
		restored := io.MultiReader(bytes.NewReader(buf), input)

		// The peek buffer may split a multi-byte UTF-8 sequence at the
		// boundary, making utf8.Valid reject otherwise valid UTF-8 content.
		// Trim any trailing incomplete sequence before validation.
		checkBuf := trimIncompleteUTF8(buf)

		if utf8.Valid(checkBuf) && containsNonASCII(checkBuf) {
			log.Warn("XML declares non-UTF-8 encoding but content is valid UTF-8, ignoring declared encoding",
				zap.String("declared", label))
			return restored, nil
		}

		// Content is not valid UTF-8 — honour the declared encoding.
		return charset.NewReaderLabel(label, restored)
	}
}

// trimIncompleteUTF8 returns buf with any trailing incomplete multi-byte UTF-8
// sequence removed. This is needed when a fixed-size peek buffer splits a
// multi-byte character at the boundary.
func trimIncompleteUTF8(buf []byte) []byte {
	if len(buf) == 0 || buf[len(buf)-1] < 0x80 {
		return buf // ends with ASCII — nothing to trim
	}
	// Walk backwards to find the start of the last (possibly incomplete) rune.
	// UTF-8 continuation bytes are 10xxxxxx (0x80..0xBF), leader bytes start
	// with 11xxxxxx (0xC0+). We need at most 3 trailing continuation bytes
	// before hitting a leader.
	i := len(buf) - 1
	for i > 0 && i > len(buf)-4 && buf[i]&0xC0 == 0x80 {
		i--
	}
	// i now points at the leader byte of the last rune. If DecodeRune
	// returns RuneError, the sequence is incomplete — trim it off.
	r, _ := utf8.DecodeRune(buf[i:])
	if r == utf8.RuneError {
		return buf[:i]
	}
	return buf
}

// containsNonASCII reports whether buf contains at least one byte > 0x7F.
// A pure-ASCII payload is ambiguous (valid in both UTF-8 and any single-byte
// encoding), so we only override the declared charset when we see multi-byte
// UTF-8 sequences that would be damaged by wrong transcoding.
func containsNonASCII(buf []byte) bool {
	for _, b := range buf {
		if b > 0x7F {
			return true
		}
	}
	return false
}
