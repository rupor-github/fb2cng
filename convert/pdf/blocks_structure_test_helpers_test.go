package pdf

import (
	"strings"

	"fbc/fb2"
)

func appendTitleBlocks(blocks *[]pdfTextBlock, title *fb2.Title, depth int) {
	appendTitleBlocksWithOptions(blocks, pdfTitleBlockOptions{Title: title, Depth: depth, HeaderStyleName: pdfHeadingStyleName(depth)})
}

func appendTitleBlocksWithIDAndClasses(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string, styleClasses string) {
	appendTitleBlocksWithOptions(blocks, pdfTitleBlockOptions{
		Title:           title,
		Depth:           depth,
		ID:              id,
		HeaderStyleName: pdfHeadingStyleName(depth),
		StyleClasses:    styleClasses,
		ContextClasses:  strings.TrimSpace(styleClasses),
	})
}

func appendTitleBlocksWithIDHeaderAndClasses(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string, headerStyleName string, styleClasses string) {
	appendTitleBlocksWithOptions(blocks, pdfTitleBlockOptions{
		Title:           title,
		Depth:           depth,
		ID:              id,
		HeaderStyleName: headerStyleName,
		StyleClasses:    styleClasses,
		ContextClasses:  strings.TrimSpace(styleClasses),
	})
}

func appendParagraphBlockWithClasses(blocks *[]pdfTextBlock, kind pdfBlockKind, paragraph *fb2.Paragraph, depth int, styleClasses string) {
	appendParagraphBlockWithOptions(blocks, pdfParagraphBlockOptions{Kind: kind, Paragraph: paragraph, Depth: depth, StyleClasses: styleClasses, ContextClasses: strings.TrimSpace(styleClasses)})
}

func appendEpigraphBlocks(blocks *[]pdfTextBlock, epigraph *fb2.Epigraph) {
	appendEpigraphBlocksFull(blocks, nil, epigraph, "", false)
}
