package pdf

import (
	"slices"

	"fbc/convert/pdf/docwriter"
)

func pdfDebugFonts(resources []pdfFontResource) []pdfDebugFont {
	out := make([]pdfDebugFont, 0, len(resources))
	for _, resource := range resources {
		if resource.Face == nil {
			continue
		}
		usedGlyphIDs := make([]uint16, 0, len(resource.Used))
		for glyphID := range resource.Used {
			usedGlyphIDs = append(usedGlyphIDs, glyphID)
		}
		slices.Sort(usedGlyphIDs)
		originalSize := len(resource.Face.Data)
		embeddedSize := len(resource.Objects.FontFileData)
		subset := embeddedSize > 0 && embeddedSize < originalSize
		program := pdfDebugFontProgram(resource.Face)
		pdfCIDs, subsetGlyphIDs, glyphIDMap := pdfDebugFontGlyphIDMapping(resource, usedGlyphIDs, subset && program.TrueTypeOutlines)
		out = append(out, pdfDebugFont{
			ResourceName:               resource.Name,
			Family:                     resource.Key.Family,
			Bold:                       resource.Key.Bold,
			Italic:                     resource.Key.Italic,
			PostScriptName:             resource.Face.PostScriptName,
			PDFBaseFont:                pdfDebugFontBaseName(resource),
			OutlineKind:                program.OutlineKind,
			PDFCIDFontSubtype:          string(program.CIDFontSubtype),
			PDFEmbeddedFontFile:        program.FontFileKey,
			UnitsPerEm:                 resource.Face.UnitsPerEm,
			Ascent:                     resource.Face.Ascent,
			Descent:                    resource.Face.Descent,
			CapHeight:                  resource.Face.CapHeight,
			BBox:                       resource.Face.BBox,
			Flags:                      resource.Face.Flags,
			ItalicAngle:                resource.Face.ItalicAngle,
			OriginalFontFileSize:       originalSize,
			EmbeddedFontFileSize:       embeddedSize,
			EmbeddedFontFileStreamSize: compressedPDFStreamSize(resource.Objects.FontFileData),
			ToUnicodeStreamSize:        compressedPDFStreamSize(resource.Objects.ToUnicode),
			Subset:                     subset,
			UsedGlyphCount:             len(usedGlyphIDs),
			UsedGlyphIDs:               usedGlyphIDs,
			OriginalGlyphIDs:           usedGlyphIDs,
			PDFCIDs:                    pdfCIDs,
			SubsetGlyphIDs:             subsetGlyphIDs,
			GlyphIDMap:                 glyphIDMap,
		})
	}
	return out
}

func pdfDebugFontGlyphIDMapping(
	resource pdfFontResource,
	usedGlyphIDs []uint16,
	includeSubsetGIDs bool,
) ([]uint16, []uint16, []pdfDebugFontGlyphIDMap) {
	used := make(map[uint16]bool, len(usedGlyphIDs))
	pdfCIDs := make([]uint16, 0, len(usedGlyphIDs))
	for _, originalGlyphID := range usedGlyphIDs {
		used[originalGlyphID] = true
		if cid, ok := resource.CIDMap[originalGlyphID]; ok {
			pdfCIDs = append(pdfCIDs, cid)
		}
	}
	slices.Sort(pdfCIDs)
	pdfCIDs = compactSortedUint16s(pdfCIDs)

	originalIDs := make([]int, 0, len(resource.CIDMap))
	for originalGlyphID := range resource.CIDMap {
		originalIDs = append(originalIDs, int(originalGlyphID))
	}
	slices.Sort(originalIDs)

	glyphIDMap := make([]pdfDebugFontGlyphIDMap, 0, len(originalIDs))
	subsetGlyphIDs := make([]uint16, 0, len(originalIDs))
	for _, originalGlyphIDInt := range originalIDs {
		originalGlyphID := uint16(originalGlyphIDInt)
		cid := resource.CIDMap[originalGlyphID]
		entry := pdfDebugFontGlyphIDMap{
			OriginalGlyphID: originalGlyphID,
			PDFCID:          cid,
			Used:            used[originalGlyphID],
		}
		if includeSubsetGIDs {
			entry.SubsetGlyphID = cid
			subsetGlyphIDs = append(subsetGlyphIDs, cid)
		}
		glyphIDMap = append(glyphIDMap, entry)
	}
	slices.Sort(subsetGlyphIDs)
	subsetGlyphIDs = compactSortedUint16s(subsetGlyphIDs)
	return pdfCIDs, subsetGlyphIDs, glyphIDMap
}

func compactSortedUint16s(values []uint16) []uint16 {
	if len(values) < 2 {
		return values
	}
	out := values[:0]
	var previous uint16
	for i, value := range values {
		if i > 0 && value == previous {
			continue
		}
		out = append(out, value)
		previous = value
	}
	return out
}

func pdfDebugFontProgram(face *builtinFontFace) pdfFontProgramInfo {
	if face == nil {
		return pdfFontProgramInfo{}
	}
	program, err := pdfFontProgram(face.Data)
	if err != nil {
		return pdfFontProgramInfo{}
	}
	return program
}

func pdfDebugFontBaseName(resource pdfFontResource) string {
	baseFont, ok := resource.Objects.Type0Font["BaseFont"].(docwriter.Name)
	if !ok {
		return ""
	}
	return string(baseFont)
}
