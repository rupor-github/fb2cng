package fb2

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/beevik/etree"
	"go.uber.org/zap"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// XML parsing functions for FictionBook2 format.
// We want exaustive parsing, it is not very effective but ensures full
// correctness, gives us detailed debug output and the result should be easy to
// extend if necessary. It is relatively simple to map back to XSD schema.

// ParseBookXML walks the etree DOM and constructs a strongly typed
// representation following the FictionBook 2.0 schema definitions.
func ParseBookXML(doc *etree.Document, footnoteNames []string, log *zap.Logger) (*FictionBook, error) {
	if doc == nil {
		return nil, fmt.Errorf("nil document")
	}

	root := doc.Root()
	if root == nil {
		return nil, fmt.Errorf("document has no root element")
	}
	if root.Tag != "FictionBook" {
		return nil, fmt.Errorf("unexpected root element %q", root.Tag)
	}

	book := &FictionBook{}
	for _, child := range root.ChildElements() {
		switch child.Tag {
		case "stylesheet":
			book.Stylesheets = append(book.Stylesheets, parseStylesheet(child, log))
		case "description":
			desc, err := parseDescription(child, log)
			if err != nil {
				return nil, fmt.Errorf("description: %w", err)
			}
			book.Description = desc
		case "body":
			body, err := parseBody(child, log)
			if err != nil {
				return nil, fmt.Errorf("body: %w", err)
			}
			// Classify body kind using provided footnote names (case-insensitive).
			if len(book.Bodies) == 0 {
				body.Kind = BodyMain
			} else {
				matched := false
				for _, fn := range footnoteNames {
					if strings.EqualFold(body.Name, fn) {
						matched = true
						break
					}
				}
				if matched {
					body.Kind = BodyFootnotes
				} else {
					body.Kind = BodyOther
				}
			}
			book.Bodies = append(book.Bodies, body)
		case "binary":
			bin, err := parseBinary(child, log)
			if err != nil {
				return nil, fmt.Errorf("binary: %w", err)
			}
			book.Binaries = append(book.Binaries, bin)
		default:
			log.Warn("Unexpected tag in FictionBook, ignoring", zap.String("parent", root.Tag), zap.String("tag", child.Tag))
		}
	}

	return book, nil
}

func parseStylesheet(el *etree.Element, _ *zap.Logger) Stylesheet {
	return Stylesheet{
		Type: el.SelectAttrValue("type", ""),
		Data: strings.TrimSpace(el.Text()),
	}
}

func parseDescription(el *etree.Element, log *zap.Logger) (Description, error) {
	var desc Description
	for _, child := range el.ChildElements() {
		switch child.Tag {
		case "title-info":
			info, err := parseTitleInfo(child, log)
			if err != nil {
				return desc, fmt.Errorf("title-info: %w", err)
			}
			desc.TitleInfo = info
		case "src-title-info":
			info, err := parseTitleInfo(child, log)
			if err != nil {
				return desc, fmt.Errorf("src-title-info: %w", err)
			}
			desc.SrcTitleInfo = &info
		case "document-info":
			info, err := parseDocumentInfo(child, log)
			if err != nil {
				return desc, fmt.Errorf("document-info: %w", err)
			}
			desc.DocumentInfo = info
		case "publish-info":
			info, err := parsePublishInfo(child, log)
			if err != nil {
				return desc, fmt.Errorf("publish-info: %w", err)
			}
			desc.PublishInfo = &info
		case "custom-info":
			custom := parseCustomInfo(child, log)
			desc.CustomInfo = append(desc.CustomInfo, custom)
		case "output":
			out, err := parseOutputInstruction(child, log)
			if err != nil {
				return desc, fmt.Errorf("output: %w", err)
			}
			desc.Output = append(desc.Output, out)
		default:
			log.Warn("Unexpected tag in description, ignoring", zap.String("parent", el.Tag), zap.String("tag", child.Tag))
		}
	}
	return desc, nil
}

