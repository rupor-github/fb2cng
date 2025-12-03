package fb2

// Clone and deep copy functions for FictionBook structures.
// This is needed to safely manipulate copies of the book, which is very useful
// for debugging and testing.

// clone creates a deep copy of the FictionBook.
func (fb *FictionBook) clone() *FictionBook {
	if fb == nil {
		return nil
	}

	clone := &FictionBook{
		Stylesheets: cloneStylesheets(fb.Stylesheets),
		Description: cloneDescription(&fb.Description),
		Bodies:      cloneBodies(fb.Bodies),
		Binaries:    cloneBinaries(fb.Binaries),
	}

	return clone
}

// Clone helper functions for deep copying

func cloneStylesheets(stylesheets []Stylesheet) []Stylesheet {
	if stylesheets == nil {
		return nil
	}
	result := make([]Stylesheet, len(stylesheets))
	copy(result, stylesheets)
	return result
}

func cloneBinaries(binaries []BinaryObject) []BinaryObject {
	if binaries == nil {
		return nil
	}
	result := make([]BinaryObject, len(binaries))
	for i := range binaries {
		result[i] = BinaryObject{
			ContentType: binaries[i].ContentType,
			ID:          binaries[i].ID,
			Data:        append([]byte(nil), binaries[i].Data...),
		}
	}
	return result
}

func cloneDescription(d *Description) Description {
	return Description{
		TitleInfo:    cloneTitleInfo(&d.TitleInfo),
		SrcTitleInfo: cloneTitleInfoPtr(d.SrcTitleInfo),
		DocumentInfo: cloneDocumentInfo(&d.DocumentInfo),
		PublishInfo:  clonePublishInfoPtr(d.PublishInfo),
		CustomInfo:   cloneCustomInfos(d.CustomInfo),
		Output:       cloneOutputInstructions(d.Output),
	}
}

func cloneTitleInfo(ti *TitleInfo) TitleInfo {
	return TitleInfo{
		Genres:      cloneGenreRefs(ti.Genres),
		Authors:     cloneAuthors(ti.Authors),
		BookTitle:   ti.BookTitle,
		Annotation:  cloneFlowPtr(ti.Annotation),
		Keywords:    cloneTextFieldPtr(ti.Keywords),
		Date:        cloneDatePtr(ti.Date),
		Coverpage:   cloneInlineImages(ti.Coverpage),
		Lang:        ti.Lang,
		SrcLang:     ti.SrcLang,
		Translators: cloneAuthors(ti.Translators),
		Sequences:   cloneSequences(ti.Sequences),
	}
}

func cloneTitleInfoPtr(ti *TitleInfo) *TitleInfo {
	if ti == nil {
		return nil
	}
	result := cloneTitleInfo(ti)
	return &result
}

func cloneDocumentInfo(di *DocumentInfo) DocumentInfo {
	return DocumentInfo{
		Authors:     cloneAuthors(di.Authors),
		ProgramUsed: cloneTextFieldPtr(di.ProgramUsed),
		Date:        di.Date,
		SourceURLs:  cloneStringSlice(di.SourceURLs),
		SourceOCR:   cloneTextFieldPtr(di.SourceOCR),
		ID:          di.ID,
		Version:     di.Version,
		History:     cloneFlowPtr(di.History),
		Publishers:  cloneAuthors(di.Publishers),
	}
}

func clonePublishInfoPtr(pi *PublishInfo) *PublishInfo {
	if pi == nil {
		return nil
	}
	result := PublishInfo{
		BookName:  pi.BookName,
		Publisher: cloneTextFieldPtr(pi.Publisher),
		City:      cloneTextFieldPtr(pi.City),
		Year:      pi.Year,
		ISBN:      cloneTextFieldPtr(pi.ISBN),
		Sequences: cloneSequences(pi.Sequences),
	}
	return &result
}

func cloneCustomInfos(customs []CustomInfo) []CustomInfo {
	if customs == nil {
		return nil
	}
	result := make([]CustomInfo, len(customs))
	copy(result, customs)
	return result
}

func cloneOutputInstructions(outputs []OutputInstruction) []OutputInstruction {
	if outputs == nil {
		return nil
	}
	result := make([]OutputInstruction, len(outputs))
	for i := range outputs {
		result[i] = OutputInstruction{
			Mode:       outputs[i].Mode,
			IncludeAll: outputs[i].IncludeAll,
			Parts:      clonePartInstructions(outputs[i].Parts),
			Documents:  cloneOutputDocuments(outputs[i].Documents),
		}
	}
	return result
}

func clonePartInstructions(parts []PartInstruction) []PartInstruction {
	if parts == nil {
		return nil
	}
	result := make([]PartInstruction, len(parts))
	copy(result, parts)
	return result
}

