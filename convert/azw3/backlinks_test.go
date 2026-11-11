package azw3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/beevik/etree"

	"fbc/content"
)

func TestCompileBacklinks(t *testing.T) {
	for _, template := range []string{
		"[loc {{.LocationNumber}}]",
		"{{.Context}}|{{.Format}}|{{.TargetID}}|{{.RefID}}|{{.RefNumber}}|{{.Filename}}|" +
			"{{.Href}}|{{.PageNumber}}|{{.LocationNumber}}|{{.ChapterTitle}}|{{.SectionTitle}}",
		"plain label", "{{.MissingField}}", "",
	} {
		t.Run(template, func(t *testing.T) {
			pub := textTestPublication(t)
			notes := pub.Chapters[2].Doc
			back := notes.FindElement("//a[@id='back']")
			back.SetText("old label")
			back2 := notes.FindElement("//aside").CreateElement("a")
			back2.CreateAttr("id", "back2")
			back2.CreateAttr("href", "#ref2")
			back2.SetText("old label")
			ref2 := notes.FindElement("//aside").CreateElement("a")
			ref2.CreateAttr("id", "ref2")
			ref2.CreateAttr("href", "#n_é")
			ref2.SetText("Cyclic reference after its own generated backlink")
			pub.Backlinks = map[*etree.Element]string{back: "ref1", back2: "ref2"}
			refs := []content.BackLinkRef{
				{RefID: "ref1", TargetID: "n_é", Filename: "Text/first.xhtml", Href: "Text/first.xhtml#ref1",
					Format: "azw3", RefNumber: 1, PageNumber: 3, ChapterTitle: "Chapter café", SectionTitle: "Section 🙂"},
				{RefID: "ref2", TargetID: "n_é", Filename: "Notes/notes.xhtml", Href: "Notes/notes.xhtml#ref2",
					Format: "azw3", RefNumber: 2, PageNumber: 9, ChapterTitle: "Notes", SectionTitle: "Note"},
			}
			c := &content.Content{BackLinkIndex: map[string][]content.BackLinkRef{"n_é": refs}, BacklinkTemplate: template}
			before := snapshotPublication(t, pub)
			resolved, text, err := compileBacklinks(t.Context(), c, pub)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, snapshotPublication(t, pub)) {
				t.Fatal("backlink resolution mutated the source publication")
			}
			for element, refID := range resolved.Backlinks {
				ref, found := c.BackLinkRefByID(refID)
				if !found || ref.LocationNumber <= 0 {
					t.Fatal("missing final reference location", refID)
				}
				var base int
				for _, doc := range text.Documents {
					if doc.Filename != ref.Filename {
						base += len(doc.Data)
						continue
					}
					id := bytes.Index(doc.Data, []byte(`id="`+refID+`"`))
					if id < 0 {
						t.Fatal("missing rendered reference", refID)
					}
					start := bytes.LastIndexByte(doc.Data[:id], '<')
					if want := (base+start)/150 + 1; ref.LocationNumber != want {
						t.Fatalf("location for %s = %d, want %d from final document bytes", refID, ref.LocationNumber, want)
					}
					break
				}
				if element.Text() != c.BacklinkText(ref) {
					t.Fatal("generated label does not use all final template fields", element.Text())
				}
			}
			if template == "[loc {{.LocationNumber}}]" && c.BackLinkIndex["n_é"][0].LocationNumber >= c.BackLinkIndex["n_é"][1].LocationNumber {
				t.Fatal("repeated references must use their individual source locations")
			}
			if !strings.Contains(template, "LocationNumber") {
				for _, ref := range c.BackLinkIndex["n_é"] {
					if ref.RefNumber == 1 && (ref.PageNumber != 3 || ref.SectionTitle != "Section 🙂") {
						t.Fatal("location resolution changed unrelated template context")
					}
				}
			}
			againPub, againText, err := compileBacklinks(t.Context(), c, resolved)
			if err != nil || !bytes.Equal(text.Raw, againText.Raw) ||
				!reflect.DeepEqual(snapshotPublication(t, resolved), snapshotPublication(t, againPub)) {
				t.Fatal("final backlink compilation is not idempotent", err)
			}
		})
	}
}

func TestCompileBacklinksRejectsMissingContext(t *testing.T) {
	for _, missing := range []string{"context", "reference", "filename"} {
		t.Run(missing, func(t *testing.T) {
			pub := textTestPublication(t)
			pub.Backlinks = map[*etree.Element]string{pub.Chapters[2].Doc.FindElement("//a[@id='back']"): "ref1"}
			ref := content.BackLinkRef{RefID: "ref1", Filename: "Text/first.xhtml"}
			switch missing {
			case "reference":
				ref.RefID = "unknown"
			case "filename":
				ref.Filename = "unknown.xhtml"
			}
			c := &content.Content{BackLinkIndex: map[string][]content.BackLinkRef{"note": {ref}}}
			if missing == "context" {
				c.BackLinkIndex = nil
			}
			if _, _, err := compileBacklinks(t.Context(), c, pub); err == nil {
				t.Fatal("accepted a backlink with no resolvable reference")
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := compileBacklinks(ctx, &content.Content{}, textTestPublication(t)); !errors.Is(err, context.Canceled) {
		t.Fatal("ignored canceled compilation", err)
	}
	if _, _, err := compileBacklinks(t.Context(), nil, textTestPublication(t)); err == nil {
		t.Fatal("accepted nil content")
	}
	if _, _, err := compileBacklinks(t.Context(), &content.Content{}, nil); err == nil {
		t.Fatal("accepted nil publication")
	}
	pub := textTestPublication(t)
	pub.Backlinks = map[*etree.Element]string{etree.NewElement("a"): "orphan"}
	if _, _, err := compileBacklinks(t.Context(), &content.Content{}, pub); err == nil {
		t.Fatal("silently dropped a detached generated backlink")
	}
}

func TestCompileBacklinksRejectsUnstableTemplate(t *testing.T) {
	pub := textTestPublication(t)
	back := pub.Chapters[2].Doc.FindElement("//a[@id='back']")
	back.SetText("y")
	ref := pub.Chapters[2].Doc.FindElement("//aside").CreateElement("a")
	ref.CreateAttr("id", "cycle")
	ref.CreateAttr("href", "#n_é")
	ref.SetText("Reference after its own backlink")
	back.CreateAttr("href", "#cycle")
	pub.Backlinks = map[*etree.Element]string{back: "cycle"}
	initial, err := compileText(t.Context(), pub)
	if err != nil {
		t.Fatal(err)
	}
	parity := (initial.Positions[linkTarget{Filename: "Notes/notes.xhtml", ID: "cycle"}].Position/150 + 1) % 2
	c := &content.Content{
		BackLinkIndex: map[string][]content.BackLinkRef{"n_é": {{RefID: "cycle", Filename: "Notes/notes.xhtml"}}},
		// The 150-byte label-length difference moves the later reference by
		// exactly one location, making its label oscillate forever.
		BacklinkTemplate: fmt.Sprintf("{{if eq (mod .LocationNumber 2) %d}}%s{{else}}y{{end}}", parity, strings.Repeat("x", 151)),
	}
	before := snapshotPublication(t, pub)
	if _, _, err := compileBacklinks(t.Context(), c, pub); err == nil || !strings.Contains(err.Error(), "did not stabilize") {
		t.Fatal("expected bounded nonconvergence failure", err)
	}
	if c.BackLinkIndex["n_é"][0].LocationNumber != 0 || !reflect.DeepEqual(before, snapshotPublication(t, pub)) {
		t.Fatal("failed resolution changed source publication or reference contexts")
	}
}
