package builders

import (
	"fmt"
	"strings"

	"github.com/amazon-ion/ion-go/ion"

	"fbc/content"
)

type readingOrder struct {
	Name     string   `ion:"$178,symbol"`
	Sections []string `ion:"$170,symbol"`
}

type decimalWithUnit struct {
	Value *ion.Decimal `ion:"$307"`
	Unit  string       `ion:"$306,symbol"`
}

type documentData struct {
	V16   decimalWithUnit `ion:"$16"`
	V42   decimalWithUnit `ion:"$42"`
	V112  string          `ion:"$112,symbol"`
	V192  string          `ion:"$192,symbol"`
	V436  string          `ion:"$436,symbol"`
	V477  string          `ion:"$477,symbol"`
	V560  string          `ion:"$560,symbol"`
	MaxID int64           `ion:"max_id"`
	RO    []any           `ion:"$169"`
}

type section struct {
	ID   string `ion:"$174,symbol"`
	Refs []any  `ion:"$141"`
}

type externalResource struct {
	Name     string `ion:"$175,symbol"`
	Location string `ion:"$165"`
	Format   string `ion:"$161,symbol"`
	Width    int64  `ion:"$422"`
	Height   int64  `ion:"$423"`
}

type entityMapContainerInfo struct {
	ContainerID string   `ion:"$155"`
	FragmentIDs []string `ion:"$181,symbol"`
}

type entityMapSectionResources struct {
	SectionID string   `ion:"$155,symbol"`
	Resources []string `ion:"$254,symbol"`
}

type entityMap struct {
	Containers []entityMapContainerInfo    `ion:"$252"`
	Resources  []entityMapSectionResources `ion:"$253"`
}

type navContainer struct {
	Type string `ion:"$235,symbol"`
	// keep empty list
	Items []any `ion:"$247"`
}

type navRoot struct {
	ROName string `ion:"$178,symbol"`
	Navs   []any  `ion:"$392"`
}

func BuildFormatCapabilities() any {
	// Match what your known-good KFX samples use.
	return []any{
		map[string]any{"$492": "kfxgen.pidMapWithOffset", "version": int64(1)},
		map[string]any{"$492": "kfxgen.textBlock", "version": int64(1)},
	}
}

type metadataKV struct {
	Key   string `ion:"$492"`
	Value any    `ion:"$307"`
}

type metadataGroup struct {
	Category string       `ion:"$495"`
	Items    []metadataKV `ion:"$258"`
}

type bookMetadata struct {
	Groups []metadataGroup `ion:"$491"`
}

func BuildBookMetadata(c *content.Content, containerID, coverResourceID string) any {
	title := c.Book.Description.TitleInfo.BookTitle.Value
	authors := make([]string, 0, len(c.Book.Description.TitleInfo.Authors))
	for _, a := range c.Book.Description.TitleInfo.Authors {
		name := strings.TrimSpace(fmt.Sprintf("%s, %s %s", a.LastName, a.FirstName, a.MiddleName))
		name = strings.TrimSpace(strings.Trim(name, ","))
		if name != "" {
			authors = append(authors, name)
		}
	}
	if len(authors) == 0 {
		authors = append(authors, "Unknown")
	}

	desc := ""
	if c.Book.Description.TitleInfo.Annotation != nil {
		desc = c.Book.Description.TitleInfo.Annotation.AsPlainText()
	}

	asin := strings.ToUpper(strings.ReplaceAll(c.Book.Description.DocumentInfo.ID, "-", ""))
	if len(asin) > 32 {
		asin = asin[:32]
	}
	if len(asin) < 32 {
		asin = asin + strings.Repeat("0", 32-len(asin))
	}

	lang := c.Book.Description.TitleInfo.Lang.String()
	if lang == "" {
		lang = "en"
	}

	items := []metadataKV{
		{Key: "ASIN", Value: asin},
		{Key: "asset_id", Value: containerID},
		{Key: "author", Value: authors[0]},
		{Key: "book_id", Value: c.Book.Description.DocumentInfo.ID},
		{Key: "cde_content_type", Value: "PDOC"},
		{Key: "content_id", Value: asin},
		{Key: "description", Value: desc},
		{Key: "is_sample", Value: false},
		{Key: "language", Value: lang},
		{Key: "override_kindle_font", Value: false},
		{Key: "title", Value: title},
	}
	if coverResourceID != "" {
		items = append(items, metadataKV{Key: "cover_image", Value: coverResourceID})
	}
	// Additional ebook metadata that doesn't produce warnings.
	ebook := []metadataKV{
		{Key: "nested_span", Value: "enabled"},
		{Key: "selection", Value: "enabled"},
	}

	return bookMetadata{Groups: []metadataGroup{
		{Category: "kindle_title_metadata", Items: items},
		{Category: "kindle_ebook_metadata", Items: ebook},
	}}
}

func BuildReadingOrders(sectionIDs []string) []any {
	return []any{readingOrder{Name: "$351", Sections: sectionIDs}}
}

func BuildDocumentData(readingOrders []any) any {
	// Copy stable keys from your passing samples to avoid deep decoder assumptions.
	return documentData{
		V16:   decimalWithUnit{Value: ion.MustParseDecimal("1"), Unit: "$308"},
		V42:   decimalWithUnit{Value: ion.MustParseDecimal("1.2"), Unit: "$308"},
		V112:  "$383",
		V192:  "$376",
		V436:  "$441",
		V477:  "$56",
		V560:  "$557",
		MaxID: 851,
		RO:    readingOrders,
	}
}

func BuildMetadataReadingOrders(readingOrders []any) any {
	return map[string]any{"$169": readingOrders}
}

func BuildResourcePath() any {
	return map[string]any{"$247": []any{}}
}

func BuildNavigation() any {
	// Minimal nav: one reading order, one empty TOC container.
	toc := annotatedValue{
		Value:       navContainer{Type: "$212", Items: []any{}},
		Annotations: annot("$391"),
	}
	return []any{navRoot{ROName: "$351", Navs: []any{toc}}}
}

func BuildEmptyListRoot() []any { return []any{} }

func BuildExternalResource(resourceID, location string, width, height int, format string) any {
	return externalResource{Name: resourceID, Location: location, Format: format, Width: int64(width), Height: int64(height)}
}

func BuildSection(sectionID string) any {
	return section{ID: sectionID, Refs: []any{}}
}

func BuildEntityMap(containerID string, fragmentIDs []string, sectionID string, sectionResources []string) any {
	return entityMap{
		Containers: []entityMapContainerInfo{{ContainerID: containerID, FragmentIDs: fragmentIDs}},
		Resources:  []entityMapSectionResources{{SectionID: sectionID, Resources: sectionResources}},
	}
}
