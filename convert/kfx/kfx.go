package kfx

import (
	"context"
	"fmt"
	"os"

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

	log.Info("Generating KFX", zap.String("output", outputPath))

	containerID := "CR!" + c.Book.Description.DocumentInfo.ID

	// Use numeric symbol IDs ("$701" etc) to avoid depending on document-local symbols.
	sectionID := "$701"
	storylineID := "$705"
	contentID := "$704"
	coverResourceID := "$702"
	coverMediaID := "$703"

	yjSST := symbols.SharedYJSymbols(842)
	prolog, err := ionutil.BuildProlog(nil, yjSST)
	if err != nil {
		return fmt.Errorf("build document symbols: %w", err)
	}

	readingOrders := builders.BuildReadingOrders([]string{sectionID})

	// Minimal content fragment.
	type contentFragment struct {
		Name string   `ion:"name,symbol"`
		T    []string `ion:"$146"`
		V436 int64    `ion:"$436"`
		V305 []any    `ion:"$305"`
	}

	// Minimal storyline that references the content fragment.
	type contentRef struct {
		Name string `ion:"name,symbol"`
		V403 int64  `ion:"$403"`
	}
	type innerNode struct {
		EID  int64      `ion:"$155"`
		V159 string     `ion:"$159,symbol"`
		V145 contentRef `ion:"$145"`
	}
	type outerNode struct {
		EID  int64  `ion:"$155"`
		V156 string `ion:"$156,symbol"`
		V159 string `ion:"$159,symbol"`
		Kids []any  `ion:"$146"`
		V790 int64  `ion:"$790"`
	}
	type storyline struct {
		ID  string `ion:"$176,symbol"`
		Seq []any  `ion:"$146"`
	}

	// Minimal section that references the storyline.
	type sectionStoryline struct {
		EID  int64  `ion:"$155"`
		SL   string `ion:"$176,symbol"`
		V156 string `ion:"$156,symbol"`
		V140 string `ion:"$140,symbol"`
		V159 string `ion:"$159,symbol"`
		V66  int64  `ion:"$66"`
		V67  int64  `ion:"$67"`
	}
	type section struct {
		ID   string `ion:"$174,symbol"`
		Rows []any  `ion:"$141"`
	}

	// Minimal location / PID maps.
	type locEntry struct {
		EID int64 `ion:"$155"`
		Off int64 `ion:"$143"`
	}
	type locationMap struct {
		Entries []locEntry `ion:"$181"`
	}
	type pidMapWithOffsetEntry struct {
		Range []int64 `ion:"$181"`
		Sec   string  `ion:"$174,symbol"`
	}
	type pidMapEntry struct {
		K int64 `ion:"$184"`
		V int64 `ion:"$185"`
	}

	fragments := []model.Fragment{
		{FID: "$538", FType: "$538", Value: builders.BuildDocumentData(readingOrders)},
		{FID: "$258", FType: "$258", Value: builders.BuildMetadataReadingOrders(readingOrders)},
		{FID: "$490", FType: "$490", Value: builders.BuildBookMetadata(c, containerID, coverResourceID)},
		{FID: "$389", FType: "$389", Value: builders.BuildNavigation()},
		{FID: "$550", FType: "$550", Value: locationMap{Entries: []locEntry{{EID: 0, Off: 0}}}},
		{FID: "$264", FType: "$264", Value: []pidMapWithOffsetEntry{{Range: []int64{0, 1}, Sec: sectionID}}},
		{FID: "$265", FType: "$265", Value: []pidMapEntry{{K: 0, V: 0}}},
		{FID: "$395", FType: "$395", Value: builders.BuildResourcePath()},
		{FID: contentID, FType: "$145", Value: contentFragment{Name: contentID, T: []string{c.Book.Description.TitleInfo.BookTitle.Value}, V436: 0, V305: []any{}}},
		{FID: storylineID, FType: "$259", Value: storyline{ID: storylineID, Seq: []any{outerNode{EID: 0, V156: "$323", V159: "$270", V790: 1, Kids: []any{innerNode{EID: 0, V159: "$269", V145: contentRef{Name: contentID, V403: 0}}}}}}},
		{FID: sectionID, FType: "$260", Value: section{ID: sectionID, Rows: []any{sectionStoryline{EID: 0, SL: storylineID, V156: "$326", V140: "$320", V159: "$270", V66: 0, V67: 0}}}},
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
	entityMapIDs := []string{sectionID, storylineID, contentID}
	sectionResources := []string{}
	if c.CoverID != "" {
		entityMapIDs = append(entityMapIDs, coverResourceID, coverMediaID)
		sectionResources = append(sectionResources, coverResourceID)
	}

	fragments = append(fragments, model.Fragment{FID: "$419", FType: "$419", Value: builders.BuildEntityMap(containerID, entityMapIDs, sectionID, sectionResources)})

	data, err := container.Pack(&container.PackParams{
		ContainerID:              containerID,
		KfxgenApplicationVersion: "kfxlib-20251012",
		KfxgenPackageVersion:     "",
		DocumentSymbols:          prolog.Bytes,
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
