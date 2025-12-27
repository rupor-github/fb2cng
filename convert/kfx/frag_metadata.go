package kfx

import (
	"go.uber.org/zap"
	"golang.org/x/text/language"

	"fbc/config"
	"fbc/content"
	"fbc/fb2"
	"fbc/misc"
)

// BuildMetadataFragment creates the $258 metadata fragment from content.
// This contains reading_orders for navigation.
func BuildMetadataFragment(sectionNames []string) *Fragment {
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

// BuildBookMetadataFragment creates the $490 book_metadata fragment.
// This contains categorised metadata: title, author, language, etc.
func BuildBookMetadataFragment(c *content.Content, cfg *config.DocumentConfig, log *zap.Logger) *Fragment {
	// Kindle title metadata
	titleMetadata := make([]any, 0)

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
	}
	categories = append(categories, NewCategorisedMetadata("kindle_ebook_metadata", ebookMetadata))

	// Kindle capability metadata (empty but required)
	categories = append(categories, NewCategorisedMetadata("kindle_capability_metadata", []any{}))

	bookMetadata := NewStruct().SetList(SymCatMetadata, categories) // $491

	return NewRootFragment(SymBookMetadata, bookMetadata)
}

// BuildDocumentDataFragment creates the $538 document_data fragment.
// This contains reading orders and is required for KFX v2.
func BuildDocumentDataFragment(sectionNames []string) *Fragment {
	// Build reading order with section list as symbol references
	sections := make([]any, 0, len(sectionNames))
	for _, name := range sectionNames {
		sections = append(sections, SymbolByName(name))
	}

	readingOrder := NewReadingOrder(SymDefault, sections) // $351 = default

	docData := NewStruct().
		SetList(SymReadingOrders, []any{readingOrder}) // $169 = reading_orders

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
