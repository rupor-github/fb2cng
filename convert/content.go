package convert

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
	"fbc/convert/text"
	"fbc/fb2"
	"fbc/misc"
	"fbc/state"
)

// Content encapsulates both the raw FB2 XML document and the structured
// normalized internal representation derived from the official FictionBook 2.0
// schemas. https://github.com/gribuser/fb2.git commit
// 4d3740e319039911c30d291abb0c8b26ec99703b
type Content struct {
	srcName string
	doc     *etree.Document

	book           *fb2.FictionBook
	coverID        string
	footnotesIndex fb2.FootnoteRefs
	imagesIndex    fb2.BookImages
	idsIndex       fb2.IDIndex
	linksRevIndex  fb2.ReverseLinkIndex

	splitter *text.Splitter
	hyphen   *text.Hyphenator
	tmpDir   string
}

// Accessor methods to expose Content fields to avoid cyclic imports in
// generator packages

func (c *Content) Book() *fb2.FictionBook { return c.book }

func (c *Content) CoverID() string { return c.coverID }

func (c *Content) FootnotesIndex() fb2.FootnoteRefs { return c.footnotesIndex }

func (c *Content) ImagesIndex() fb2.BookImages { return c.imagesIndex }

func (c *Content) IDsIndex() fb2.IDIndex { return c.idsIndex }

func (c *Content) LinksRevIndex() fb2.ReverseLinkIndex { return c.linksRevIndex }

func (c *Content) WorkDir() string { return c.tmpDir }

// prepareContent reads, parses, and prepares FB2 content for conversion.
func prepareContent(ctx context.Context, r io.Reader, srcName string, kindle bool, log *zap.Logger) (*Content, error) {
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

	if refID != uuid.Nil {
		book.Description.DocumentInfo.ID = refID.String()
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
	// Build id and link indexes replacing/removing broken links (may add not-found image binary)
	book, ids, links := book.NormalizeLinks(log)

	// Process all binary objects creating actual images and reference index
	// This happens after NormalizeLinks so the not-found image binary is included
	allImages, err := book.PrepareImages(kindle, &env.Cfg.Document.Images, log)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare images: %w", err)
	}

	// Filter images to only include those that are actually referenced
	imagesIndex := filterReferencedImages(allImages, links, coverID, log)

	content := &Content{
		srcName:        srcName,
		doc:            doc,
		book:           book,
		coverID:        coverID,
		footnotesIndex: footnotes,
		imagesIndex:    imagesIndex,
		idsIndex:       ids,
		linksRevIndex:  links,
		tmpDir:         tmpDir,
	}

	if env.Cfg.Document.InsertSoftHyphen {
		content.hyphen = text.NewHyphenator(book.Description.TitleInfo.Lang, log)
	}

	// TODO: old converter only turned on sentences tokenizer for kepub (where
	// actual sentences are used), should I keep the same logic?
	if env.OutputFormat == config.OutputFmtKepub {
		content.splitter = text.NewSplitter(book.Description.TitleInfo.Lang, log)
	}

	// Save prepared document to file for debugging
	if env.Rpt != nil {
		if err := os.WriteFile(filepath.Join(tmpDir, baseSrcName+"_prepared"), []byte(content.String()), 0644); err != nil {
			return nil, fmt.Errorf("unable to write prepared doc for debugging: %w", err)
		}
	}

	return content, nil
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
