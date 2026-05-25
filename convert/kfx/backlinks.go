package kfx

import (
	"sort"

	"fbc/content"
)

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
	positionLookup := newKFXPositionLookup(posItems)
	pageLookup := newKFXPageLookup(CalculateApproximatePages(posItems, c.PageSize), positionLookup)
	for targetID, refs := range c.BackLinkIndex {
		for i := range refs {
			target, ok := idToEID[refs[i].RefID]
			if !ok || target.EID <= 0 {
				continue
			}
			pid, ok := positionLookup.pidForEIDOffset(target.EID, target.Offset)
			if !ok {
				continue
			}
			refs[i].LocationNumber = pid/40 + 1
			if page := pageLookup.pageForPID(pid); page > 0 {
				refs[i].PageNumber = page
			}
		}
		c.BackLinkIndex[targetID] = refs
	}
}

func kfxBacklinkRefsNeedLocation(refs []content.BackLinkRef) bool {
	for _, ref := range refs {
		if ref.LocationNumber == 0 {
			return true
		}
	}
	return false
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

type kfxPositionLookup struct {
	startPIDByEID map[int]int
	lengthByEID   map[int]int
}

type kfxPageLookup struct {
	starts []kfxPageStart
}

type kfxPageStart struct {
	pid  int
	page int
}

func newKFXPositionLookup(items []PositionItem) kfxPositionLookup {
	lookup := kfxPositionLookup{
		startPIDByEID: make(map[int]int, len(items)),
		lengthByEID:   make(map[int]int, len(items)),
	}
	pid := 0
	for _, item := range items {
		length := item.Length
		if length <= 0 {
			length = 1
		}
		if _, exists := lookup.startPIDByEID[item.EID]; !exists {
			lookup.startPIDByEID[item.EID] = pid
			lookup.lengthByEID[item.EID] = length
		}
		pid += length
	}
	return lookup
}

func (l kfxPositionLookup) pidForEIDOffset(eid int, offset int) (int, bool) {
	startPID, ok := l.startPIDByEID[eid]
	if !ok {
		return 0, false
	}
	length := l.lengthByEID[eid]
	if length <= 0 {
		length = 1
	}
	return startPID + min(max(offset, 0), length-1), true
}

func newKFXPageLookup(pages []PageEntry, positions kfxPositionLookup) kfxPageLookup {
	lookup := kfxPageLookup{starts: make([]kfxPageStart, 0, len(pages))}
	for _, page := range pages {
		startPID, ok := positions.startPIDByEID[page.EID]
		if !ok {
			continue
		}
		lookup.starts = append(lookup.starts, kfxPageStart{
			pid:  startPID + int(page.Offset),
			page: page.PageNumber,
		})
	}
	sort.SliceStable(lookup.starts, func(i, j int) bool {
		return lookup.starts[i].pid < lookup.starts[j].pid
	})
	return lookup
}

func (l kfxPageLookup) pageForPID(pid int) int {
	if len(l.starts) == 0 || pid < 0 {
		return 0
	}
	idx := sort.Search(len(l.starts), func(i int) bool {
		return l.starts[i].pid > pid
	}) - 1
	if idx < 0 {
		return 0
	}
	return l.starts[idx].page
}
