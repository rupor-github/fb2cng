package fb2

import (
	"fmt"
	"strconv"

	"fbc/utils/debug"
)

type treeWriter struct {
	*debug.TreeWriter
}

// String returns a readable tree of the parsed FictionBook. It omits binary payloads
// to keep the output compact while preserving character data via escaped control sequences.
// It exists solely for manual inspection during debugging.
func (b *FictionBook) String() string {
	if b == nil {
		return "<nil FictionBook>"
	}
	return treeWriter{debug.NewTreeWriter()}.fictionBook(b).String()
}

func (tw treeWriter) fictionBook(book *FictionBook) treeWriter {
	tw.Line(0, "FictionBook")
	if book.NotFoundImageID != "" {
		tw.Line(1, "NotFoundImageID=%q", book.NotFoundImageID)
	}
	if len(book.VignetteIDs) > 0 {
		tw.Line(1, "VignetteIDs: %d", len(book.VignetteIDs))
		for pos, id := range book.VignetteIDs {
			tw.Line(2, "Vignette[%s]=%q", pos, id)
		}
	}
	for i, sheet := range book.Stylesheets {
		tw.Line(1, "Stylesheet[%d] type=%q bytes=%d", i, sheet.Type, len(sheet.Data))
	}
	tw.description(1, &book.Description)
	for i := range book.Bodies {
		tw.body(1, &book.Bodies[i], i)
	}
	if len(book.Binaries) > 0 {
		tw.Line(1, "Binaries: %d", len(book.Binaries))
		for i := range book.Binaries {
			bin := &book.Binaries[i]
			tw.Line(2, "Binary[%d] id=%q contentType=%q bytes=%d", i, bin.ID, bin.ContentType, len(bin.Data))
		}
	}
	return tw
}

func (tw treeWriter) description(depth int, desc *Description) {
	if desc == nil {
		return
	}
	tw.Line(depth, "Description")
	tw.titleInfo(depth+1, "TitleInfo", &desc.TitleInfo)
	if desc.SrcTitleInfo != nil {
		tw.titleInfo(depth+1, "SrcTitleInfo", desc.SrcTitleInfo)
	}
	tw.documentInfo(depth+1, &desc.DocumentInfo)
	if desc.PublishInfo != nil {
		tw.publishInfo(depth+1, desc.PublishInfo)
	}
	for i := range desc.CustomInfo {
		ci := desc.CustomInfo[i]
		tw.Line(depth+1, "CustomInfo[%d] type=%q", i, ci.Type)
		tw.TextBlock(depth+2, "Value", ci.Value.Value)
	}
	for i := range desc.Output {
		tw.outputInstruction(depth+1, &desc.Output[i], i)
	}
}

func (tw treeWriter) titleInfo(depth int, label string, info *TitleInfo) {
	if info == nil {
		return
	}
	tw.Line(depth, "%s lang=%q srcLang=%q", label, info.Lang.String(), info.SrcLang.String())
	for i := range info.Genres {
		g := info.Genres[i]
		tw.Line(depth+1, "Genre[%d] value=%q match=%d", i, g.Value, g.Match)
	}
	for i := range info.Authors {
		tw.author(depth+1, "Author", &info.Authors[i], i)
	}
	tw.TextBlock(depth+1, "BookTitle", info.BookTitle.Value)
	if info.Annotation != nil {
		tw.Line(depth+1, "Annotation")
		tw.flow(depth+2, info.Annotation)
	}
	if info.Keywords != nil {
		tw.TextBlock(depth+1, "Keywords", info.Keywords.Value)
	}
	if info.Date != nil {
		tw.Line(depth+1, "Date value=%q display=%q", info.Date.Value, info.Date.Display)
	}
	if len(info.Coverpage) > 0 {
		for i := range info.Coverpage {
			tw.inlineImage(depth+1, "CoverImage", &info.Coverpage[i], i)
		}
	}
	for i := range info.Translators {
		tw.author(depth+1, "Translator", &info.Translators[i], i)
	}
	for i := range info.Sequences {
		tw.sequence(depth+1, &info.Sequences[i], i)
	}
}

func (tw treeWriter) author(depth int, label string, author *Author, index int) {
	if author == nil {
		return
	}
	tw.Line(depth, "%s[%d] first=%q middle=%q last=%q nickname=%q id=%q", label, index, author.FirstName, author.MiddleName, author.LastName, author.Nickname, author.ID)
	for i := range author.HomePages {
		tw.Line(depth+1, "HomePage[%d]=%q", i, author.HomePages[i])
	}
	for i := range author.Emails {
		tw.Line(depth+1, "Email[%d]=%q", i, author.Emails[i])
	}
}

