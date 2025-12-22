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
	FontSize           decimalWithUnit `ion:"$16"`
	LineHeight         decimalWithUnit `ion:"$42"`
	ColumnCount        string          `ion:"$112,symbol"`
	Direction          string          `ion:"$192,symbol"`
	Selection          string          `ion:"$436,symbol"`
	SpacingPercentBase string          `ion:"$477,symbol"`
	WritingMode        string          `ion:"$560,symbol"`
	MaxID              int64           `ion:"max_id"`
	RO                 []any           `ion:"$169"`
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
	Name string `ion:"$239,symbol"`
	Type string `ion:"$235,symbol"`
	// keep empty list
	Items []any `ion:"$247"`
}

type navRoot struct {
	ROName string   `ion:"$178,symbol"`
	Navs   []string `ion:"$392,symbol"`
}

func BuildFormatCapabilities() any {
	// Match what your known-good KFX samples use.
	return []any{
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

func BuildDocumentData(readingOrders []any, maxID int64) any {
	// Copy stable keys from your passing samples to avoid deep decoder assumptions.
	return documentData{
		FontSize:           decimalWithUnit{Value: ion.MustParseDecimal("1"), Unit: "$308"},
		LineHeight:         decimalWithUnit{Value: ion.MustParseDecimal("1.2"), Unit: "$308"},
		ColumnCount:        "$383",
		Direction:          "$376",
		Selection:          "$441",
		SpacingPercentBase: "$56",
		WritingMode:        "$557",
		MaxID:              maxID,
		RO:                 readingOrders,
	}
}

func BuildMetadataReadingOrders(readingOrders []any) any {
	return map[string]any{"$169": readingOrders}
}

func BuildResourcePath() any {
	return map[string]any{"$247": []any{}}
}

func BuildNavigation(tocID string) any {
	return []any{navRoot{ROName: "$351", Navs: []string{tocID}}}
}

func BuildNavContainerTOC(tocID string, items []any) any {
	return navContainer{Name: tocID, Type: "$212", Items: items}
}

func BuildSectionMetadata(id string) any {
	type sectionMetadata struct {
		Items   []metadataKV `ion:"$258"`
		Section string       `ion:"$598,symbol"`
	}
	return sectionMetadata{Items: []metadataKV{{Key: "IS_TARGET_SECTION", Value: true}}, Section: id}
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