func cloneOutputDocuments(docs []OutputDocument) []OutputDocument {
	if docs == nil {
		return nil
	}
	result := make([]OutputDocument, len(docs))
	copy(result, docs)
	return result
}

func cloneGenreRefs(genres []GenreRef) []GenreRef {
	if genres == nil {
		return nil
	}
	result := make([]GenreRef, len(genres))
	copy(result, genres)
	return result
}

func cloneAuthors(authors []Author) []Author {
	if authors == nil {
		return nil
	}
	result := make([]Author, len(authors))
	for i := range authors {
		result[i] = Author{
			FirstName:  authors[i].FirstName,
			MiddleName: authors[i].MiddleName,
			LastName:   authors[i].LastName,
			Nickname:   authors[i].Nickname,
			HomePages:  cloneStringSlice(authors[i].HomePages),
			Emails:     cloneStringSlice(authors[i].Emails),
			ID:         authors[i].ID,
		}
	}
	return result
}

func cloneTextFieldPtr(tf *TextField) *TextField {
	if tf == nil {
		return nil
	}
	result := *tf
	return &result
}

func cloneDatePtr(d *Date) *Date {
	if d == nil {
		return nil
	}
	result := *d
	return &result
}

func cloneInlineImages(images []InlineImage) []InlineImage {
	if images == nil {
		return nil
	}
	result := make([]InlineImage, len(images))
	copy(result, images)
	return result
}

func cloneSequences(seqs []Sequence) []Sequence {
	if seqs == nil {
		return nil
	}
	result := make([]Sequence, len(seqs))
	for i := range seqs {
		var num *int
		if seqs[i].Number != nil {
			n := *seqs[i].Number
			num = &n
		}
		result[i] = Sequence{
			Name:     seqs[i].Name,
			Number:   num,
			Lang:     seqs[i].Lang,
			Children: cloneSequences(seqs[i].Children),
		}
	}
	return result
}

func cloneStringSlice(strs []string) []string {
	if strs == nil {
		return nil
	}
	result := make([]string, len(strs))
	copy(result, strs)
	return result
}

func cloneFlowPtr(f *Flow) *Flow {
	if f == nil {
		return nil
	}
	result := cloneFlow(f)
	return &result
}

func cloneFlow(f *Flow) Flow {
	return Flow{
		ID:    f.ID,
		Lang:  f.Lang,
		Items: cloneFlowItems(f.Items),
	}
}

func cloneFlowItems(items []FlowItem) []FlowItem {
	if items == nil {
		return nil
	}
	result := make([]FlowItem, len(items))
	for i := range items {
		result[i] = cloneFlowItem(&items[i])
	}
	return result
}

func cloneFlowItem(item *FlowItem) FlowItem {
	return FlowItem{
		Kind:      item.Kind,
		Paragraph: cloneParagraphPtr(item.Paragraph),
		Image:     cloneImagePtr(item.Image),
		Poem:      clonePoemPtr(item.Poem),
		Subtitle:  cloneParagraphPtr(item.Subtitle),
		Cite:      cloneCitePtr(item.Cite),
		Table:     cloneTablePtr(item.Table),
		Section:   cloneSectionPtr(item.Section),
	}
}

func cloneBodies(bodies []Body) []Body {
	if bodies == nil {
		return nil
	}
	result := make([]Body, len(bodies))
	for i := range bodies {
		result[i] = cloneBody(&bodies[i])
	}
	return result
}

func cloneBody(b *Body) Body {
	return Body{
		Name:      b.Name,
		Lang:      b.Lang,
		Kind:      b.Kind,
		Image:     cloneImagePtr(b.Image),
		Title:     cloneTitlePtr(b.Title),
		Epigraphs: cloneEpigraphs(b.Epigraphs),
		Sections:  cloneSections(b.Sections),
	}
}

func cloneSections(sections []Section) []Section {
	if sections == nil {
		return nil
	}
	result := make([]Section, len(sections))
	for i := range sections {
		result[i] = cloneSection(&sections[i])
	}
	return result
}

func cloneSection(s *Section) Section {
	return Section{
		ID:         s.ID,
		Lang:       s.Lang,
		Title:      cloneTitlePtr(s.Title),
		Epigraphs:  cloneEpigraphs(s.Epigraphs),
		Image:      cloneImagePtr(s.Image),
		Annotation: cloneFlowPtr(s.Annotation),
		Content:    cloneFlowItems(s.Content),
	}
}

func cloneSectionPtr(s *Section) *Section {
	if s == nil {
		return nil
	}
	result := cloneSection(s)
	return &result
}

func cloneTitlePtr(t *Title) *Title {
	if t == nil {
		return nil
	}
	result := Title{
		Lang:  t.Lang,
		Items: cloneTitleItems(t.Items),
	}
	return &result
}

