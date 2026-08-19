package metainfo

import (
	"fmt"
	"strings"
	"time"

	"fbc/common"
	"fbc/config"
	"fbc/fb2"
)

// SubjectKind identifies the source/semantics of a metadata subject.
type SubjectKind string

const (
	SubjectGenre   SubjectKind = "genre"
	SubjectKeyword SubjectKind = "keyword"
)

// Subject represents one dc:subject value plus its source kind.
type Subject struct {
	Value string
	Kind  SubjectKind
}

// PersonName formats an FB2 author/person for output metadata.
func PersonName(
	book *fb2.FictionBook,
	cfg *config.MetainformationConfig,
	format common.OutputFmt,
	index int,
	person *fb2.Author,
) (string, error) {
	if person == nil {
		return "", nil
	}

	name := ""
	var retErr error
	if cfg != nil && strings.TrimSpace(cfg.CreatorNameTemplate) != "" && book != nil {
		expanded, err := book.ExpandTemplateAuthorName(
			config.MetaCreatorNameTemplateFieldName,
			cfg.CreatorNameTemplate,
			format,
			index,
			person,
		)
		if err != nil {
			retErr = err
		} else {
			name = strings.TrimSpace(expanded)
		}
	}
	if name == "" {
		name = DefaultPersonName(*person)
	}
	if cfg != nil && cfg.Transliterate {
		name = fb2.Transliterate(name)
	}
	return strings.TrimSpace(name), retErr
}

// DefaultPersonName formats name parts with nickname fallback.
func DefaultPersonName(person fb2.Author) string {
	parts := make([]string, 0, 3)
	if person.FirstName != "" {
		parts = append(parts, person.FirstName)
	}
	if person.MiddleName != "" {
		parts = append(parts, person.MiddleName)
	}
	if person.LastName != "" {
		parts = append(parts, person.LastName)
	}
	if len(parts) == 0 {
		return strings.TrimSpace(person.Nickname)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// BookDate returns the work/publication date for metadata.
func BookDate(desc *fb2.Description) string {
	if desc == nil {
		return ""
	}
	if date := Date(desc.TitleInfo.Date); date != "" {
		return date
	}
	if desc.PublishInfo != nil {
		return strings.TrimSpace(desc.PublishInfo.Year)
	}
	return ""
}

// OPFDate returns a W3CDTF-compatible date string for OPF dc:date metadata.
func OPFDate(desc *fb2.Description) (string, error) {
	if desc == nil {
		return "", nil
	}
	if date := desc.TitleInfo.Date; date != nil {
		if !date.Value.IsZero() {
			return date.Value.Format(time.DateOnly), nil
		}
		if normalized, ok := normalizeMetadataDate(date.Display); ok {
			return normalized, nil
		}
		return "", fmt.Errorf("invalid title date metadata: %q", strings.TrimSpace(date.Display))
	}
	if desc.PublishInfo != nil {
		if normalized, ok := normalizeMetadataDate(desc.PublishInfo.Year); ok {
			return normalized, nil
		}
		if strings.TrimSpace(desc.PublishInfo.Year) != "" {
			return "", fmt.Errorf("invalid publish year metadata: %q", strings.TrimSpace(desc.PublishInfo.Year))
		}
	}
	return "", nil
}

// Publisher returns trimmed publisher text from publish-info.
func Publisher(pub *fb2.PublishInfo) string {
	if pub == nil || pub.Publisher == nil {
		return ""
	}
	return strings.TrimSpace(pub.Publisher.Value)
}

// Date formats FB2 date value, preferring machine-readable date.
func Date(date *fb2.Date) string {
	if date == nil {
		return ""
	}
	if !date.Value.IsZero() {
		return date.Value.Format(time.DateOnly)
	}
	return strings.TrimSpace(date.Display)
}

// KindleIssueDate returns a conservative date string for KFX issue_date metadata.
func KindleIssueDate(desc *fb2.Description) (string, error) {
	if desc == nil {
		return "", nil
	}
	if date := desc.TitleInfo.Date; date != nil {
		if !date.Value.IsZero() {
			return date.Value.Format(time.DateOnly), nil
		}
		if normalized, ok := normalizeMetadataDate(date.Display); ok {
			return normalized, nil
		}
		return "", fmt.Errorf("invalid title date metadata: %q", strings.TrimSpace(date.Display))
	}
	if desc.PublishInfo != nil {
		if normalized, ok := normalizeMetadataDate(desc.PublishInfo.Year); ok {
			return normalized, nil
		}
		if strings.TrimSpace(desc.PublishInfo.Year) != "" {
			return "", fmt.Errorf("invalid publish year metadata: %q", strings.TrimSpace(desc.PublishInfo.Year))
		}
	}
	return "", nil
}

func normalizeMetadataDate(value string) (string, bool) {
	value = strings.TrimSpace(value)
	switch len(value) {
	case 4:
		return value, allDigits(value)
	case 7:
		_, err := time.Parse("2006-01", value)
		return value, err == nil
	case 10:
		if _, err := time.Parse(time.DateOnly, value); err == nil {
			return value, true
		}
		if date, err := time.Parse("02.01.2006", value); err == nil {
			return date.Format(time.DateOnly), true
		}
		return "", false
	default:
		return "", false
	}
}

func allDigits(value string) bool {
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return value != ""
}

// Subjects combines FB2 genres and keyword text into unique subject values.
func Subjects(genres []fb2.GenreRef, keywords *fb2.TextField) []Subject {
	seen := make(map[string]struct{})
	result := make([]Subject, 0, len(genres))
	add := func(value string, kind SubjectKind) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		result = append(result, Subject{Value: value, Kind: kind})
	}

	for _, genre := range genres {
		add(genre.Value, SubjectGenre)
	}
	if keywords != nil {
		for _, keyword := range strings.FieldsFunc(keywords.Value, isKeywordSeparator) {
			add(keyword, SubjectKeyword)
		}
	}
	return result
}

// Keywords splits FB2 keyword text into unique values.
func Keywords(keywords *fb2.TextField) []string {
	if keywords == nil {
		return nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, keyword := range strings.FieldsFunc(keywords.Value, isKeywordSeparator) {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		key := strings.ToLower(keyword)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, keyword)
	}
	return result
}

func isKeywordSeparator(r rune) bool {
	return r == ',' || r == ';'
}

// ISBN returns normalized, checksum-valid ISBN metadata from publish-info.
func ISBN(pub *fb2.PublishInfo) (string, common.ISBNKind, error) {
	if pub == nil || pub.ISBN == nil {
		return "", "", nil
	}
	return common.NormalizeISBN(pub.ISBN.Value)
}
