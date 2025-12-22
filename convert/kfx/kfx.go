package kfx

import (
	"context"
	"fmt"
	"os"
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

	// if true {
	// 	return fmt.Errorf("KFX generation is experimental and not yet implemented")
	// }

	log.Info("Generating KFX", zap.String("output", outputPath))

	containerID := "CR!" + c.Book.Description.DocumentInfo.ID

	// Use numeric symbol IDs ("$701" etc) to avoid depending on document-local symbols.
	sectionID := "$701"
	storylineID := "$705"
	coverResourceID := "$702"
	coverMediaID := "$703"

	readingOrders := builders.BuildReadingOrders([]string{sectionID})

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
	type innerNode struct {
		EID     int64      `ion:"$155"`
		Type    string     `ion:"$159,symbol"`
		Content contentRef `ion:"$145"`
	}
	type outerNode struct {
		EID          int64  `ion:"$155"`
		Layout       string `ion:"$156,symbol"`
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

	eidTextBase := int64(2000)
	eidStoryOuter := int64(3000)

	lt := linearizeMainText(c, 4000)
	contentBlocks := lt.Blocks

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

	type contentChunk struct {
		EID    int64
		Length int64
	}
	chunks := make([]contentChunk, 0, 2+len(lt.Nodes))
	chunks = append(chunks, contentChunk{EID: eidTextBase, Length: 1}, contentChunk{EID: eidStoryOuter, Length: 1})
	for i, n := range lt.Nodes {
		chunks = append(chunks, contentChunk{EID: n.EID, Length: int64(utf8.RuneCountInString(contentBlocks[i]))})
	}

	positionEIDs := make([]int64, 0, len(chunks))
	for _, ch := range chunks {
		positionEIDs = append(positionEIDs, ch.EID)
	}

	pidMap := make([]pidMapEntry, 0, len(chunks)+1)
	pid := int64(0)
	for _, ch := range chunks {
		pidMap = append(pidMap, pidMapEntry{PID: pid, EID: ch.EID})
		pid += ch.Length
	}
	pidEnd := pid
	pidMap = append(pidMap, pidMapEntry{PID: pidEnd, EID: 0})

	locs := make([]location, 0, len(chunks))
	for _, ch := range chunks {
		locs = append(locs, location{EID: ch.EID, Offset: 0})
	}

	kids := make([]any, 0, len(lt.Nodes))
	for i, n := range lt.Nodes {
		kids = append(kids, innerNode{EID: n.EID, Type: "$269", Content: blockRefs[i]})
	}

	localSymbols := make([]string, 0, len(contentDefs)+8)
	for _, d := range contentDefs {
		localSymbols = append(localSymbols, d.ID)
	}

	tocID := "toc"
	localSymbols = append(localSymbols, tocID)

	yjSST := symbols.SharedYJSymbols(851)
	prolog, err := ionutil.BuildProlog(localSymbols, yjSST)
	if err != nil {
		return fmt.Errorf("build document symbols: %w", err)
	}

	reflowSectionSize := int64(1)
	if pidEnd > 65536 {
		reflowSectionSize = ((pidEnd - 65536) / (16 * 1024)) + 2
		if reflowSectionSize > 256 {
			reflowSectionSize = 256
		}
	}

	fragments := []model.Fragment{
		{FID: "$538", FType: "$538", Value: builders.BuildDocumentData(readingOrders, 851)},
		{FID: "$258", FType: "$258", Value: builders.BuildMetadataReadingOrders(readingOrders)},
		{FID: "$490", FType: "$490", Value: builders.BuildBookMetadata(c, containerID, coverResourceID)},
		{FID: tocID, FType: "$391", Value: builders.BuildNavContainerTOC(tocID)},
		{FID: "$389", FType: "$389", Value: builders.BuildNavigation(tocID)},
		{FID: "$585", FType: "$585", Value: builders.BuildConversionFeatures(reflowSectionSize)},

		// $264: position_map (maps EIDs to sections).
		{FID: "$264", FType: "$264", Value: []positionMapSection{{Section: sectionID, EIDs: positionEIDs}}},

		// $265: position_id_map (PID -> EID mapping, terminated by EID=0).
		{FID: "$265", FType: "$265", Value: pidMap},

		// $550: location_map.
		{FID: "$550", FType: "$550", Value: []locationMapRoot{{ROName: "$351", Locations: locs}}},

		{FID: "$395", FType: "$395", Value: builders.BuildResourcePath()},
		{FID: storylineID, FType: "$259", Value: storyline{ID: storylineID, Seq: []any{outerNode{EID: eidStoryOuter, Layout: "$323", Type: "$270", HeadingLevel: 1, Kids: kids}}}},
		{FID: sectionID, FType: "$260", Value: section{ID: sectionID, Rows: []any{sectionStoryline{EID: eidTextBase, SL: storylineID, Layout: "$326", Float: "$320", Type: "$270", FixedWidth: 0, FixedHeight: 0}}}},
	}

	contentIDs := make([]string, 0, len(contentDefs))
	for _, d := range contentDefs {
		contentIDs = append(contentIDs, d.ID)
		fragments = append(fragments, model.Fragment{FID: d.ID, FType: "$145", Value: contentFragment{Name: d.ID, ContentList: d.Blocks, Selection: 0, ScreenActualHeight: []any{}}})
	}

	// Cover image.
	if c.CoverID != "" {
		if img := c.ImagesIndex[c.CoverID]; img != nil {
			fragments = append(fragments,
				model.Fragment{
					FID:   coverResourceID,
					FType: "$164",
					Value: builders.BuildExternalResource(coverResourceID, coverMediaID, img.Dim.Width, img.Dim.Height, "$285"),
				},
				model.Fragment{FID: coverMediaID, FType: "$417", Value: img.Data},
			)
		}
	}

	// Entity map must reference all non-singleton fragment IDs.
	entityMapIDs := make([]string, 0, 3+len(contentIDs))
	entityMapIDs = append(entityMapIDs, sectionID, storylineID, tocID)
	entityMapIDs = append(entityMapIDs, contentIDs...)
	sectionResources := []string{}
	if c.CoverID != "" {
		entityMapIDs = append(entityMapIDs, coverResourceID, coverMediaID)
		sectionResources = append(sectionResources, coverResourceID)
	}

	fragments = append(fragments, model.Fragment{FID: "$419", FType: "$419", Value: builders.BuildEntityMap(containerID, entityMapIDs, sectionID, sectionResources)})

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
