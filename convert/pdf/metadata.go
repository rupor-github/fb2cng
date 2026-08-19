package pdf

import (
	"bytes"
	"compress/zlib"
	"encoding/xml"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/text/language"

	"fbc/config"
	"fbc/content"
	contenttext "fbc/content/text"
	"fbc/convert/metainfo"
	"fbc/convert/pdf/docwriter"
	"fbc/fb2"
)

type pdfXMPDocumentMetadata struct {
	Title        string
	Creators     []string
	Contributors []string
	Description  string
	Subjects     []string
	Publisher    string
	Date         string
	Identifiers  []string
	Language     string
	Keywords     string
}

func pdfHyphenator(c *content.Content, log *zap.Logger) paragraphHyphenator {
	if c == nil || c.Book == nil || c.Book.Description.TitleInfo.Lang == language.Und {
		return nil
	}
	if log == nil {
		log = zap.NewNop()
	}
	return contenttext.NewHyphenator(c.Book.Description.TitleInfo.Lang, log)
}

func infoDictionary(doc pdfDocumentSpec) docwriter.Dict {
	info := docwriter.Dict{
		"Creator":  docwriter.UTF16TextString("fbc"),
		"Producer": docwriter.UTF16TextString("fbc"),
	}
	if doc.Title != "" {
		info["Title"] = docwriter.UTF16TextString(doc.Title)
	}
	if doc.Author != "" {
		info["Author"] = docwriter.UTF16TextString(doc.Author)
	}
	if doc.Subject != "" {
		info["Subject"] = docwriter.UTF16TextString(doc.Subject)
	}
	if doc.Keywords != "" {
		info["Keywords"] = docwriter.UTF16TextString(doc.Keywords)
	}
	return info
}

func flateStream(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return nil, fmt.Errorf("compress content stream: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finish content stream compression: %w", err)
	}
	return buf.Bytes(), nil
}

func bookTitle(c *content.Content, cfg *config.DocumentConfig, log *zap.Logger) string {
	if c == nil || c.Book == nil {
		return fallbackBookTitle(c)
	}
	if log == nil {
		log = zap.NewNop()
	}
	title := c.Book.Description.TitleInfo.BookTitle.Value
	if cfg != nil && cfg.Metainformation.TitleTemplate != "" {
		expanded, err := c.Book.ExpandTemplateMetainfo(
			config.MetaTitleTemplateFieldName,
			cfg.Metainformation.TitleTemplate,
			c.SrcName,
			c.OutputFormat,
		)
		if err != nil {
			log.Warn("Unable to expand title template for PDF metadata", zap.Error(err))
		} else {
			title = expanded
		}
	}
	if cfg != nil && cfg.Metainformation.Transliterate {
		title = fb2.Transliterate(title)
	}
	if title = strings.TrimSpace(title); title != "" {
		return title
	}
	return fallbackBookTitle(c)
}

func fallbackBookTitle(c *content.Content) string {
	if c == nil {
		return ""
	}
	return strings.TrimSuffix(c.SrcName, ".fb2")
}

func bookAuthors(c *content.Content, cfg *config.DocumentConfig, log *zap.Logger) string {
	return strings.Join(bookAuthorList(c, cfg, log), ", ")
}

func bookAuthorList(c *content.Content, cfg *config.DocumentConfig, log *zap.Logger) []string {
	if c == nil || c.Book == nil {
		return nil
	}
	return bookPersonList(c, cfg, log, c.Book.Description.TitleInfo.Authors, "author")
}

func bookTranslatorList(c *content.Content, cfg *config.DocumentConfig, log *zap.Logger) []string {
	if c == nil || c.Book == nil {
		return nil
	}
	return bookPersonList(c, cfg, log, c.Book.Description.TitleInfo.Translators, "translator")
}

func bookPersonList(c *content.Content, cfg *config.DocumentConfig, log *zap.Logger, people []fb2.Author, label string) []string {
	if log == nil {
		log = zap.NewNop()
	}
	var metaCfg *config.MetainformationConfig
	if cfg != nil {
		metaCfg = &cfg.Metainformation
	}
	result := make([]string, 0, len(people))
	for i := range people {
		name, err := metainfo.PersonName(c.Book, metaCfg, c.OutputFormat, i, &people[i])
		if err != nil {
			log.Warn("Unable to expand "+label+" name template for PDF metadata", zap.Error(err))
		}
		if name = strings.TrimSpace(name); name != "" {
			result = append(result, name)
		}
	}
	return result
}

func bookSubject(c *content.Content) string {
	if c == nil || c.Book == nil || c.Book.Description.TitleInfo.Annotation == nil {
		return ""
	}
	return metadataExcerpt(c.Book.Description.TitleInfo.Annotation.AsPlainText(), metadataExcerptMaxRunes)
}

func bookKeywords(c *content.Content) string {
	if c == nil || c.Book == nil || c.Book.Description.TitleInfo.Keywords == nil {
		return ""
	}
	return metadataExcerpt(c.Book.Description.TitleInfo.Keywords.Value, metadataExcerptMaxRunes)
}