func parseTitleInfo(el *etree.Element, log *zap.Logger) (TitleInfo, error) {
	info := TitleInfo{}
	for _, child := range el.ChildElements() {
		switch child.Tag {
		case "genre":
			match := 100
			if attr := child.SelectAttrValue("match", ""); attr != "" {
				if v, err := strconv.Atoi(attr); err == nil {
					match = v
				}
			}
			if value := strings.TrimSpace(child.Text()); len(value) > 0 {
				info.Genres = append(info.Genres, GenreRef{
					Value: strings.TrimSpace(child.Text()),
					Match: match,
				})
			} else {
				log.Debug("Empty genre value, skipping")
			}
		case "author":
			info.Authors = append(info.Authors, parseAuthor(child, log))
		case "book-title":
			info.BookTitle = parseTextField(child, log)
		case "annotation":
			flow, err := parseFlow(child, log)
			if err != nil {
				return info, fmt.Errorf("annotation: %w", err)
			}
			info.Annotation = &flow
		case "keywords":
			tf := parseTextField(child, log)
			info.Keywords = &tf
		case "date":
			date := parseDate(child, log)
			info.Date = &date
		case "coverpage":
			for _, img := range child.ChildElements() {
				if img.Tag != "image" {
					continue
				}
				info.Coverpage = append(info.Coverpage, parseInlineImage(img, log))
			}
		case "lang":
			info.Lang = parseBookLang(child.Text(), log)
		case "src-lang":
			info.SrcLang = parseBookLang(child.Text(), log)
		case "translator":
			info.Translators = append(info.Translators, parseAuthor(child, log))
		case "sequence":
			seq := parseSequence(child, log)
			info.Sequences = append(info.Sequences, seq)
		default:
			log.Warn("Unexpected tag in title-info, ignoring", zap.String("parent", el.Tag), zap.String("tag", child.Tag))
		}
	}
	return info, nil
}

func parseBookLang(in string, log *zap.Logger) language.Tag {
	lang := strings.TrimSpace(in)
	if lang == "" {
		return language.Und
	}

	tag, err := language.Parse(lang)
	if err == nil {
		return tag
	}

	// last resort - try names directly
	for _, supportedTag := range display.Supported.Tags() {
		if strings.EqualFold(display.Self.Name(supportedTag), lang) {
			return supportedTag
		}
	}
	log.Warn("Unable to parse book language", zap.String("lang", lang))
	return language.Und
}

func parseAuthor(el *etree.Element, log *zap.Logger) Author {
	author := Author{}
	for _, child := range el.ChildElements() {
		switch child.Tag {
		case "first-name":
			author.FirstName = strings.TrimSpace(child.Text())
		case "middle-name":
			author.MiddleName = strings.TrimSpace(child.Text())
		case "last-name":
			author.LastName = strings.TrimSpace(child.Text())
		case "nickname":
			author.Nickname = strings.TrimSpace(child.Text())
		case "home-page":
			author.HomePages = append(author.HomePages, strings.TrimSpace(child.Text()))
		case "email":
			author.Emails = append(author.Emails, strings.TrimSpace(child.Text()))
		case "id":
			author.ID = strings.TrimSpace(child.Text())
		default:
			log.Warn("Unexpected tag in author, ignoring", zap.String("parent", el.Tag), zap.String("tag", child.Tag))
		}
	}
	return author
}

func parseTextField(el *etree.Element, _ *zap.Logger) TextField {
	return TextField{
		Value: strings.TrimSpace(el.Text()),
		Lang:  xmlLang(el),
	}
}

func parseDate(el *etree.Element, log *zap.Logger) Date {
	value := el.SelectAttrValue("value", "")

	d := Date{
		Display: strings.TrimSpace(el.Text()),
		Lang:    xmlLang(el),
	}

	if value != "" {
		// xs:date format is YYYY-MM-DD (ISO 8601 date format)
		// Try parsing with timezone first, then without
		formats := []string{
			"2006-01-02Z07:00",
			"2006-01-02",
		}

		var parsed time.Time
		var err error
		for _, format := range formats {
			parsed, err = time.Parse(format, value)
			if err == nil {
				d.Value = parsed
				break
			}
		}

		if err != nil && log != nil {
			log.Debug("Failed to parse date value", zap.String("value", value), zap.Error(err))
		}
	}

	return d
}

func parseFlow(el *etree.Element, log *zap.Logger) (Flow, error) {
	flow := Flow{
		ID:   el.SelectAttrValue("id", ""),
		Lang: xmlLang(el),
	}
	for _, node := range el.Child {
		child, ok := node.(*etree.Element)
		if !ok {
			continue
		}
		item, err := parseFlowItem(child, el.Tag, log)
		if err != nil {
			return flow, err
		}
		if item != nil {
			flow.Items = append(flow.Items, *item)
		}
	}
	return flow, nil
}

