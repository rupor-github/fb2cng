package content

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/beevik/etree"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/net/html/charset"

	"fbc/config"
	"fbc/content/text"
	"fbc/fb2"
	"fbc/misc"
	"fbc/state"
)

// Content encapsulates both the raw FB2 XML document and the structured
// normalized internal representation derived from the official FictionBook 2.0
// schemas. https://github.com/gribuser/fb2.git commit 4d3740e319039911c30d291abb0c8b26ec99703b
type Content struct {
	SrcName      string
	Doc          *etree.Document
	OutputFormat config.OutputFmt

	Book           *fb2.FictionBook
	CoverID        string
	FootnotesIndex fb2.FootnoteRefs
	ImagesIndex    fb2.BookImages
	IDsIndex       fb2.IDIndex
	LinksRevIndex  fb2.ReverseLinkIndex

	Splitter *text.Splitter
	Hyphen   *text.Hyphenator
	WorkDir  string

	koboSpanParagraphs int
	koboSpanSentences  int
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

// Prepare reads, parses, and prepares FB2 content for conversion.
func Prepare(ctx context.Context, r io.Reader, srcName string, outputFormat config.OutputFmt, log *zap.Logger) (*Content, error) {
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
		CharsetReader: charset.NewReaderLabel,
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

	// Normalize footnote bodies and build footnote index
	book, footnotes := book.NormalizeFootnoteBodies(log)
	// Flatten grouping sections (sections without titles that only contain other sections)
	book = book.NormalizeSections(log)
	// Build id and link indexes replacing/removing broken links (may add not-found image binary)
	book, ids, links := book.NormalizeLinks(log)
	// Assign sequential IDs to all sections and subtitles without IDs
	// (avoiding collisions with existing IDs) - we will need it for ToC. This
	// also updates the ID index with generated IDs marked as "TYPE-generated"
	book, ids = book.NormalizeIDs(ids, log)

	// Process all binary objects creating actual images and reference index
	// This happens after NormalizeLinks so the not-found image binary is included
	allImages := book.PrepareImages(outputFormat.ForKindle(), &env.Cfg.Document.Images, log)

	// Filter images to only include those that are actually referenced
	imagesIndex := filterReferencedImages(allImages, links, coverID, log)

	c := &Content{
		SrcName:        srcName,
		Doc:            doc,
		OutputFormat:   outputFormat,
		Book:           book,
		CoverID:        coverID,
		FootnotesIndex: footnotes,
		ImagesIndex:    imagesIndex,
		IDsIndex:       ids,
		LinksRevIndex:  links,
		WorkDir:        tmpDir,
	}

	if env.Cfg.Document.InsertSoftHyphen {
		c.Hyphen = text.NewHyphenator(book.Description.TitleInfo.Lang, log)
	}

	// TODO: old converter only used sentences tokenizer for kepub (where
	// actual sentences are necessary), should I keep the same logic?
	if outputFormat == config.OutputFmtKepub {
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

// filterReferencedImages returns only images that are actually referenced in the book
func filterReferencedImages(allImages fb2.BookImages, links fb2.ReverseLinkIndex, coverID string, log *zap.Logger) fb2.BookImages {
	referenced := make(map[string]bool)

	// Always include the not-found image if it exists (it may be needed for broken links)
	if _, exists := allImages[fb2.NotFoundImageID]; exists {
		referenced[fb2.NotFoundImageID] = true
	}

	// Add cover image if present
	if coverID != "" {
		referenced[coverID] = true
	}

	// Add all images referenced in links
	for targetID, refs := range links {
		if len(refs) == 0 {
			continue
		}

		// Check if any reference is an image type
		for _, ref := range refs {
			switch ref.Type {
			case "coverpage", "block-image", "inline-image":
				referenced[targetID] = true
			}
		}
	}

	// Build filtered index
	filtered := make(fb2.BookImages)
	for id := range referenced {
		if img, exists := allImages[id]; exists {
			filtered[id] = img
			continue
		}
		log.Debug("Referenced image not found in prepared images", zap.String("id", id))
	}

	log.Debug("Filtered images index", zap.Int("total", len(allImages)), zap.Int("referenced", len(filtered)))
	for id, img := range allImages {
		if _, exists := filtered[id]; !exists {
			log.Debug("Excluding unreferenced image", zap.String("id", id), zap.String("type", img.MimeType))
		}
	}

	return filtered
}
