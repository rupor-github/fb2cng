package pdf

// pdfPageLayout is the mutable state for one pagination pass. The pass walks the
// flat block list, appends logical drawing operations to pages, and records the
// glyphs required by the resulting text.
type pdfPageLayout struct {
	doc pdfDocumentSpec

	styles      *pdfStyleResolver
	blockStyles []pdfBlockResolvedStyle
	used        map[pdfFontKey]map[uint16]shapedGlyph
	pages       []pdfPage
	page        *pdfPage

	// Content margins are physical page margins. The rootless variants are used by
	// generated full-width blocks that should ignore body horizontal margins but
	// still live inside the same top/bottom text area.
	contentLeft   float64
	contentRight  float64
	contentTop    float64
	contentBottom float64

	rootlessContentLeft  float64
	rootlessContentRight float64
	contentWidth         float64
	rootlessContentWidth float64

	printedFootnoteReserve pdfDynamicPrintedFootnoteReserveTracker

	// top and bottom bound the current text area; y is the next baseline to draw.
	// bottom may be raised by page-specific printed-footnote reserves.
	top    float64
	bottom float64
	y      float64

	pageHasText           bool
	previousRenderedImage bool
	activeDropcap         *pdfActiveDropcap
	titleGroup            pdfTitleVignetteContentGroup
}

func layoutPDFPages(doc pdfDocumentSpec) ([]pdfPage, map[pdfFontKey]map[uint16]shapedGlyph, error) {
	layout := newPDFPageLayout(doc)
	if err := layout.layout(); err != nil {
		return nil, nil, err
	}
	return layout.pages, layout.used, nil
}

func newPDFPageLayout(doc pdfDocumentSpec) *pdfPageLayout {
	return &pdfPageLayout{
		doc:   doc,
		used:  make(map[pdfFontKey]map[uint16]shapedGlyph),
		pages: make([]pdfPage, 0, 2),
	}
}

func (l *pdfPageLayout) layout() error {
	// A cover is represented as an image-only page before the main text stream. It
	// is intentionally independent from the block list so books without body text
	// can still produce a valid cover-only PDF.
	l.addCoverPage()
	if len(l.doc.Blocks) == 0 {
		if len(l.pages) == 0 {
			l.addPage()
		}
		return nil
	}

	l.initTextLayout()
	if err := l.layoutBlocks(); err != nil {
		return err
	}
	return l.finish()
}

func (l *pdfPageLayout) addCoverPage() {
	cover := l.doc.Images[l.doc.CoverID]
	if cover == nil {
		return
	}
	rect, ok := fitPDFImageInBox(l.doc, cover, 0, 0, l.doc.PageWidth, l.doc.PageHeight)
	if !ok {
		return
	}
	coverPage := l.addPage()
	addPDFPageAnchor(coverPage, l.doc.CoverID)
	coverPage.Images = append(coverPage.Images, pdfPageImage{
		ImageID: l.doc.CoverID,
		X:       rect.X1,
		Y:       rect.Y1,
		Width:   rect.X2 - rect.X1,
		Height:  rect.Y2 - rect.Y1,
	})
}

func (l *pdfPageLayout) initTextLayout() {
	// Resolve all block styles up front so pagination can cheaply look at current
	// and following blocks for keep-with-next/orphan/widow decisions.
	l.styles = l.doc.Styles
	if l.styles == nil {
		l.styles = newPDFStyleResolver(nil, nil)
	}
	l.blockStyles = l.styles.collapsedBlockStyles(l.doc.Blocks, l.doc.Images)
	l.contentLeft, l.contentRight, l.contentTop, l.contentBottom = pdfContentMargins(l.doc, l.styles, pdfDefaultPageMargin, false)
	l.rootlessContentLeft, l.rootlessContentRight, _, _ = pdfContentMargins(l.doc, l.styles, pdfDefaultPageMargin, true)
	l.contentWidth = max(l.doc.PageWidth-l.contentLeft-l.contentRight, 12)
	l.rootlessContentWidth = max(l.doc.PageWidth-l.rootlessContentLeft-l.rootlessContentRight, 12)
	l.printedFootnoteReserve = newPDFDynamicPrintedFootnoteReserveTracker(l.doc, l.styles, l.contentLeft, l.contentWidth, l.contentBottom)
	l.top = l.doc.PageHeight - l.contentTop
	l.page = l.addPage()
	l.bottom = l.pageBottom(len(l.pages) - 1)
	l.y = l.top
}

