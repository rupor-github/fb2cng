package fields

import (
	"strings"
	"testing"

	"golang.org/x/text/language"

	"fbc/config"
	"fbc/fb2"
)

func setupTestBook(t *testing.T, book *fb2.FictionBook) *fb2.FictionBook {
	t.Helper()
	if book == nil {
		book = &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book"},
					Lang:      language.MustParse("en"),
				},
				DocumentInfo: fb2.DocumentInfo{
					ID: "test-id",
				},
			},
		}
	}
	return book
}

func TestExpandTemplate_SimpleText(t *testing.T) {
	book := setupTestBook(t, nil)

	result, err := Expand(config.OutputNameTemplateFieldName, "simple-text", -1, book, "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if result != "simple-text" {
		t.Errorf("Expand() = %q, want %q", result, "simple-text")
	}
}

func TestExpandTemplate_Title(t *testing.T) {
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "My Great Book"},
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := Expand(config.OutputNameTemplateFieldName, "{{ .Title }}", -1, book, "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if result != "My Great Book" {
		t.Errorf("Expand() = %q, want %q", result, "My Great Book")
	}
}

func TestExpandTemplate_Authors(t *testing.T) {
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Book"},
				Authors: []fb2.Author{
					{FirstName: "John", LastName: "Doe"},
					{FirstName: "Jane", LastName: "Smith"},
				},
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := Expand(config.OutputNameTemplateFieldName, "{{ (index .Authors 0).LastName }}", -1, book, "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if result != "Doe" {
		t.Errorf("Expand() = %q, want %q", result, "Doe")
	}
}

func TestExpandTemplate_Series(t *testing.T) {
	num := 5
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Book"},
				Sequences: []fb2.Sequence{
					{Name: "Fantasy Series", Number: &num},
				},
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := Expand(config.OutputNameTemplateFieldName, "{{ (index .Series 0).Name }}", -1, book, "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if result != "Fantasy Series" {
		t.Errorf("Expand() = %q, want %q", result, "Fantasy Series")
	}
}

func TestExpandTemplate_SeriesNumber(t *testing.T) {
	num := 5
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Book"},
				Sequences: []fb2.Sequence{
					{Name: "Fantasy Series", Number: &num},
				},
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := Expand(config.OutputNameTemplateFieldName, "{{ (index .Series 0).Number }}", -1, book, "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if result != "5" {
		t.Errorf("Expand() = %q, want %q", result, "5")
	}
}

func TestExpandTemplate_Language(t *testing.T) {
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Book"},
				Lang:      language.MustParse("ru"),
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := Expand(config.OutputNameTemplateFieldName, "{{ .Language }}", -1, book, "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if result != "ru" {
		t.Errorf("Expand() = %q, want %q", result, "ru")
	}
}

func TestExpandTemplate_Format(t *testing.T) {
	book := setupTestBook(t, nil)

	result, err := Expand(config.OutputNameTemplateFieldName, "{{ .Format }}", -1, book, "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if result != "epub3" {
		t.Errorf("Expand() = %q, want %q", result, "epub3")
	}
}

func TestExpandTemplate_SourceFile(t *testing.T) {
	book := setupTestBook(t, nil)

	result, err := Expand(config.OutputNameTemplateFieldName, "{{ .SourceFile }}", -1, book, "path/to/mybook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if result != "mybook" {
		t.Errorf("Expand() = %q, want %q", result, "mybook")
	}
}

func TestExpandTemplate_BookID(t *testing.T) {
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Book"},
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "unique-book-id-123",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := Expand(config.OutputNameTemplateFieldName, "{{ .BookID }}", -1, book, "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if result != "unique-book-id-123" {
		t.Errorf("Expand() = %q, want %q", result, "unique-book-id-123")
	}
}

func TestExpandTemplate_Genres(t *testing.T) {
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Book"},
				Genres: []fb2.GenreRef{
					{Value: "sci_fi"},
					{Value: "adventure"},
				},
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := Expand(config.OutputNameTemplateFieldName, "{{ index .Genres 0 }}", -1, book, "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if result != "sci_fi" {
		t.Errorf("Expand() = %q, want %q", result, "sci_fi")
	}
}

