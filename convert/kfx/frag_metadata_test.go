package kfx

import (
	"testing"

	"go.uber.org/zap"
	"golang.org/x/text/language"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/fb2"
)

func TestBuildBookMetadata_Transliterate(t *testing.T) {
	log := zap.NewNop()

	// Create content with Cyrillic title and author
	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Война и мир"},
					Authors: []fb2.Author{
						{FirstName: "Лев", MiddleName: "Николаевич", LastName: "Толстой"},
					},
					Lang: language.Russian,
				},
				DocumentInfo: fb2.DocumentInfo{
					ID: "test-book-id",
				},
			},
		},
		OutputFormat: common.OutputFmtKfx,
	}

	t.Run("without transliteration", func(t *testing.T) {
		cfg := &config.DocumentConfig{
			Metainformation: config.MetainformationConfig{
				Transliterate: false,
			},
		}

		frag := BuildBookMetadata(c, cfg, "container-id", "cover.jpg", log)
		if frag == nil {
			t.Fatal("Expected fragment, got nil")
		}

		// Extract metadata and check title/author are in original Cyrillic
		metadata := extractTitleMetadata(t, frag)

		title, ok := metadata["title"]
		if !ok {
			t.Fatal("Expected title in metadata")
		}
		if title != "Война и мир" {
			t.Errorf("Expected Cyrillic title 'Война и мир', got %q", title)
		}

		author, ok := metadata["author"]
		if !ok {
			t.Fatal("Expected author in metadata")
		}
		if author != "Лев Николаевич Толстой" {
			t.Errorf("Expected Cyrillic author 'Лев Николаевич Толстой', got %q", author)
		}
	})

	t.Run("with transliteration", func(t *testing.T) {
		cfg := &config.DocumentConfig{
			Metainformation: config.MetainformationConfig{
				Transliterate: true,
			},
		}

		frag := BuildBookMetadata(c, cfg, "container-id", "cover.jpg", log)
		if frag == nil {
			t.Fatal("Expected fragment, got nil")
		}

		metadata := extractTitleMetadata(t, frag)

		title, ok := metadata["title"]
		if !ok {
			t.Fatal("Expected title in metadata")
		}
		// fb2.Transliterate preserves spaces and capitalization
		if title != "Voina i mir" {
			t.Errorf("Expected transliterated title 'Voina i mir', got %q", title)
		}

		author, ok := metadata["author"]
		if !ok {
			t.Fatal("Expected author in metadata")
		}
		if author != "Lev Nikolaevich Tolstoi" {
			t.Errorf("Expected transliterated author 'Lev Nikolaevich Tolstoi', got %q", author)
		}
	})
}

func TestBuildBookMetadata_TitleTemplate(t *testing.T) {
	log := zap.NewNop()

	c := &content.Content{
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book"},
					Authors: []fb2.Author{
						{FirstName: "John", LastName: "Doe"},
					},
					Lang: language.English,
				},
				DocumentInfo: fb2.DocumentInfo{
					ID: "test-id",
				},
			},
		},
		SrcName:      "test.fb2",
		OutputFormat: common.OutputFmtKfx,
	}

	t.Run("with title template", func(t *testing.T) {
		cfg := &config.DocumentConfig{
			Metainformation: config.MetainformationConfig{
				TitleTemplate: "{{.Title}} (FB2)",
			},
		}

		frag := BuildBookMetadata(c, cfg, "", "", log)
		metadata := extractTitleMetadata(t, frag)

		title, ok := metadata["title"]
		if !ok {
			t.Fatal("Expected title in metadata")
		}
		if title != "Test Book (FB2)" {
			t.Errorf("Expected 'Test Book (FB2)', got %q", title)
		}
	})

	t.Run("with template and transliteration", func(t *testing.T) {
		// Create content with Cyrillic
		c := &content.Content{
			Book: &fb2.FictionBook{
				Description: fb2.Description{
					TitleInfo: fb2.TitleInfo{
						BookTitle: fb2.TextField{Value: "Книга"},
						Authors:   []fb2.Author{{FirstName: "Автор"}},
						Lang:      language.Russian,
					},
					DocumentInfo: fb2.DocumentInfo{ID: "test"},
				},
			},
			SrcName:      "test.fb2",
			OutputFormat: common.OutputFmtKfx,
		}

		cfg := &config.DocumentConfig{
			Metainformation: config.MetainformationConfig{
				TitleTemplate: "{{.Title}} - копия",
				Transliterate: true,
			},
		}

		frag := BuildBookMetadata(c, cfg, "", "", log)
		metadata := extractTitleMetadata(t, frag)

		title, ok := metadata["title"]
		if !ok {
			t.Fatal("Expected title in metadata")
		}
		// Template applied first, then transliterated
		if title == "Книга - копия" {
			t.Errorf("Expected transliterated title, got Cyrillic %q", title)
		}
	})
}

// extractTitleMetadata extracts key-value pairs from kindle_title_metadata category.
func extractTitleMetadata(t *testing.T, frag *Fragment) map[string]string {
	t.Helper()

	result := make(map[string]string)

	// Fragment value is a StructValue with $491 (categorised_metadata) list
	sv, ok := frag.Value.(StructValue)
	if !ok {
		t.Fatalf("Expected StructValue, got %T", frag.Value)
	}

	catMetaList, ok := sv[SymCatMetadata]
	if !ok {
		t.Fatal("Expected $491 (categorised_metadata) in fragment")
	}

	// Can be ListValue or []any depending on how it was built
	var categories []any
	switch v := catMetaList.(type) {
	case ListValue:
		categories = v
	case []any:
		categories = v
	default:
		t.Fatalf("Expected ListValue or []any for categories, got %T", catMetaList)
	}

	for _, cat := range categories {
		catStruct, ok := cat.(StructValue)
		if !ok {
			continue
		}

		// Check if this is kindle_title_metadata
		catName, ok := catStruct[SymCategory]
		if !ok {
			continue
		}
		if catName != "kindle_title_metadata" {
			continue
		}

		// Extract metadata entries
		metaList, ok := catStruct[SymMetadata]
		if !ok {
			continue
		}

		var entries []any
		switch v := metaList.(type) {
		case ListValue:
			entries = v
		case []any:
			entries = v
		default:
			continue
		}

		for _, entry := range entries {
			entryStruct, ok := entry.(StructValue)
			if !ok {
				continue
			}
			key, keyOK := entryStruct[SymKey]
			val, valOK := entryStruct[SymValue]
			if keyOK && valOK {
				if keyStr, ok := key.(string); ok {
					if valStr, ok := val.(string); ok {
						result[keyStr] = valStr
					}
				}
			}
		}
	}

	return result
}
