package kfx

import "testing"

func TestMarkPreviousEntryStripMBTransferPolicy(t *testing.T) {
	zero, margin := 0.0, 0.25
	for _, scope := range []string{"storyline", "block"} {
		for _, tt := range []struct {
			name     string
			transfer *float64
		}{
			{"automatic", nil},
			{"explicit zero", &zero},
			{"explicit margin", &margin},
		} {
			t.Run(scope+"/"+tt.name, func(t *testing.T) {
				entries := []ContentRef{{Type: SymText}, {Type: SymText}}
				sb := &StorylineBuilder{}
				if scope == "block" {
					sb.blockStack = []*BlockBuilder{{children: entries}}
				} else {
					sb.contentEntries = entries
				}
				sb.MarkPreviousEntryStripMB(tt.transfer)
				if entries[0].StripMarginBottom || !entries[1].StripMarginBottom {
					t.Fatal("only the immediately preceding entry must be marked")
				}
				if entries[1].StripMarginBottomTransfer != tt.transfer {
					t.Fatal("automatic, zero, and nonzero transfer policies must remain distinct")
				}
			})
		}
		t.Run(scope+"/no previous entry", func(_ *testing.T) {
			sb := &StorylineBuilder{}
			if scope == "block" {
				sb.blockStack = []*BlockBuilder{{}}
			}
			sb.MarkPreviousEntryStripMB(nil)
		})
	}
}