// isKnownTextTag checks if a tag is a known FB2 text-containing element
// that could reasonably be converted to a paragraph when found unexpectedly
func isKnownTextTag(tag string) bool {
	knownTextTags := map[string]bool{
		"text-author": true,
		"date":        true,
		"v":           true, // verse
		"stanza":      true,
		"epigraph":    true,
		"annotation":  true,
	}
	return knownTextTags[tag]
}

// extractAllText recursively extracts all text content from an element,
// including from nested block and inline elements
func extractAllText(el *etree.Element) string {
	var text strings.Builder
	for _, node := range el.Child {
		switch token := node.(type) {
		case *etree.CharData:
			text.WriteString(token.Data)
		case *etree.Element:
			// Recursively extract text from nested elements
			text.WriteString(extractAllText(token))
		}
	}
	return text.String()
}

// parseUnexpectedAsParagraph extracts all text content from an element and its children
// and creates a paragraph with it. Used when handling unexpected tags.
func parseUnexpectedAsParagraph(el *etree.Element, log *zap.Logger) Paragraph {
	para := Paragraph{
		ID:    el.SelectAttrValue("id", ""),
		Style: el.SelectAttrValue("style", ""),
		Lang:  xmlLang(el),
	}
	// Set style to mark this as an unexpected conversion
	if para.Style == "" {
		para.Style = "unexpected " + el.Tag
	} else {
		para.Style = "unexpected " + el.Tag + " " + para.Style
	}
	// Extract all text content recursively
	allText := extractAllText(el)
	if allText != "" {
		para.Text = []InlineSegment{{Kind: InlineText, Text: allText}}
	}
	return para
}

func parseFlowItem(el *etree.Element, parentTag string, log *zap.Logger) (*FlowItem, error) {
	switch el.Tag {
	case "p":
		para := parseParagraph(el, false, log)
		return &FlowItem{Kind: FlowParagraph, Paragraph: &para}, nil
	case "poem":
		poem, err := parsePoem(el, log)
		if err != nil {
			return nil, err
		}
		p := poem
		return &FlowItem{Kind: FlowPoem, Poem: &p}, nil
	case "cite":
		cite, err := parseCite(el, log)
		if err != nil {
			return nil, err
		}
		c := cite
		return &FlowItem{Kind: FlowCite, Cite: &c}, nil
	case "subtitle":
		para := parseParagraph(el, true, log)
		return &FlowItem{Kind: FlowSubtitle, Subtitle: &para}, nil
	case "empty-line":
		return &FlowItem{Kind: FlowEmptyLine}, nil
	case "table":
		table, err := parseTable(el, log)
		if err != nil {
			return nil, err
		}
		t := table
		return &FlowItem{Kind: FlowTable, Table: &t}, nil
	case "image":
		img := parseImage(el, log)
		return &FlowItem{Kind: FlowImage, Image: &img}, nil
	case "section":
		section, err := parseSection(el, log)
		if err != nil {
			return nil, err
		}
		s := section
		return &FlowItem{Kind: FlowSection, Section: &s}, nil
	default:
		msg := fmt.Sprintf("Unexpected tag in %s, ", parentTag)
		// Check if this is a known text-containing tag that we can handle as a paragraph
		if isKnownTextTag(el.Tag) {
			log.Warn(msg+"converting to paragraph", zap.String("tag", el.Tag))
			para := parseUnexpectedAsParagraph(el, log)
			return &FlowItem{Kind: FlowParagraph, Paragraph: &para}, nil
		}
		log.Warn(msg+"ignoring", zap.String("tag", el.Tag))
		return nil, nil
	}
}

func parseParagraph(el *etree.Element, special bool, log *zap.Logger) Paragraph {
	para := Paragraph{
		ID:      el.SelectAttrValue("id", ""),
		Style:   el.SelectAttrValue("style", ""),
		Lang:    xmlLang(el),
		Special: special,
	}
	para.Text = parseInlineSegments(el, log)
	return para
}

