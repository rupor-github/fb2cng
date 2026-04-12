package structure

import (
	"errors"
	"fmt"

	"fbc/content"
	"fbc/fb2"
)

// BuildPlan converts normalized content into a renderer-neutral structural plan.
func BuildPlan(c *content.Content) (*Plan, error) {
	if c == nil || c.Book == nil {
		return nil, errors.New("nil content or book")
	}

	b := &builder{
		c:    c,
		plan: &Plan{},
	}

	b.addCover()
	b.walkBodies()

	return b.plan, nil
}

type builder struct {
	c    *content.Content
	plan *Plan

	startSet       bool
	footnoteBodies []*fb2.Body
}

func (b *builder) addCover() {
	if b.c.CoverID == "" || len(b.c.Book.Description.TitleInfo.Coverpage) == 0 {
		return
	}

	b.plan.Units = append(b.plan.Units, Unit{
		Kind:         UnitCover,
		ID:           b.c.CoverID,
		Title:        "Cover",
		ForceNewPage: true,
	})
	b.plan.Landmarks.CoverID = b.c.CoverID
}

func (b *builder) walkBodies() {
	for i := range b.c.Book.Bodies {
		body := &b.c.Book.Bodies[i]

		if body.Footnotes() {
			b.footnoteBodies = append(b.footnoteBodies, body)
			continue
		}

		b.addBodyIntroIfNeeded(i, body)
		b.addTopLevelSections(body)
	}

	b.addFootnoteBodies()
}

func (b *builder) addBodyIntroIfNeeded(index int, body *fb2.Body) {
	if body == nil || body.Title == nil {
		return
	}

	title := body.AsTitleText(body.Name)

	if body.Image != nil && b.c.Book.BodyTitleNeedsBreak() {
		id := fmt.Sprintf("a-body-image-%d", index)
		b.plan.Units = append(b.plan.Units, Unit{
			Kind:         UnitBodyImage,
			ID:           id,
			Title:        title,
			ForceNewPage: true,
			Body:         body,
		})
		if !b.startSet {
			b.plan.Landmarks.StartID = id
			b.startSet = true
		}
	}

	id := fmt.Sprintf("a-body-%d", index)
	b.plan.Units = append(b.plan.Units, Unit{
		Kind:         UnitBodyIntro,
		ID:           id,
		Title:        title,
		ForceNewPage: true,
		Body:         body,
	})

	if !b.startSet && body.Main() {
		b.plan.Landmarks.StartID = id
		b.startSet = true
	}

	if tocTitle := body.Title.AsTOCText("Untitled"); tocTitle != "" {
		b.plan.TOC = append(b.plan.TOC, &TOCEntry{
			ID:           id,
			Title:        tocTitle,
			IncludeInTOC: true,
		})
	}
}

func (b *builder) addTopLevelSections(body *fb2.Body) {
	if body == nil {
		return
	}
	for i := range body.Sections {
		entries := b.addSectionUnit(&body.Sections[i], 1, 1, true)
		b.plan.TOC = append(b.plan.TOC, entries...)
	}
}

func (b *builder) addSectionUnit(section *fb2.Section, depth int, titleDepth int, isTopLevel bool) []*TOCEntry {
	if section == nil {
		return nil
	}

	b.plan.Units = append(b.plan.Units, Unit{
		Kind:         UnitSection,
		ID:           section.ID,
		Title:        section.AsTitleText(""),
		Depth:        depth,
		TitleDepth:   titleDepth,
		ForceNewPage: true,
		Section:      section,
		IsTopLevel:   isTopLevel,
	})

	if !b.startSet {
		b.plan.Landmarks.StartID = section.ID
		b.startSet = true
	}

	childTOC := b.collectSectionChildren(section, depth, titleDepth)

	if section.HasTitle() {
		return []*TOCEntry{{
			ID:           section.ID,
			Title:        section.AsTitleText(""),
			IncludeInTOC: true,
			Children:     childTOC,
		}}
	}

	return childTOC
}

func (b *builder) collectSectionChildren(section *fb2.Section, depth int, titleDepth int) []*TOCEntry {
	if section == nil {
		return nil
	}

	childDepth := depth + 1
	childTitleDepth := titleDepth
	if section.HasTitle() {
		childTitleDepth = titleDepth + 1
	}

	var out []*TOCEntry
	var lastTitled *TOCEntry

	for i := range section.Content {
		item := &section.Content[i]
		if item.Kind != fb2.FlowSection || item.Section == nil {
			continue
		}

		child := item.Section
		shouldSplit := child.HasTitle() && b.c.Book.SectionNeedsBreak(childDepth)

		var entries []*TOCEntry
		if shouldSplit {
			entries = b.addSectionUnit(child, childDepth, childTitleDepth, false)
		} else {
			entries = b.collectInlineSectionTOC(child, childDepth, childTitleDepth)
		}

		if child.HasTitle() {
			out = append(out, entries...)
			if len(entries) > 0 {
				lastTitled = entries[len(entries)-1]
			}
			continue
		}

		if lastTitled != nil {
			lastTitled.Children = append(lastTitled.Children, entries...)
		} else {
			out = append(out, entries...)
		}
	}

	return out
}

func (b *builder) collectInlineSectionTOC(section *fb2.Section, depth int, titleDepth int) []*TOCEntry {
	if section == nil {
		return nil
	}

	childTOC := b.collectSectionChildren(section, depth, titleDepth)

	if section.HasTitle() {
		return []*TOCEntry{{
			ID:           section.ID,
			Title:        section.AsTitleText(""),
			IncludeInTOC: true,
			Children:     childTOC,
		}}
	}

	return childTOC
}

func (b *builder) addFootnoteBodies() {
	for i, body := range b.footnoteBodies {
		id := fmt.Sprintf("a-notes-%d", i)
		title := body.AsTitleText(body.Name)

		b.plan.Units = append(b.plan.Units, Unit{
			Kind:         UnitFootnotesBody,
			ID:           id,
			Title:        title,
			ForceNewPage: true,
			Body:         body,
		})

		if title == "" {
			continue
		}

		entry := &TOCEntry{
			ID:           id,
			Title:        title,
			IncludeInTOC: true,
		}

		if !b.c.FootnotesMode.IsFloat() {
			for j := range body.Sections {
				section := &body.Sections[j]
				if !section.HasTitle() {
					continue
				}
				entry.Children = append(entry.Children, &TOCEntry{
					ID:           section.ID,
					Title:        section.AsTitleText(""),
					IncludeInTOC: true,
				})
			}
		}

		b.plan.TOC = append(b.plan.TOC, entry)
	}
}
