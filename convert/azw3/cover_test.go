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

func TestPrepareCoverDocument(t *testing.T) {
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
			prepared, err := prepareCoverDocument(context.Background(), pub, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared.Chapters) != 2 || prepared.Chapters[0].Filename != "cover.xhtml" || len(prepared.Landmarks) != 2 ||
				prepared.Landmarks[0] != pub.Landmarks[0] {
				t.Error("cover preparation changed original reading/landmark semantics")
			}
			svg := prepared.Chapters[0].Doc.FindElement("//svg")
			wantViewBox, wantAspect := "0 0 2 2", "xMidYMid meet"
			if mode == common.ImageResizeModeStretch {
				wantViewBox, wantAspect = "0 0 100 100", "xMidYMid slice"
			}
			if svg.SelectAttrValue("viewBox", "") != wantViewBox || svg.SelectAttrValue("preserveAspectRatio", "") != wantAspect {
				t.Error("native cover wrapper differs from the existing cover resize policy")
			}
			text, err := compileText(context.Background(), prepared)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(text.Documents[0].Data, []byte(`xlink:href="kindle:embed:0002?mime=image/jpeg"`)) {
				t.Error("inline SVG cover raster reference was not lowered before layout freeze")
			}
			for _, page := range pub.Pages {
				if _, found := text.Positions[linkTarget{Filename: page.Filename}]; !found {
					t.Errorf("page %d has no cover/text reading position", page.PageNum)
				}
			}
			navigation, err := compileNavigation(context.Background(), prepared, text, common.TOCTypeNormal)
			if err != nil {
				t.Fatal(err)
			}
			if navigation.Start == nil || navigation.Start.FID != 1 {
				t.Error("adding cover must not redirect start-reading away from original text")
			}
			if !reflect.DeepEqual(before, snapshotPublication(t, pub)) || len(pub.Chapters) != 1 || len(pub.Landmarks) != 1 {
				t.Error("cover preparation mutated the shared publication")
			}
		})
	}
}

func TestPrepareCoverDocumentValidation(t *testing.T) {
	pub, _ := resourceTestPublication(t)
	cfg := imageTestConfig()
	if got, err := prepareCoverDocument(context.Background(), pub, cfg); err != nil || got != pub {
		t.Error("coverless publication should not acquire an extra document")
	}
	pub.CoverID = "missing"
	if _, err := prepareCoverDocument(context.Background(), pub, cfg); err == nil {
		t.Error("missing cover resource accepted")
	}
	pub.CoverID = "a"
	pub.Chapters[0].Filename = "cover.xhtml"
	if _, err := prepareCoverDocument(context.Background(), pub, cfg); err == nil {
		t.Error("cover wrapper overwrote an existing document")
	}
	pub.Chapters[0].Filename = "chapter.xhtml"
	pub.Landmarks = []publication.Landmark{{Kind: "cover", Href: "chapter.xhtml"}}
	if _, err := prepareCoverDocument(context.Background(), pub, cfg); err == nil {
		t.Error("duplicate cover landmark accepted")
	}
	if _, err := prepareCoverDocument(context.Background(), nil, nil); err == nil {
		t.Error("missing cover inputs accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := prepareCoverDocument(ctx, nil, nil); !errors.Is(err, context.Canceled) {
		t.Error("cover preparation did not preserve cancellation")
	}
}
