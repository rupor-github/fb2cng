package fb2

import (
	_ "embed"
	"fmt"
	"maps"
	"strings"

	"go.uber.org/zap"
)

//go:embed not_found.png
var notFoundImagePNG []byte

// NotFoundImageID is the ID used for the placeholder image when broken image links are found
const NotFoundImageID = "fbc-not-found-image"

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

// NormalizeLinks validates all links and replaces broken ones with text or points broken image links to notFoundImage.
// Returns a new FictionBook with broken links replaced, along with corrected ID and link indexes.
// The returned indexes reflect the state after link replacements. The original remains unchanged.
func (fb *FictionBook) NormalizeLinks(log *zap.Logger) (*FictionBook, IDIndex, ReverseLinkIndex) {
	result := fb.clone()

	// Rebuild indexes for the cloned book since the original indexes reference the original book's pointers
	resultIDs := result.buildIDIndex(log)
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
					// Ensure not-found image binary is present if we replaced an image link
					result.ensureNotFoundImageBinary()
				}
			}

		case "broken-link":
			// Broken external link - replace with text
			for _, ref := range refs {
				log.Warn("Broken external link detected", zap.String("location", FormatRefPath(ref.Path)))
				if result.replaceBrokenLink(ref, targetID, log) {
					// Ensure not-found image binary is present if we replaced an image link
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
					// Ensure not-found image binary is present if we replaced an image link
					result.ensureNotFoundImageBinary()
				}
			}
		}
	}

	// Rebuild link index after replacements to remove references to replaced links
	resultLinks = result.buildReverseLinkIndex(log)

	return result, resultIDs, resultLinks
}

// NormalizeIDs assigns sequential IDs to all sections and subtitles that don't have IDs.
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

// assignBodyIDs recursively assigns IDs to sections and subtitles in a body
func (fb *FictionBook) assignBodyIDs(body *Body, path []any, existingIDs, updatedIDs IDIndex, sectionCounter, subtitleCounter *int, log *zap.Logger) {
	for i := range body.Sections {
		sectionPath := append(append([]any{}, path...), &body.Sections[i])
		fb.assignSectionIDs(&body.Sections[i], sectionPath, existingIDs, updatedIDs, sectionCounter, subtitleCounter, log)
	}
}

