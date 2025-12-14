package fb2

import (
	"strings"
	"testing"

	"golang.org/x/text/language"

	"fbc/config"
)

func setupTestBook(t *testing.T, book *FictionBook) *FictionBook {
	t.Helper()
	if book == nil {
		book = &FictionBook{
			Description: Description{
				TitleInfo: TitleInfo{
					BookTitle: TextField{Value: "Test Book"},
					Lang:      language.MustParse("en"),
				},
				DocumentInfo: DocumentInfo{
					ID: "test-id",
				},
			},
		}
	}
	return book
}

func TestExpandTemplate_SimpleText(t *testing.T) {
	book := setupTestBook(t, nil)

	result, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "simple-text", "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("ExpandTemplateSimple() error = %v", err)
	}
	if result != "simple-text" {
		t.Errorf("ExpandTemplateSimple() = %q, want %q", result, "simple-text")
	}
}

func TestExpandTemplate_Title(t *testing.T) {
	book := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				BookTitle: TextField{Value: "My Great Book"},
			},
			DocumentInfo: DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "{{ .Title }}", "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}
	if result != "My Great Book" {
		t.Errorf("ExpandTemplate() = %q, want %q", result, "My Great Book")
	}
}

func TestExpandTemplate_Authors(t *testing.T) {
	book := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				BookTitle: TextField{Value: "Book"},
				Authors: []Author{
					{FirstName: "John", LastName: "Doe"},
					{FirstName: "Jane", LastName: "Smith"},
				},
			},
			DocumentInfo: DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "{{ (index .Authors 0).LastName }}", "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}
	if result != "Doe" {
		t.Errorf("ExpandTemplate() = %q, want %q", result, "Doe")
	}
}

func TestExpandTemplate_Series(t *testing.T) {
	num := 5
	book := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				BookTitle: TextField{Value: "Book"},
				Sequences: []Sequence{
					{Name: "Fantasy Series", Number: &num},
				},
			},
			DocumentInfo: DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "{{ (index .Series 0).Name }}", "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}
	if result != "Fantasy Series" {
		t.Errorf("ExpandTemplate() = %q, want %q", result, "Fantasy Series")
	}
}

func TestExpandTemplate_SeriesNumber(t *testing.T) {
	num := 5
	book := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				BookTitle: TextField{Value: "Book"},
				Sequences: []Sequence{
					{Name: "Fantasy Series", Number: &num},
				},
			},
			DocumentInfo: DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "{{ (index .Series 0).Number }}", "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}
	if result != "5" {
		t.Errorf("ExpandTemplate() = %q, want %q", result, "5")
	}
}

func TestExpandTemplate_Language(t *testing.T) {
	book := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				BookTitle: TextField{Value: "Book"},
				Lang:      language.MustParse("ru"),
			},
			DocumentInfo: DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "{{ .Language }}", "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}
	if result != "ru" {
		t.Errorf("ExpandTemplate() = %q, want %q", result, "ru")
	}
}

func TestExpandTemplate_Format(t *testing.T) {
	book := setupTestBook(t, nil)

	result, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "{{ .Format }}", "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}
	if result != "epub3" {
		t.Errorf("ExpandTemplate() = %q, want %q", result, "epub3")
	}
}

func TestExpandTemplate_SourceFile(t *testing.T) {
	book := setupTestBook(t, nil)

	result, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "{{ .SourceFile }}", "path/to/mybook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}
	if result != "mybook" {
		t.Errorf("ExpandTemplate() = %q, want %q", result, "mybook")
	}
}

func TestExpandTemplate_BookID(t *testing.T) {
	book := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				BookTitle: TextField{Value: "Book"},
			},
			DocumentInfo: DocumentInfo{
				ID: "unique-book-id-123",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "{{ .BookID }}", "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}
	if result != "unique-book-id-123" {
		t.Errorf("ExpandTemplate() = %q, want %q", result, "unique-book-id-123")
	}
}

func TestExpandTemplate_Genres(t *testing.T) {
	book := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				BookTitle: TextField{Value: "Book"},
				Genres: []GenreRef{
					{Value: "sci_fi"},
					{Value: "adventure"},
				},
			},
			DocumentInfo: DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "{{ index .Genres 0 }}", "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}
	if result != "sci_fi" {
		t.Errorf("ExpandTemplate() = %q, want %q", result, "sci_fi")
	}
}

func TestExpandTemplate_ComplexTemplate(t *testing.T) {
	num := 3
	book := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				BookTitle: TextField{Value: "The Great Book"},
				Authors: []Author{
					{FirstName: "John", LastName: "Doe"},
				},
				Sequences: []Sequence{
					{Name: "Epic Series", Number: &num},
				},
				Lang: language.MustParse("en"),
			},
			DocumentInfo: DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	template := "{{ (index .Authors 0).LastName }}/{{ (index .Series 0).Name }}/{{ printf \"%02d\" (index .Series 0).Number }} - {{ .Title }}"
	result, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, template, "source.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}

	expected := "Doe/Epic Series/03 - The Great Book"
	if result != expected {
		t.Errorf("ExpandTemplate() = %q, want %q", result, expected)
	}
}

