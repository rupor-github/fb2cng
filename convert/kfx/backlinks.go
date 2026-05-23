package kfx

import "fbc/content"

func resolveKFXBacklinkLocations(c *content.Content, fragments *FragmentList, contentSnapshot map[string][]string, sectionNames sectionNameList, chapterStartSections map[string]bool, idToEID eidByFB2ID, pending *StorylineBuilder) {
	if c == nil || len(c.BackLinkIndex) == 0 || fragments == nil {
		return
	}
	posFragments := fragments
	if len(contentSnapshot) > 0 || pending != nil {
		posFragments = positionSnapshotFragments(fragments, contentSnapshot, pending)
	}
	posItems := CollectPositionItems(posFragments, sectionNames, chapterStartSections)
	if len(posItems) == 0 {
		return
	}
	pages := CalculateApproximatePages(posItems, c.PageSize)
	for _, refs := range c.BackLinkIndex {
		for _, ref := range refs {
			target, ok := idToEID[ref.RefID]
			if !ok || target.EID <= 0 {
				continue
			}
			pid, ok := kfxPIDForEIDOffset(posItems, target.EID, target.Offset)
			if !ok {
				continue
			}
			c.SetBackLinkRefLocation(ref.RefID, pid/40+1)
			if page := kfxPageForPID(pages, posItems, pid); page > 0 {
				c.SetBackLinkRefPage(ref.RefID, page)
			}
		}
	}
}

func positionSnapshotFragments(fragments *FragmentList, contentSnapshot map[string][]string, pending *StorylineBuilder) *FragmentList {
	out := NewFragmentList()
	for _, frag := range fragments.All() {
		_ = out.Add(frag)
	}
	for name, contentList := range contentSnapshot {
		frag := buildContentFragmentByName(name, contentList)
		if out.byKey[frag.Key()] != nil {
			continue
		}
		_ = out.Add(frag)
	}
	if pending != nil {
		storylineFrag, sectionFrag := storylineSnapshotFragments(pending)
		if out.byKey[storylineFrag.Key()] == nil {
			_ = out.Add(storylineFrag)
		}
		if out.byKey[sectionFrag.Key()] == nil {
			_ = out.Add(sectionFrag)
		}
	}
	return out
}

func storylineSnapshotFragments(sb *StorylineBuilder) (*Fragment, *Fragment) {
	entries := make([]any, 0, len(sb.contentEntries))
	for _, ref := range sb.contentEntries {
		entries = append(entries, NewContentEntry(ref))
	}
	storylineFrag := BuildStoryline(sb.name, entries)
	sectionFrag := BuildSection(sb.sectionName, []any{NewPageTemplateEntry(sb.pageTemplateEID, sb.name)})
	return storylineFrag, sectionFrag
}

func kfxPIDForEIDOffset(items []PositionItem, eid int, offset int) (int, bool) {
	pid := 0
	for _, item := range items {
		length := item.Length
		if length <= 0 {
			length = 1
		}
		if item.EID == eid {
			return pid + min(max(offset, 0), length-1), true
		}
		pid += length
	}
	return 0, false
}

func kfxPageForPID(pages []PageEntry, items []PositionItem, pid int) int {
	if len(pages) == 0 || len(items) == 0 || pid < 0 {
		return 0
	}
	itemStarts := kfxItemStartPIDs(items)
	bestPage := 0
	bestPID := -1
	for _, page := range pages {
		startPID, ok := itemStarts[page.EID]
		if !ok {
			continue
		}
		pagePID := startPID + int(page.Offset)
		if pagePID <= pid && pagePID >= bestPID {
			bestPID = pagePID
			bestPage = page.PageNumber
		}
	}
	return bestPage
}

func kfxItemStartPIDs(items []PositionItem) map[int]int {
	out := make(map[int]int, len(items))
	pid := 0
	for _, item := range items {
		if _, exists := out[item.EID]; !exists {
			out[item.EID] = pid
		}
		length := item.Length
		if length <= 0 {
			length = 1
		}
		pid += length
	}
	return out
}
