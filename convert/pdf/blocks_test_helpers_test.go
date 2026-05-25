package pdf

import (
	"fbc/common"
	"fbc/convert/pdf/structure"
)

func buildTOCPageBlocks(entries []*structure.TOCEntry, includeUntitled bool, tocType common.TOCType) []pdfTextBlock {
	return buildTOCPageBlocksWithTitle(pdfTitleFromStrings("Contents"), entries, includeUntitled, tocType)
}
