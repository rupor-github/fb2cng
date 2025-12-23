package kfx

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
	"fbc/convert/kfx/builders"
	"fbc/convert/kfx/container"
	"fbc/convert/kfx/ionutil"
	"fbc/convert/kfx/model"
	"fbc/convert/kfx/symbols"
)

// Generate creates the KFX output file.
func Generate(ctx context.Context, c *content.Content, outputPath string, cfg *config.DocumentConfig, log *zap.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if true {
		return fmt.Errorf("KFX generation is experimental and not yet implemented")
	}

	log.Info("Generating KFX", zap.String("output", outputPath))

	containerID := "CR!" + c.Book.Description.DocumentInfo.ID

	// Minimal content fragment.
	type contentFragment struct {
		Name               string   `ion:"name,symbol"`
		ContentList        []string `ion:"$146"`
		Selection          int64    `ion:"$436"`
		ScreenActualHeight []any    `ion:"$305"`
	}

	// Minimal storyline that references the content fragment.
	type contentRef struct {
		Name  string `ion:"name,symbol"`
		Index int64  `ion:"$403"`
	}
	type styleEvent struct {
		Offset int64  `ion:"$143"`
		Length int64  `ion:"$144"`
		LinkTo string `ion:"$179,symbol,omitempty"`
	}
	type innerNode struct {
		EID     int64      `ion:"$155"`
		Style   string     `ion:"$157,symbol,omitempty"`
		Type    string     `ion:"$159,symbol"`
		Content contentRef `ion:"$145"`
	}
	type innerNodeWithAnchor struct {
		StyleEvents []styleEvent `ion:"$142,omitempty"`
		EID         int64        `ion:"$155"`
		Style       string       `ion:"$157,symbol,omitempty"`
		Type        string       `ion:"$159,symbol"`
		Content     contentRef   `ion:"$145"`
	}
	// imageStoryNode represents an inline image in storyline content.
	// Type $271 = image_container_block, $175 = resource_name referencing $164.
	type imageStoryNode struct {
		EID          int64  `ion:"$155"`
		Style        string `ion:"$157,symbol,omitempty"`
		Type         string `ion:"$159,symbol"`
		ResourceName string `ion:"$175,symbol"`
	}
	type outerNode struct {
		EID          int64  `ion:"$155"`
		Layout       string `ion:"$156,symbol"`
		Style        string `ion:"$157,symbol,omitempty"`
		Type         string `ion:"$159,symbol"`
		Kids         []any  `ion:"$146"`
		HeadingLevel int64  `ion:"$790"`
	}
	type storyline struct {
		ID  string `ion:"$176,symbol"`
		Seq []any  `ion:"$146"`
	}

	// Minimal section that references the storyline.
	type sectionStoryline struct {
		EID         int64  `ion:"$155"`
		SL          string `ion:"$176,symbol"`
		Layout      string `ion:"$156,symbol"`
		Float       string `ion:"$140,symbol"`
		Type        string `ion:"$159,symbol"`
		FixedWidth  int64  `ion:"$66"`
		FixedHeight int64  `ion:"$67"`
	}
	type section struct {
		ID   string `ion:"$174,symbol"`
		Rows []any  `ion:"$141"`
	}

	// Minimal location / PID maps (kfxlib/yj_position_location.py expectations).
	type location struct {
		EID    int64 `ion:"$155"`
		Offset int64 `ion:"$143,omitempty"`
	}
	type locationMapRoot struct {
		ROName    string     `ion:"$178,symbol"`
		Locations []location `ion:"$182"`
	}
	type positionMapSection struct {
		Section string  `ion:"$174,symbol"`
		EIDs    []int64 `ion:"$181"`
	}
	type pidMapEntry struct {
		PID    int64 `ion:"$184"`
		EID    int64 `ion:"$185"`
		Offset int64 `ion:"$143,omitempty"`
	}

	eidSectionRowBase := int64(2000)
	eidStoryOuterBase := int64(3000)
	eidTextBase := int64(4000)

	lt, tocNodes, _ := linearizeMainTextWithTOC(c, eidTextBase)
	contentBlocks := lt.Blocks

	// Generate reflow sections based on TOC entry anchors (closer to Kindle-produced KFX).
	tocFlat := make([]*tocNode, 0, 128)
	var walkTOC func(nodes []*tocNode)
	walkTOC = func(nodes []*tocNode) {
		for _, n := range nodes {
			if n == nil || n.EID < eidTextBase || n.Title == "" {
				continue
			}
			tocFlat = append(tocFlat, n)
			walkTOC(n.Children)
		}
	}
	walkTOC(tocNodes)

	sort.Slice(tocFlat, func(i, j int) bool { return tocFlat[i].EID < tocFlat[j].EID })
	uniq := tocFlat[:0]
	var lastEID int64 = -1
	for _, n := range tocFlat {
		if n.EID != lastEID {
			uniq = append(uniq, n)
			lastEID = n.EID
		}
	}
	tocFlat = uniq

	spans := make([]topSectionSpan, 0, len(tocFlat)+1)
	if len(tocFlat) > 0 {
		first := int(tocFlat[0].EID - eidTextBase)
		if first > 0 {
			spans = append(spans, topSectionSpan{Title: c.Book.Description.TitleInfo.BookTitle.Value, AnchorEID: eidTextBase, StartBlock: 0, EndBlock: first})
		}
	}
	for i, n := range tocFlat {
		start := int(n.EID - eidTextBase)
		end := len(lt.Nodes)
		if i+1 < len(tocFlat) {
			end = int(tocFlat[i+1].EID - eidTextBase)
		}
		if start < 0 || start >= len(lt.Nodes) {
			continue
		}
		if end <= start {
			end = min(start+1, len(lt.Nodes))
		}
		spans = append(spans, topSectionSpan{Title: n.Title, AnchorEID: n.EID, StartBlock: start, EndBlock: end})
	}

	const maxContentBytes = 7000
	type contentFragmentDef struct {
		ID     string
		Blocks []string
	}

	blockRefs := make([]contentRef, len(contentBlocks))
	contentDefs := make([]contentFragmentDef, 0, 8)

	nextContentIdx := 0
	curID := ""
	curBlocks := make([]string, 0, 64)
	curBytes := 0

	flush := func() error {
		if curID == "" {
			return nil
		}
		contentDefs = append(contentDefs, contentFragmentDef{ID: curID, Blocks: curBlocks})
		curID = ""
		curBlocks = make([]string, 0, 64)
		curBytes = 0
		return nil
	}
	allocID := func() (string, error) {
		id := fmt.Sprintf("content_%d", nextContentIdx)
		nextContentIdx++
		return id, nil
	}

	for i, b := range contentBlocks {
		bBytes := len([]byte(b))
		if curID != "" && curBytes+bBytes > maxContentBytes && len(curBlocks) > 0 {
			if err := flush(); err != nil {
				return err
			}
		}
		if curID == "" {
			id, err := allocID()
			if err != nil {
				return err
			}
			curID = id
		}
		idx := int64(len(curBlocks))
		curBlocks = append(curBlocks, b)
		curBytes += bBytes
		blockRefs[i] = contentRef{Name: curID, Index: idx}
	}
	if err := flush(); err != nil {
		return err
	}

	if len(spans) == 0 {
		spans = append(spans, topSectionSpan{Title: "", AnchorEID: eidTextBase, StartBlock: 0, EndBlock: len(lt.Nodes)})
	}

	type kfxSectionDef struct {
		SectionID   string
		StorylineID string
		Span        topSectionSpan
		SectionEID  int64
		OuterEID    int64
	}

	// $597 auxiliary_data (section_metadata) fragment type.
	type sectionMetadata struct {
		Items     []any  `ion:"$258"`
		SectionID string `ion:"$598,symbol"`
	}

	// $266 anchor (position_anchor) fragment type.
	type positionAnchor struct {
		AnchorName string `ion:"$180,symbol"`
		Position   any    `ion:"$183"`
	}

	sections := make([]kfxSectionDef, 0, len(spans))
	sectionIDs := make([]string, 0, len(spans))
	storylineIDs := make([]string, 0, len(spans))

	for i, sp := range spans {
		sid := fmt.Sprintf("sec_%d", i)
		slid := fmt.Sprintf("sl_%d", i)
		sections = append(sections, kfxSectionDef{SectionID: sid, StorylineID: slid, Span: sp, SectionEID: eidSectionRowBase + int64(i), OuterEID: eidStoryOuterBase + int64(i)})
		sectionIDs = append(sectionIDs, sid)
		storylineIDs = append(storylineIDs, slid)
	}

	readingOrders := builders.BuildReadingOrders(sectionIDs)

	buildTOCItems := func(nodes []*tocNode) []any {
		var rec func(in []*tocNode) []any
		rec = func(in []*tocNode) []any {
			out := make([]any, 0, len(in))
			for _, n := range in {
				if n == nil || n.EID <= 0 || n.Title == "" {
					continue
				}
				m := map[string]any{
					"$241": map[string]any{"$244": n.Title},
					"$246": map[string]any{"$143": int64(0), "$155": n.EID},
				}
				if kids := rec(n.Children); len(kids) > 0 {
					m["$247"] = kids
				}
				out = append(out, m)
			}
			return out
		}
		return rec(nodes)
	}

	// All EIDs used by pid/location maps (in reading order).
	type contentChunk struct {
		EID    int64
		Length int64
	}

	chunks := make([]contentChunk, 0, 2*len(sections)+len(lt.Nodes))
	maxEID := int64(0)
	for _, s := range sections {
		chunks = append(chunks, contentChunk{EID: s.SectionEID, Length: 1}, contentChunk{EID: s.OuterEID, Length: 1})
		if s.SectionEID > maxEID {
			maxEID = s.SectionEID
		}
		if s.OuterEID > maxEID {
			maxEID = s.OuterEID
		}

		for bi := s.Span.StartBlock; bi < s.Span.EndBlock && bi < len(lt.Nodes); bi++ {
			eid := lt.Nodes[bi].EID
			chunks = append(chunks, contentChunk{EID: eid, Length: int64(utf8.RuneCountInString(contentBlocks[bi]))})
			if eid > maxEID {
				maxEID = eid
			}
		}
	}

	pidMap := make([]pidMapEntry, 0, len(chunks)+1)
	locs := make([]location, 0, len(chunks))
	pid := int64(0)
	for _, ch := range chunks {
		pidMap = append(pidMap, pidMapEntry{PID: pid, EID: ch.EID})
		locs = append(locs, location{EID: ch.EID, Offset: 0})
		pid += ch.Length
	}
	pidEnd := pid
	pidMap = append(pidMap, pidMapEntry{PID: pidEnd, EID: 0})

	reflowSectionSize := int64(1)

	posMaps := make([]positionMapSection, 0, len(sections))
	for _, s := range sections {
		eids := make([]int64, 0, 2+(s.Span.EndBlock-s.Span.StartBlock))
		eids = append(eids, s.SectionEID, s.OuterEID)
		for bi := s.Span.StartBlock; bi < s.Span.EndBlock && bi < len(lt.Nodes); bi++ {
			eids = append(eids, lt.Nodes[bi].EID)
		}
		// NOTE: Image EIDs are NOT added to position_map since they don't have PIDs.
		// They are only added to the storyline structure.
		posMaps = append(posMaps, positionMapSection{Section: s.SectionID, EIDs: eids})
	}

	// Generate section_metadata IDs ($597).
	sectionMetaIDs := make([]string, 0, len(sections))
	for _, s := range sections {
		sectionMetaIDs = append(sectionMetaIDs, s.SectionID+"-ad")
	}

	// Generate anchor IDs ($266) for each TOC entry and map EID -> anchor ID.
	anchorIDs := make([]string, 0, len(tocFlat))
	eidToAnchor := make(map[int64]string) // EID -> anchor ID mapping
	for i, n := range tocFlat {
		anchorID := fmt.Sprintf("anc_%d", i)
		anchorIDs = append(anchorIDs, anchorID)
		eidToAnchor[n.EID] = anchorID
	}

	// Build image resource IDs for all images found during linearization.
	type imageRef struct {
		ResourceID string // FID for $164 external_resource
		Location   string // $165 location string AND FID for $417 raw_media
		EID        int64  // EID for this image in storyline
	}
	imageRefs := make(map[string]imageRef)  // FB2 image ID -> KFX IDs
	eidToImageRes := make(map[int64]string) // Image EID -> resource ID for storyline
	imageResourceIDs := make([]string, 0, len(lt.Images))
	imageLocationIDs := make([]string, 0, len(lt.Images))

	// Track cover resource ID separately.
	var coverResourceID string

	for i, imgNode := range lt.Images {
		// Skip if image doesn't exist in index.
		if _, ok := c.ImagesIndex[imgNode.ImageID]; !ok {
			continue
		}
		resourceID := fmt.Sprintf("img_res_%d", i)
		location := fmt.Sprintf("resource/img%d", i)
		imageRefs[imgNode.ImageID] = imageRef{ResourceID: resourceID, Location: location, EID: imgNode.EID}
		eidToImageRes[imgNode.EID] = resourceID
		imageResourceIDs = append(imageResourceIDs, resourceID)
		imageLocationIDs = append(imageLocationIDs, location)

		// Check if this is the cover image.
		if c.CoverID != "" && imgNode.ImageID == c.CoverID {
			coverResourceID = resourceID
		}
	}

	// If cover wasn't found in linearized images, add it separately.
	if c.CoverID != "" && coverResourceID == "" {
		if img := c.ImagesIndex[c.CoverID]; img != nil {
			resourceID := "cover_res"
			location := "resource/cover"
			imageRefs[c.CoverID] = imageRef{ResourceID: resourceID, Location: location, EID: 0}
			imageResourceIDs = append(imageResourceIDs, resourceID)
			imageLocationIDs = append(imageLocationIDs, location)
			coverResourceID = resourceID
		}
	}

	// Build all available styles - we'll filter to used ones later.
	allStyles := builders.BuildDefaultStyles()
	styleMap := make(map[string]builders.StyleDef, len(allStyles))
	for _, s := range allStyles {
		styleMap[s.ID] = s
	}

	// First pass: determine which styles are used.
	// We use: sD7 (outer), sF (first para), sDC (body), sH (heading), s6P (image)
	usedStyleNames := make(map[string]bool)
	usedStyleNames["sD7"] = true // outer container
	usedStyleNames["s6P"] = true // image container (if images exist)
	for _, s := range sections {
		isFirstBlock := true
		for bi := s.Span.StartBlock; bi < s.Span.EndBlock && bi < len(lt.Nodes); bi++ {
			eid := lt.Nodes[bi].EID
			if _, isHeading := eidToAnchor[eid]; isHeading {
				usedStyleNames["sH"] = true
			} else if isFirstBlock {
				usedStyleNames["sF"] = true
				isFirstBlock = false
			} else {
				usedStyleNames["sDC"] = true
			}
		}
	}

	// Collect only used styles.
	usedStyles := make([]builders.StyleDef, 0, len(usedStyleNames))
	styleIDs := make([]string, 0, len(usedStyleNames))
	for name := range usedStyleNames {
		if s, ok := styleMap[name]; ok {
			usedStyles = append(usedStyles, s)
			styleIDs = append(styleIDs, s.ID)
		}
	}

	localSymbols := make([]string, 0, len(contentDefs)+len(sections)*4+len(anchorIDs)+len(imageResourceIDs)*2+len(styleIDs)+8)
	for _, d := range contentDefs {
		localSymbols = append(localSymbols, d.ID)
	}

	tocID := "toc"
	localSymbols = append(localSymbols, tocID)
	localSymbols = append(localSymbols, sectionIDs...)
	localSymbols = append(localSymbols, storylineIDs...)
	localSymbols = append(localSymbols, sectionMetaIDs...)
	localSymbols = append(localSymbols, anchorIDs...)
	localSymbols = append(localSymbols, imageResourceIDs...)
	localSymbols = append(localSymbols, imageLocationIDs...)
	localSymbols = append(localSymbols, styleIDs...)

	yjSST := symbols.SharedYJSymbols(851)
	prolog, err := ionutil.BuildProlog(localSymbols, yjSST)
	if err != nil {
		return fmt.Errorf("build document symbols: %w", err)
	}

	fragments := []model.Fragment{
		{FID: "$538", FType: "$538", Value: builders.BuildDocumentData(readingOrders, maxEID)},
		{FID: "$258", FType: "$258", Value: builders.BuildMetadataReadingOrders(readingOrders)},
		{FID: "$490", FType: "$490", Value: builders.BuildBookMetadata(c, containerID, coverResourceID)},
		{FID: tocID, FType: "$391", Value: builders.BuildNavContainerTOC(tocID, buildTOCItems(tocNodes))},
		{FID: "$389", FType: "$389", Value: builders.BuildNavigation(tocID)},
		{FID: "$585", FType: "$585", Value: builders.BuildConversionFeatures(reflowSectionSize)},

		// $264: position_map (maps EIDs to sections).
		{FID: "$264", FType: "$264", Value: posMaps},

		// $265: position_id_map (PID -> EID mapping, terminated by EID=0).
		{FID: "$265", FType: "$265", Value: pidMap},

		// $550: location_map.
		{FID: "$550", FType: "$550", Value: []locationMapRoot{{ROName: "$351", Locations: locs}}},

		{FID: "$395", FType: "$395", Value: builders.BuildResourcePath()},
	}

	// Add only used style fragments.
	for _, s := range usedStyles {
		fragments = append(fragments, model.Fragment{FID: s.ID, FType: "$157", Value: s.Style})
	}

	for si, s := range sections {
		kids := make([]any, 0, s.Span.EndBlock-s.Span.StartBlock+len(lt.Images))
		isFirstBlock := true
		for bi := s.Span.StartBlock; bi < s.Span.EndBlock && bi < len(lt.Nodes); bi++ {
			eid := lt.Nodes[bi].EID
			// Choose style based on block type:
			// - TOC entries (headings) use sH (chapter heading)
			// - First paragraph in section uses sF (first para, left-aligned)
			// - Other paragraphs use sDC (body text with indent)
			var paraStyle string
			if _, isHeading := eidToAnchor[eid]; isHeading {
				paraStyle = "sH"
			} else if isFirstBlock {
				paraStyle = "sF"
				isFirstBlock = false
			} else {
				paraStyle = "sDC"
			}
			// If this block corresponds to a TOC entry, include anchor reference via style_events.
			if anchorID, ok := eidToAnchor[eid]; ok {
				kids = append(kids, innerNodeWithAnchor{
					StyleEvents: []styleEvent{{Offset: 0, Length: 1, LinkTo: anchorID}},
					EID:         eid,
					Style:       paraStyle,
					Type:        "$269",
					Content:     blockRefs[bi],
				})
			} else {
				kids = append(kids, innerNode{EID: eid, Style: paraStyle, Type: "$269", Content: blockRefs[bi]})
			}
		}

		// Add image nodes only to the first section to ensure they're referenced.
		if si == 0 {
			for _, imgNode := range lt.Images {
				if resourceID, ok := eidToImageRes[imgNode.EID]; ok {
					kids = append(kids, imageStoryNode{
						EID:          imgNode.EID,
						Style:        "s6P",
						Type:         "$271",
						ResourceName: resourceID,
					})
				}
			}
		}

		sectionMetaID := s.SectionID + "-ad"
		fragments = append(fragments,
			model.Fragment{FID: s.StorylineID, FType: "$259", Value: storyline{ID: s.StorylineID, Seq: []any{outerNode{EID: s.OuterEID, Layout: "$323", Style: "sD7", Type: "$270", HeadingLevel: 1, Kids: kids}}}},
			model.Fragment{FID: s.SectionID, FType: "$260", Value: section{ID: s.SectionID, Rows: []any{sectionStoryline{EID: s.SectionEID, SL: s.StorylineID, Layout: "$326", Float: "$320", Type: "$270", FixedWidth: 0, FixedHeight: 0}}}},
			// $597: auxiliary_data (IS_TARGET_SECTION marker).
			model.Fragment{FID: sectionMetaID, FType: "$597", Value: sectionMetadata{
				Items:     []any{map[string]any{"$307": true, "$492": "IS_TARGET_SECTION"}},
				SectionID: sectionMetaID,
			}},
		)
	}

	// $266: anchor (position_anchor) fragments for TOC entries.
	for i, n := range tocFlat {
		anchorID := anchorIDs[i]
		fragments = append(fragments, model.Fragment{
			FID:   anchorID,
			FType: "$266",
			Value: positionAnchor{
				AnchorName: anchorID,
				Position:   map[string]any{"$155": n.EID},
			},
		})
	}

	contentIDs := make([]string, 0, len(contentDefs))
	for _, d := range contentDefs {
		contentIDs = append(contentIDs, d.ID)
		fragments = append(fragments, model.Fragment{FID: d.ID, FType: "$145", Value: contentFragment{Name: d.ID, ContentList: d.Blocks, Selection: 0, ScreenActualHeight: []any{}}})
	}

	// Build image fragments for all images using pre-computed IDs.
	for imgID, ref := range imageRefs {
		img := c.ImagesIndex[imgID]
		if img == nil {
			continue
		}

		format := "$285" // JPEG default
		switch {
		case strings.Contains(img.MimeType, "png"):
			format = "$286"
		case strings.Contains(img.MimeType, "gif"):
			format = "$284"
		case strings.Contains(img.MimeType, "svg"):
			format = "$548"
		case strings.Contains(img.MimeType, "webp"):
			format = "$565"
		}

		fragments = append(fragments,
			model.Fragment{
				FID:   ref.ResourceID,
				FType: "$164",
				Value: builders.BuildExternalResource(ref.ResourceID, ref.Location, img.Dim.Width, img.Dim.Height, format),
			},
			model.Fragment{FID: ref.Location, FType: "$417", Value: img.Data},
		)
	}

	// Entity map must reference all non-singleton fragment IDs.
	entityMapIDs := make([]string, 0, 1+len(sectionIDs)+len(storylineIDs)+len(contentIDs)+len(sectionMetaIDs)+len(anchorIDs)+len(imageResourceIDs)*2+len(styleIDs)+4)
	entityMapIDs = append(entityMapIDs, tocID)
	entityMapIDs = append(entityMapIDs, sectionIDs...)
	entityMapIDs = append(entityMapIDs, storylineIDs...)
	entityMapIDs = append(entityMapIDs, contentIDs...)
	entityMapIDs = append(entityMapIDs, sectionMetaIDs...)
	entityMapIDs = append(entityMapIDs, anchorIDs...)
	entityMapIDs = append(entityMapIDs, imageResourceIDs...)
	entityMapIDs = append(entityMapIDs, imageLocationIDs...)
	entityMapIDs = append(entityMapIDs, styleIDs...)

	mainSectionID := sectionIDs[0]
	sectionResources := make([]string, 0, len(imageResourceIDs)+1)
	if coverResourceID != "" {
		sectionResources = append(sectionResources, coverResourceID)
	}

	fragments = append(fragments, model.Fragment{FID: "$419", FType: "$419", Value: builders.BuildEntityMap(containerID, entityMapIDs, mainSectionID, sectionResources)})

	if err := dumpDebug(c, containerID, prolog, fragments, log); err != nil {
		return fmt.Errorf("store kfx debug dumps: %w", err)
	}

	data, err := container.Pack(&container.PackParams{
		ContainerID:              containerID,
		KfxgenApplicationVersion: "kfxlib-20251012",
		KfxgenPackageVersion:     "",
		DocumentSymbols:          prolog.DocSymbols,
		FormatCapabilities:       builders.BuildFormatCapabilities(),
		Prolog:                   prolog,
		Fragments:                fragments,
	})
	if err != nil {
		return fmt.Errorf("pack kfx: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return err
	}

	return nil
}