func (tw treeWriter) documentInfo(depth int, info *DocumentInfo) {
	if info == nil {
		return
	}
	tw.Line(depth, "DocumentInfo id=%q version=%q", info.ID, info.Version)
	for i := range info.Authors {
		tw.author(depth+1, "Author", &info.Authors[i], i)
	}
	if info.ProgramUsed != nil {
		tw.TextBlock(depth+1, "ProgramUsed", info.ProgramUsed.Value)
	}
	tw.Line(depth+1, "Date value=%q display=%q", info.Date.Value, info.Date.Display)
	for i := range info.SourceURLs {
		tw.Line(depth+1, "SourceURL[%d]=%q", i, info.SourceURLs[i])
	}
	if info.SourceOCR != nil {
		tw.TextBlock(depth+1, "SourceOCR", info.SourceOCR.Value)
	}
	if info.History != nil {
		tw.Line(depth+1, "History")
		tw.flow(depth+2, info.History)
	}
	for i := range info.Publishers {
		tw.author(depth+1, "Publisher", &info.Publishers[i], i)
	}
}

func (tw treeWriter) publishInfo(depth int, info *PublishInfo) {
	if info == nil {
		return
	}
	tw.Line(depth, "PublishInfo year=%q", info.Year)
	if info.BookName != nil {
		tw.TextBlock(depth+1, "BookName", info.BookName.Value)
	}
	if info.Publisher != nil {
		tw.TextBlock(depth+1, "Publisher", info.Publisher.Value)
	}
	if info.City != nil {
		tw.TextBlock(depth+1, "City", info.City.Value)
	}
	if info.ISBN != nil {
		tw.TextBlock(depth+1, "ISBN", info.ISBN.Value)
	}
	for i := range info.Sequences {
		tw.sequence(depth+1, &info.Sequences[i], i)
	}
}

func (tw treeWriter) sequence(depth int, seq *Sequence, index int) {
	if seq == nil {
		return
	}
	num := ""
	if seq.Number != nil {
		num = strconv.Itoa(*seq.Number)
	}
	tw.Line(depth, "Sequence[%d] name=%q number=%q lang=%q", index, seq.Name, num, seq.Lang)
	for i := range seq.Children {
		tw.sequence(depth+1, &seq.Children[i], i)
	}
}

func (tw treeWriter) outputInstruction(depth int, out *OutputInstruction, index int) {
	if out == nil {
		return
	}
	directive := string(out.IncludeAll)
	price := ""
	if out.Price != nil {
		price = fmt.Sprintf("%.2f", *out.Price)
	}
	tw.Line(depth, "Output[%d] mode=%q includeAll=%q currency=%q price=%q", index, out.Mode, directive, out.Currency, price)
	for i := range out.Parts {
		part := out.Parts[i]
		tw.Line(depth+1, "Part[%d] href=%q include=%q", i, part.Href, part.Include)
	}
	for i := range out.Documents {
		doc := out.Documents[i]
		create := ""
		if doc.Create != nil {
			create = string(*doc.Create)
		}
		priceDoc := ""
		if doc.Price != nil {
			priceDoc = fmt.Sprintf("%.2f", *doc.Price)
		}
		tw.Line(depth+1, "Document[%d] name=%q create=%q price=%q", i, doc.Name, create, priceDoc)
		for j := range doc.Parts {
			part := doc.Parts[j]
			tw.Line(depth+2, "Part[%d] href=%q include=%q", j, part.Href, part.Include)
		}
	}
}

func (tw treeWriter) body(depth int, body *Body, index int) {
	if body == nil {
		return
	}
	tw.Line(depth, "Body[%d] name=%q lang=%q kind=%q", index, body.Name, body.Lang, body.Kind)
	if body.Image != nil {
		tw.image(depth+1, "Image", body.Image)
	}
	if body.Title != nil {
		tw.title(depth+1, "Title", body.Title)
	}
	for i := range body.Epigraphs {
		tw.epigraph(depth+1, &body.Epigraphs[i], i)
	}
	for i := range body.Sections {
		tw.section(depth+1, &body.Sections[i], i)
	}
}

func (tw treeWriter) title(depth int, label string, title *Title) {
	if title == nil {
		return
	}
	tw.Line(depth, "%s lang=%q", label, title.Lang)
	for i := range title.Items {
		item := title.Items[i]
		if item.Paragraph != nil {
			tw.paragraph(depth+1, fmt.Sprintf("Paragraph[%d]", i), item.Paragraph)
		} else if item.EmptyLine {
			tw.Line(depth+1, "EmptyLine[%d]", i)
		}
	}
}

