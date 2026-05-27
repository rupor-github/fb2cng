package pdf

import (
	"errors"
	"fmt"

	"fbc/config"
	"fbc/convert/pdf/docwriter"
)

func pageSizePoints(screen config.ScreenConfig) (float64, float64, error) {
	dpi := screen.DPI
	if screen.Width <= 0 {
		return 0, 0, fmt.Errorf("invalid pdf screen width: %d", screen.Width)
	}
	if screen.Height <= 0 {
		return 0, 0, fmt.Errorf("invalid pdf screen height: %d", screen.Height)
	}
	if dpi <= 0 {
		return 0, 0, fmt.Errorf("invalid pdf screen dpi: %d", screen.DPI)
	}

	return float64(screen.Width) * pdfPointsPerInch / float64(dpi), float64(screen.Height) * pdfPointsPerInch / float64(dpi), nil
}

func layoutPDFDocumentPages(doc pdfDocumentSpec) ([]pdfPage, map[pdfFontKey]map[uint16]shapedGlyph, pdfDebugPrintedFootnotes, error) {
	if doc.TextShapers == nil {
		doc.TextShapers = newPDFTextShaperCache()
	}
	if !pdfPrintedFootnotesEnabled(doc.Content) || len(doc.PrintedFootnotes) == 0 {
		pages, used, err := layoutPDFPages(doc)
		return pages, used, pdfDebugPrintedFootnotes{SourcePageCount: len(pages), FinalPageCount: len(pages)}, err
	}

	// Printed footnotes need a two-stage layout. First lay out the main text and
	// discover which page references each note. Then reserve bottom space on those
	// source pages and lay out again so body text does not collide with the note
	// chunks that appendPDFPrintedFootnotePagePlans will render later.
	reserved, err := layoutPDFPagesWithPrintedFootnoteReserves(doc)
	if err != nil {
		return nil, nil, pdfDebugPrintedFootnotes{}, err
	}
	printedFootnotes := pdfDebugPrintedFootnotesFromReserved(doc, reserved)
	pages := appendPDFPrintedFootnotePagePlans(
		doc,
		reserved.Pages,
		reserved.Plans,
		reserved.FootnoteTextHeight,
		reserved.UsedGlyphs,
		&printedFootnotes,
	)
	printedFootnotes.FinalPageCount = len(pages)
	pdfDebugPrintedFootnotesSyncCounts(&printedFootnotes)
	return pages, reserved.UsedGlyphs, printedFootnotes, nil
}

