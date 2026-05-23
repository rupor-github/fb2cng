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

func buildPDFDocument(doc pdfDocumentSpec) ([]byte, error) {
	doc.Blocks = applyPDFPseudoContentToBlocks(doc.Blocks, doc.Styles)

	writer := docwriter.NewWriter(pdfVersion)

	const (
		catalogID      = 1
		pagesID        = 2
		firstPageID    = 3
		firstContentID = 4
		infoID         = 5
	)

	fontFace, err := builtinFont("sans-serif", false, false)
	if err != nil {
		return nil, err
	}
	pages, usedGlyphs, err := layoutPDFPages(doc, fontFace)
	if err != nil {
		return nil, err
	}
	for range 3 {
		if !resolvePDFBacklinkPagesAndText(&doc, pages) {
			break
		}
		pages, usedGlyphs, err = layoutPDFPages(doc, fontFace)
		if err != nil {
			return nil, err
		}
	}
	if len(pages) == 0 {
		return nil, errors.New("pdf document must contain at least one page")
	}

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
	fontResources, err := preparePDFFontResources(doc.Fonts, usedGlyphs, &nextObjectID)
	if err != nil {
		return nil, err
	}
	assignPDFFontResourceNames(pages, fontResources)
	if err := writePDFDebugDumps(doc, pages, fontResources); err != nil {
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
