package pdf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func writePDFDebugDumps(doc pdfDocumentSpec, pages []pdfPage, fontResources []pdfFontResource, printedFootnotes pdfDebugPrintedFootnotes) error {
	if !doc.Debug || doc.WorkDir == "" {
		return nil
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-structure-plan.json"), doc.DebugPlan); err != nil {
		return err
	}
	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-resolved-styles.json"), styles.debugStyles()); err != nil {
		return err
	}

	blocks := make([]pdfDebugBlock, 0, len(doc.Blocks))
	for i, block := range doc.Blocks {
		styleName := pdfStyleNameForBlock(block)
		styles.tracer.traceAssign(block, styleName, styles.styleForBlock(block))
		blocks = append(blocks, pdfDebugBlock{
			Index:        i,
			Kind:         block.Kind.String(),
			ID:           block.ID,
			Depth:        block.Depth,
			StyleName:    styleName,
			StyleClasses: strings.TrimSpace(block.StyleClasses),
			ImageID:      block.ImageID,
			Text:         block.Text,
		})
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-text-blocks.json"), blocks); err != nil {
		return err
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-printed-footnotes.json"), printedFootnotes); err != nil {
		return err
	}
	styles.tracer.flush()

	debugPages, debugImages, debugLinks := pdfDebugPages(pages)
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-layout-pages.json"), debugPages); err != nil {
		return err
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-line-glyphs.json"), pdfDebugLineGlyphs(pages)); err != nil {
		return err
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-justification.json"), pdfDebugJustificationLines(pages)); err != nil {
		return err
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-images.json"), debugImages); err != nil {
		return err
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-links.json"), debugLinks); err != nil {
		return err
	}
	return writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-fonts.json"), pdfDebugFonts(fontResources))
}

func writeJSONDebugDump(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}
