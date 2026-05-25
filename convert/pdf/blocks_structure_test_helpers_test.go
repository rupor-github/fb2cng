package pdf

import (
	"strings"

	"fbc/fb2"
)

func appendTitleBlocks(blocks *[]pdfTextBlock, title *fb2.Title, depth int) {
	appendTitleBlocksFull(blocks, nil, title, depth, "", pdfHeadingStyleName(depth), "", "", false)
}

func appendTitleBlocksWithIDAndClasses(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string, styleClasses string) {
	appendTitleBlocksFull(blocks, nil, title, depth, id, pdfHeadingStyleName(depth), styleClasses, strings.TrimSpace(styleClasses), false)
}

func appendTitleBlocksWithIDHeaderAndClasses(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string, headerStyleName string, styleClasses string) {
	appendTitleBlocksFull(blocks, nil, title, depth, id, headerStyleName, styleClasses, strings.TrimSpace(styleClasses), false)
}

func appendParagraphBlockWithClasses(blocks *[]pdfTextBlock, kind pdfBlockKind, paragraph *fb2.Paragraph, depth int, styleClasses string) {
	appendParagraphBlockFull(blocks, nil, kind, paragraph, depth, styleClasses, strings.TrimSpace(styleClasses), false)
}

func appendEpigraphBlocks(blocks *[]pdfTextBlock, epigraph *fb2.Epigraph) {
	appendEpigraphBlocksFull(blocks, nil, epigraph, "", false)
}
