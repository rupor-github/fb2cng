package convert

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	sprig "github.com/go-task/slim-sprig/v3"

	"fbc/config"
	"fbc/fb2"
)

type SequenceDefinition struct {
	Name   string
	Number int
}

type AuthorDefinition struct {
	FirstName, MiddleName, LastName string
}

// Values is a struct that holds variables we make available for template expansion
type Values struct {
	Context    string
	Title      string
	Series     []SequenceDefinition
	Language   string
	Date       string
	Authors    []AuthorDefinition
	Format     string
	SourceFile string
	BookID     string
	Genres     []string
}

// NOTE: according to XSD sequences could be nested. I have never seen book
// which uses this, so we are going to ignore nesting and only use first level.
// Nested children would be skipped.
func buildSequences(sequences []fb2.Sequence) []SequenceDefinition {
	result := make([]SequenceDefinition, 0, len(sequences))

	for _, seq := range sequences {
		if seq.Name == "" {
			continue
		}
		def := SequenceDefinition{
			Name: seq.Name,
		}
		if seq.Number != nil {
			def.Number = *seq.Number
		}
		result = append(result, def)
	}
	return result
}

func buildDate(date *fb2.Date) string {
	if date == nil {
		return ""
	}
	if date.Value.IsZero() {
		return date.Display
	}
	return date.Value.Format("2006-01-02")
}

func buildGenres(genres []fb2.GenreRef) []string {
	result := make([]string, 0, len(genres))
	for _, g := range genres {
		result = append(result, g.Value)
	}
	return result
}

func buildAuthors(authors []fb2.Author) []AuthorDefinition {
	result := make([]AuthorDefinition, 0, len(authors))
	for _, a := range authors {
		def := AuthorDefinition{
			FirstName:  a.FirstName,
			MiddleName: a.MiddleName,
			LastName:   a.LastName,
		}
		result = append(result, def)
	}
	return result
}

func (c *Content) expandTemplate(name config.TemplateFieldName, field string, format config.OutputFmt) (string, error) {
	funcMap := sprig.FuncMap()

	tmpl, err := template.New(string(name)).Funcs(funcMap).Parse(field)
	if err != nil {
		return "", fmt.Errorf("unable to parse template field %s: %w", name, err)
	}

	values := Values{
		Context:    string(name),
		Title:      c.book.Description.TitleInfo.BookTitle.Value,
		Series:     buildSequences(c.book.Description.TitleInfo.Sequences),
		Language:   c.book.Description.TitleInfo.Lang.String(),
		Date:       buildDate(c.book.Description.TitleInfo.Date),
		Authors:    buildAuthors(c.book.Description.TitleInfo.Authors),
		Format:     format.String(),
		SourceFile: strings.TrimSuffix(filepath.Base(c.srcName), filepath.Ext(c.srcName)),
		BookID:     c.book.Description.DocumentInfo.ID,
		Genres:     buildGenres(c.book.Description.TitleInfo.Genres),
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, values); err != nil {
		return "", err
	}
	return buf.String(), nil
}
