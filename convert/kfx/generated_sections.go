package kfx

import (
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
)

func nextContentBaseCounter(fragments *FragmentList) int {
	maxN := 0
	for _, f := range fragments.GetByType(SymContent) {
		name := f.FIDName
		if !strings.HasPrefix(name, "content_") {
			continue
		}
		rest := strings.TrimPrefix(name, "content_")
		parts := strings.SplitN(rest, "_", 2)
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		if n > maxN {
			maxN = n
		}
	}
	return maxN + 1
}

func nextSectionIndex(sectionNames []string) int {
	maxN := -1
	for _, s := range sectionNames {
		if !strings.HasPrefix(s, "c") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(s, "c"))
		if err != nil {
			continue
		}
		if n > maxN {
			maxN = n
		}
	}
	return maxN + 1
}

func addText(sb *StorylineBuilder, styles *StyleRegistry, ca *ContentAccumulator, text, style string) {
	if text == "" {
		return
	}
	styles.EnsureStyle(style)
	name, off := ca.Add(text)
	sb.AddContent(SymText, name, off, style)
}

func flattenTOCEntries(entries []*TOCEntry, includeUntitled bool) []string {
	var out []string
	var walk func(es []*TOCEntry, depth int)
	walk = func(es []*TOCEntry, depth int) {
		for _, e := range es {
			if e == nil {
				continue
			}
			if !e.IncludeInTOC && !includeUntitled {
				continue
			}
			t := e.Title
			if t == "" {
				t = "Untitled"
			}
			indent := strings.Repeat("  ", max(depth-1, 0))
			out = append(out, fmt.Sprintf("%s• %s", indent, t))
			if len(e.Children) > 0 {
				walk(e.Children, depth+1)
			}
		}
	}
	walk(entries, 1)
	return out
}

func addGeneratedSections(
	c *content.Content,
	cfg *config.DocumentConfig,
	log *zap.Logger,
	styles *StyleRegistry,
	imageResourceNames map[string]string,
	fragments *FragmentList,
	sectionNames []string,
	tocEntries []*TOCEntry,
	sectionEIDs map[string][]int,
	nextEID int,
) ([]string, []*TOCEntry, map[string][]int, int, error) {
	annotationEnabled := cfg.Annotation.Enable && c.Book.Description.TitleInfo.Annotation != nil
	tocPageEnabled := cfg.TOCPage.Placement != common.TOCPagePlacementNone

	before := make([]string, 0)
	after := make([]string, 0)

	sectionIdx := nextSectionIndex(sectionNames)
	storyIdx := len(sectionNames) + 1
	contentCounter := nextContentBaseCounter(fragments)

	if annotationEnabled {
		storyName := fmt.Sprintf("l%d", storyIdx)
		sectionName := fmt.Sprintf("c%d", sectionIdx)
		storyIdx++
		sectionIdx++

		sb := NewStorylineBuilder(storyName, sectionName, nextEID)
		ca := NewContentAccumulator(contentCounter)
		contentCounter++

		addText(sb, styles, ca, cfg.Annotation.Title, "annotation-title")
		for i := range c.Book.Description.TitleInfo.Annotation.Items {
			item := &c.Book.Description.TitleInfo.Annotation.Items[i]
			text := item.AsPlainText()
			if text == "" {
				continue
			}
			addText(sb, styles, ca, text, "annotation")
		}

		for name, list := range ca.Finish() {
			if err := fragments.Add(buildContentFragmentByName(name, list)); err != nil {
				return nil, nil, nil, 0, err
			}
		}

		sectionEIDs[sectionName] = sb.AllEIDs()
		nextEID = sb.NextEID()

		storyFrag, secFrag := sb.Build(600, 800)
		if err := fragments.Add(storyFrag); err != nil {
			return nil, nil, nil, 0, err
		}
		if err := fragments.Add(secFrag); err != nil {
			return nil, nil, nil, 0, err
		}

		annotationEntry := &TOCEntry{
			ID:           "annotation-page",
			Title:        cfg.Annotation.Title,
			SectionName:  sectionName,
			StoryName:    storyName,
			FirstEID:     sb.FirstEID(),
			IncludeInTOC: cfg.Annotation.InTOC,
		}
		if cfg.Annotation.InTOC {
			tocEntries = append([]*TOCEntry{annotationEntry}, tocEntries...)
		}
		before = append(before, sectionName)
	}

	var tocSectionName string
	if tocPageEnabled {
		storyName := fmt.Sprintf("l%d", storyIdx)
		tocSectionName = fmt.Sprintf("c%d", sectionIdx)
		storyIdx++
		sectionIdx++

		sb := NewStorylineBuilder(storyName, tocSectionName, nextEID)
		ca := NewContentAccumulator(contentCounter)
		contentCounter++

		addText(sb, styles, ca, c.Book.Description.TitleInfo.BookTitle.Value, "toc-title")
		if cfg.TOCPage.AuthorsTemplate != "" {
			expanded, err := c.Book.ExpandTemplateMetainfo(config.AuthorsTemplateFieldName, cfg.TOCPage.AuthorsTemplate, c.SrcName, c.OutputFormat)
			if err != nil {
				log.Warn("Unable to prepare list of authors for TOC", zap.Error(err))
			} else {
				addText(sb, styles, ca, expanded, "toc-title")
			}
		}

		lines := flattenTOCEntries(tocEntries, cfg.TOCPage.ChaptersWithoutTitle)
		for _, line := range lines {
			addText(sb, styles, ca, line, "toc-item")
		}

		for name, list := range ca.Finish() {
			if err := fragments.Add(buildContentFragmentByName(name, list)); err != nil {
				return nil, nil, nil, 0, err
			}
		}

		sectionEIDs[tocSectionName] = sb.AllEIDs()
		nextEID = sb.NextEID()

		storyFrag, secFrag := sb.Build(600, 800)
		if err := fragments.Add(storyFrag); err != nil {
			return nil, nil, nil, 0, err
		}
		if err := fragments.Add(secFrag); err != nil {
			return nil, nil, nil, 0, err
		}

		if cfg.TOCPage.Placement == common.TOCPagePlacementBefore {
			before = append([]string{tocSectionName}, before...)
		} else {
			after = append(after, tocSectionName)
		}
	}

	newOrder := make([]string, 0, len(sectionNames)+len(before)+len(after))
	newOrder = append(newOrder, before...)
	newOrder = append(newOrder, sectionNames...)
	newOrder = append(newOrder, after...)
	return newOrder, tocEntries, sectionEIDs, nextEID, nil
}
