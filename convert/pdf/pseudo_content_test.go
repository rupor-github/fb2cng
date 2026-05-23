package pdf

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"fbc/fb2"
)

func TestPDFPseudoContentDecoratesFootnoteLinks(t *testing.T) {
	resolver := newPDFStyleResolver(&fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{Type: "text/css", Data: `
		.link-footnote::before { content: "["; }
		.link-footnote::after { content: "]"; }
	`}}}, nil)
	blocks := applyPDFPseudoContentToBlocks([]pdfTextBlock{{
		Text: "Body 1.",
		Runs: []pdfInlineRun{
			{Text: "Body "},
			{Text: "1", StyleClasses: pdfStyleLinkFootnote, LinkHref: "#n1", AnchorID: "ref-n1-1"},
			{Text: "."},
		},
	}}, resolver)
	if len(blocks) != 1 {
		t.Fatalf("blocks = %#v, want one block", blocks)
	}
	if got := blocks[0].Text; got != "Body [1]." {
		t.Fatalf("block text = %q, want Body [1].", got)
	}
	if got := blocks[0].Runs[1]; got.Text != "[1]" || got.LinkHref != "#n1" || got.AnchorID != "ref-n1-1" {
		t.Fatalf("footnote run = %#v, want decorated linked run with anchor", got)
	}
}

func TestPDFPseudoContentLogsSummaryCount(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	newPDFStyleResolver(&fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{Type: "text/css", Data: `
		.link-footnote::before { content: "["; }
		.link-footnote::after { content: "]"; }
	`}}}, zap.New(core))

	entries := logs.FilterMessage("CSS styles loaded").All()
	if len(entries) != 1 {
		t.Fatalf("CSS styles loaded log entries = %d, want 1", len(entries))
	}
	fields := pdfZapIntFields(entries[0])
	if fields["rules"] != 2 || fields["styles"] != 0 || fields["warnings"] != 0 || fields["pseudo_content"] != 1 {
		t.Fatalf("CSS styles loaded fields = %#v, want rules=2 styles=0 warnings=0 pseudo_content=1", fields)
	}
}

func TestPDFPseudoContentWrapsContiguousStyledLinkGroup(t *testing.T) {
	resolver := &pdfStyleResolver{pseudoContent: map[string]pdfPseudoElementContent{
		pdfStyleLinkFootnote: {Before: "[", After: "]"},
	}}
	runs := applyPDFPseudoContentToInlineRuns([]pdfInlineRun{
		{Text: "2.", StyleClasses: pdfStyleLinkFootnote, LinkHref: "#n1"},
		{Text: "1", StyleClasses: joinStyleClasses(pdfStyleLinkFootnote, "accent"), LinkHref: "#n1"},
		{Text: " and "},
		{Text: "3", StyleClasses: pdfStyleLinkFootnote, LinkHref: "#n2"},
	}, resolver)
	want := []string{"[2.", "1]", " and ", "[3]"}
	if len(runs) != len(want) {
		t.Fatalf("runs = %#v, want %d runs", runs, len(want))
	}
	for i := range want {
		if runs[i].Text != want[i] {
			t.Fatalf("run %d text = %q, want %q; runs=%#v", i, runs[i].Text, want[i], runs)
		}
	}
}

func pdfZapIntFields(entry observer.LoggedEntry) map[string]int64 {
	fields := make(map[string]int64)
	for _, field := range entry.Context {
		switch field.Type {
		case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type:
			fields[field.Key] = field.Integer
		}
	}
	return fields
}
