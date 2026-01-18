package kfx

import (
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"sync"

	"github.com/amazon-ion/ion-go/ion"
)

//go:embed data/*.ion
var ionDataFS embed.FS

// Package-level variables populated during init from embedded ion files.
var (
	defaultStyleListEntries  []StyleListEntry
	defaultStyleMapEntries   []StyleMapEntry
	defaultIgnorablePatterns []IgnorablePattern

	initOnce sync.Once
	initErr  error
)

// initIonData loads and decodes all embedded ion data files.
// Called automatically on first access to any of the data slices.
func initIonData() {
	initOnce.Do(func() {
		initErr = loadAllIonData()
	})
}

// mustInitIonData ensures ion data is loaded, panics on error.
func mustInitIonData() {
	initIonData()
	if initErr != nil {
		panic(fmt.Sprintf("failed to load embedded ion data: %v", initErr))
	}
}

func loadAllIonData() error {
	var err error

	defaultStyleListEntries, err = loadStyleList()
	if err != nil {
		return fmt.Errorf("load stylelist: %w", err)
	}

	defaultStyleMapEntries, err = loadStyleMap()
	if err != nil {
		return fmt.Errorf("load stylemap: %w", err)
	}

	defaultIgnorablePatterns, err = loadIgnorablePatterns()
	if err != nil {
		return fmt.Errorf("load ignorable patterns: %w", err)
	}

	return nil
}

// readGzippedIonFile reads and decompresses a gzipped ion file from the embedded FS.
func readGzippedIonFile(name string) ([]byte, error) {
	f, err := ionDataFS.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	return io.ReadAll(gr)
}

// Ion struct types for unmarshaling.
// These map to the local symbol tables in each ion file.

// ionStyleListEntry matches the stylelist.ion structure.
type ionStyleListEntry struct {
	Key   string `ion:"YJPropertyKey"`
	Class string `ion:"YJPropertyKeyClass"`
}

// ionStyleMapEntry matches the stylemap.ion structure.
type ionStyleMapEntry struct {
	HTMLTag            string        `ion:"html_tag"`
	HTMLAttribute      string        `ion:"html_attribute"`
	HTMLAttributeUnit  string        `ion:"html_attribute_value_unit"`
	HTMLAttributeValue string        `ion:"html_attribute_value"`
	YJProperty         string        `ion:"yj_property"`
	YJValueType        string        `ion:"yj_value_type"`
	YJUnit             string        `ion:"yj_unit"`
	YJValue            string        `ion:"yj_value"`
	CSSStyles          []ionCSSStyle `ion:"css_styles"`
	Display            string        `ion:"display"`
	ConverterClassname string        `ion:"converter_classname"`
	IgnoreForMapping   string        `ion:"ignore_for_yj_to_html_mapping"` // "true" or "false" string
}

// ionCSSStyle represents a single CSS style in the css_styles list.
type ionCSSStyle struct {
	Name  string `ion:"style_name"`
	Value string `ion:"style_value"`
}

// ionIgnorablePattern matches the mapping_ignorable_patterns.ion structure.
type ionIgnorablePattern struct {
	Tag   string `ion:"Tag"`
	Style string `ion:"Style"`
	Value string `ion:"Value"`
	Unit  string `ion:"Unit"`
}

func loadStyleList() ([]StyleListEntry, error) {
	data, err := readGzippedIonFile("data/stylelist.ion")
	if err != nil {
		return nil, err
	}

	// Ion file contains multiple top-level structs, not a list
	dec := ion.NewDecoder(ion.NewReader(bytes.NewReader(data)))
	var entries []StyleListEntry
	for {
		var e ionStyleListEntry
		if err := dec.DecodeTo(&e); err != nil {
			if err == ion.ErrNoInput {
				break
			}
			return nil, fmt.Errorf("decode stylelist entry: %w", err)
		}
		entries = append(entries, StyleListEntry(e))
	}
	return entries, nil
}

func loadStyleMap() ([]StyleMapEntry, error) {
	data, err := readGzippedIonFile("data/stylemap.ion")
	if err != nil {
		return nil, err
	}

	// Ion file contains multiple top-level structs, not a list
	dec := ion.NewDecoder(ion.NewReader(bytes.NewReader(data)))
	var entries []StyleMapEntry
	for {
		var e ionStyleMapEntry
		if err := dec.DecodeTo(&e); err != nil {
			if err == ion.ErrNoInput {
				break
			}
			return nil, fmt.Errorf("decode stylemap entry: %w", err)
		}
		// Convert css_styles list to map
		var cssStyles map[string]string
		if len(e.CSSStyles) > 0 {
			cssStyles = make(map[string]string, len(e.CSSStyles))
			for _, cs := range e.CSSStyles {
				cssStyles[cs.Name] = cs.Value
			}
		}
		entries = append(entries, StyleMapEntry{
			Key: HTMLKey{
				Tag:   e.HTMLTag,
				Attr:  e.HTMLAttribute,
				Value: e.HTMLAttributeValue,
				Unit:  e.HTMLAttributeUnit,
			},
			Property:    e.YJProperty,
			Value:       e.YJValue,
			Unit:        e.YJUnit,
			ValueType:   e.YJValueType,
			Display:     e.Display,
			Transformer: e.ConverterClassname,
			CSSStyles:   cssStyles,
			IgnoreHTML:  e.IgnoreForMapping == "true",
		})
	}
	return entries, nil
}

func loadIgnorablePatterns() ([]IgnorablePattern, error) {
	data, err := readGzippedIonFile("data/mapping_ignorable_patterns.ion")
	if err != nil {
		return nil, err
	}

	// Ion file contains multiple top-level structs, not a list
	dec := ion.NewDecoder(ion.NewReader(bytes.NewReader(data)))
	var entries []IgnorablePattern
	for {
		var e ionIgnorablePattern
		if err := dec.DecodeTo(&e); err != nil {
			if err == ion.ErrNoInput {
				break
			}
			return nil, fmt.Errorf("decode ignorable pattern: %w", err)
		}
		entries = append(entries, IgnorablePattern(e))
	}
	return entries, nil
}

// init ensures ion data is loaded at package initialization.
func init() {
	mustInitIonData()
}
