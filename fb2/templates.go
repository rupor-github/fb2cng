package fb2

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	sprig "github.com/go-task/slim-sprig/v3"

	"fbc/config"
)

type sequenceDefinition struct {
	Name   string
	Number int
}

type authorDefinition struct {
	FirstName, MiddleName, LastName string
}

// templateValues is a struct that holds variables we make available for template expansion
type templateValues struct {
	Context    string
	Index      int
	Title      string
	Series     []sequenceDefinition
	Language   string
	Date       string
	Authors    []authorDefinition
	Format     string
	SourceFile string
	BookID     string
	Genres     []string
}

// footnoteValues is a struct that holds variables for footnote label templates
type footnoteValues struct {
	Context    string
	BodyNumber int
	NoteNumber int
}

// ExpandTemplateSimple expands a template string with book metadata without an index
func (fb *FictionBook) ExpandTemplateSimple(name config.TemplateFieldName, field string, srcName string, format config.OutputFmt) (string, error) {
	values := &templateValues{
		Context:    string(name),
		Index:      -1,
		Title:      fb.Description.TitleInfo.BookTitle.Value,
		Series:     fb.buildSequences(),
		Language:   fb.Description.TitleInfo.Lang.String(),
		Date:       fb.buildDate(),
		Authors:    fb.buildAuthors(),
		Format:     format.String(),
		SourceFile: strings.TrimSuffix(filepath.Base(srcName), filepath.Ext(srcName)),
		BookID:     fb.Description.DocumentInfo.ID,
		Genres:     fb.buildGenres(),
	}
	return expandTemplate(name, field, values)
}

// ExpandTemplateIndexed expands a template string with book metadata and an index
func (fb *FictionBook) ExpandTemplateIndexed(name config.TemplateFieldName, field string, index int, srcName string, format config.OutputFmt) (string, error) {
	values := &templateValues{
		Context:    string(name),
		Index:      index,
		Title:      fb.Description.TitleInfo.BookTitle.Value,
		Series:     fb.buildSequences(),
		Language:   fb.Description.TitleInfo.Lang.String(),
		Date:       fb.buildDate(),
		Authors:    fb.buildAuthors(),
		Format:     format.String(),
		SourceFile: strings.TrimSuffix(filepath.Base(srcName), filepath.Ext(srcName)),
		BookID:     fb.Description.DocumentInfo.ID,
		Genres:     fb.buildGenres(),
	}
	return expandTemplate(name, field, values)
}

// ExpandTemplateFootnoteLabel expands a template string for footnote labels with body and note numbers
func (fb *FictionBook) ExpandTemplateFootnoteLabel(name config.TemplateFieldName, field string, bodyNum, noteNum int) (string, error) {
	funcMap := sprig.FuncMap()

	tmpl, err := template.New(string(name)).Funcs(funcMap).Parse(field)
	if err != nil {
		return "", fmt.Errorf("unable to parse template field %s: %w", name, err)
	}

	values := &footnoteValues{
		Context:    string(name),
		BodyNumber: bodyNum,
		NoteNumber: noteNum,
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, values); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// expandTemplate is the private implementation that expands a template string with initialized values
func expandTemplate(name config.TemplateFieldName, field string, values *templateValues) (string, error) {
	funcMap := sprig.FuncMap()

	tmpl, err := template.New(string(name)).Funcs(funcMap).Parse(field)
	if err != nil {
		return "", fmt.Errorf("unable to parse template field %s: %w", name, err)
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, values); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// NOTE: according to XSD sequences could be nested. I have never seen book
// which uses this, so we are going to ignore nesting and only use first level.
// Nested children would be skipped.
func (fb *FictionBook) buildSequences() []sequenceDefinition {
	sequences := fb.Description.TitleInfo.Sequences
	result := make([]sequenceDefinition, 0, len(sequences))

	for _, seq := range sequences {
		if seq.Name == "" {
			continue
		}
		def := sequenceDefinition{
			Name: seq.Name,
		}
		if seq.Number != nil {
			def.Number = *seq.Number
		}
		result = append(result, def)
	}
	return result
}

func (fb *FictionBook) buildDate() string {
	date := fb.Description.TitleInfo.Date
	if date == nil {
		return ""
	}
	if date.Value.IsZero() {
		return date.Display
	}
	return date.Value.Format("2006-01-02")
}

func (fb *FictionBook) buildGenres() []string {
	genres := fb.Description.TitleInfo.Genres
	result := make([]string, 0, len(genres))
	for _, g := range genres {
		result = append(result, g.Value)
	}
	return result
}

func (fb *FictionBook) buildAuthors() []authorDefinition {
	authors := fb.Description.TitleInfo.Authors
	result := make([]authorDefinition, 0, len(authors))
	for _, a := range authors {
		def := authorDefinition{
			FirstName:  a.FirstName,
			MiddleName: a.MiddleName,
			LastName:   a.LastName,
		}
		result = append(result, def)
	}
	return result
}
