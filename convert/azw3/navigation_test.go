package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/beevik/etree"

	"fbc/common"
	"fbc/convert/publication"
	"fbc/convert/tocnav"
)

func navigationTestPublication(t *testing.T) *publication.Publication {
	t.Helper()
	pub := textTestPublication(t)
	pub.Navigation = []tocnav.Item{
		{Title: "Intro & <Title>", Href: "Text/first.xhtml#first", Level: 1},
		{Title: "Repeated", Href: "Text/first.xhtml#same", Level: 1},
		{Title: "Repeated", Href: "Text/first.xhtml#next", Level: 2},
		{Title: "Grandchild", Href: "Text/first.xhtml#external", Level: 3},
		{Title: "Second", Href: "Text/second.xhtml", Level: 2},
		{Title: "Notes é", Href: "Notes/notes.xhtml#n_%C3%A9", Level: 1},
	}
	pub.Landmarks = []publication.Landmark{
		{Kind: "toc", Title: "Contents", Href: "Text/second.xhtml"},
		{Kind: "bodymatter", Title: "Start", Href: "Text/first.xhtml"},
	}
	return pub
}

func indexWithoutStrings(t *testing.T, records [][]byte) [][]byte {
	t.Helper()
	count := binary.BigEndian.Uint32(records[0][24:])
	return records[:1+count]
}

func TestCompileNavigation(t *testing.T) {
	pub := navigationTestPublication(t)
	text, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	rawBefore := bytes.Clone(text.Raw)
	landmarksBefore, navBefore := slices.Clone(pub.Landmarks), slices.Clone(pub.Navigation)
	compiled, err := compileNavigation(context.Background(), pub, text, common.TOCTypeNormal)
	if err != nil {
		t.Fatal(err)
	}
	positions := []textPosition{
		text.Positions[linkTarget{"Text/first.xhtml", "first"}], text.Positions[linkTarget{"Text/first.xhtml", "same"}],
		text.Positions[linkTarget{"Notes/notes.xhtml", "n_é"}], text.Positions[linkTarget{"Text/first.xhtml", "next"}],
		text.Positions[linkTarget{Filename: "Text/second.xhtml"}], text.Positions[linkTarget{"Text/first.xhtml", "external"}],
	}
	titles := []string{"Intro & <Title>", "Repeated", "Notes é", "Repeated", "Second", "Grandchild"}
	levels := []uint32{0, 0, 0, 1, 1, 2}
	parents := []uint32{missingRecord, missingRecord, missingRecord, 1, 1, 3}
	ends := []uint32{
		positions[1].Position, positions[2].Position, text.MainLength,
		positions[4].Position, positions[2].Position, positions[4].Position,
	}
	var entries []indexEntry
	var nextOffset uint32
	labelOffsets := make(map[string]uint32)
	for i, position := range positions {
		offset, exists := labelOffsets[titles[i]]
		if !exists {
			offset = nextOffset
			labelOffsets[titles[i]] = offset
			nextOffset += 1 + uint32(len(titles[i]))
		}
		values := map[byte][]uint32{
			1: {position.Position}, 2: {ends[i] - position.Position}, 3: {offset},
			4: {levels[i]}, 6: {position.FID, position.Offset},
		}
		if parents[i] != missingRecord {
			values[21] = []uint32{parents[i]}
		}
		if i == 1 {
			values[22], values[23] = []uint32{3}, []uint32{4}
		} else if i == 3 {
			values[22], values[23] = []uint32{5}, []uint32{5}
		}
		entries = append(entries, indexEntry{Key: fmt.Sprintf("%X", i), Values: values})
	}
	assertIndexEntries(t, indexWithoutStrings(t, compiled.TOC), entries)
	start := text.Positions[linkTarget{Filename: "Text/first.xhtml"}]
	second := text.Positions[linkTarget{Filename: "Text/second.xhtml"}]
	assertIndexEntries(t, indexWithoutStrings(t, compiled.Guide), []indexEntry{
		{Key: "text", Values: map[byte][]uint32{1: {0}, 6: {start.FID, start.Offset}}},
		{Key: "toc", Values: map[byte][]uint32{1: {6}, 6: {second.FID, second.Offset}}},
	})
	if compiled.Start == nil || *compiled.Start != start {
		t.Error("start-reading metadata coordinate was not retained")
	}
	again, err := compileNavigation(context.Background(), pub, text, common.TOCTypeNormal)
	if err != nil || !reflect.DeepEqual(again, compiled) {
		t.Error("binary navigation compilation is not deterministic")
	}
	if !bytes.Equal(rawBefore, text.Raw) || !reflect.DeepEqual(pub.Landmarks, landmarksBefore) || !reflect.DeepEqual(pub.Navigation, navBefore) {
		t.Error("navigation compilation changed shared publication content or frozen text")
	}
}

func TestCompileNavigationRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*publication.Publication, *textLayout)
	}{
		{"external TOC target", func(p *publication.Publication, _ *textLayout) { p.Navigation[0].Href = "https://example.org" }},
		{"missing anchor", func(p *publication.Publication, _ *textLayout) { p.Navigation[0].Href = "Text/first.xhtml#missing" }},
		{"reversed targets", func(p *publication.Publication, _ *textLayout) {
			p.Navigation[0], p.Navigation[5] = p.Navigation[5], p.Navigation[0]
		}},
		{"bad title encoding", func(p *publication.Publication, _ *textLayout) { p.Navigation[0].Title = string([]byte{0xff}) }},
		{"duplicate guide kind", func(p *publication.Publication, _ *textLayout) { p.Landmarks = append(p.Landmarks, p.Landmarks[0]) }},
		{"unknown guide kind", func(p *publication.Publication, _ *textLayout) { p.Landmarks[0].Kind = "unknown" }},
		{"missing guide target", func(p *publication.Publication, _ *textLayout) { p.Landmarks[0].Href = "missing.xhtml" }},
		{"position in CSS flow", func(_ *publication.Publication, text *textLayout) {
			key := linkTarget{"Text/first.xhtml", "first"}
			position := text.Positions[key]
			position.Position = text.MainLength
			text.Positions[key] = position
		}},
		{"inconsistent fragment offset", func(_ *publication.Publication, text *textLayout) {
			key := linkTarget{"Text/first.xhtml", "first"}
			position := text.Positions[key]
			position.Offset++
			text.Positions[key] = position
		}},
		{"inconsistent main length", func(_ *publication.Publication, text *textLayout) { text.MainLength++ }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pub := navigationTestPublication(t)
			text, err := compileText(context.Background(), pub)
			if err != nil {
				t.Fatal(err)
			}
			tt.change(pub, text)
			if result, err := compileNavigation(context.Background(), pub, text, common.TOCTypeNormal); err == nil || result != nil {
				t.Error("invalid navigation accepted")
			}
		})
	}
	if _, err := compileNavigation(context.Background(), nil, nil, common.TOCTypeNormal); err == nil {
		t.Error("missing inputs accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := compileNavigation(ctx, nil, nil, common.TOCTypeNormal); !errors.Is(err, context.Canceled) {
		t.Error("cancellation must precede input validation")
	}
}

func TestNavigationEmptyAndGuideOffsets(t *testing.T) {
	pub := textTestPublication(t)
	text, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compileNavigation(context.Background(), pub, text, common.TOCTypeNormal)
	if err != nil || len(compiled.TOC) != 0 || len(compiled.Guide) != 0 || compiled.Start != nil {
		t.Fatalf("empty logical navigation should omit its binary indexes: %v", err)
	}
	pub.Landmarks = []publication.Landmark{{Kind: "bodymatter", Title: "Start", Href: "Text/first.xhtml#same"}}
	compiled, err = compileNavigation(context.Background(), pub, text, common.TOCTypeNormal)
	if err != nil {
		t.Fatal(err)
	}
	position := text.Positions[linkTarget{"Text/first.xhtml", "same"}]
	if position.Offset == 0 || compiled.Start == nil || *compiled.Start != position {
		t.Error("nonzero guide/start offset was discarded")
	}
	assertIndexEntries(t, indexWithoutStrings(t, compiled.Guide), []indexEntry{
		{Key: "text", Values: map[byte][]uint32{1: {0}, 6: {position.FID, position.Offset}}},
	})
	if _, err := compileNavigation(context.Background(), pub, text, common.TOCType(-1)); err == nil {
		t.Error("unknown TOC hierarchy accepted")
	}
}

func TestNavigationKindleUnpack(t *testing.T) {
	for _, shape := range []common.TOCType{common.TOCTypeNormal, common.TOCTypeOldKindle, common.TOCTypeFlat} {
		t.Run(shape.String(), func(t *testing.T) {
			pub := navigationTestPublication(t)
			text, err := compileText(context.Background(), pub)
			if err != nil {
				t.Fatal(err)
			}
			navigation, err := compileNavigation(context.Background(), pub, text, shape)
			if err != nil {
				t.Fatal(err)
			}
			name := "publication-navigation-" + shape.String()
			data := assembleTextProbe(t, text, navigation, nil)
			writeProbeArtifacts(t, map[string][]byte{name + ".azw3": data})
			work := unpackProbe(t, name, data)
			assertUnpackedNavigation(t, work, pub, text, shape)
		})
	}
}