func TestExpandTemplate_SprigFunctions(t *testing.T) {
	book := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				BookTitle: TextField{Value: "test book"},
			},
			DocumentInfo: DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "{{ .Title | title }}", "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}
	if result != "Test Book" {
		t.Errorf("ExpandTemplate() = %q, want %q", result, "Test Book")
	}
}

func TestExpandTemplate_InvalidTemplate(t *testing.T) {
	book := setupTestBook(t, nil)

	_, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "{{ .Title", "testbook.fb2", config.OutputFmtEpub3)
	if err == nil {
		t.Error("ExpandTemplate() expected error for invalid template, got nil")
	}
}

func TestExpandTemplate_InvalidField(t *testing.T) {
	book := setupTestBook(t, nil)

	_, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "{{ .NonExistentField }}", "testbook.fb2", config.OutputFmtEpub3)
	if err == nil {
		t.Error("ExpandTemplate() expected error for invalid field, got nil")
	}
}

func TestBuildSequences(t *testing.T) {
	num1 := 5
	num2 := 10
	book := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				Sequences: []Sequence{
					{Name: "Series One", Number: &num1},
					{Name: "Series Two", Number: &num2},
					{Name: ""}, // Should be skipped
					{Name: "Series Three", Number: nil},
				},
			},
		},
	}

	result := book.buildSequences()

	if len(result) != 3 {
		t.Errorf("buildSequences() length = %d, want 3", len(result))
	}

	if result[0].Name != "Series One" || result[0].Number != 5 {
		t.Errorf("buildSequences()[0] = %+v, want {Name:Series One, Number:5}", result[0])
	}

	if result[2].Name != "Series Three" || result[2].Number != 0 {
		t.Errorf("buildSequences()[2] = %+v, want {Name:Series Three, Number:0}", result[2])
	}
}

func TestBuildDate(t *testing.T) {
	tests := []struct {
		name     string
		date     *Date
		expected string
	}{
		{"nil date", nil, ""},
		{"display only", &Date{Display: "circa 2020"}, "circa 2020"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			book := &FictionBook{
				Description: Description{
					TitleInfo: TitleInfo{
						Date: tt.date,
					},
				},
			}
			result := book.buildDate()
			if result != tt.expected {
				t.Errorf("buildDate() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildGenres(t *testing.T) {
	book := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				Genres: []GenreRef{
					{Value: "sci_fi"},
					{Value: "adventure"},
					{Value: "fantasy"},
				},
			},
		},
	}

	result := book.buildGenres()

	if len(result) != 3 {
		t.Errorf("buildGenres() length = %d, want 3", len(result))
	}

	if result[0] != "sci_fi" || result[1] != "adventure" || result[2] != "fantasy" {
		t.Errorf("buildGenres() = %v, want [sci_fi adventure fantasy]", result)
	}
}

func TestBuildAuthors(t *testing.T) {
	book := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				Authors: []Author{
					{FirstName: "John", MiddleName: "Q", LastName: "Doe"},
					{FirstName: "Jane", LastName: "Smith"},
					{FirstName: "Bob"},
				},
			},
		},
	}

	result := book.buildAuthors()

	if len(result) != 3 {
		t.Errorf("buildAuthors() length = %d, want 3", len(result))
	}

	if result[0].FirstName != "John" || result[0].MiddleName != "Q" || result[0].LastName != "Doe" {
		t.Errorf("buildAuthors()[0] = %+v, want {FirstName:John MiddleName:Q LastName:Doe}", result[0])
	}

	if result[1].FirstName != "Jane" || result[1].LastName != "Smith" {
		t.Errorf("buildAuthors()[1] = %+v, want {FirstName:Jane MiddleName: LastName:Smith}", result[1])
	}
}

func TestExpandTemplate_PathSeparators(t *testing.T) {
	book := &FictionBook{
		Description: Description{
			TitleInfo: TitleInfo{
				BookTitle: TextField{Value: "Book"},
				Authors: []Author{
					{LastName: "Author"},
				},
			},
			DocumentInfo: DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := book.ExpandTemplateSimple(config.OutputNameTemplateFieldName, "{{ (index .Authors 0).LastName }}/{{ .Title }}", "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("ExpandTemplate() error = %v", err)
	}

	// Should contain forward slash for path separation
	if !strings.Contains(result, "/") {
		t.Errorf("ExpandTemplate() = %q, want to contain /", result)
	}
}
