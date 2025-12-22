package kfx

import (
	"fmt"
	"sort"
	"strings"

	"github.com/amazon-ion/ion-go/ion"

	"fbc/convert/kfx/model"
	"fbc/utils/debug"
)

type kfxTreeWriter struct {
	*debug.TreeWriter
}

func BuildDebugTree(containerID string, prologSymbols int, fragments []model.Fragment) string {
	return buildKFXDebugTree(containerID, prologSymbols, fragments)
}

func buildKFXDebugTree(containerID string, prologSymbols int, fragments []model.Fragment) string {
	tw := kfxTreeWriter{debug.NewTreeWriter()}

	tw.Line(0, "KFX")
	tw.Line(1, "container_id=%s", containerID)
	tw.Line(1, "document_symbols_bytes=%d", prologSymbols)

	byType := map[string][]model.Fragment{}
	for _, fr := range fragments {
		byType[fr.FType] = append(byType[fr.FType], fr)
	}

	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Strings(types)

	tw.Line(1, "fragments=%d", len(fragments))
	for _, t := range types {
		list := byType[t]
		sort.Slice(list, func(i, j int) bool { return list[i].FID < list[j].FID })
		tw.Line(2, "type=%s (%d)", t, len(list))
		for _, fr := range list {
			tw.Line(3, "fid=%s", fr.FID)

			// A compact, human-friendly preview.
			switch v := fr.Value.(type) {
			case []byte:
				tw.Line(4, "raw_bytes=%d", len(v))
			default:
				val := fr.Value
				// Normalize KFXInput-style decoded maps for stable diffs vs our own tree.txt.
				if fr.FType == "$145" {
					type contentFragmentTree struct {
						Name        string `ion:"name,symbol"`
						ContentList any    `ion:"$146"`
					}

					var contentList any
					switch m := fr.Value.(type) {
					case map[string]any:
						contentList = m["$146"]
					case map[ion.SymbolToken]any:
						for k, v := range m {
							if k.Text != nil && *k.Text == "$146" {
								contentList = v
								break
							}
						}
					}

					val = contentFragmentTree{Name: fr.FID, ContentList: contentList}
				}

				// ion.MarshalText gives a stable-ish readable view; keep it short.
				b, err := ion.MarshalText(val)
				if err != nil {
					tw.Line(4, "value=<ion marshal error: %v>", err)
					continue
				}
				s := strings.TrimSpace(string(b))
				if len(s) > 800 {
					s = s[:800] + " …"
				}
				// Put it on a separate indented line to preserve structure.
				tw.Line(4, "ion=%s", fmt.Sprintf("%q", s))
			}
		}
	}

	return tw.String()
}