func parseInlineSegments(parent *etree.Element, log *zap.Logger) []InlineSegment {
	var segments []InlineSegment
	for _, node := range parent.Child {
		switch token := node.(type) {
		case *etree.CharData:
			if token.Data == "" {
				continue
			}
			segments = append(segments, InlineSegment{Kind: InlineText, Text: token.Data})
		case *etree.Element:
			kind := mapInlineKind(token.Tag)
			segment := InlineSegment{
				Kind:     kind,
				Lang:     xmlLang(token),
				Name:     token.SelectAttrValue("name", ""),
				Style:    token.SelectAttrValue("style", ""),
				Href:     attrValue(token, "xlink", "href"),
				LinkType: token.SelectAttrValue("type", ""),
			}
			if kind == InlineImageSegment {
				img := parseInlineImage(token, log)
				segment.Image = &img
			} else {
				segment.Children = parseInlineSegments(token, log)
			}
			segments = append(segments, segment)
		}
	}
	return segments
}

func mapInlineKind(tag string) InlineSegmentKind {
	switch tag {
	case "strong":
		return InlineStrong
	case "emphasis":
		return InlineEmphasis
	case "style":
		return InlineNamedStyle
	case "a":
		return InlineLink
	case "strikethrough":
		return InlineStrikethrough
	case "sub":
		return InlineSub
	case "sup":
		return InlineSup
	case "code":
		return InlineCode
	case "image":
		return InlineImageSegment
	default:
		return InlineText
	}
}

func parseInlineImage(el *etree.Element, _ *zap.Logger) InlineImage {
	return InlineImage{
		Href: attrValue(el, "xlink", "href"),
		Type: attrValue(el, "xlink", "type"),
		Alt:  el.SelectAttrValue("alt", ""),
	}
}

func parseImage(el *etree.Element, _ *zap.Logger) Image {
	return Image{
		Href:  attrValue(el, "xlink", "href"),
		Type:  attrValue(el, "xlink", "type"),
		Alt:   el.SelectAttrValue("alt", ""),
		Title: el.SelectAttrValue("title", ""),
		ID:    el.SelectAttrValue("id", ""),
	}
}

func parseBody(el *etree.Element, log *zap.Logger) (Body, error) {
	body := Body{
		Name: el.SelectAttrValue("name", ""),
		Lang: xmlLang(el),
	}
	for _, child := range el.ChildElements() {
		switch child.Tag {
		case "image":
			img := parseImage(child, log)
			body.Image = &img
		case "title":
			title := parseTitle(child, log)
			body.Title = title
		case "epigraph":
			epi, err := parseEpigraph(child, log)
			if err != nil {
				return body, err
			}
			body.Epigraphs = append(body.Epigraphs, epi)
		case "section":
			section, err := parseSection(child, log)
			if err != nil {
				return body, err
			}
			body.Sections = append(body.Sections, section)
		default:
			log.Warn("Unexpected tag in body, ignoring", zap.String("parent", el.Tag), zap.String("tag", child.Tag))
		}
	}
	return body, nil
}

func parseTitle(el *etree.Element, log *zap.Logger) *Title {
	title := &Title{Lang: xmlLang(el)}
	for _, node := range el.Child {
		child, ok := node.(*etree.Element)
		if !ok {
			continue
		}
		switch child.Tag {
		case "p":
			para := parseParagraph(child, true, log)
			title.Items = append(title.Items, TitleItem{Paragraph: &para})
		case "empty-line":
			title.Items = append(title.Items, TitleItem{EmptyLine: true})
		default:
			log.Warn("Unexpected tag in title, ignoring", zap.String("parent", el.Tag), zap.String("tag", child.Tag))
		}
	}
	if len(title.Items) == 0 {
		return nil
	}
	return title
}

func parseEpigraph(el *etree.Element, log *zap.Logger) (Epigraph, error) {
	epi := Epigraph{}
	epi.Flow = Flow{ID: el.SelectAttrValue("id", ""), Lang: xmlLang(el)}
	for _, node := range el.Child {
		child, ok := node.(*etree.Element)
		if !ok {
			continue
		}
		if child.Tag == "text-author" {
			para := parseParagraph(child, true, log)
			epi.TextAuthors = append(epi.TextAuthors, para)
			continue
		}
		item, err := parseFlowItem(child, el.Tag, log)
		if err != nil {
			return epi, err
		}
		if item != nil {
			epi.Flow.Items = append(epi.Flow.Items, *item)
		}
	}
	return epi, nil
}

