package dumputil

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"fbc/convert/kfx"
	"fbc/utils/debug"
)

// DumpMarginsTxt writes the vertical margin tree to <stem>-margins.txt.
func DumpMarginsTxt(container *kfx.Container, inPath, outDir string, overwrite bool) error {
	dump, count := dumpMarginTree(container)
	dump += "\n"
	if err := WriteOutput(inPath, outDir, "-margins.txt", []byte(dump), overwrite); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stderr, "margins: wrote %d storyline(s) into output\n", count)
	return nil
}

// marginCtx holds context for margin tree formatting.
type marginCtx struct {
	c             *kfx.Container
	styleMargins  map[string]*marginInfo
	contentDefs   map[string]*ContentInfo
	resourceDefs  map[string]*ResourceInfo
	structureDefs map[string]any // structure fragment ID -> parsed value (KDF only)
}

// marginInfo holds margin values for a style.
type marginInfo struct {
	marginTop    string
	marginBottom string
	marginLeft   string
	marginRight  string
	lineHeight   string
}

// dumpMarginTree generates a focused view of vertical margins for easy comparison.
func dumpMarginTree(c *kfx.Container) (string, int) {
	styleMargins := collectStyleMargins(c)
	contentDefs := collectContentDefinitions(c)
	resourceDefs := collectResourceDefinitions(c)
	structureDefs := collectStructureDefinitions(c)

	tw := debug.NewTreeWriter()
	tw.Line(0, "=== KFX Vertical Margin Tree ===")
	tw.Line(0, "")

	if c.Fragments == nil {
		tw.Line(0, "(no fragments)")
		return tw.String(), 0
	}

	frags := c.Fragments.GetByType(kfx.SymStoryline)
	if len(frags) == 0 {
		tw.Line(0, "(no fragments)")
		return tw.String(), 0
	}

	sortedFrags := make([]*kfx.Fragment, len(frags))
	copy(sortedFrags, frags)
	slices.SortFunc(sortedFrags, func(a, b *kfx.Fragment) int {
		if a.IsRoot() && !b.IsRoot() {
			return -1
		}
		if !a.IsRoot() && b.IsRoot() {
			return 1
		}
		aFID := a.FID
		if a.FIDName != "" && aFID == 0 {
			aFID = c.GetLocalSymbolID(a.FIDName)
		}
		bFID := b.FID
		if b.FIDName != "" && bFID == 0 {
			bFID = c.GetLocalSymbolID(b.FIDName)
		}
		return int(aFID - bFID)
	})

	ctx := &marginCtx{
		c:             c,
		styleMargins:  styleMargins,
		contentDefs:   contentDefs,
		resourceDefs:  resourceDefs,
		structureDefs: structureDefs,
	}

	for _, f := range sortedFrags {
		storyName := ""
		if f.FIDName != "" {
			storyName = f.FIDName
		} else if c.DocSymbolTable != nil {
			if name, ok := c.DocSymbolTable.FindByID(uint64(f.FID)); ok && !strings.HasPrefix(name, "$") {
				storyName = name
			}
		}

		tw.Line(0, "storyline: %s", storyName)

		contentList := extractContentList(f.Value)
		formatMarginElements(tw, ctx, contentList, 1, "", "")
		tw.Line(0, "")
	}

	return tw.String(), len(sortedFrags)
}

// collectStyleMargins extracts margin values from all style definitions.
func collectStyleMargins(c *kfx.Container) map[string]*marginInfo {
	margins := make(map[string]*marginInfo)
	if c.Fragments == nil {
		return margins
	}

	for _, frag := range c.Fragments.GetByType(kfx.SymStyle) {
		styleName := ""
		if frag.FIDName != "" {
			styleName = frag.FIDName
		} else if c.DocSymbolTable != nil {
			if name, ok := c.DocSymbolTable.FindByID(uint64(frag.FID)); ok && !strings.HasPrefix(name, "$") {
				styleName = name
			}
		}
		if styleName == "" {
			continue
		}

		info := &marginInfo{}
		props := kfx.NormalizeStyleMap(frag.Value)
		if props == nil {
			continue
		}

		if mt, ok := props["margin_top"]; ok {
			info.marginTop = kfx.FormatDimensionAsCSS(mt)
		}
		if mb, ok := props["margin_bottom"]; ok {
			info.marginBottom = kfx.FormatDimensionAsCSS(mb)
		}
		if ml, ok := props["margin_left"]; ok {
			info.marginLeft = kfx.FormatDimensionAsCSS(ml)
		}
		if mr, ok := props["margin_right"]; ok {
			info.marginRight = kfx.FormatDimensionAsCSS(mr)
		}
		if lh, ok := props["line_height"]; ok {
			info.lineHeight = kfx.FormatDimensionAsCSS(lh)
		}

		margins[styleName] = info
	}
	return margins
}

// extractContentList extracts the content_list array from a storyline value.
func extractContentList(v any) []any {
	m := toMapAny(v)
	if m == nil {
		return nil
	}
	if cl, ok := m["$146"]; ok {
		return toListAny(cl)
	}
	return nil
}