func (tw treeWriter) epigraph(depth int, epi *Epigraph, index int) {
	if epi == nil {
		return
	}
	tw.Line(depth, "Epigraph[%d]", index)
	tw.flow(depth+1, &epi.Flow)
	for i := range epi.TextAuthors {
		tw.paragraph(depth+1, fmt.Sprintf("TextAuthor[%d]", i), &epi.TextAuthors[i])
	}
}

func (tw treeWriter) section(depth int, section *Section, index int) {
	if section == nil {
		return
	}
	tw.Line(depth, "Section[%d] id=%q lang=%q", index, section.ID, section.Lang)
	if section.Title != nil {
		tw.title(depth+1, "Title", section.Title)
	}
	for i := range section.Epigraphs {
		tw.epigraph(depth+1, &section.Epigraphs[i], i)
	}
	if section.Image != nil {
		tw.image(depth+1, "Image", section.Image)
	}
	if section.Annotation != nil {
		tw.Line(depth+1, "Annotation")
		tw.flow(depth+2, section.Annotation)
	}
	for i := range section.Content {
		tw.flowItem(depth+1, &section.Content[i], i)
	}
}

func (tw treeWriter) paragraph(depth int, label string, p *Paragraph) {
	if p == nil {
		return
	}
	tw.Line(depth, "%s id=%q lang=%q style=%q special=%t", label, p.ID, p.Lang, p.Style, p.Special)
	if len(p.Text) == 0 {
		return
	}
	tw.Line(depth+1, "Segments")
	for i := range p.Text {
		tw.inlineSegment(depth+2, &p.Text[i], i)
	}
}

func (tw treeWriter) inlineSegment(depth int, seg *InlineSegment, index int) {
	if seg == nil {
		return
	}
	switch seg.Kind {
	case InlineText:
		tw.TextBlock(depth, fmt.Sprintf("Text[%d]", index), seg.Text)
	case InlineImageSegment:
		if seg.Image != nil {
			tw.inlineImage(depth, "InlineImage", seg.Image, index)
		} else {
			tw.Line(depth, "InlineImage[%d] (missing)", index)
		}
	default:
		tw.Line(depth, "Inline[%d] kind=%q name=%q style=%q href=%q type=%q", index, seg.Kind, seg.Name, seg.Style, seg.Href, seg.LinkType)
		for i := range seg.Children {
			tw.inlineSegment(depth+1, &seg.Children[i], i)
		}
	}
}

func (tw treeWriter) inlineImage(depth int, label string, img *InlineImage, index int) {
	if img == nil {
		return
	}
	tw.Line(depth, "%s[%d] href=%q type=%q alt=%q", label, index, img.Href, img.Type, img.Alt)
}

func (tw treeWriter) image(depth int, label string, img *Image) {
	if img == nil {
		return
	}
	tw.Line(depth, "%s href=%q type=%q alt=%q title=%q id=%q", label, img.Href, img.Type, img.Alt, img.Title, img.ID)
}

func (tw treeWriter) flow(depth int, flow *Flow) {
	if flow == nil {
		return
	}
	tw.Line(depth, "Flow id=%q lang=%q", flow.ID, flow.Lang)
	for i := range flow.Items {
		tw.flowItem(depth+1, &flow.Items[i], i)
	}
}

func (tw treeWriter) flowItem(depth int, item *FlowItem, index int) {
	if item == nil {
		return
	}
	switch item.Kind {
	case FlowParagraph:
		tw.paragraph(depth, fmt.Sprintf("Paragraph[%d]", index), item.Paragraph)
	case FlowPoem:
		if item.Poem != nil {
			tw.poem(depth, item.Poem, index)
		}
	case FlowCite:
		if item.Cite != nil {
			tw.cite(depth, item.Cite, index)
		}
	case FlowSubtitle:
		if item.Subtitle != nil {
			tw.paragraph(depth, fmt.Sprintf("Subtitle[%d]", index), item.Subtitle)
		}
	case FlowEmptyLine:
		tw.Line(depth, "EmptyLine[%d]", index)
	case FlowTable:
		if item.Table != nil {
			tw.table(depth, item.Table, index)
		}
	case FlowImage:
		if item.Image != nil {
			tw.image(depth, fmt.Sprintf("Image[%d]", index), item.Image)
		}
	case FlowSection:
		if item.Section != nil {
			tw.section(depth, item.Section, index)
		}
	default:
		tw.Line(depth, "FlowItem[%d] kind=%q", index, item.Kind)
	}
}