func parseSection(el *etree.Element, log *zap.Logger) (Section, error) {
	section := Section{
		ID:   el.SelectAttrValue("id", ""),
		Lang: xmlLang(el),
	}
	for _, node := range el.Child {
		child, ok := node.(*etree.Element)
		if !ok {
			continue
		}
		switch child.Tag {
		case "title":
			section.Title = parseTitle(child, log)
		case "epigraph":
			epi, err := parseEpigraph(child, log)
			if err != nil {
				return section, err
			}
			section.Epigraphs = append(section.Epigraphs, epi)
		case "annotation":
			flow, err := parseFlow(child, log)
			if err != nil {
				return section, err
			}
			section.Annotation = &flow
		case "section":
			sub, err := parseSection(child, log)
			if err != nil {
				return section, err
			}
			subCopy := sub
			section.Content = append(section.Content, FlowItem{Kind: FlowSection, Section: &subCopy})
		default:
			item, err := parseFlowItem(child, el.Tag, log)
			if err != nil {
				return section, err
			}
			if item != nil {
				section.Content = append(section.Content, *item)
			}
		}
	}
	return section, nil
}

func parsePoem(el *etree.Element, log *zap.Logger) (Poem, error) {
	poem := Poem{
		ID:   el.SelectAttrValue("id", ""),
		Lang: xmlLang(el),
	}
	for _, node := range el.Child {
		child, ok := node.(*etree.Element)
		if !ok {
			continue
		}
		switch child.Tag {
		case "title":
			poem.Title = parseTitle(child, log)
		case "epigraph":
			epi, err := parseEpigraph(child, log)
			if err != nil {
				return poem, err
			}
			poem.Epigraphs = append(poem.Epigraphs, epi)
		case "subtitle":
			para := parseParagraph(child, false, log)
			poem.Subtitles = append(poem.Subtitles, para)
		case "stanza":
			stanza, err := parseStanza(child, log)
			if err != nil {
				return poem, err
			}
			poem.Stanzas = append(poem.Stanzas, stanza)
		case "text-author":
			para := parseParagraph(child, true, log)
			poem.TextAuthors = append(poem.TextAuthors, para)
		case "date":
			date := parseDate(child, log)
			poem.Date = &date
		default:
			log.Warn("Unexpected tag in poem, ignoring", zap.String("parent", el.Tag), zap.String("tag", child.Tag))
		}
	}
	return poem, nil
}

func parseStanza(el *etree.Element, log *zap.Logger) (Stanza, error) {
	stanza := Stanza{Lang: xmlLang(el)}
	for _, node := range el.Child {
		child, ok := node.(*etree.Element)
		if !ok {
			continue
		}
		switch child.Tag {
		case "title":
			stanza.Title = parseTitle(child, log)
		case "subtitle":
			para := parseParagraph(child, false, log)
			stanza.Subtitle = &para
		case "v":
			para := parseParagraph(child, false, log)
			stanza.Verses = append(stanza.Verses, para)
		default:
			log.Warn("Unexpected tag in stanza, ignoring", zap.String("parent", el.Tag), zap.String("tag", child.Tag))
		}
	}
	return stanza, nil
}

func parseCite(el *etree.Element, log *zap.Logger) (Cite, error) {
	cite := Cite{
		ID:   el.SelectAttrValue("id", ""),
		Lang: xmlLang(el),
	}
	for _, node := range el.Child {
		child, ok := node.(*etree.Element)
		if !ok {
			continue
		}
		if child.Tag == "text-author" {
			para := parseParagraph(child, true, log)
			cite.TextAuthors = append(cite.TextAuthors, para)
			continue
		}
		item, err := parseFlowItem(child, el.Tag, log)
		if err != nil {
			return cite, err
		}
		if item != nil {
			cite.Items = append(cite.Items, *item)
		}
	}
	return cite, nil
}

func parseTable(el *etree.Element, log *zap.Logger) (Table, error) {
	table := Table{
		ID:    el.SelectAttrValue("id", ""),
		Style: el.SelectAttrValue("style", ""),
	}
	for _, rowEl := range el.SelectElements("tr") {
		row := TableRow{
			Style: rowEl.SelectAttrValue("style", ""),
			Align: rowEl.SelectAttrValue("align", ""),
		}
		for _, cellNode := range rowEl.ChildElements() {
			cell, err := parseTableCell(cellNode, log)
			if err != nil {
				return table, err
			}
			row.Cells = append(row.Cells, cell)
		}
		table.Rows = append(table.Rows, row)
	}
	return table, nil
}