func TestGuideOnlyKindleUnpack(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("guide=%t", enabled), func(t *testing.T) {
			pub := textTestPublication(t)
			if enabled {
				pub.Landmarks = []publication.Landmark{{Kind: "bodymatter", Title: "Start", Href: "Text/first.xhtml"}}
			}
			text, err := compileText(context.Background(), pub)
			if err != nil {
				t.Fatal(err)
			}
			navigation, err := compileNavigation(context.Background(), pub, text, common.TOCTypeNormal)
			if err != nil {
				t.Fatal(err)
			}
			name := fmt.Sprintf("publication-guide-only-%t", enabled)
			data := assembleTextProbe(t, text, navigation, nil)
			writeProbeArtifacts(t, map[string][]byte{name + ".azw3": data})
			work := unpackProbe(t, name, data)
			assertUnpackedNavigation(t, work, pub, text, common.TOCTypeNormal)
		})
	}
}

func assertUnpackedNavigation(
	t *testing.T, work string, pub *publication.Publication, text *textLayout, shape common.TOCType,
) {
	t.Helper()
	base := filepath.Join(work, "unpacked", "mobi8", "OEBPS")
	readXML := func(filename string) *etree.Document {
		t.Helper()
		data, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		doc := etree.NewDocument()
		if err := doc.ReadFromBytes(data); err != nil {
			t.Fatal(err)
		}
		return doc
	}
	fileNames := make(map[string]string)
	for i, doc := range text.Documents {
		fileNames[doc.Filename] = fmt.Sprintf("part%04d.xhtml", i)
	}
	checkTarget := func(href, reconstructed string) {
		t.Helper()
		target, _, err := resolveTextTarget("nav.xhtml", href)
		if err != nil {
			t.Fatal(err)
		}
		uri, err := url.Parse(reconstructed)
		if err != nil || path.Base(uri.Path) != fileNames[target.Filename] || (target.ID != "" && uri.Fragment != target.ID) {
			t.Errorf("navigation destination %s reconstructed as %s", href, reconstructed)
		}
	}
	if len(pub.Navigation) > 0 {
		ncx := readXML(filepath.Join(base, "toc.ncx"))
		var checkNodes func([]*etree.Element, []*tocnav.Node)
		checkNodes = func(got []*etree.Element, want []*tocnav.Node) {
			t.Helper()
			if len(got) != len(want) {
				t.Fatalf("TOC sibling count changed: got %d, want %d", len(got), len(want))
			}
			for i, node := range want {
				if got[i].FindElement("navLabel/text").Text() != node.Item.Title {
					t.Errorf("TOC label changed: got %q, want %q", got[i].FindElement("navLabel/text").Text(), node.Item.Title)
				}
				checkTarget(node.Item.Href, got[i].SelectElement("content").SelectAttrValue("src", ""))
				checkNodes(got[i].SelectElements("navPoint"), node.Children)
			}
		}
		checkNodes(ncx.FindElement("//navMap").SelectElements("navPoint"), tocnav.Shape(pub.Navigation, shape))
	}
	opfFiles, err := filepath.Glob(filepath.Join(base, "*.opf"))
	if err != nil || len(opfFiles) != 1 {
		t.Fatal("expected one independently reconstructed OPF")
	}
	opf := readXML(opfFiles[0])
	refs := opf.FindElements("//guide/reference")
	if len(refs) != len(pub.Landmarks) {
		t.Fatalf("guide reference count changed: got %d, want %d", len(refs), len(pub.Landmarks))
	}
	for _, landmark := range pub.Landmarks {
		kind := guideType(landmark.Kind)
		found := false
		for _, ref := range refs {
			if ref.SelectAttrValue("type", "") != kind {
				continue
			}
			found = true
			if ref.SelectAttrValue("title", "") != landmark.Title {
				t.Errorf("guide label changed for %s", kind)
			}
			checkTarget(landmark.Href, ref.SelectAttrValue("href", ""))
		}
		if !found {
			t.Errorf("missing reconstructed guide target %s", kind)
		}
	}
}

func TestNavigationLargeLabelPools(t *testing.T) {
	pub := textTestPublication(t)
	for i := range 6000 {
		pub.Navigation = append(pub.Navigation, tocnav.Item{
			Title: strings.Repeat("Кириллица ", 10) + fmt.Sprint(i), Href: "Text/first.xhtml#first", Level: 1,
		})
	}
	text, err := compileText(context.Background(), pub)
	if err != nil {
		t.Fatal(err)
	}
	navigation, err := compileNavigation(context.Background(), pub, text, common.TOCTypeFlat)
	if err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(navigation.TOC[0][24:]) < 2 || binary.BigEndian.Uint32(navigation.TOC[0][52:]) < 2 {
		t.Fatal("fixture must exercise multiple INDX data records and CTOC string records")
	}
	if os.Getenv("AZW3_KINDLEUNPACK_TEST") == "1" {
		data := assembleTextProbe(t, text, navigation, nil)
		writeProbeArtifacts(t, map[string][]byte{"publication-large-navigation.azw3": data})
		work := unpackProbe(t, "publication-large-navigation", data)
		assertUnpackedNavigation(t, work, pub, text, common.TOCTypeFlat)
	}
}
