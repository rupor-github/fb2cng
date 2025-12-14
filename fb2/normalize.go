package fb2

import (
	_ "embed"
	"fmt"
	"maps"
	"strings"
	"unicode"

	"go.uber.org/zap"

	"fbc/config"
)

var notFoundImage = []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 200 200">
  <title>Not found image</title>

  <!-- background -->
  <rect x="0" y="0" width="200" height="200" fill="#ffffff"/>

  <!-- border -->
  <rect x="6" y="6" width="188" height="188" fill="none" stroke="#333" stroke-width="4" rx="4" ry="4"/>

  <!-- centered text -->
  <text x="100" y="100" text-anchor="middle" dominant-baseline="middle"
        font-family="Helvetica, Arial, sans-serif" font-weight="700"
        font-size="20" fill="#E60000">IMAGE NOT FOUND</text>
</svg>`)

// Normalization functions for footnotes and links.

// NormalizeFootnoteBodies walks all bodies of the book and normalizes any
// footnotes body by replacing its Sections slice with the flattened result
// produced by Body.normalizeFootnotes(). Non-footnote bodies are left
// untouched.
// This returns a new FictionBook with normalized bodies and the footnote index.
// The returned FictionBook is a deep copy, so the original remains unchanged.
func (fb *FictionBook) NormalizeFootnoteBodies(log *zap.Logger) (*FictionBook, FootnoteRefs) {
	result := fb.clone()
	for i := range result.Bodies {
		if result.Bodies[i].Footnotes() {
			result.Bodies[i] = result.Bodies[i].normalizeFootnotes(log)
		}
	}

	// Build footnote index from the normalized bodies
	footnotesIndex := result.buildFootnotesIndex(log)

	return result, footnotesIndex
}

// normalizeFootnotes processes a footnotes body to ensure proper structure.
// Each top-level section with an ID becomes a standalone footnote section.
// Sections without IDs have their metadata promoted to the Body level, and
// their nested sections are extracted. Nested sections are flattened into the
// parent's content. Each footnote section is guaranteed to have a non-empty
// title. Mutates the Body in place.
func (b *Body) normalizeFootnotes(log *zap.Logger) Body {
	if !b.Footnotes() {
		return *b
	}

	var (
		normalized []Section
		idsSeen    = make(map[string]struct{})
	)

	result := *b

	for _, section := range b.Sections {
		// If section has no ID, promote its metadata to Body level and extract nested sections
		if section.ID == "" {
			log.Debug("Footnote section without ID during normalization, extracting nested sections")

			// Promote wrapper section metadata to Body level if Body doesn't have it
			if result.Title == nil && section.Title != nil {
				result.Title = section.Title
			}
			if result.Image == nil && section.Image != nil {
				result.Image = section.Image
			}
			if len(result.Epigraphs) == 0 && len(section.Epigraphs) > 0 {
				result.Epigraphs = section.Epigraphs
			}

			// Extract nested sections
			normalized = append(normalized, extractFootnoteSections(section.Content, idsSeen, log)...)
			continue
		}

		normalized = append(normalized, normalizeFootnoteSection(&section, idsSeen, log)...)
	}

	result.Sections = normalized
	return result
}

// normalizeFootnoteSection processes a section with an ID and returns a normalized footnote section.
// Assumes the section has an ID (caller must check).
func normalizeFootnoteSection(section *Section, idsSeen map[string]struct{}, log *zap.Logger) []Section {
	// Skip duplicate IDs
	if _, exists := idsSeen[section.ID]; exists {
		log.Warn("Duplicate footnote ID detected during normalization, skipping", zap.String("id", section.ID))
		return nil
	}
	idsSeen[section.ID] = struct{}{}

	// Create normalized footnote section
	note := Section{
		ID:         section.ID,
		Lang:       section.Lang,
		Title:      footnoteTitle(section.Title, section.ID, section.Lang),
		Epigraphs:  section.Epigraphs,
		Image:      section.Image,
		Annotation: section.Annotation,
		Content:    flattenSectionContent(section.Content),
	}

	return []Section{note}
}

// extractFootnoteSections recursively extracts sections with IDs from content items
func extractFootnoteSections(items []FlowItem, idsSeen map[string]struct{}, log *zap.Logger) []Section {
	var sections []Section

	for _, item := range items {
		if item.Kind == FlowSection && item.Section != nil {
			nested := item.Section

			// If nested section has no ID, recurse deeper
			if nested.ID == "" {
				sections = append(sections, extractFootnoteSections(nested.Content, idsSeen, log)...)
				continue
			}

			sections = append(sections, normalizeFootnoteSection(nested, idsSeen, log)...)
		}
	}

	return sections
}

// isTitleEmpty returns true if the title has no non-whitespace textual content.
func isTitleEmpty(t *Title) bool {
	if t == nil {
		return true
	}
	for _, item := range t.Items {
		if item.Paragraph == nil {
			continue
		}
		for _, seg := range item.Paragraph.Text {
			if strings.TrimSpace(seg.Text) != "" {
				return false
			}
		}
	}
	return true
}

// footnoteTitle returns a non-empty title for a footnote section.
// If the original title is nil or empty, it fabricates one using the section ID.
func footnoteTitle(orig *Title, id, lang string) *Title {
	if !isTitleEmpty(orig) {
		return orig
	}
	para := &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: "~ " + id + " ~"}}}
	return &Title{Lang: lang, Items: []TitleItem{{Paragraph: para}}}
}

// NormalizeFootnoteLabels renumbers footnotes and updates their titles and link text.
// For floatRenumbered mode, it assigns sequential numbers to each footnote within
// each body and updates:
// 1. The FootnoteRefs index with BodyNum, NoteNum, and DisplayText
// 2. Footnote section titles to use the formatted label
// 3. Link text in main body content that references footnotes
// Returns a new FictionBook with updated labels. The original remains unchanged.
func (fb *FictionBook) NormalizeFootnoteLabels(footnotesIndex FootnoteRefs, template string, log *zap.Logger) (*FictionBook, FootnoteRefs) {
	result := fb.clone()
	updatedIndex := make(FootnoteRefs, len(footnotesIndex))

	// Count total footnote bodies first
	totalFootnoteBodies := 0
	for i := range result.Bodies {
		if result.Bodies[i].Footnotes() {
			totalFootnoteBodies++
		}
	}

	// First pass: compute numbering for all footnotes
	bodyNumCounter := 0
	for i := range result.Bodies {
		if !result.Bodies[i].Footnotes() {
			continue
		}
		bodyNumCounter++

		for j := range result.Bodies[i].Sections {
			section := &result.Bodies[i].Sections[j]
			if section.ID == "" {
				continue
			}

			noteNum := j + 1

			// Use 0 for template expansion if there's only one footnote body
			templateBodyNum := bodyNumCounter
			if totalFootnoteBodies == 1 {
				templateBodyNum = 0
			}

			displayText, err := fb.ExpandTemplateFootnoteLabel(config.LabelTemplateFieldName, template, templateBodyNum, noteNum)
			if err != nil {
				log.Warn("Failed to expand footnote label template, using default formatter",
					zap.Int("body", templateBodyNum),
					zap.Int("note", noteNum),
					zap.Error(err))
				displayText = fmt.Sprintf("%d.%d", templateBodyNum, noteNum)
			}

			// Update the index with numbering info (keep original bodyNumCounter)
			updatedIndex[section.ID] = FootnoteRef{
				BodyIdx:     i,
				SectionIdx:  j,
				BodyNum:     bodyNumCounter,
				NoteNum:     noteNum,
				DisplayText: displayText,
			}

			// Update footnote section title
			section.Title = createFootnoteLabelTitle(displayText, section.Lang)

			log.Debug("Renumbered footnote",
				zap.String("id", section.ID),
				zap.String("label", displayText))
		}
	}

	// Second pass: update link text everywhere

	// Update links in TitleInfo annotation
	if result.Description.TitleInfo.Annotation != nil {
		updateFootnoteLinksInFlow(result.Description.TitleInfo.Annotation, updatedIndex)
	}

	// Update links in all bodies (including footnote bodies for cross-references)
	for i := range result.Bodies {
		// Update links in body epigraphs
		for j := range result.Bodies[i].Epigraphs {
			updateFootnoteLinksInEpigraph(&result.Bodies[i].Epigraphs[j], updatedIndex)
		}

		// Update links in sections
		for j := range result.Bodies[i].Sections {
			updateFootnoteLinksInSection(&result.Bodies[i].Sections[j], updatedIndex)
		}
	}

	return result, updatedIndex
}

// createFootnoteLabelTitle creates a title with the formatted label text.
func createFootnoteLabelTitle(label, lang string) *Title {
	para := &Paragraph{Text: []InlineSegment{{Kind: InlineText, Text: label}}}
	return &Title{Lang: lang, Items: []TitleItem{{Paragraph: para}}}
}

// updateFootnoteLinksInEpigraph updates footnote link text in an epigraph.
func updateFootnoteLinksInEpigraph(epigraph *Epigraph, index FootnoteRefs) {
	updateFootnoteLinksInFlow(&epigraph.Flow, index)
	for i := range epigraph.TextAuthors {
		updateFootnoteLinksInSegments(epigraph.TextAuthors[i].Text, index)
	}
}

// updateFootnoteLinksInSection recursively updates footnote link text in a section.
func updateFootnoteLinksInSection(section *Section, index FootnoteRefs) {
	// Update links in title
	updateFootnoteLinksInTitle(section.Title, index)

	// Update links in epigraphs
	for i := range section.Epigraphs {
		updateFootnoteLinksInEpigraph(&section.Epigraphs[i], index)
	}

	// Update links in annotation
	if section.Annotation != nil {
		updateFootnoteLinksInFlow(section.Annotation, index)
	}

	// Update links in content
	for i := range section.Content {
		updateFootnoteLinksInFlowItem(&section.Content[i], index)
	}
}

// updateFootnoteLinksInFlow updates footnote link text in a flow.
func updateFootnoteLinksInFlow(flow *Flow, index FootnoteRefs) {
	for i := range flow.Items {
		updateFootnoteLinksInFlowItem(&flow.Items[i], index)
	}
}

// updateFootnoteLinksInFlowItem updates footnote link text in a flow item.
func updateFootnoteLinksInFlowItem(item *FlowItem, index FootnoteRefs) {
	switch item.Kind {
	case FlowParagraph:
		if item.Paragraph != nil {
			updateFootnoteLinksInSegments(item.Paragraph.Text, index)
		}
	case FlowSubtitle:
		if item.Subtitle != nil {
			updateFootnoteLinksInSegments(item.Subtitle.Text, index)
		}
	case FlowPoem:
		if item.Poem != nil {
			updateFootnoteLinksInTitle(item.Poem.Title, index)
			for i := range item.Poem.Epigraphs {
				updateFootnoteLinksInEpigraph(&item.Poem.Epigraphs[i], index)
			}
			for i := range item.Poem.Subtitles {
				updateFootnoteLinksInSegments(item.Poem.Subtitles[i].Text, index)
			}
			for i := range item.Poem.Stanzas {
				updateFootnoteLinksInTitle(item.Poem.Stanzas[i].Title, index)
				if item.Poem.Stanzas[i].Subtitle != nil {
					updateFootnoteLinksInSegments(item.Poem.Stanzas[i].Subtitle.Text, index)
				}
				for j := range item.Poem.Stanzas[i].Verses {
					updateFootnoteLinksInSegments(item.Poem.Stanzas[i].Verses[j].Text, index)
				}
			}
			for i := range item.Poem.TextAuthors {
				updateFootnoteLinksInSegments(item.Poem.TextAuthors[i].Text, index)
			}
		}
	case FlowCite:
		if item.Cite != nil {
			for i := range item.Cite.Items {
				updateFootnoteLinksInFlowItem(&item.Cite.Items[i], index)
			}
			for i := range item.Cite.TextAuthors {
				updateFootnoteLinksInSegments(item.Cite.TextAuthors[i].Text, index)
			}
		}
	case FlowTable:
		if item.Table != nil {
			for i := range item.Table.Rows {
				for j := range item.Table.Rows[i].Cells {
					updateFootnoteLinksInSegments(item.Table.Rows[i].Cells[j].Content, index)
				}
			}
		}
	case FlowSection:
		if item.Section != nil {
			updateFootnoteLinksInSection(item.Section, index)
		}
	}
}

// updateFootnoteLinksInTitle updates footnote link text in a title.
func updateFootnoteLinksInTitle(title *Title, index FootnoteRefs) {
	if title == nil {
		return
	}
	for i := range title.Items {
		if title.Items[i].Paragraph != nil {
			updateFootnoteLinksInSegments(title.Items[i].Paragraph.Text, index)
		}
	}
}

// updateFootnoteLinksInSegments updates footnote link text in inline segments.
func updateFootnoteLinksInSegments(segments []InlineSegment, index FootnoteRefs) {
	for i := range segments {
		seg := &segments[i]

		if seg.Kind == InlineLink && seg.Href != "" {
			// Check if this is a footnote link
			if targetID, ok := strings.CutPrefix(seg.Href, "#"); ok {
				if ref, isFootnote := index[targetID]; isFootnote && ref.DisplayText != "" {
					// Replace link children with the formatted label
					seg.Children = []InlineSegment{{
						Kind: InlineText,
						Text: ref.DisplayText,
					}}
				}
			}
		}

		// Recursively process children
		if len(seg.Children) > 0 {
			updateFootnoteLinksInSegments(seg.Children, index)
		}
	}
}

// flattenSectionContent recursively flattens nested sections into a single content flow.
// Nested section metadata (title, epigraphs, etc.) is converted to flow items,
// and the section's content is merged into the parent flow.
func flattenSectionContent(items []FlowItem) []FlowItem {
	var flattened []FlowItem

	for _, item := range items {
		if item.Kind == FlowSection && item.Section != nil {
			// Found a nested section - flatten it
			nested := item.Section

			// Convert section title to subtitle if present
			if nested.Title != nil {
				for _, titleItem := range nested.Title.Items {
					if titleItem.Paragraph != nil {
						flattened = append(flattened, FlowItem{
							Kind:     FlowSubtitle,
							Subtitle: titleItem.Paragraph,
						})
					} else if titleItem.EmptyLine {
						flattened = append(flattened, FlowItem{
							Kind: FlowEmptyLine,
						})
					}
				}
			}

			// Add section image if present
			if nested.Image != nil {
				flattened = append(flattened, FlowItem{
					Kind:  FlowImage,
					Image: nested.Image,
				})
			}

			// Add section epigraphs
			for i := range nested.Epigraphs {
				// Create a cite from epigraph flow
				cite := Cite{
					ID:          nested.Epigraphs[i].Flow.ID,
					Lang:        nested.Epigraphs[i].Flow.Lang,
					Items:       nested.Epigraphs[i].Flow.Items,
					TextAuthors: nested.Epigraphs[i].TextAuthors,
				}
				flattened = append(flattened, FlowItem{
					Kind: FlowCite,
					Cite: &cite,
				})
			}

			// Add section annotation as regular flow
			if nested.Annotation != nil {
				flattened = append(flattened, nested.Annotation.Items...)
			}

			// Recursively flatten the nested section's content
			flattened = append(flattened, flattenSectionContent(nested.Content)...)
		} else {
			// Regular flow item - keep as is
			flattened = append(flattened, item)
		}
	}

	return flattened
}

// NormalizeLinks validates all links and replaces broken ones with text or
// points broken image links to notFoundImage. Processes vignettes and adds
// them to binaries with unique IDs. Returns a new FictionBook with broken
// links replaced, along with corrected ID and link indexes. The returned
// indexes reflect the state after link replacements. The original remains
// unchanged.
func (fb *FictionBook) NormalizeLinks(vignettes map[config.VignettePos]*BinaryObject, log *zap.Logger) (*FictionBook, IDIndex, ReverseLinkIndex) {
	result := fb.clone()

	// Rebuild indexes for the cloned book since the original indexes reference the original book's pointers
	resultIDs := result.buildIDIndex(log)

	// Initialize NotFoundImageID with a unique value
	counter := 0
	for {
		candidateID := fmt.Sprintf("not-found-%d", counter)
		if _, exists := resultIDs[candidateID]; !exists {
			result.NotFoundImageID = candidateID
			break
		}
		counter++
	}

	// Process vignettes: create unique IDs for enabled decorations and add
	// them to book binaries
	// NOTE: we do not have to follow the same logic as for non found image
	// since we are sure that those will be used, so we always need them
	result.VignetteIDs = make(map[config.VignettePos]string)
	for pos, v := range vignettes {
		// Generate unique ID for this vignette position
		counter := 0
		for {
			candidateID := fmt.Sprintf("%s-%d", pos.String(), counter)
			if _, exists := resultIDs[candidateID]; !exists {
				result.VignetteIDs[pos] = candidateID
				// Add to binaries
				result.Binaries = append(result.Binaries, BinaryObject{
					ID:          candidateID,
					ContentType: v.ContentType,
					Data:        v.Data,
				})
				break
			}
			counter++
		}
	}

	resultLinks := result.buildReverseLinkIndex(log)

	for targetID, refs := range resultLinks {
		// Check the type of link
		linkType := refs[0].Type // All refs for same target should have same type

		switch linkType {
		case "external-link":
			// Valid external links - leave them alone
			continue

		case "empty-href-link":
			// Empty href - replace with text
			for _, ref := range refs {
				log.Warn("Link with empty href detected", zap.String("location", FormatRefPath(ref.Path)))
				if result.replaceBrokenLink(ref, "", log) {
					// Ensure not found image binary is present if we replaced an image link
					result.ensureNotFoundImageBinary()
				}
			}

		case "broken-link":
			// Broken external link - replace with text
			for _, ref := range refs {
				log.Warn("Broken external link detected", zap.String("location", FormatRefPath(ref.Path)))
				if result.replaceBrokenLink(ref, targetID, log) {
					// Ensure not found image binary is present if we replaced an image link
					result.ensureNotFoundImageBinary()
				}
			}

		default:
			// Internal link - check if target ID exists
			if _, exists := resultIDs[targetID]; exists {
				// Valid internal link - leave it alone
				continue
			}

			// Broken internal link - replace with text
			for _, ref := range refs {
				log.Warn("Broken internal link detected", zap.String("target", targetID), zap.String("location", FormatRefPath(ref.Path)))
				if result.replaceBrokenLink(ref, targetID, log) {
					// Ensure not found image binary is present if we replaced an image link
					result.ensureNotFoundImageBinary()
				}
			}
		}
	}

	// Rebuild link index after replacements to remove references to replaced links
	resultLinks = result.buildReverseLinkIndex(log)

	return result, resultIDs, resultLinks
}

// NormalizeIDs assigns sequential IDs to all sections that don't have IDs.
// It uses the provided IDIndex to avoid ID collisions with existing IDs.
// Returns a new FictionBook with IDs assigned and an updated IDIndex that includes the generated IDs.
// The original FictionBook remains unchanged.
func (fb *FictionBook) NormalizeIDs(existingIDs IDIndex, log *zap.Logger) (*FictionBook, IDIndex) {
	result := fb.clone()
	// Create a new index with existing IDs
	updatedIDs := make(IDIndex, len(existingIDs))
	maps.Copy(updatedIDs, existingIDs)

	sectionCounter, subtitleCounter := 0, 0
	for i := range result.Bodies {
		bodyPath := []any{&result.Bodies[i]}
		result.assignBodyIDs(&result.Bodies[i], bodyPath, existingIDs, updatedIDs, &sectionCounter, &subtitleCounter, log)
	}

	return result, updatedIDs
}

// assignBodyIDs recursively assigns IDs to sections in a body
func (fb *FictionBook) assignBodyIDs(body *Body, path []any, existingIDs, updatedIDs IDIndex, sectionCounter, subtitleCounter *int, log *zap.Logger) {
	for i := range body.Sections {
		sectionPath := append(append([]any{}, path...), &body.Sections[i])
		fb.assignSectionIDs(&body.Sections[i], sectionPath, existingIDs, updatedIDs, sectionCounter, subtitleCounter, log)
	}
}

// assignSectionIDs recursively assigns IDs to a section and its child sections
func (fb *FictionBook) assignSectionIDs(section *Section, path []any, existingIDs, updatedIDs IDIndex, sectionCounter, subtitleCounter *int, log *zap.Logger) {
	// Assign ID to section if it doesn't have one
	if section.ID == "" {
		// Find a unique ID that doesn't collide
		for {
			*sectionCounter++
			candidateID := fmt.Sprintf("sect_%d", *sectionCounter)
			if _, exists := existingIDs[candidateID]; !exists {
				section.ID = candidateID
				// Add to updated index with special type
				updatedIDs[candidateID] = ElementRef{
					Type: "section-generated",
					Path: path,
				}
				log.Debug("Generated section id", zap.String("ID", candidateID))
				break
			}
		}
	}

	// Process content items
	for i := range section.Content {
		if section.Content[i].Kind == FlowSection && section.Content[i].Section != nil {
			// Recurse into nested sections
			childPath := append(append([]any{}, path...), &section.Content[i], section.Content[i].Section)
			fb.assignSectionIDs(section.Content[i].Section, childPath, existingIDs, updatedIDs, sectionCounter, subtitleCounter, log)
		}
	}
}

// replaceBrokenLink replaces a broken link with text or points broken image links to notFoundImage
func (fb *FictionBook) replaceBrokenLink(ref ElementRef, targetID string, log *zap.Logger) (addedNotFoundImage bool) {
	// Navigate to the element containing the link and replace it
	switch ref.Type {
	case "inline-link", "empty-href-link", "broken-link":
		if len(ref.Path) > 0 {
			if segment, ok := ref.Path[len(ref.Path)-1].(*InlineSegment); ok {
				replacementText := createBrokenLinkText(targetID, segment, ref.Type)
				*segment = InlineSegment{
					Kind: InlineText,
					Text: replacementText,
				}
			}
		}
	case "coverpage":
		// Point coverpage to not found image
		if len(ref.Path) > 0 {
			if img, ok := ref.Path[len(ref.Path)-1].(*InlineImage); ok {
				img.Href = "#" + fb.NotFoundImageID
				log.Debug("Broken coverpage image link redirected to not found image", zap.String("original", targetID))
				addedNotFoundImage = true
			}
		}
	case "block-image":
		// Point block image to not found image
		if len(ref.Path) > 0 {
			if img, ok := ref.Path[len(ref.Path)-1].(*Image); ok {
				img.Href = "#" + fb.NotFoundImageID
				log.Debug("Broken block image link redirected to not found image", zap.String("original", targetID))
				addedNotFoundImage = true
			}
		}
	case "inline-image":
		// Point inline image to not found image
		if len(ref.Path) > 0 {
			if segment, ok := ref.Path[len(ref.Path)-1].(*InlineSegment); ok {
				if segment.Image != nil {
					segment.Image.Href = "#" + fb.NotFoundImageID
					log.Debug("Broken inline image link redirected to not found image", zap.String("original", targetID))
					addedNotFoundImage = true
				}
			}
		}
	}
	return
}

// createBrokenLinkText generates replacement text for a broken link
func createBrokenLinkText(targetID string, segment *InlineSegment, refType string) string {
	// Extract the visible text from the link's children
	linkText := extractLinkText(segment)

	var suffix string
	if targetID == "" || refType == "empty-href-link" {
		// Empty link case
		suffix = "[empty link]"
	} else if refType == "broken-link" {
		// Broken external link case
		suffix = "[broken external link: " + targetID + "]"
	} else {
		// Broken internal link case (inline-link)
		suffix = "[broken link: #" + targetID + "]"
	}

	if linkText == "" {
		return suffix
	}
	return linkText + " " + suffix
}

// extractLinkText recursively extracts text content from inline segments
func extractLinkText(segment *InlineSegment) string {
	if segment == nil {
		return ""
	}

	if segment.Kind == InlineText {
		return segment.Text
	}

	var text string
	for i := range segment.Children {
		text += extractLinkText(&segment.Children[i])
	}
	return text
}

// ensureNotFoundImageBinary adds the not found image binary if it doesn't already exist
func (fb *FictionBook) ensureNotFoundImageBinary() {
	// Check if not found image binary already exists
	for i := range fb.Binaries {
		if fb.Binaries[i].ID == fb.NotFoundImageID {
			return
		}
	}

	// Add not found image binary
	fb.Binaries = append(fb.Binaries, BinaryObject{
		ID:          fb.NotFoundImageID,
		ContentType: "image/svg+xml",
		Data:        notFoundImage,
	})
}

// FilterReferencedImages returns only images that are actually referenced in the book
func (fb *FictionBook) FilterReferencedImages(allImages BookImages, links ReverseLinkIndex, coverID string, log *zap.Logger) BookImages {
	referenced := make(map[string]bool)

	// Always include the not found image if it exists (it may be needed for broken links)
	if fb.NotFoundImageID != "" {
		if _, exists := allImages[fb.NotFoundImageID]; exists {
			referenced[fb.NotFoundImageID] = true
		}
	}

	// Always include vignette images
	for _, vignetteID := range fb.VignetteIDs {
		if _, exists := allImages[vignetteID]; exists {
			referenced[vignetteID] = true
		}
	}

	// Add cover image if present
	if coverID != "" {
		referenced[coverID] = true
	}

	// Add all images referenced in links
	for targetID, refs := range links {
		if len(refs) == 0 {
			continue
		}

		// Check if any reference is an image type
		for _, ref := range refs {
			switch ref.Type {
			case "coverpage", "block-image", "inline-image":
				referenced[targetID] = true
			}
		}
	}

	// Build filtered index
	filtered := make(BookImages)
	for id := range referenced {
		if img, exists := allImages[id]; exists {
			filtered[id] = img
			continue
		}
		log.Debug("Referenced image not found in prepared images", zap.String("id", id))
	}

	log.Debug("Filtered images index", zap.Int("total", len(allImages)), zap.Int("referenced", len(filtered)))
	for id, img := range allImages {
		if _, exists := filtered[id]; !exists {
			log.Debug("Excluding unreferenced image", zap.String("id", id), zap.String("type", img.MimeType))
		}
	}

	return filtered
}

// MarkDropcaps walks all main bodies of the book and marks first text paragraphs
// in each section with "has-dropcap" style for drop cap rendering. Only paragraphs
// at the section level are considered - paragraphs inside titles, epigraphs, poems,
// cites, tables, or other nested structures are ignored.
// If the first word contains any symbol from cfg.IgnoreSymbols or starts with a
// Unicode space, the paragraph is left unchanged. Otherwise, "has-dropcap" is appended
// to the paragraph's Style field, allowing renderers to apply special formatting by
// extracting the first character during rendering.
// Returns a new FictionBook with marked drop caps. The original remains unchanged.
func (fb *FictionBook) MarkDropcaps(cfg *config.DropcapsConfig) *FictionBook {
	if cfg == nil || !cfg.Enable {
		return fb
	}

	result := fb.clone()
	for i := range result.Bodies {
		if result.Bodies[i].Main() {
			for j := range result.Bodies[i].Sections {
				markSectionDropcaps(&result.Bodies[i].Sections[j], string(cfg.IgnoreSymbols))
			}
		}
	}
	return result
}

// markSectionDropcaps marks drop caps for the first paragraph in a section and its nested sections
func markSectionDropcaps(section *Section, ignoreSymbols string) {
	// Track whether we've marked a paragraph or encountered a nested section at current level
	markedOrNested := false

	// Find and mark the first text paragraph in this section's content
	for i := range section.Content {
		if section.Content[i].Kind == FlowParagraph && section.Content[i].Paragraph != nil {
			// Only process paragraphs if we haven't already marked one or hit a nested section
			if !markedOrNested {
				if !markParagraphDropcap(section.Content[i].Paragraph, ignoreSymbols) {
					// Successfully marked, stop looking for more paragraphs in this section
					markedOrNested = true
				}
				// If couldn't mark, continue looking for next paragraph
			}
		} else if section.Content[i].Kind == FlowSection && section.Content[i].Section != nil {
			// Once we hit a nested section, mark it and stop processing paragraphs at this level
			markedOrNested = true
			// Recursively process nested sections - they each get their own dropcap
			markSectionDropcaps(section.Content[i].Section, ignoreSymbols)
		}
	}
}

// markParagraphDropcap marks a paragraph for drop cap rendering by appending "has-dropcap" to its Style.
// Returns true to continue looking for more paragraphs, false if marked or explicitly skipped (Unicode space or ignored symbol).
func markParagraphDropcap(para *Paragraph, ignoreSymbols string) bool {
	if len(para.Text) == 0 {
		return true // Continue looking (empty paragraph)
	}

	// Find the first non-empty text segment to analyze
	for i := range para.Text {
		seg := &para.Text[i]

		// Only process plain text segments
		if seg.Kind != InlineText || seg.Text == "" {
			continue
		}

		// Get the first rune
		runes := []rune(seg.Text)
		if len(runes) == 0 {
			continue
		}

		firstRune := runes[0]

		// Check if first character is a Unicode space - stop looking, this section shouldn't have dropcap
		if unicode.IsSpace(firstRune) {
			return false // Stop looking (found paragraph with space)
		}

		// Check if first character should be ignored - stop looking, this section shouldn't have dropcap
		if strings.ContainsRune(ignoreSymbols, firstRune) {
			return false // Stop looking (found paragraph with ignored symbol)
		}

		// Append has-dropcap style to existing style
		if para.Style == "" {
			para.Style = "has-dropcap"
		} else {
			para.Style = para.Style + " has-dropcap"
		}
		return false // Stop looking, successfully marked
	}

	return true // Continue looking (no valid text found in this paragraph)
}