func parseTableCell(el *etree.Element, log *zap.Logger) (TableCell, error) {
	cell := TableCell{Header: el.Tag == "th"}
	cell.ID = el.SelectAttrValue("id", "")
	cell.Style = el.SelectAttrValue("style", "")
	if raw := el.SelectAttrValue("colspan", ""); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return cell, fmt.Errorf("table cell colspan: %w", err)
		}
		cell.ColSpan = v
	}
	if raw := el.SelectAttrValue("rowspan", ""); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return cell, fmt.Errorf("table cell rowspan: %w", err)
		}
		cell.RowSpan = v
	}
	cell.Align = el.SelectAttrValue("align", "")
	cell.VAlign = el.SelectAttrValue("valign", "")
	cell.Content = parseInlineSegments(el, log)
	return cell, nil
}

func parseDocumentInfo(el *etree.Element, log *zap.Logger) (DocumentInfo, error) {
	info := DocumentInfo{}
	for _, child := range el.ChildElements() {
		switch child.Tag {
		case "author":
			info.Authors = append(info.Authors, parseAuthor(child, log))
		case "program-used":
			tf := parseTextField(child, log)
			info.ProgramUsed = &tf
		case "date":
			info.Date = parseDate(child, log)
		case "src-url":
			info.SourceURLs = append(info.SourceURLs, strings.TrimSpace(child.Text()))
		case "src-ocr":
			tf := parseTextField(child, log)
			info.SourceOCR = &tf
		case "id":
			info.ID = strings.TrimSpace(child.Text())
		case "version":
			info.Version = strings.TrimSpace(child.Text())
		case "history":
			flow, err := parseFlow(child, log)
			if err != nil {
				return info, fmt.Errorf("history: %w", err)
			}
			info.History = &flow
		case "publisher":
			info.Publishers = append(info.Publishers, parseAuthor(child, log))
		default:
			log.Warn("Unexpected tag in document-info, ignoring", zap.String("parent", el.Tag), zap.String("tag", child.Tag))
		}
	}
	return info, nil
}

func parsePublishInfo(el *etree.Element, log *zap.Logger) (PublishInfo, error) {
	info := PublishInfo{}
	for _, child := range el.ChildElements() {
		switch child.Tag {
		case "book-name":
			tf := parseTextField(child, log)
			info.BookName = &tf
		case "publisher":
			tf := parseTextField(child, log)
			info.Publisher = &tf
		case "city":
			tf := parseTextField(child, log)
			info.City = &tf
		case "year":
			info.Year = strings.TrimSpace(child.Text())
		case "isbn":
			tf := parseTextField(child, log)
			info.ISBN = &tf
		case "sequence":
			seq := parseSequence(child, log)
			info.Sequences = append(info.Sequences, seq)
		default:
			log.Warn("Unexpected tag in publish-info, ignoring", zap.String("parent", el.Tag), zap.String("tag", child.Tag))
		}
	}
	return info, nil
}

func parseSequence(el *etree.Element, log *zap.Logger) Sequence {
	seq := Sequence{
		Name: el.SelectAttrValue("name", ""),
		Lang: xmlLang(el),
	}
	if raw := el.SelectAttrValue("number", ""); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			seq.Number = &v
		} else {
			log.Warn("Invalid sequence number, ignoring", zap.String("name", seq.Name), zap.String("raw", raw), zap.Error(err))
		}
	}
	for _, child := range el.ChildElements() {
		if child.Tag != "sequence" {
			continue
		}
		nested := parseSequence(child, log)
		seq.Children = append(seq.Children, nested)
	}
	return seq
}

func parseCustomInfo(el *etree.Element, log *zap.Logger) CustomInfo {
	return CustomInfo{
		Type:  el.SelectAttrValue("info-type", ""),
		Value: parseTextField(el, log),
	}
}

