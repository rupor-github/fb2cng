package pdf

func shapeOpenTypeText(face *builtinFontFace, text string) (shapedText, error) {
	shaper := openTypePDFTextShaper{face: face}
	return shaper.Shape(text, pdfShapeOptions{})
}

func utf16BEHex(r rune) string {
	return utf16BEHexString(string(r))
}