func (l *pdfPageLayout) addPage() *pdfPage {
	l.pages = append(l.pages, pdfPage{})
	return &l.pages[len(l.pages)-1]
}

func (l *pdfPageLayout) pageBottom(pageIndex int) float64 {
	// PageBottomReserves is produced by the printed-footnote prepass. Raising the
	// bottom edge here makes the ordinary text paginator leave room for notes.
	reserve := 0.0
	if pageIndex >= 0 && pageIndex < len(l.doc.PageBottomReserves) {
		reserve = l.doc.PageBottomReserves[pageIndex]
	}
	return pdfReservedContentBottom(l.contentBottom, l.doc.PageHeight-l.contentTop, reserve)
}

func (l *pdfPageLayout) newTextPage() {
	l.titleGroup.reset()
	l.page = l.addPage()
	l.bottom = l.pageBottom(len(l.pages) - 1)
	l.y = l.top
	l.pageHasText = false
	l.previousRenderedImage = false
	l.activeDropcap = nil
	l.printedFootnoteReserve.ResetPage()
}

func (l *pdfPageLayout) finish() error {
	// Page-local footnote numbering needs the final source pages. Apply it after
	// pagination, then rejustify affected lines because label widths may change.
	if pdfPrintedFootnoteReferencesRenumbered(l.doc.Content) && len(l.doc.PrintedFootnotes) > 0 {
		if err := applyPDFPageLocalFootnoteReferenceLabels(l.pages, l.doc.Fonts, l.used, l.styles, l.doc.TextShapers); err != nil {
			return err
		}
	}

	if len(l.pages[len(l.pages)-1].Lines) == 0 && len(l.pages[len(l.pages)-1].Images) == 0 {
		l.pages = l.pages[:len(l.pages)-1]
	}
	return nil
}

func (l *pdfPageLayout) layoutBlocks() error {
	for blockIndex, block := range l.doc.Blocks {
		if block.Kind == pdfBlockPageBreak {
			if l.pageHasText {
				l.newTextPage()
			}
			addPDFPageAnchor(l.page, block.ID)
			continue
		}

		// Dropcaps create an exclusion shape only for immediately following paragraph
		// lines. Any non-paragraph block closes the exclusion and moves below the cap
		// if it is still occupying vertical space on the current page.
		if l.activeDropcap != nil && block.Kind != pdfBlockParagraph {
			if !pdfDropcapExpired(l.activeDropcap, l.page, l.y) {
				l.y = l.activeDropcap.BottomY
			}
			l.activeDropcap = nil
		}

		style := l.blockStyles[blockIndex]
		if style.Hidden {
			continue
		}
		if pdfStyleForcesPageBreakBefore(style) && l.pageHasText {
			l.newTextPage()
		}

		// Most blocks live inside the normal content box. Some generated title and
		// vignette blocks opt into the rootless width to match full-width Kindle-style
		// decorations while preserving the same vertical pagination model.
		blockLeft := l.contentLeft
		blockWidthLimit := l.contentWidth
		if block.StripRootHorizontalMargins {
			blockLeft = l.rootlessContentLeft
			blockWidthLimit = l.rootlessContentWidth
		}

		if block.Kind == pdfBlockTable {
			if err := l.layoutTableBlock(block, style, blockLeft, blockWidthLimit); err != nil {
				return err
			}
			continue
		}

		if block.Kind == pdfBlockImage {
			if err := l.layoutImageBlock(blockIndex, block, style, blockLeft, blockWidthLimit); err != nil {
				return err
			}
			continue
		}

		if err := l.layoutTextBlock(blockIndex, block, style, blockLeft, blockWidthLimit); err != nil {
			return err
		}

	}

	return nil
}