func parseOutputInstruction(el *etree.Element, log *zap.Logger) (OutputInstruction, error) {
	mode, err := parseShareMode(el.SelectAttrValue("mode", ""))
	if err != nil {
		return OutputInstruction{}, err
	}
	include, err := parseShareDirective(el.SelectAttrValue("include-all", ""))
	if err != nil {
		return OutputInstruction{}, err
	}
	instruction := OutputInstruction{
		Mode:       mode,
		IncludeAll: include,
		Currency:   el.SelectAttrValue("currency", ""),
	}
	if raw := el.SelectAttrValue("price", ""); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return instruction, fmt.Errorf("output price: %w", err)
		}
		instruction.Price = &v
	}
	for _, child := range el.ChildElements() {
		switch child.Tag {
		case "part":
			part, err := parsePartInstruction(child, log)
			if err != nil {
				return instruction, err
			}
			instruction.Parts = append(instruction.Parts, part)
		case "output-document-class":
			doc, err := parseOutputDocument(child, log)
			if err != nil {
				return instruction, err
			}
			instruction.Documents = append(instruction.Documents, doc)
		default:
			log.Warn("Unexpected tag in output instruction, ignoring", zap.String("parent", el.Tag), zap.String("tag", child.Tag))
		}
	}
	return instruction, nil
}

func parsePartInstruction(el *etree.Element, _ *zap.Logger) (PartInstruction, error) {
	href := attrValue(el, "xlink", "href")
	if href == "" {
		return PartInstruction{}, fmt.Errorf("part missing xlink:href")
	}
	include, err := parseShareDirective(el.SelectAttrValue("include", ""))
	if err != nil {
		return PartInstruction{}, err
	}
	return PartInstruction{Href: href, Include: include}, nil
}

func parseOutputDocument(el *etree.Element, log *zap.Logger) (OutputDocument, error) {
	if raw := el.SelectAttrValue("name", ""); raw == "" {
		return OutputDocument{}, fmt.Errorf("output-document-class missing name")
	}
	doc := OutputDocument{Name: el.SelectAttrValue("name", "")}
	if raw := el.SelectAttrValue("create", ""); raw != "" {
		directive, err := parseShareDirective(raw)
		if err != nil {
			return doc, err
		}
		doc.Create = &directive
	}
	if raw := el.SelectAttrValue("price", ""); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return doc, fmt.Errorf("output-document price: %w", err)
		}
		doc.Price = &v
	}
	for _, child := range el.ChildElements() {
		if child.Tag != "part" {
			continue
		}
		part, err := parsePartInstruction(child, log)
		if err != nil {
			return doc, err
		}
		doc.Parts = append(doc.Parts, part)
	}
	return doc, nil
}

func parseBinary(el *etree.Element, log *zap.Logger) (BinaryObject, error) {
	id := el.SelectAttrValue("id", "")
	contentType := el.SelectAttrValue("content-type", "")
	data, err := base64.StdEncoding.DecodeString(normalizeBase64(el.Text()))
	if err != nil {
		var corruptErr base64.CorruptInputError
		if errors.As(err, &corruptErr) && len(data) > 0 {
			log.Warn("Unable to fully decode binary", zap.String("id", id), zap.String("contentType", contentType), zap.Error(err))
			return BinaryObject{ID: id, ContentType: contentType, Data: data}, nil
		}
		return BinaryObject{}, fmt.Errorf("decode binary %q: %w", id, err)
	}
	return BinaryObject{ID: id, ContentType: contentType, Data: data}, nil
}

func parseShareMode(raw string) (ShareMode, error) {
	switch ShareMode(raw) {
	case ShareModeFree:
		return ShareModeFree, nil
	case ShareModePaid:
		return ShareModePaid, nil
	default:
		return "", fmt.Errorf("unknown share mode %q", raw)
	}
}

func parseShareDirective(raw string) (ShareDirective, error) {
	switch ShareDirective(raw) {
	case ShareRequire, ShareAllow, ShareDeny:
		return ShareDirective(raw), nil
	default:
		return "", fmt.Errorf("unknown share directive %q", raw)
	}
}

func attrValue(el *etree.Element, space, key string) string {
	if space == "" {
		return el.SelectAttrValue(key, "")
	}
	for _, attr := range el.Attr {
		if (attr.Space == space || strings.HasSuffix(attr.NamespaceURI(), "/"+space)) && attr.Key == key {
			return attr.Value
		}
	}
	return ""
}

func xmlLang(el *etree.Element) string {
	for _, attr := range el.Attr {
		if (attr.Space == "xml" || strings.HasSuffix(attr.NamespaceURI(), "/xml")) && attr.Key == "lang" {
			return attr.Value
		}
	}
	return ""
}

func normalizeBase64(input string) string {
	var builder strings.Builder
	builder.Grow(len(input))
	for _, r := range input {
		if !unicode.IsSpace(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
