package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/text/language"

	"fbc/content"
	contenttext "fbc/content/text"
	"fbc/convert/pdf/docwriter"
	"fbc/fb2"
)

func pdfHyphenator(c *content.Content, log *zap.Logger) paragraphHyphenator {
	if c == nil || c.Book == nil || c.Book.Description.TitleInfo.Lang == language.Und {
		return nil
	}
	if log == nil {
		log = zap.NewNop()
	}
	return contenttext.NewHyphenator(c.Book.Description.TitleInfo.Lang, log)
}

func infoDictionary(doc skeletonDocument) docwriter.Dict {
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

func bookTitle(c *content.Content) string {
	if c.Book == nil {
		return strings.TrimSuffix(c.SrcName, ".fb2")
	}
	if title := strings.TrimSpace(c.Book.Description.TitleInfo.BookTitle.Value); title != "" {
		return title
	}
	return strings.TrimSuffix(c.SrcName, ".fb2")
}

func bookAuthors(c *content.Content) string {
	if c.Book == nil {
		return ""
	}

	authors := make([]string, 0, len(c.Book.Description.TitleInfo.Authors))
	for i := range c.Book.Description.TitleInfo.Authors {
		name := authorName(&c.Book.Description.TitleInfo.Authors[i])
		if name != "" {
			authors = append(authors, name)
		}
	}
	return strings.Join(authors, ", ")
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

func authorName(author *fb2.Author) string {
	if author == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	for _, part := range []string{author.FirstName, author.MiddleName, author.LastName} {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) != 0 {
		return strings.Join(parts, " ")
	}
	return strings.TrimSpace(author.Nickname)
}
