package azw3

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"fbc/common"
	"fbc/content"
	"fbc/convert/publication"
)

func TestPrepareCoverPublication(t *testing.T) {
	for _, mode := range []common.ImageResizeMode{
		common.ImageResizeModeNone, common.ImageResizeModeKeepAR, common.ImageResizeModeStretch, common.ImageResizeModeFit,
	} {
		t.Run(mode.String(), func(t *testing.T) {
			pub, _ := resourceTestPublication(t)
			pub.CoverID = "a"
			pub.Landmarks = []publication.Landmark{{Kind: "bodymatter", Title: "Start", Href: "chapter.xhtml"}}
			pub.Pages = []content.PageMapEntry{{PageNum: 1, Filename: "cover.xhtml"}, {PageNum: 2, Filename: "chapter.xhtml"}}
			before := snapshotPublication(t, pub)
			cfg := imageTestConfig()
			cfg.Cover.Resize = mode
			prepared, err := prepareCoverPublication(context.Background(), pub, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared.Chapters) != 1 || prepared.Chapters[0].Filename == "cover.xhtml" || len(prepared.Landmarks) != 1 ||
				prepared.Landmarks[0] != pub.Landmarks[0] {
				t.Error("cover preparation changed original reading/landmark semantics")
			}
			if len(prepared.Pages) != 1 || prepared.Pages[0].PageNum != 2 || len(pub.Pages) != 2 {
				t.Fatal("virtual cover must omit only its synthetic page without renumbering or mutating source pages")
			}
			text, err := compileText(context.Background(), prepared)
			if err != nil {
				t.Fatal(err)
			}
			cover := prepared.Images[prepared.CoverID]
			reference := text.References[cover.Filename]
			if reference.Kind != resourceImage || !bytes.Equal(text.Resources[reference.Number-1], cover.Data) {
				t.Fatal("virtual cover must retain its original prepared raster resource")
			}
			if _, found := text.Positions[linkTarget{Filename: "cover.xhtml"}]; found {
				t.Fatal("virtual cover acquired a physical reading position")
			}
			for _, page := range prepared.Pages {
				if _, found := text.Positions[linkTarget{Filename: page.Filename}]; !found {
					t.Errorf("page %d has no cover/text reading position", page.PageNum)
				}
			}
			navigation, err := compileNavigation(context.Background(), prepared, text, common.TOCTypeNormal)
			if err != nil {
				t.Fatal(err)
			}
			if navigation.Start == nil || navigation.Start.FID != 0 {
				t.Error("adding cover must not redirect start-reading away from original text")
			}
			if !reflect.DeepEqual(before, snapshotPublication(t, pub)) || len(pub.Chapters) != 1 || len(pub.Landmarks) != 1 {
				t.Error("cover preparation mutated the shared publication")
			}
		})
	}
}

func TestPrepareCoverPublicationValidation(t *testing.T) {
	pub, _ := resourceTestPublication(t)
	cfg := imageTestConfig()
	if got, err := prepareCoverPublication(context.Background(), pub, cfg); err != nil || got != pub {
		t.Error("coverless publication should not acquire an extra document")
	}
	pub.CoverID = "missing"
	if _, err := prepareCoverPublication(context.Background(), pub, cfg); err == nil {
		t.Error("missing cover resource accepted")
	}
	pub.CoverID = "a"
	pub.Chapters[0].Filename = "cover.xhtml"
	if _, err := prepareCoverPublication(context.Background(), pub, cfg); err == nil {
		t.Error("cover wrapper overwrote an existing document")
	}
	pub.Chapters[0].Filename = "chapter.xhtml"
	pub.Landmarks = []publication.Landmark{{Kind: "cover", Href: "chapter.xhtml"}}
	if _, err := prepareCoverPublication(context.Background(), pub, cfg); err == nil {
		t.Error("duplicate cover landmark accepted")
	}
	if _, err := prepareCoverPublication(context.Background(), nil, nil); err == nil {
		t.Error("missing cover inputs accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := prepareCoverPublication(ctx, nil, nil); !errors.Is(err, context.Canceled) {
		t.Error("cover preparation did not preserve cancellation")
	}
}