func (tw treeWriter) poem(depth int, poem *Poem, index int) {
	if poem == nil {
		return
	}
	tw.Line(depth, "Poem[%d] id=%q lang=%q", index, poem.ID, poem.Lang)
	if poem.Title != nil {
		tw.title(depth+1, "Title", poem.Title)
	}
	for i := range poem.Epigraphs {
		tw.epigraph(depth+1, &poem.Epigraphs[i], i)
	}
	for i := range poem.Subtitles {
		tw.paragraph(depth+1, fmt.Sprintf("Subtitle[%d]", i), &poem.Subtitles[i])
	}
	for i := range poem.Stanzas {
		tw.stanza(depth+1, &poem.Stanzas[i], i)
	}
	for i := range poem.TextAuthors {
		tw.paragraph(depth+1, fmt.Sprintf("TextAuthor[%d]", i), &poem.TextAuthors[i])
	}
	if poem.Date != nil {
		tw.Line(depth+1, "Date value=%q display=%q", poem.Date.Value, poem.Date.Display)
	}
}

func (tw treeWriter) stanza(depth int, stanza *Stanza, index int) {
	if stanza == nil {
		return
	}
	tw.Line(depth, "Stanza[%d] lang=%q", index, stanza.Lang)
	if stanza.Title != nil {
		tw.title(depth+1, "Title", stanza.Title)
	}
	if stanza.Subtitle != nil {
		tw.paragraph(depth+1, "Subtitle", stanza.Subtitle)
	}
	for i := range stanza.Verses {
		tw.paragraph(depth+1, fmt.Sprintf("Verse[%d]", i), &stanza.Verses[i])
	}
}

func (tw treeWriter) cite(depth int, cite *Cite, index int) {
	if cite == nil {
		return
	}
	tw.Line(depth, "Cite[%d] id=%q lang=%q", index, cite.ID, cite.Lang)
	for i := range cite.Items {
		tw.flowItem(depth+1, &cite.Items[i], i)
	}
	for i := range cite.TextAuthors {
		tw.paragraph(depth+1, fmt.Sprintf("TextAuthor[%d]", i), &cite.TextAuthors[i])
	}
}

func (tw treeWriter) table(depth int, table *Table, index int) {
	if table == nil {
		return
	}
	tw.Line(depth, "Table[%d] id=%q style=%q", index, table.ID, table.Style)
	for i := range table.Rows {
		row := table.Rows[i]
		tw.Line(depth+1, "Row[%d] align=%q", i, row.Align)
		for j := range row.Cells {
			cell := row.Cells[j]
			tw.Line(depth+2, "Cell[%d] header=%t id=%q style=%q colspan=%d rowspan=%d align=%q valign=%q", j, cell.Header, cell.ID, cell.Style, cell.ColSpan, cell.RowSpan, cell.Align, cell.VAlign)
			for k := range cell.Content {
				tw.inlineSegment(depth+3, &cell.Content[k], k)
			}
		}
	}
}

func FormatRefPath(path []any) string {
	if len(path) == 0 {
		return "[]"
	}
	result := "["
	for i, elem := range path {
		if i > 0 {
			result += " -> "
		}
		switch v := elem.(type) {
		case *Body:
			result += fmt.Sprintf("Body{name:%q,kind:%s}", v.Name, v.Kind)
		case *Section:
			result += fmt.Sprintf("Section{id:%q}", v.ID)
		case *Paragraph:
			result += fmt.Sprintf("Paragraph{id:%q}", v.ID)
		case *Flow:
			result += fmt.Sprintf("Flow{id:%q}", v.ID)
		case *FlowItem:
			result += fmt.Sprintf("FlowItem{kind:%s}", v.Kind)
		case *Epigraph:
			result += "Epigraph"
		case *Poem:
			result += fmt.Sprintf("Poem{id:%q}", v.ID)
		case *Cite:
			result += fmt.Sprintf("Cite{id:%q}", v.ID)
		case *Table:
			result += fmt.Sprintf("Table{id:%q}", v.ID)
		case *Image:
			result += fmt.Sprintf("Image{id:%q,href:%q}", v.ID, v.Href)
		case *InlineImage:
			result += fmt.Sprintf("InlineImage{href:%q}", v.Href)
		case *InlineSegment:
			result += fmt.Sprintf("InlineSegment{kind:%s}", v.Kind)
		case *Stanza:
			result += "Stanza"
		case *BinaryObject:
			result += fmt.Sprintf("Binary{id:%q}", v.ID)
		case *Author:
			result += fmt.Sprintf("Author{id:%q,name:%s %s}", v.ID, v.FirstName, v.LastName)
		case *Description:
			result += "Description"
		case *TitleInfo:
			result += "TitleInfo"
		case *DocumentInfo:
			result += "DocumentInfo"
		default:
			result += fmt.Sprintf("%T", v)
		}
	}
	result += "]"
	return result
}
