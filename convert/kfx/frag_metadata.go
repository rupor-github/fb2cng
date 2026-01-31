package kfx

import (
	"go.uber.org/zap"
	"golang.org/x/text/language"

	"fbc/config"
	"fbc/content"
	"fbc/fb2"
	"fbc/misc"
)

// BuildMetadata creates the $258 metadata fragment from content.
// This contains reading_orders for navigation.
func BuildMetadata(sectionNames sectionNameList) *Fragment {
	metadata := NewStruct()

	// Reading orders ($169) - must match document_data
	if len(sectionNames) > 0 {
		sections := make([]any, 0, len(sectionNames))
		for _, name := range sectionNames {
			sections = append(sections, SymbolByName(name))
		}
		readingOrder := NewReadingOrder(SymDefault, sections)
		metadata.SetList(SymReadingOrders, []any{readingOrder})
	}

	return NewRootFragment(SymMetadata, metadata)
}

// BuildBookMetadata creates the $490 book_metadata fragment.
// This contains categorised metadata: title, author, language, etc.
func BuildBookMetadata(c *content.Content, cfg *config.DocumentConfig, containerID, coverResourceName string, log *zap.Logger) *Fragment {
	// Kindle title metadata
	titleMetadata := make([]any, 0)

	// Stable IDs (these don't need to match Amazon exactly, but having them helps parity/compat)
	asin := hashToAlphanumeric(c.Book.Description.DocumentInfo.ID, 32)
	if asin != "" {
		titleMetadata = append(titleMetadata, NewMetadataEntry("ASIN", asin))
		titleMetadata = append(titleMetadata, NewMetadataEntry("content_id", asin))
	}
	if containerID != "" {
		titleMetadata = append(titleMetadata, NewMetadataEntry("asset_id", containerID))
	}
	bookID := c.Book.Description.DocumentInfo.ID
	if bookID != "" {
		titleMetadata = append(titleMetadata, NewMetadataEntry("book_id", bookID))
	}
	contentType := "PDOC"
	if c.KindleEbook {
		contentType = "EBOK"
	}
	titleMetadata = append(titleMetadata, NewMetadataEntry("cde_content_type", contentType))
	if coverResourceName != "" {
		titleMetadata = append(titleMetadata, NewMetadataEntry("cover_image", coverResourceName))
	}
	// Common boolean flags present in reference output.
	titleMetadata = append(titleMetadata, NewMetadataEntry("is_sample", false))
	titleMetadata = append(titleMetadata, NewMetadataEntry("override_kindle_font", false))

	// Title
	title := c.Book.Description.TitleInfo.BookTitle.Value
	if cfg.Metainformation.TitleTemplate != "" {
		expanded, err := c.Book.ExpandTemplateMetainfo(
			config.MetaTitleTemplateFieldName,
			cfg.Metainformation.TitleTemplate,
			c.SrcName,
			c.OutputFormat,
		)
		if err != nil {
			log.Warn("Unable to expand title template for KFX metadata", zap.Error(err))
		} else {
			title = expanded
		}
	}
	if cfg.Metainformation.Transliterate {
		title = fb2.Transliterate(title)
	}
	if title != "" {
		titleMetadata = append(titleMetadata, NewMetadataEntry("title", title))
	}

	// Author(s)
	for _, author := range c.Book.Description.TitleInfo.Authors {
		authorName := formatAuthorName(author)
		if cfg.Metainformation.CreatorNameTemplate != "" {
			expanded, err := c.Book.ExpandTemplateAuthorName(
				config.MetaCreatorNameTemplateFieldName,
				cfg.Metainformation.CreatorNameTemplate,
				0,
				&author,
			)
			if err != nil {
				log.Warn("Unable to expand author name template for KFX metadata", zap.Error(err))
			} else {
				authorName = expanded
			}
		}
		if cfg.Metainformation.Transliterate {
			authorName = fb2.Transliterate(authorName)
		}
		if authorName != "" {
			titleMetadata = append(titleMetadata, NewMetadataEntry("author", authorName))
		}
	}

	// Language
	if lang := c.Book.Description.TitleInfo.Lang; lang != language.Und {
		titleMetadata = append(titleMetadata, NewMetadataEntry("language", lang.String()))
	}

	// Publisher
	if pub := c.Book.Description.PublishInfo; pub != nil && pub.Publisher != nil && pub.Publisher.Value != "" {
		titleMetadata = append(titleMetadata, NewMetadataEntry("publisher", pub.Publisher.Value))
	}

	// Description/annotation
	if annot := c.Book.Description.TitleInfo.Annotation; annot != nil {
		desc := annot.AsPlainText()
		if desc != "" {
			titleMetadata = append(titleMetadata, NewMetadataEntry("description", desc))
		}
	}

	// Build categorised metadata structure
	categories := make([]any, 0)
	if len(titleMetadata) > 0 {
		categories = append(categories, NewCategorisedMetadata("kindle_title_metadata", titleMetadata))
	}

	// Kindle audit metadata (creator info)
	auditMetadata := []any{
		NewMetadataEntry("creator_version", misc.GetVersion()),
		NewMetadataEntry("file_creator", misc.GetAppName()),
	}
	categories = append(categories, NewCategorisedMetadata("kindle_audit_metadata", auditMetadata))

	// Kindle ebook metadata (capabilities)
	ebookMetadata := []any{
		NewMetadataEntry("selection", "enabled"),
		NewMetadataEntry("nested_span", "enabled"),
	}
	categories = append(categories, NewCategorisedMetadata("kindle_ebook_metadata", ebookMetadata))

	// Kindle capability metadata (empty but required)
	categories = append(categories, NewCategorisedMetadata("kindle_capability_metadata", []any{}))

	bookMetadata := NewStruct().SetList(SymCatMetadata, categories) // $491

	return NewRootFragment(SymBookMetadata, bookMetadata)
}

// BuildDocumentData creates the $538 document_data fragment.
// This contains reading orders and global reading defaults.
func BuildDocumentData(sectionNames sectionNameList, maxID int) *Fragment {
	// Build reading order with section list as symbol references
	sections := make([]any, 0, len(sectionNames))
	for _, name := range sectionNames {
		sections = append(sections, SymbolByName(name))
	}

	readingOrder := NewReadingOrder(SymDefault, sections) // $351 = default

	// Use a string-keyed map because KP3 includes a non-$ field name: "max_id".
	// This will be added to the local symbol table automatically.
	docData := map[string]any{
		"$16":    DimensionValue(1.0, SymUnitEm),
		"$42":    DimensionValue(1.2, SymUnitEm),
		"$112":   SymbolValue(SymAuto),
		"$192":   SymbolValue(SymLtr),
		"$436":   SymbolValue(SymEnabled),
		"$477":   SymbolValue(SymWidth),
		"$560":   SymbolValue(SymHorizontalTb),
		"$169":   []any{readingOrder},
		"max_id": int64(maxID),
	}

	return NewRootFragment(SymDocumentData, docData)
}

// formatAuthorName formats an author's name from FB2 author struct.
func formatAuthorName(author fb2.Author) string {
	var parts []string
	if author.FirstName != "" {
		parts = append(parts, author.FirstName)
	}
	if author.MiddleName != "" {
		parts = append(parts, author.MiddleName)
	}
	if author.LastName != "" {
		parts = append(parts, author.LastName)
	}

	if len(parts) == 0 && author.Nickname != "" {
		return author.Nickname
	}

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
}
