package pdf

type pdfTitleVignetteContentGroup struct {
	active          bool
	page            *pdfPage
	topBoundary     float64
	lineStart       int
	imageStart      int
	annotationStart int
}

func (g *pdfTitleVignetteContentGroup) start(page *pdfPage, topBoundary float64) {
	if page == nil {
		g.reset()
		return
	}
	g.active = true
	g.page = page
	g.topBoundary = topBoundary
	g.lineStart = len(page.Lines)
	g.imageStart = len(page.Images)
	g.annotationStart = len(page.Annotations)
}

func (g *pdfTitleVignetteContentGroup) finish(page *pdfPage, bottomBoundary float64, fonts *pdfFontRegistry) {
	if !g.active || g.page != page || page == nil {
		g.reset()
		return
	}
	lineEnd := len(page.Lines)
	imageEnd := len(page.Images) - 1
	annotationEnd := len(page.Annotations)
	visualTop, visualBottom, ok := pdfPageTitleContentVisualBounds(page, g.lineStart, lineEnd, g.imageStart, imageEnd, fonts)
	if ok && g.topBoundary > bottomBoundary {
		shift := (g.topBoundary+bottomBoundary)/2 - (visualTop+visualBottom)/2
		shiftPDFTitleContent(page, g.lineStart, lineEnd, g.imageStart, imageEnd, g.annotationStart, annotationEnd, shift)
	}
	g.reset()
}

func (g *pdfTitleVignetteContentGroup) reset() {
	*g = pdfTitleVignetteContentGroup{}
}

func pdfPageTitleContentVisualBounds(page *pdfPage, lineStart, lineEnd int, imageStart, imageEnd int, fonts *pdfFontRegistry) (float64, float64, bool) {
	if page == nil {
		return 0, 0, false
	}
	var top float64
	var bottom float64
	ok := false
	include := func(itemTop float64, itemBottom float64) {
		if itemTop <= itemBottom {
			return
		}
		if !ok || itemTop > top {
			top = itemTop
		}
		if !ok || itemBottom < bottom {
			bottom = itemBottom
		}
		ok = true
	}
	for i := max(lineStart, 0); i < lineEnd && i < len(page.Lines); i++ {
		includePDFLineVisualBounds(include, page.Lines[i], fonts)
	}
	for i := max(imageStart, 0); i < imageEnd && i < len(page.Images); i++ {
		image := page.Images[i]
		include(image.Y+image.Height, image.Y)
	}
	return top, bottom, ok
}

func includePDFLineVisualBounds(include func(float64, float64), line pdfPageLine, fonts *pdfFontRegistry) {
	if len(line.Fragments) == 0 {
		face, err := resolvePDFFontFace(fonts, line.FontKey)
		if err != nil {
			face = nil
		}
		ascent, descent := pdfFontVisualMetrics(face, line.FontSize)
		include(line.Y+ascent, line.Y-descent)
		return
	}
	for _, fragment := range line.Fragments {
		baseline := line.Y + fragment.BaselineShift
		if fragment.ImageID != "" && fragment.ImageHeight > 0 {
			include(baseline+fragment.ImageHeight, baseline)
			continue
		}
		face, err := resolvePDFFontFace(fonts, fragment.FontKey)
		if err != nil {
			face = nil
		}
		ascent, descent := pdfFontVisualMetrics(face, fragment.FontSize)
		include(baseline+ascent, baseline-descent)
	}
}

func pdfFontVisualMetrics(face *builtinFontFace, fontSize float64) (float64, float64) {
	if fontSize <= 0 {
		fontSize = pdfBaseFontSize
	}
	if face == nil || face.UnitsPerEm <= 0 {
		return fontSize * 0.8, fontSize * 0.2
	}
	ascent := float64(face.Ascent) * fontSize / float64(face.UnitsPerEm)
	descent := -float64(face.Descent) * fontSize / float64(face.UnitsPerEm)
	if ascent <= 0 || descent < 0 {
		return fontSize * 0.8, fontSize * 0.2
	}
	return ascent, descent
}

func shiftPDFTitleContent(page *pdfPage, lineStart, lineEnd int, imageStart, imageEnd int, annotationStart, annotationEnd int, shift float64) {
	if page == nil || shift == 0 {
		return
	}
	for i := max(lineStart, 0); i < lineEnd && i < len(page.Lines); i++ {
		page.Lines[i].Y += shift
	}
	for i := max(imageStart, 0); i < imageEnd && i < len(page.Images); i++ {
		page.Images[i].Y += shift
	}
	for i := max(annotationStart, 0); i < annotationEnd && i < len(page.Annotations); i++ {
		page.Annotations[i].Rect.Y1 += shift
		page.Annotations[i].Rect.Y2 += shift
	}
}
