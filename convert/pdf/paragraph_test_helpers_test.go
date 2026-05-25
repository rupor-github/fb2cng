package pdf

func chooseParagraphBreaks(units []paragraphUnit, spaceWidth float64, style paragraphStyle, maxWidth float64) []paragraphBreak {
	return chooseParagraphBreaksWithShape(units, spaceWidth, style, maxWidth, paragraphLineShape{})
}

func paragraphLineJustificationAvailable(line paragraphLine, fontSize float64, letterSpacing float64, available float64) float64 {
	return paragraphJustificationAvailableForOverhang(available, paragraphLineVisualRightReserve(line, fontSize, letterSpacing))
}