func TestExpandTemplate_ComplexTemplate(t *testing.T) {
	num := 3
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "The Great Book"},
				Authors: []fb2.Author{
					{FirstName: "John", LastName: "Doe"},
				},
				Sequences: []fb2.Sequence{
					{Name: "Epic Series", Number: &num},
				},
				Lang: language.MustParse("en"),
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	template := "{{ (index .Authors 0).LastName }}/{{ (index .Series 0).Name }}/{{ printf \"%02d\" (index .Series 0).Number }} - {{ .Title }}"
	result, err := Expand(config.OutputNameTemplateFieldName, template, -1, book, "source.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}

	expected := "Doe/Epic Series/03 - The Great Book"
	if result != expected {
		t.Errorf("Expand() = %q, want %q", result, expected)
	}
}

func TestExpandTemplate_SprigFunctions(t *testing.T) {
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "test book"},
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := Expand(config.OutputNameTemplateFieldName, "{{ .Title | title }}", -1, book, "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if result != "Test Book" {
		t.Errorf("Expand() = %q, want %q", result, "Test Book")
	}
}

func TestExpandTemplate_InvalidTemplate(t *testing.T) {
	book := setupTestBook(t, nil)

	_, err := Expand(config.OutputNameTemplateFieldName, "{{ .Title", -1, book, "testbook.fb2", config.OutputFmtEpub3)
	if err == nil {
		t.Error("Expand() expected error for invalid template, got nil")
	}
}

func TestExpandTemplate_InvalidField(t *testing.T) {
	book := setupTestBook(t, nil)

	_, err := Expand(config.OutputNameTemplateFieldName, "{{ .NonExistentField }}", -1, book, "testbook.fb2", config.OutputFmtEpub3)
	if err == nil {
		t.Error("Expand() expected error for invalid field, got nil")
	}
}

func TestBuildSequences(t *testing.T) {
	num1 := 5
	num2 := 10
	sequences := []fb2.Sequence{
		{Name: "Series One", Number: &num1},
		{Name: "Series Two", Number: &num2},
		{Name: ""}, // Should be skipped
		{Name: "Series Three", Number: nil},
	}

	result := buildSequences(sequences)

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
		date     *fb2.Date
		expected string
	}{
		{"nil date", nil, ""},
		{"display only", &fb2.Date{Display: "circa 2020"}, "circa 2020"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildDate(tt.date)
			if result != tt.expected {
				t.Errorf("buildDate() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildGenres(t *testing.T) {
	genres := []fb2.GenreRef{
		{Value: "sci_fi"},
		{Value: "adventure"},
		{Value: "fantasy"},
	}

	result := buildGenres(genres)

	if len(result) != 3 {
		t.Errorf("buildGenres() length = %d, want 3", len(result))
	}

	if result[0] != "sci_fi" || result[1] != "adventure" || result[2] != "fantasy" {
		t.Errorf("buildGenres() = %v, want [sci_fi adventure fantasy]", result)
	}
}

func TestBuildAuthors(t *testing.T) {
	authors := []fb2.Author{
		{FirstName: "John", MiddleName: "Q", LastName: "Doe"},
		{FirstName: "Jane", LastName: "Smith"},
		{FirstName: "Bob"},
	}

	result := buildAuthors(authors)

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
	book := &fb2.FictionBook{
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Book"},
				Authors: []fb2.Author{
					{LastName: "Author"},
				},
			},
			DocumentInfo: fb2.DocumentInfo{
				ID: "test-id",
			},
		},
	}
	book = setupTestBook(t, book)

	result, err := Expand(config.OutputNameTemplateFieldName, "{{ (index .Authors 0).LastName }}/{{ .Title }}", -1, book, "testbook.fb2", config.OutputFmtEpub3)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}

	// Should contain forward slash for path separation
	if !strings.Contains(result, "/") {
		t.Errorf("Expand() = %q, want to contain /", result)
	}
}