// formatMarginElements formats a list of content elements showing only margins.
func formatMarginElements(tw *debug.TreeWriter, ctx *marginCtx, items []any, depth int, indexPrefix string, parentLineHeight string) {
	for i, item := range items {
		idx := fmt.Sprintf("%d", i)
		if indexPrefix != "" {
			idx = indexPrefix + "." + idx
		}
		// Resolve string references to structure fragments (KDF).
		resolved := resolveStructureRefMargin(ctx, item)
		formatMarginElement(tw, ctx, resolved, depth, idx, parentLineHeight)
	}
}

// resolveStructureRefMargin resolves a string reference to a structure fragment value.
func resolveStructureRefMargin(ctx *marginCtx, item any) any {
	if ctx.structureDefs == nil {
		return item
	}
	if s, ok := item.(string); ok {
		if val, found := ctx.structureDefs[s]; found {
			return val
		}
	}
	if s, ok := item.(kfx.ReadSymbolValue); ok {
		str := string(s)
		if !strings.HasPrefix(str, "$") {
			if val, found := ctx.structureDefs[str]; found {
				return val
			}
		}
	}
	return item
}

// formatMarginElement formats a single content element showing margins.
func formatMarginElement(tw *debug.TreeWriter, ctx *marginCtx, v any, depth int, idx string, parentLineHeight string) {
	m := toMapAny(v)
	if m == nil {
		return
	}

	elemType := "unknown"
	if typeVal, ok := m["$159"]; ok {
		elemType = extractSymbolName(typeVal)
	}

	styleName := ""
	if styleVal, ok := m["$157"]; ok {
		styleName = extractSymbolName(styleVal)
	}

	var mi *marginInfo
	if styleName != "" {
		mi = ctx.styleMargins[styleName]
	}

	effectiveLineHeight := parentLineHeight
	if mi != nil && mi.lineHeight != "" {
		effectiveLineHeight = mi.lineHeight
		if mi.lineHeight == parentLineHeight {
			copied := *mi
			copied.lineHeight = ""
			mi = &copied
		}
	}

	preview := ""
	switch elemType {
	case "text", "$269":
		preview = getTextPreview(ctx, m)
	case "image", "$271":
		preview = getImageAlt(ctx, m)
	}

	nestedList := extractContentList(v)
	isContainer := len(nestedList) > 0

	marginStr := formatMarginStr(mi)

	if isContainer {
		childCount := len(nestedList)
		tw.Line(depth, "[%s] container (%d items)%s", idx, childCount, marginStr)
		formatMarginElements(tw, ctx, nestedList, depth+1, idx, effectiveLineHeight)
	} else {
		previewStr := ""
		if preview != "" {
			previewStr = fmt.Sprintf(" %q", truncateText(preview, 40))
		}
		tw.Line(depth, "[%s] %s%s%s", idx, elemType, previewStr, marginStr)
	}
}

// formatMarginStr formats margin info as a compact string.
func formatMarginStr(mi *marginInfo) string {
	if mi == nil {
		return ""
	}

	parts := []string{}
	if mi.marginTop != "" && mi.marginTop != "0" {
		parts = append(parts, fmt.Sprintf("mt=%s", mi.marginTop))
	}
	if mi.marginBottom != "" && mi.marginBottom != "0" {
		parts = append(parts, fmt.Sprintf("mb=%s", mi.marginBottom))
	}
	if mi.marginLeft != "" && mi.marginLeft != "0" && mi.marginLeft != "0%" {
		parts = append(parts, fmt.Sprintf("ml=%s", mi.marginLeft))
	}
	if mi.lineHeight != "" {
		parts = append(parts, fmt.Sprintf("lh=%s", mi.lineHeight))
	}

	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// getTextPreview extracts text preview from content reference.
// Handles both KFX format (content is a struct with name/index referencing a $145 fragment)
// and KDF format (content is a direct inline string).
func getTextPreview(ctx *marginCtx, m map[string]any) string {
	contentVal, ok := m["$145"]
	if !ok {
		return ""
	}

	// KDF: content ($145) is a direct string.
	if s, ok := contentVal.(string); ok {
		return truncateText(s, 60)
	}

	// KFX: content ($145) is a struct with name and index referencing a content fragment.
	cm := toMapAny(contentVal)
	if cm == nil {
		return ""
	}
	contentName := ""
	if nameVal, ok := cm["name"]; ok {
		contentName = extractSymbolName(nameVal)
	}
	var contentIndex int64 = -1
	if idxVal, ok := cm["$403"]; ok {
		contentIndex = toInt64(idxVal)
	}
	if contentName != "" && contentIndex >= 0 {
		if info, ok := ctx.contentDefs[contentName]; ok {
			if int(contentIndex) < len(info.Texts) {
				return info.Texts[contentIndex]
			}
		}
	}
	return ""
}

// getImageAlt extracts alt text or resource name for an image.
func getImageAlt(ctx *marginCtx, m map[string]any) string {
	if altVal, ok := m["$584"]; ok {
		if s, ok := altVal.(string); ok {
			return s
		}
	}
	if resVal, ok := m["$175"]; ok {
		resName := extractSymbolName(resVal)
		if info, ok := ctx.resourceDefs[resName]; ok {
			if info.Location != "" {
				return info.Location
			}
		}
		return resName
	}
	return ""
}
