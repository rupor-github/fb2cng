package kfx

import (
	"go.uber.org/zap"
	"golang.org/x/text/language"

	"fbc/config"
	"fbc/content"
	"fbc/fb2"
)

// BuildMetadataFragment creates the $258 metadata fragment from content.
// This contains basic book metadata: title, author, language, etc.
func BuildMetadataFragment(c *content.Content, cfg *config.DocumentConfig, log *zap.Logger) *Fragment {
	metadata := NewStruct()

	// Title ($153) - use template if configured
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
		metadata.SetString(SymTitle, title)
	}

	// Author ($222) - use template if configured, otherwise format from FB2 author
	if len(c.Book.Description.TitleInfo.Authors) > 0 {
		author := c.Book.Description.TitleInfo.Authors[0]
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
			metadata.SetString(SymAuthor, authorName)
		}
	}

	// Language ($10)
	if lang := c.Book.Description.TitleInfo.Lang; lang != language.Und {
		metadata.SetString(SymLanguage, lang.String())
	}

	// Publisher ($232)
	if pub := c.Book.Description.PublishInfo; pub != nil && pub.Publisher != nil && pub.Publisher.Value != "" {
		metadata.SetString(SymPublisher, pub.Publisher.Value)
	}

	// ISBN ($223)
	if pub := c.Book.Description.PublishInfo; pub != nil && pub.ISBN != nil && pub.ISBN.Value != "" {
		metadata.SetString(SymISBN, pub.ISBN.Value)
	}

	return NewRootFragment(SymMetadata, metadata)
}

// BuildBookMetadataFragment creates the $490 book_metadata fragment.
// This is the categorised metadata format used by newer KFX files.
func BuildBookMetadataFragment(c *content.Content, cfg *config.DocumentConfig, log *zap.Logger) *Fragment {
	// Build kindle_title_metadata category
	titleMetadata := make([]any, 0)

	// Title - use template if configured
	title := c.Book.Description.TitleInfo.BookTitle.Value
	if cfg.Metainformation.TitleTemplate != "" {
		expanded, err := c.Book.ExpandTemplateMetainfo(
			config.MetaTitleTemplateFieldName,
			cfg.Metainformation.TitleTemplate,
			c.SrcName,
			c.OutputFormat,
		)
		if err != nil {
			log.Warn("Unable to expand title template for KFX book metadata", zap.Error(err))
		} else {
			title = expanded
		}
	}
	if title != "" {
		titleMetadata = append(titleMetadata, NewMetadataEntry("title", title))
	}

	// Author - use template if configured
	if len(c.Book.Description.TitleInfo.Authors) > 0 {
		author := c.Book.Description.TitleInfo.Authors[0]
		authorName := formatAuthorName(author)

		if cfg.Metainformation.CreatorNameTemplate != "" {
			expanded, err := c.Book.ExpandTemplateAuthorName(
				config.MetaCreatorNameTemplateFieldName,
				cfg.Metainformation.CreatorNameTemplate,
				0,
				&author,
			)
			if err != nil {
				log.Warn("Unable to expand author name template for KFX book metadata", zap.Error(err))
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

	// Build categorised_metadata list
	catMetadata := ListValue{
		NewCategorisedMetadata("kindle_title_metadata", titleMetadata),
	}

	bookMetadata := NewStruct().
		SetList(SymCatMetadata, catMetadata.ToSlice()) // $491 = categorised_metadata

	return NewRootFragment(SymBookMetadata, bookMetadata)
}

// BuildDocumentDataFragment creates the $538 document_data fragment.
// This contains reading orders and is required for KFX v2.
func BuildDocumentDataFragment(readingOrderName string, sectionIDs []string) *Fragment {
	// Build reading order with section list
	sections := make([]any, 0, len(sectionIDs))
	for _, sid := range sectionIDs {
		sections = append(sections, SymbolValue(SymbolID(sid)))
	}

	readingOrder := NewReadingOrder(readingOrderName, sections)

	docData := NewStruct().
		SetList(SymReadingOrders, []any{readingOrder}) // $169 = reading_orders

	return NewRootFragment(SymDocumentData, docData)
}

// BuildDocumentDataFragmentSimple creates a simple $538 with just an ID.
func BuildDocumentDataFragmentSimple(readingOrderName string) *Fragment {
	readingOrder := NewStruct().SetString(SymUniqueID, readingOrderName)

	docData := NewStruct().
		SetList(SymReadingOrders, []any{readingOrder})

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