// assignSectionIDs recursively assigns IDs to a section, its subtitles, and its child sections
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
		if section.Content[i].Kind == FlowSubtitle && section.Content[i].Subtitle != nil && section.Content[i].Subtitle.ID == "" {
			// Subtitle at section level without ID - assign one
			// Find a unique ID that doesn't collide
			for {
				*subtitleCounter++
				candidateID := fmt.Sprintf("subtitle_%d", *subtitleCounter)
				if _, exists := existingIDs[candidateID]; !exists {
					section.Content[i].Subtitle.ID = candidateID
					// Add to updated index with special type
					subtitlePath := append(append([]any{}, path...), &section.Content[i], section.Content[i].Subtitle)
					updatedIDs[candidateID] = ElementRef{
						Type: "subtitle-generated",
						Path: subtitlePath,
					}
					log.Debug("Generated subtitle id", zap.String("ID", candidateID))
					break
				}
			}
		} else if section.Content[i].Kind == FlowSection && section.Content[i].Section != nil {
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
		// Point coverpage to not-found image
		if len(ref.Path) > 0 {
			if img, ok := ref.Path[len(ref.Path)-1].(*InlineImage); ok {
				img.Href = "#" + NotFoundImageID
				log.Debug("Broken coverpage image link redirected to not-found image", zap.String("original", targetID))
				addedNotFoundImage = true
			}
		}
	case "block-image":
		// Point block image to not-found image
		if len(ref.Path) > 0 {
			if img, ok := ref.Path[len(ref.Path)-1].(*Image); ok {
				img.Href = "#" + NotFoundImageID
				log.Debug("Broken block image link redirected to not-found image", zap.String("original", targetID))
				addedNotFoundImage = true
			}
		}
	case "inline-image":
		// Point inline image to not-found image
		if len(ref.Path) > 0 {
			if segment, ok := ref.Path[len(ref.Path)-1].(*InlineSegment); ok {
				if segment.Image != nil {
					segment.Image.Href = "#" + NotFoundImageID
					log.Debug("Broken inline image link redirected to not-found image", zap.String("original", targetID))
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

// ensureNotFoundImageBinary adds the not-found image binary if it doesn't already exist
func (fb *FictionBook) ensureNotFoundImageBinary() {
	// Check if not-found image binary already exists
	for i := range fb.Binaries {
		if fb.Binaries[i].ID == NotFoundImageID {
			return
		}
	}

	// Add not-found image binary
	fb.Binaries = append(fb.Binaries, BinaryObject{
		ID:          NotFoundImageID,
		ContentType: "image/png",
		Data:        notFoundImagePNG,
	})
}

// NormalizeSections flattens grouping sections (sections without titles that only contain other sections)
// by promoting their children up to the parent level. This ensures that section hierarchy is clean and
// doesn't contain unnecessary nesting levels. Returns a new FictionBook with normalized sections.
func (fb *FictionBook) NormalizeSections(log *zap.Logger) *FictionBook {
	result := fb.clone()
	for i := range result.Bodies {
		result.Bodies[i].Sections = normalizeSections(result.Bodies[i].Sections, log)
	}
	return result
}

// normalizeSections recursively normalizes sections by flattening grouping sections
func normalizeSections(sections []Section, log *zap.Logger) []Section {
	var result []Section
	for i := range sections {
		section := &sections[i]
		if section.Title == nil && isGroupingSection(section) {
			// Grouping section without title - unwrap and flatten
			log.Debug("Flattening grouping section", zap.String("id", section.ID))
			flattened := extractSectionsFromContent(section.Content, log)
			result = append(result, flattened...)
		} else {
			// Regular section - normalize its content recursively
			section.Content = normalizeFlowItems(section.Content, log)
			result = append(result, *section)
		}
	}
	return result
}

// isGroupingSection checks if a section is a pure grouping container (no content except nested sections)
func isGroupingSection(section *Section) bool {
	if section.Title != nil || section.Image != nil || section.Annotation != nil || len(section.Epigraphs) > 0 {
		return false
	}
	if len(section.Content) == 0 {
		return false
	}
	for _, item := range section.Content {
		if item.Kind != FlowSection {
			return false
		}
	}
	return true
}

// extractSectionsFromContent extracts and normalizes sections from flow items
func extractSectionsFromContent(items []FlowItem, log *zap.Logger) []Section {
	var result []Section
	for i := range items {
		if items[i].Kind == FlowSection && items[i].Section != nil {
			nested := items[i].Section
			if nested.Title == nil && isGroupingSection(nested) {
				// Recursively flatten nested grouping sections
				flattened := extractSectionsFromContent(nested.Content, log)
				result = append(result, flattened...)
			} else {
				// Regular section - normalize its content recursively
				nested.Content = normalizeFlowItems(nested.Content, log)
				result = append(result, *nested)
			}
		}
	}
	return result
}

// normalizeFlowItems recursively normalizes flow items by flattening grouping sections
func normalizeFlowItems(items []FlowItem, log *zap.Logger) []FlowItem {
	var result []FlowItem
	for i := range items {
		item := items[i]
		if item.Kind == FlowSection && item.Section != nil {
			section := item.Section
			if section.Title == nil && isGroupingSection(section) {
				// Grouping section - unwrap and add its children directly
				result = append(result, section.Content...)
			} else {
				// Regular section - normalize its content recursively
				section.Content = normalizeFlowItems(section.Content, log)
				item.Section = section
				result = append(result, item)
			}
		} else {
			result = append(result, item)
		}
	}
	return result
}