func pdfXMPMetadata(c *content.Content, cfg *config.DocumentConfig, log *zap.Logger, title, description, keywords string) []byte {
	if c == nil || c.Book == nil {
		return xmpMetadataPacket(pdfXMPDocumentMetadata{Title: title, Description: description, Keywords: keywords})
	}
	if log == nil {
		log = zap.NewNop()
	}
	desc := &c.Book.Description
	metadata := pdfXMPDocumentMetadata{
		Title:        title,
		Creators:     bookAuthorList(c, cfg, log),
		Contributors: bookTranslatorList(c, cfg, log),
		Description:  description,
		Publisher:    metainfo.Publisher(desc.PublishInfo),
		Language:     pdfMetadataLanguage(desc.TitleInfo.Lang),
		Keywords:     keywords,
	}
	if date, err := metainfo.OPFDate(desc); err != nil {
		log.Debug("Invalid date metadata, skipping", zap.Error(err))
	} else if date != "" {
		metadata.Date = date
	}
	if id := strings.TrimSpace(desc.DocumentInfo.ID); id != "" {
		metadata.Identifiers = append(metadata.Identifiers, id)
	}
	if isbn, _, err := metainfo.ISBN(desc.PublishInfo); err != nil {
		log.Debug("Invalid ISBN metadata, skipping", zap.Error(err))
	} else if isbn != "" {
		metadata.Identifiers = append(metadata.Identifiers, "ISBN:"+isbn)
	}
	for _, subject := range metainfo.Subjects(desc.TitleInfo.Genres, desc.TitleInfo.Keywords) {
		metadata.Subjects = append(metadata.Subjects, subject.Value)
	}
	return xmpMetadataPacket(metadata)
}

func pdfMetadataLanguage(tag language.Tag) string {
	if tag == language.Und {
		return ""
	}
	return strings.TrimSpace(tag.String())
}

func xmpMetadataPacket(metadata pdfXMPDocumentMetadata) []byte {
	var b strings.Builder
	b.WriteString(`<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>`)
	b.WriteByte('\n')
	b.WriteString(`<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">`)
	b.WriteByte('\n')
	b.WriteString(`<rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:pdf="http://ns.adobe.com/pdf/1.3/" xmlns:xmp="http://ns.adobe.com/xap/1.0/">`)
	b.WriteByte('\n')
	if metadata.Title != "" {
		writeXMPAlt(&b, "dc:title", metadata.Title)
	}
	writeXMPSeq(&b, "dc:creator", metadata.Creators)
	writeXMPBag(&b, "dc:contributor", metadata.Contributors)
	if metadata.Description != "" {
		writeXMPAlt(&b, "dc:description", metadata.Description)
	}
	writeXMPBag(&b, "dc:subject", metadata.Subjects)
	writeXMPBag(&b, "dc:publisher", stringSlice(metadata.Publisher))
	writeXMPSeq(&b, "dc:date", stringSlice(metadata.Date))
	writeXMPBag(&b, "dc:identifier", metadata.Identifiers)
	writeXMPBag(&b, "dc:language", stringSlice(metadata.Language))
	b.WriteString(`<xmp:CreatorTool>fbc</xmp:CreatorTool>`)
	b.WriteByte('\n')
	b.WriteString(`<pdf:Producer>fbc</pdf:Producer>`)
	b.WriteByte('\n')
	if metadata.Keywords != "" {
		writeXMPSimple(&b, "pdf:Keywords", metadata.Keywords)
	}
	b.WriteString(`</rdf:Description>`)
	b.WriteByte('\n')
	b.WriteString(`</rdf:RDF></x:xmpmeta>`)
	b.WriteByte('\n')
	b.WriteString(`<?xpacket end="w"?>`)
	return []byte(b.String())
}

func writeXMPSimple(b *strings.Builder, name, value string) {
	b.WriteByte('<')
	b.WriteString(name)
	b.WriteByte('>')
	b.WriteString(xmlText(value))
	b.WriteString("</")
	b.WriteString(name)
	b.WriteByte('>')
	b.WriteByte('\n')
}

func writeXMPAlt(b *strings.Builder, name, value string) {
	b.WriteByte('<')
	b.WriteString(name)
	b.WriteString(`><rdf:Alt><rdf:li xml:lang="x-default">`)
	b.WriteString(xmlText(value))
	b.WriteString(`</rdf:li></rdf:Alt></`)
	b.WriteString(name)
	b.WriteByte('>')
	b.WriteByte('\n')
}

func writeXMPSeq(b *strings.Builder, name string, values []string) {
	writeXMPArray(b, name, "Seq", values)
}

func writeXMPBag(b *strings.Builder, name string, values []string) {
	writeXMPArray(b, name, "Bag", values)
}

func writeXMPArray(b *strings.Builder, name, kind string, values []string) {
	values = compactStrings(values)
	if len(values) == 0 {
		return
	}
	b.WriteByte('<')
	b.WriteString(name)
	b.WriteString(`><rdf:`)
	b.WriteString(kind)
	b.WriteByte('>')
	for _, value := range values {
		b.WriteString(`<rdf:li>`)
		b.WriteString(xmlText(value))
		b.WriteString(`</rdf:li>`)
	}
	b.WriteString(`</rdf:`)
	b.WriteString(kind)
	b.WriteString(`></`)
	b.WriteString(name)
	b.WriteByte('>')
	b.WriteByte('\n')
}

func compactStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func stringSlice(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return []string{value}
}

func xmlText(value string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}

func metadataExcerpt(text string, maxRunes int) string {
	text = strings.Join(strings.Fields(text), " ")
	if maxRunes <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes])
}