func buildPDFDocument(doc pdfDocumentSpec) ([]byte, error) {
	if doc.TextShapers == nil {
		doc.TextShapers = newPDFTextShaperCache()
	}

	// CSS pseudo-elements become real text before pagination so generated labels,
	// brackets, and backlink markers participate in shaping, wrapping, and link
	// rectangle calculation just like source text.
	doc.Blocks = applyPDFPseudoContentToBlocks(doc.Blocks, doc.Styles)
	doc.PrintedFootnotes = applyPDFPseudoContentToPrintedFootnotes(doc.PrintedFootnotes, doc.Styles)

	writer := docwriter.NewWriter(pdfVersion)

	const (
		catalogID      = 1
		pagesID        = 2
		firstPageID    = 3
		firstContentID = 4
		infoID         = 5
	)

	pages, usedGlyphs, printedFootnotes, err := layoutPDFDocumentPages(doc)
	if err != nil {
		return nil, err
	}

	// Some generated backlink text includes the page number of the original
	// reference. Changing that text can affect wrapping and therefore page numbers,
	// so iterate a few times toward a fixed point. In practice one or two passes are
	// enough; the cap prevents pathological oscillation from hanging conversion.
	//
	// Backlink convergence follow-ups from code review, kept here deliberately close
	// to the current loop:
	//   - replace the literal 3 with a named max-iteration constant;
	//   - track whether the loop actually converged;
	//   - after the cap, perform a final non-mutating check against the final pages,
	//     or continue to a larger cap and warn/error if still unstable;
	//   - add a regression test with a contrived backlink template/layout that needs
	//     several passes or demonstrates non-convergence.
	for range 3 {
		if !resolvePDFBacklinkPagesAndText(&doc, pages) {
			break
		}
		pages, usedGlyphs, printedFootnotes, err = layoutPDFDocumentPages(doc)
		if err != nil {
			return nil, err
		}
	}
	if len(pages) == 0 {
		return nil, errors.New("pdf document must contain at least one page")
	}

	// Object numbers are assigned only after pagination because outlines,
	// annotations, font subsets, and image resources all depend on the final page
	// list. Reserve stable IDs for the catalog/pages/first page/info objects, then
	// hand out monotonically increasing IDs to every remaining object.
	pages[0].ObjectID = firstPageID
	pages[0].ContentID = firstContentID
	nextObjectID := infoID + 1
	for i := 1; i < len(pages); i++ {
		pages[i].ObjectID = nextObjectID
		nextObjectID++
		pages[i].ContentID = nextObjectID
		nextObjectID++
	}
	imageResources, err := preparePDFImageResources(doc.Images, pages, &nextObjectID)
	if err != nil {
		return nil, err
	}
	assignPDFImageResourceNames(pages, imageResources)
	outlines := buildOutlines(doc.TOC, pages, &nextObjectID)
	assignAnnotationObjectIDs(pages, &nextObjectID)

	// Font subsetting happens after layout so each embedded font contains exactly
	// the shaped glyphs referenced by page content streams.
	fontResources, err := preparePDFFontResources(doc.Fonts, usedGlyphs, &nextObjectID)
	if err != nil {
		return nil, err
	}
	assignPDFFontResourceNames(pages, fontResources)
	if err := writePDFDebugDumps(doc, pages, fontResources, printedFootnotes); err != nil {
		return nil, err
	}

	catalog := docwriter.Dict{
		"Pages": docwriter.Ref{ObjectNumber: pagesID},
		"Type":  docwriter.Name("Catalog"),
	}
	if outlines.RootID != 0 {
		catalog["Outlines"] = docwriter.Ref{ObjectNumber: outlines.RootID}
	}
	if names := namedDestinations(pages); names != nil {
		catalog["Names"] = names
	}
	if err := writer.Object(catalogID, catalog); err != nil {
		return nil, err
	}
	kids := make(docwriter.Array, 0, len(pages))
	for _, page := range pages {
		kids = append(kids, docwriter.Ref{ObjectNumber: page.ObjectID})
	}
	if err := writer.Object(pagesID, docwriter.Dict{
		"Count": docwriter.Integer(len(pages)),
		"Kids":  kids,
		"Type":  docwriter.Name("Pages"),
	}); err != nil {
		return nil, err
	}
	for _, page := range pages {
		resources := docwriter.Dict{
			"Font": pageFontResources(fontResources),
		}
		if xobjects := pageImageXObjects(page, imageResources); xobjects != nil {
			resources["XObject"] = xobjects
		}
		pageDict := docwriter.Dict{
			"Contents": docwriter.Ref{ObjectNumber: page.ContentID},
			"MediaBox": docwriter.Array{
				docwriter.Integer(0),
				docwriter.Integer(0),
				docwriter.Number(doc.PageWidth),
				docwriter.Number(doc.PageHeight),
			},
			"Parent":    docwriter.Ref{ObjectNumber: pagesID},
			"Resources": resources,
			"Type":      docwriter.Name("Page"),
		}
		if len(page.Annotations) != 0 {
			annots := make(docwriter.Array, 0, len(page.Annotations))
			for _, annot := range page.Annotations {
				annots = append(annots, docwriter.Ref{ObjectNumber: annot.ObjectID})
			}
			pageDict["Annots"] = annots
		}
		if err := writer.Object(page.ObjectID, pageDict); err != nil {
			return nil, err
		}
	}

	for _, page := range pages {
		// pageContent lowers the logical display list to PDF graphics/text operators;
		// the writer then compresses it into the page's Contents stream object.
		stream, err := flateStream(pageContent(page))
		if err != nil {
			return nil, err
		}
		if err := writer.StreamObject(page.ContentID, docwriter.Dict{
			"Filter": docwriter.Name("FlateDecode"),
		}, stream); err != nil {
			return nil, err
		}
	}
	if err := writer.Object(infoID, infoDictionary(doc)); err != nil {
		return nil, err
	}
	if err := writePDFFontObjects(writer, fontResources); err != nil {
		return nil, err
	}
	if err := writeOutlineObjects(writer, outlines); err != nil {
		return nil, err
	}
	if err := writeAnnotationObjects(writer, pages); err != nil {
		return nil, err
	}
	if err := writePDFImageObjects(writer, imageResources); err != nil {
		return nil, err
	}

	infoRef := docwriter.Ref{ObjectNumber: infoID}
	return writer.Finish(docwriter.Trailer{
		Root: docwriter.Ref{ObjectNumber: catalogID},
		Info: &infoRef,
	})
}
