package pdf

import (
	"fmt"
	"strings"

	"fbc/css"
)

type pdfPseudoElementContent struct {
	Before string
	After  string
}

func (r *pdfStyleResolver) extractPseudoContent(sheet *css.Stylesheet) []string {
	if r == nil || sheet == nil {
		return nil
	}
	var warnings []string
	for _, rule := range pdfStylesheetRules(sheet) {
		if rule.Selector.Pseudo == css.PseudoNone {
			continue
		}
		contentValue, hasContent := rule.GetProperty("content")
		if !hasContent {
			continue
		}
		content := pdfParseCSSContent(contentValue)
		if content == "" {
			continue
		}
		for property := range rule.Properties {
			if property != "content" {
				warnings = append(warnings, fmt.Sprintf("pseudo-element %q has property %q which will be ignored (only 'content' is supported)", rule.Selector.Raw, property))
			}
		}
		for _, name := range pdfSelectorStyleNames(rule.Selector) {
			r.registerPseudoContent(name, rule.Selector.Pseudo, content)
		}
	}
	return warnings
}

func (r *pdfStyleResolver) registerPseudoContent(styleName string, pseudo css.PseudoElement, content string) {
	if r == nil || strings.TrimSpace(styleName) == "" || content == "" {
		return
	}
	if r.pseudoContent == nil {
		r.pseudoContent = make(map[string]pdfPseudoElementContent)
	}
	entry := r.pseudoContent[styleName]
	switch pseudo {
	case css.PseudoBefore:
		entry.Before = content
	case css.PseudoAfter:
		entry.After = content
	default:
		return
	}
	r.pseudoContent[styleName] = entry
}

func (r *pdfStyleResolver) pseudoContentForClasses(classes string) (string, pdfPseudoElementContent, bool) {
	if r == nil || len(r.pseudoContent) == 0 {
		return "", pdfPseudoElementContent{}, false
	}
	for class := range strings.FieldsSeq(classes) {
		if content, ok := r.pseudoContent[class]; ok && (content.Before != "" || content.After != "") {
			return class, content, true
		}
	}
	return "", pdfPseudoElementContent{}, false
}

func pdfDecoratedFootnoteReferenceLabel(resolver *pdfStyleResolver, classes string, label string) string {
	_, content, ok := resolver.pseudoContentForClasses(classes)
	if !ok {
		return label
	}
	return content.Before + label + content.After
}

func pdfParseCSSContent(value css.Value) string {
	raw := strings.TrimSpace(value.Raw)
	if raw == "" || raw == "none" || raw == "normal" {
		return ""
	}
	if len(raw) >= 2 && ((raw[0] == '"' && raw[len(raw)-1] == '"') || (raw[0] == '\'' && raw[len(raw)-1] == '\'')) {
		return raw[1 : len(raw)-1]
	}
	return ""
}

func applyPDFPseudoContentToBlocks(blocks []pdfTextBlock, resolver *pdfStyleResolver) []pdfTextBlock {
	if resolver == nil || len(resolver.pseudoContent) == 0 {
		return blocks
	}
	out := slicesClone(blocks)
	for i := range out {
		if len(out[i].Runs) == 0 {
			continue
		}
		runs, changed := applyPDFPseudoContentToInlineRunsChanged(out[i].Runs, resolver)
		if !changed {
			continue
		}
		out[i].Runs = runs
		out[i].Text = plainInlineRunText(out[i].Runs)
	}
	return out
}

func applyPDFPseudoContentToPrintedFootnotes(
	footnotes map[string]pdfPrintedFootnote,
	resolver *pdfStyleResolver,
) map[string]pdfPrintedFootnote {
	if resolver == nil || len(resolver.pseudoContent) == 0 || len(footnotes) == 0 {
		return footnotes
	}
	out := make(map[string]pdfPrintedFootnote, len(footnotes))
	for id, footnote := range footnotes {
		footnote.TitleBlocks = applyPDFPseudoContentToBlocks(footnote.TitleBlocks, resolver)
		footnote.BodyBlocks = applyPDFPseudoContentToBlocks(footnote.BodyBlocks, resolver)
		footnote.ContinuationTitleBlocks = applyPDFPseudoContentToBlocks(footnote.ContinuationTitleBlocks, resolver)
		footnote.Blocks = applyPDFPseudoContentToBlocks(footnote.Blocks, resolver)
		footnote.ContinuationBlocks = applyPDFPseudoContentToBlocks(footnote.ContinuationBlocks, resolver)
		out[id] = footnote
	}
	return out
}

func applyPDFPseudoContentToInlineRuns(runs []pdfInlineRun, resolver *pdfStyleResolver) []pdfInlineRun {
	out, _ := applyPDFPseudoContentToInlineRunsChanged(runs, resolver)
	return out
}

func applyPDFPseudoContentToInlineRunsChanged(runs []pdfInlineRun, resolver *pdfStyleResolver) ([]pdfInlineRun, bool) {
	if resolver == nil || len(resolver.pseudoContent) == 0 || len(runs) == 0 {
		return runs, false
	}
	out := slicesClone(runs)
	changed := false
	for i := range out {
		class, content, ok := resolver.pseudoContentForClasses(out[i].StyleClasses)
		if !ok {
			continue
		}
		if content.Before != "" && pdfStartsPseudoContentGroup(out, i, class) {
			out[i].Text = content.Before + out[i].Text
			changed = true
		}
		if content.After != "" && pdfEndsPseudoContentGroup(out, i, class) {
			out[i].Text += content.After
			changed = true
		}
	}
	if !changed {
		return runs, false
	}
	return out, true
}

func pdfStartsPseudoContentGroup(runs []pdfInlineRun, index int, class string) bool {
	if index <= 0 {
		return true
	}
	return !pdfPseudoContentSameGroup(runs[index-1], runs[index], class)
}

func pdfEndsPseudoContentGroup(runs []pdfInlineRun, index int, class string) bool {
	if index >= len(runs)-1 {
		return true
	}
	return !pdfPseudoContentSameGroup(runs[index], runs[index+1], class)
}

func pdfPseudoContentSameGroup(left pdfInlineRun, right pdfInlineRun, class string) bool {
	return left.LinkHref == right.LinkHref &&
		left.FootnoteID == right.FootnoteID &&
		hasPDFStyleClass(left.StyleClasses, class) &&
		hasPDFStyleClass(right.StyleClasses, class)
}

func slicesClone[S ~[]E, E any](s S) S {
	if s == nil {
		return nil
	}
	return append(S(nil), s...)
}