func cloneTitleItems(items []TitleItem) []TitleItem {
	if items == nil {
		return nil
	}
	result := make([]TitleItem, len(items))
	for i := range items {
		result[i] = TitleItem{
			Paragraph: cloneParagraphPtr(items[i].Paragraph),
			EmptyLine: items[i].EmptyLine,
		}
	}
	return result
}

func cloneEpigraphs(epigraphs []Epigraph) []Epigraph {
	if epigraphs == nil {
		return nil
	}
	result := make([]Epigraph, len(epigraphs))
	for i := range epigraphs {
		result[i] = Epigraph{
			Flow:        cloneFlow(&epigraphs[i].Flow),
			TextAuthors: cloneParagraphs(epigraphs[i].TextAuthors),
		}
	}
	return result
}

func cloneImagePtr(img *Image) *Image {
	if img == nil {
		return nil
	}
	result := *img
	return &result
}

func cloneParagraphPtr(p *Paragraph) *Paragraph {
	if p == nil {
		return nil
	}
	result := cloneParagraph(p)
	return &result
}

func cloneParagraph(p *Paragraph) Paragraph {
	return Paragraph{
		ID:      p.ID,
		Lang:    p.Lang,
		Style:   p.Style,
		Special: p.Special,
		Text:    cloneInlineSegments(p.Text),
	}
}

func cloneParagraphs(paragraphs []Paragraph) []Paragraph {
	if paragraphs == nil {
		return nil
	}
	result := make([]Paragraph, len(paragraphs))
	for i := range paragraphs {
		result[i] = cloneParagraph(&paragraphs[i])
	}
	return result
}

func cloneInlineSegments(segments []InlineSegment) []InlineSegment {
	if segments == nil {
		return nil
	}
	result := make([]InlineSegment, len(segments))
	for i := range segments {
		result[i] = cloneInlineSegment(&segments[i])
	}
	return result
}

func cloneInlineSegment(seg *InlineSegment) InlineSegment {
	return InlineSegment{
		Kind:     seg.Kind,
		Text:     seg.Text,
		Lang:     seg.Lang,
		Name:     seg.Name,
		Style:    seg.Style,
		Href:     seg.Href,
		LinkType: seg.LinkType,
		Children: cloneInlineSegments(seg.Children),
		Image:    cloneInlineImagePtr(seg.Image),
	}
}

func cloneInlineImagePtr(img *InlineImage) *InlineImage {
	if img == nil {
		return nil
	}
	result := *img
	return &result
}

func clonePoemPtr(p *Poem) *Poem {
	if p == nil {
		return nil
	}
	result := Poem{
		ID:          p.ID,
		Lang:        p.Lang,
		Title:       cloneTitlePtr(p.Title),
		Epigraphs:   cloneEpigraphs(p.Epigraphs),
		Stanzas:     cloneStanzas(p.Stanzas),
		TextAuthors: cloneParagraphs(p.TextAuthors),
		Date:        cloneDatePtr(p.Date),
	}
	return &result
}

func cloneStanzas(stanzas []Stanza) []Stanza {
	if stanzas == nil {
		return nil
	}
	result := make([]Stanza, len(stanzas))
	for i := range stanzas {
		result[i] = Stanza{
			Lang:     stanzas[i].Lang,
			Title:    cloneTitlePtr(stanzas[i].Title),
			Subtitle: cloneParagraphPtr(stanzas[i].Subtitle),
			Verses:   cloneParagraphs(stanzas[i].Verses),
		}
	}
	return result
}

func cloneCitePtr(c *Cite) *Cite {
	if c == nil {
		return nil
	}
	result := Cite{
		ID:          c.ID,
		Lang:        c.Lang,
		Items:       cloneFlowItems(c.Items),
		TextAuthors: cloneParagraphs(c.TextAuthors),
	}
	return &result
}

func cloneTablePtr(t *Table) *Table {
	if t == nil {
		return nil
	}
	result := Table{
		ID:    t.ID,
		Style: t.Style,
		Rows:  cloneTableRows(t.Rows),
	}
	return &result
}

func cloneTableRows(rows []TableRow) []TableRow {
	if rows == nil {
		return nil
	}
	result := make([]TableRow, len(rows))
	for i := range rows {
		result[i] = TableRow{
			Align: rows[i].Align,
			Cells: cloneTableCells(rows[i].Cells),
		}
	}
	return result
}

func cloneTableCells(cells []TableCell) []TableCell {
	if cells == nil {
		return nil
	}
	result := make([]TableCell, len(cells))
	for i := range cells {
		result[i] = TableCell{
			Header:  cells[i].Header,
			ID:      cells[i].ID,
			Style:   cells[i].Style,
			ColSpan: cells[i].ColSpan,
			RowSpan: cells[i].RowSpan,
			Align:   cells[i].Align,
			VAlign:  cells[i].VAlign,
			Content: cloneInlineSegments(cells[i].Content),
		}
	}
	return result
}
