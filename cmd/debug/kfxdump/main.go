package main

import (
	"archive/zip"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/amazon-ion/ion-go/ion"
	"github.com/h2non/filetype"

	"fbc/convert/kfx"
	"fbc/utils/debug"
)

func main() {
	all := flag.Bool("all", false, "enable all dump flags (-dump, -resources, -styles, -storyline)")
	dump := flag.Bool("dump", false, "dump all fragments into <file>-dump.txt")
	resources := flag.Bool("resources", false, "dump $417 (bcRawMedia) raw bytes into <file>-resources.zip")
	styles := flag.Bool("styles", false, "dump $157 (style) fragments into <file>-styles.txt")
	storyline := flag.Bool("storyline", false, "dump $259 (storyline) fragments into <file>-storyline.txt with expanded symbols and styles")
	margins := flag.Bool("margins", false, "dump vertical margin tree into <file>-margins.txt for easy comparison")
	overwrite := flag.Bool("overwrite", false, "overwrite existing output")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "usage: kfxdump [-all] [-dump] [-resources] [-styles] [-storyline] [-margins] [-overwrite] <file.kfx> [outdir]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 || flag.NArg() > 2 {
		flag.Usage()
		os.Exit(2)
	}

	if *all {
		*dump = true
		*resources = true
		*styles = true
		*storyline = true
		*margins = true
	}

	if !*dump && !*resources && !*styles && !*storyline && !*margins {
		flag.Usage()
		os.Exit(2)
	}

	defer func(startedAt time.Time) {
		duration := time.Since(startedAt)
		fmt.Fprintf(os.Stderr, "\nExecution time: %s\n", duration)
	}(time.Now())

	path := flag.Arg(0)
	outDir := ""
	if flag.NArg() == 2 {
		outDir = flag.Arg(1)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		os.Exit(1)
	}

	container, err := kfx.ReadContainer(b)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
		os.Exit(1)
	}

	if *dump {
		if err := dumpDumpTxt(container, path, outDir, *overwrite); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "dump: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(container.StatsString())
	}

	if *resources {
		if err := dumpResources(container, path, outDir, *overwrite); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "dump resources: %v\n", err)
			os.Exit(1)
		}
	}

	if *styles {
		if err := dumpStylesTxt(container, path, outDir, *overwrite); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "dump styles: %v\n", err)
			os.Exit(1)
		}
	}

	if *storyline {
		if err := dumpStorylineTxt(container, path, outDir, *overwrite); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "dump storyline: %v\n", err)
			os.Exit(1)
		}
	}

	if *margins {
		if err := dumpMarginsTxt(container, path, outDir, *overwrite); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "dump margins: %v\n", err)
			os.Exit(1)
		}
	}
}

func dumpDumpTxt(container *kfx.Container, inPath, outDir string, overwrite bool) error {
	base := filepath.Base(inPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	dir := filepath.Dir(inPath)
	if outDir != "" {
		dir = outDir
	}
	outPath := filepath.Join(dir, stem+"-dump.txt")
	if _, err := os.Stat(outPath); err == nil {
		if !overwrite {
			return fmt.Errorf("output file already exists: %s", outPath)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	dump := container.String() + "\n\n" + container.DumpFragments() + "\n"
	return os.WriteFile(outPath, []byte(dump), 0o644)
}

func dumpStylesTxt(container *kfx.Container, inPath, outDir string, overwrite bool) error {
	base := filepath.Base(inPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	dir := filepath.Dir(inPath)
	if outDir != "" {
		dir = outDir
	}
	outPath := filepath.Join(dir, stem+"-styles.txt")
	if _, err := os.Stat(outPath); err == nil {
		if !overwrite {
			return fmt.Errorf("output file already exists: %s", outPath)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	dump, count := dumpStyleFragments(container)
	dump += "\n"
	if err := os.WriteFile(outPath, []byte(dump), 0o644); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stderr, "styles: wrote %d fragment(s) into %s\n", count, outPath)
	return nil
}

func dumpStorylineTxt(container *kfx.Container, inPath, outDir string, overwrite bool) error {
	base := filepath.Base(inPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	dir := filepath.Dir(inPath)
	if outDir != "" {
		dir = outDir
	}
	outPath := filepath.Join(dir, stem+"-storyline.txt")
	if _, err := os.Stat(outPath); err == nil {
		if !overwrite {
			return fmt.Errorf("output file already exists: %s", outPath)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	dump, count := dumpStorylineFragments(container)
	dump += "\n"
	if err := os.WriteFile(outPath, []byte(dump), 0o644); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stderr, "storyline: wrote %d fragment(s) into %s\n", count, outPath)
	return nil
}

func dumpMarginsTxt(container *kfx.Container, inPath, outDir string, overwrite bool) error {
	base := filepath.Base(inPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	dir := filepath.Dir(inPath)
	if outDir != "" {
		dir = outDir
	}
	outPath := filepath.Join(dir, stem+"-margins.txt")
	if _, err := os.Stat(outPath); err == nil {
		if !overwrite {
			return fmt.Errorf("output file already exists: %s", outPath)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	dump, count := dumpMarginTree(container)
	dump += "\n"
	if err := os.WriteFile(outPath, []byte(dump), 0o644); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stderr, "margins: wrote %d storyline(s) into %s\n", count, outPath)
	return nil
}

func dumpStyleFragments(c *kfx.Container) (string, int) {
	// First, collect style usage information
	styleUsage := collectStyleUsage(c)

	return dumpFragmentsByTypeWithUsage(c, kfx.SymStyle, styleUsage)
}

func dumpStorylineFragments(c *kfx.Container) (string, int) {
	// Collect style definitions for inline expansion
	styleDefs := collectStyleDefinitions(c)
	// Collect content fragments for text preview
	contentDefs := collectContentDefinitions(c)
	// Collect resource fragments for resource info
	resourceDefs := collectResourceDefinitions(c)

	return dumpStorylineFragmentsExpanded(c, styleDefs, contentDefs, resourceDefs)
}

// dumpMarginTree generates a focused view of vertical margins for easy comparison.
// Output format shows only margin-top/margin-bottom for each element in the storyline.
func dumpMarginTree(c *kfx.Container) (string, int) {
	// Collect style definitions for margin extraction
	styleMargins := collectStyleMargins(c)
	// Collect content fragments for text preview
	contentDefs := collectContentDefinitions(c)
	// Collect resource fragments for alt text
	resourceDefs := collectResourceDefinitions(c)

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

	// Sort storyline fragments
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
		c:            c,
		styleMargins: styleMargins,
		contentDefs:  contentDefs,
		resourceDefs: resourceDefs,
	}

	for _, f := range sortedFrags {
		// Get storyline name
		storyName := ""
		if f.FIDName != "" {
			storyName = f.FIDName
		} else if c.DocSymbolTable != nil {
			if name, ok := c.DocSymbolTable.FindByID(uint64(f.FID)); ok && !strings.HasPrefix(name, "$") {
				storyName = name
			}
		}

		tw.Line(0, "storyline: %s", storyName)

		// Extract content_list from storyline
		contentList := extractContentList(f.Value)
		formatMarginElements(tw, ctx, contentList, 1, "", "")
		tw.Line(0, "")
	}

	return tw.String(), len(sortedFrags)
}

// marginCtx holds context for margin tree formatting.
type marginCtx struct {
	c            *kfx.Container
	styleMargins map[string]*marginInfo
	contentDefs  map[string]*ContentInfo
	resourceDefs map[string]*ResourceInfo
}

// marginInfo holds margin values for a style.
type marginInfo struct {
	marginTop    string
	marginBottom string
	marginLeft   string
	marginRight  string
	lineHeight   string
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

		// Extract margin values
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
	// Look for $146 (content_list)
	if cl, ok := m["$146"]; ok {
		return toListAny(cl)
	}
	return nil
}

// formatMarginElements formats a list of content elements showing only margins.
// parentLineHeight is used to omit redundant lh=... on children (KP3 dumps do this).
func formatMarginElements(tw *debug.TreeWriter, ctx *marginCtx, items []any, depth int, indexPrefix string, parentLineHeight string) {
	for i, item := range items {
		idx := fmt.Sprintf("%d", i)
		if indexPrefix != "" {
			idx = indexPrefix + "." + idx
		}
		formatMarginElement(tw, ctx, item, depth, idx, parentLineHeight)
	}
}

// formatMarginElement formats a single content element showing margins.
func formatMarginElement(tw *debug.TreeWriter, ctx *marginCtx, v any, depth int, idx string, parentLineHeight string) {
	m := toMapAny(v)
	if m == nil {
		return
	}

	// Get element type
	elemType := "unknown"
	if typeVal, ok := m["$159"]; ok {
		elemType = extractSymbolName(typeVal)
	}

	// Get style name and margins
	styleName := ""
	if styleVal, ok := m["$157"]; ok {
		styleName = extractSymbolName(styleVal)
	}

	var mi *marginInfo
	if styleName != "" {
		mi = ctx.styleMargins[styleName]
	}

	// Only display line-height if it differs from the parent's effective line-height.
	// KP3 margin dumps omit redundant lh=... for children.
	effectiveLineHeight := parentLineHeight
	if mi != nil && mi.lineHeight != "" {
		effectiveLineHeight = mi.lineHeight
		if mi.lineHeight == parentLineHeight {
			// Avoid mutating shared styleMargins entries.
			copied := *mi
			copied.lineHeight = ""
			mi = &copied
		}
	}

	// Get text preview or resource name
	preview := ""
	switch elemType {
	case "text", "$269":
		preview = getTextPreview(ctx, m)
	case "image", "$271":
		preview = getImageAlt(ctx, m)
	}

	// Check if this is a container (has nested content_list)
	nestedList := extractContentList(v)
	isContainer := len(nestedList) > 0

	// Build margin string
	marginStr := formatMarginStr(mi)

	// Output the element
	if isContainer {
		// Container element - show it as a wrapper
		childCount := len(nestedList)
		tw.Line(depth, "[%s] container (%d items)%s", idx, childCount, marginStr)

		// Recursively format children
		formatMarginElements(tw, ctx, nestedList, depth+1, idx, effectiveLineHeight)
	} else {
		// Leaf element
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
func getTextPreview(ctx *marginCtx, m map[string]any) string {
	// Check for content reference ($145)
	if contentVal, ok := m["$145"]; ok {
		cm := toMapAny(contentVal)
		if cm == nil {
			return ""
		}
		// Get content name and index
		contentName := ""
		if nameVal, ok := cm["name"]; ok {
			contentName = extractSymbolName(nameVal)
		}
		var contentIndex int64 = -1
		if idxVal, ok := cm["$403"]; ok {
			contentIndex = toInt64(idxVal)
		}
		// Look up in content definitions
		if contentName != "" && contentIndex >= 0 {
			if info, ok := ctx.contentDefs[contentName]; ok {
				if int(contentIndex) < len(info.Texts) {
					return info.Texts[contentIndex]
				}
			}
		}
	}
	return ""
}

// getImageAlt extracts alt text or resource name for an image.
func getImageAlt(ctx *marginCtx, m map[string]any) string {
	// Check for alt_text ($584)
	if altVal, ok := m["$584"]; ok {
		if s, ok := altVal.(string); ok {
			return s
		}
	}
	// Fall back to resource name
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

// collectStyleDefinitions builds a map of style name -> CSS representation.
func collectStyleDefinitions(c *kfx.Container) map[string]string {
	defs := make(map[string]string)
	if c.Fragments == nil {
		return defs
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

		// Get a compact CSS-like summary of the style
		css := kfx.FormatStylePropsAsCSS(frag.Value)
		if css == "" {
			css = "(empty)"
		}
		defs[styleName] = css
	}
	return defs
}

// ContentInfo holds text snippets for a content fragment.
type ContentInfo struct {
	Name  string   // Content fragment name
	Texts []string // Text snippets (truncated)
}

// collectContentDefinitions builds a map of content name -> text snippets.
func collectContentDefinitions(c *kfx.Container) map[string]*ContentInfo {
	defs := make(map[string]*ContentInfo)
	if c.Fragments == nil {
		return defs
	}

	for _, frag := range c.Fragments.GetByType(kfx.SymContent) {
		contentName := ""
		if frag.FIDName != "" {
			contentName = frag.FIDName
		} else if c.DocSymbolTable != nil {
			if name, ok := c.DocSymbolTable.FindByID(uint64(frag.FID)); ok && !strings.HasPrefix(name, "$") {
				contentName = name
			}
		}
		if contentName == "" {
			continue
		}

		info := &ContentInfo{Name: contentName}

		// Extract content_list ($146) which contains the text strings
		switch m := frag.Value.(type) {
		case kfx.StructValue:
			if listVal, ok := m[kfx.SymContentList]; ok {
				list := toListAny(listVal)
				for _, item := range list {
					if s, ok := item.(string); ok {
						info.Texts = append(info.Texts, truncateText(s, 60))
					}
				}
			}
		case map[kfx.KFXSymbol]any:
			if listVal, ok := m[kfx.SymContentList]; ok {
				list := toListAny(listVal)
				for _, item := range list {
					if s, ok := item.(string); ok {
						info.Texts = append(info.Texts, truncateText(s, 60))
					}
				}
			}
		case map[string]any:
			// Content list is stored as "$146"
			if listVal, ok := m["$146"]; ok {
				list := toListAny(listVal)
				for _, item := range list {
					if s, ok := item.(string); ok {
						info.Texts = append(info.Texts, truncateText(s, 60))
					}
				}
			}
		}

		defs[contentName] = info
	}
	return defs
}

// ResourceInfo holds info about an external resource.
type ResourceInfo struct {
	Name     string // Resource name
	Location string // File path ($165)
	Format   string // Format type ($161)
	MIME     string // MIME type ($162)
	Width    int64  // Width ($422)
	Height   int64  // Height ($423)
}

// collectResourceDefinitions builds a map of resource name -> resource info.
func collectResourceDefinitions(c *kfx.Container) map[string]*ResourceInfo {
	defs := make(map[string]*ResourceInfo)
	if c.Fragments == nil {
		return defs
	}

	for _, frag := range c.Fragments.GetByType(kfx.SymExtResource) {
		resourceName := ""
		if frag.FIDName != "" {
			resourceName = frag.FIDName
		} else if c.DocSymbolTable != nil {
			if name, ok := c.DocSymbolTable.FindByID(uint64(frag.FID)); ok && !strings.HasPrefix(name, "$") {
				resourceName = name
			}
		}
		if resourceName == "" {
			continue
		}

		info := &ResourceInfo{Name: resourceName}

		// Handle StructValue directly for better access
		switch m := frag.Value.(type) {
		case kfx.StructValue:
			if loc, ok := m[kfx.SymLocation]; ok {
				if s, ok := loc.(string); ok {
					info.Location = s
				}
			}
			if fmt, ok := m[kfx.SymFormat]; ok {
				info.Format = extractSymbolName(fmt)
			}
			if mime, ok := m[kfx.SymMIME]; ok {
				if s, ok := mime.(string); ok {
					info.MIME = s
				}
			}
			if w, ok := m[kfx.KFXSymbol(422)]; ok {
				info.Width = toInt64(w)
			}
			if h, ok := m[kfx.KFXSymbol(423)]; ok {
				info.Height = toInt64(h)
			}
		case map[kfx.KFXSymbol]any:
			if loc, ok := m[kfx.SymLocation]; ok {
				if s, ok := loc.(string); ok {
					info.Location = s
				}
			}
			if fmt, ok := m[kfx.SymFormat]; ok {
				info.Format = extractSymbolName(fmt)
			}
			if mime, ok := m[kfx.SymMIME]; ok {
				if s, ok := mime.(string); ok {
					info.MIME = s
				}
			}
			if w, ok := m[kfx.KFXSymbol(422)]; ok {
				info.Width = toInt64(w)
			}
			if h, ok := m[kfx.KFXSymbol(423)]; ok {
				info.Height = toInt64(h)
			}
		case map[string]any:
			// Handle "$NNN" string keys
			if loc, ok := m["$165"]; ok {
				if s, ok := loc.(string); ok {
					info.Location = s
				}
			}
			if fmt, ok := m["$161"]; ok {
				info.Format = extractSymbolName(fmt)
			}
			if mime, ok := m["$162"]; ok {
				if s, ok := mime.(string); ok {
					info.MIME = s
				}
			}
			if w, ok := m["$422"]; ok {
				info.Width = toInt64(w)
			}
			if h, ok := m["$423"]; ok {
				info.Height = toInt64(h)
			}
		}

		defs[resourceName] = info
	}
	return defs
}

// truncateText truncates a string to maxLen, adding "..." if truncated.
// Also replaces newlines with spaces for compact display.
func truncateText(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// toInt64 converts various numeric types to int64.
func toInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

// expandCtx holds all the context needed for expanding storyline values.
type expandCtx struct {
	c            *kfx.Container
	styleDefs    map[string]string
	contentDefs  map[string]*ContentInfo
	resourceDefs map[string]*ResourceInfo
}

func dumpStorylineFragmentsExpanded(c *kfx.Container, styleDefs map[string]string, contentDefs map[string]*ContentInfo, resourceDefs map[string]*ResourceInfo) (string, int) {
	tw := debug.NewTreeWriter()

	ctx := &expandCtx{
		c:            c,
		styleDefs:    styleDefs,
		contentDefs:  contentDefs,
		resourceDefs: resourceDefs,
	}

	tw.Line(0, "=== KFX Storyline Fragments (Expanded) ===")
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

	// Sort storyline fragments
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

	tw.Line(0, "$259 (storyline) - %d fragment(s)", len(sortedFrags))

	for _, f := range sortedFrags {
		if f.IsRoot() {
			tw.Line(1, "[root]")
		} else {
			var fidID kfx.KFXSymbol
			var fidName string

			if f.FIDName != "" {
				fidName = f.FIDName
				if len(c.LocalSymbols) > 0 {
					fidID = c.GetLocalSymbolID(f.FIDName)
					if fidID < 0 && f.FID > 0 {
						fidID = f.FID
					}
				} else if f.FID > 0 {
					fidID = f.FID
				}
			} else {
				fidID = f.FID
				if c.DocSymbolTable != nil {
					name, ok := c.DocSymbolTable.FindByID(uint64(fidID))
					if ok && !strings.HasPrefix(name, "$") {
						fidName = name
					}
				}
			}

			if fidName != "" {
				tw.Line(1, "id=$%d (%s)", fidID, fidName)
			} else {
				tw.Line(1, "id=$%d", fidID)
			}
		}

		// Format the storyline value with expanded symbols and styles
		formatValueTree(tw, ctx, f.Value, 2)
	}

	return tw.String(), len(sortedFrags)
}

// formatValueTree formats a value in tree format with expanded styles/content/resources.
func formatValueTree(tw *debug.TreeWriter, ctx *expandCtx, v any, depth int) {
	switch val := v.(type) {
	case nil:
		tw.Line(depth, "null")

	case bool:
		tw.Line(depth, "%v", val)

	case int:
		tw.Line(depth, "int(%d)", val)

	case int64:
		tw.Line(depth, "int(%d)", val)

	case int32:
		tw.Line(depth, "int(%d)", val)

	case float64:
		tw.Line(depth, "float(%g)", val)

	case *ion.Decimal:
		tw.Line(depth, "decimal(%s)", val.String())

	case string:
		tw.Line(depth, "%q", val)

	case []byte:
		tw.Line(depth, "blob(%d bytes)", len(val))

	case kfx.RawValue:
		tw.Line(depth, "raw(%d bytes)", len(val))

	case kfx.SymbolValue:
		tw.Line(depth, "symbol(%s)", kfx.KFXSymbol(val).String())

	case kfx.SymbolByNameValue:
		name := string(val)
		if css, ok := ctx.styleDefs[name]; ok {
			tw.Line(depth, "symbol(%q) /* %s */", name, css)
		} else {
			tw.Line(depth, "symbol(%q)", name)
		}

	case kfx.ReadSymbolValue:
		symStr := string(val)
		if strings.HasPrefix(symStr, "$") {
			if id, err := strconv.Atoi(symStr[1:]); err == nil {
				tw.Line(depth, "symbol(%s)", kfx.KFXSymbol(id).String())
			} else {
				tw.Line(depth, "symbol(%s)", symStr)
			}
		} else {
			if css, ok := ctx.styleDefs[symStr]; ok {
				tw.Line(depth, "symbol(%q) /* %s */", symStr, css)
			} else {
				tw.Line(depth, "symbol(%q)", symStr)
			}
		}

	case kfx.StructValue:
		formatStructTree(tw, ctx, val, depth)

	case map[kfx.KFXSymbol]any:
		formatStructTree(tw, ctx, val, depth)

	case map[string]any:
		formatMapStringTree(tw, ctx, val, depth)

	case kfx.ListValue:
		formatListTree(tw, ctx, []any(val), depth)

	case []any:
		formatListTree(tw, ctx, val, depth)

	default:
		tw.Line(depth, "<%T>", v)
	}
}

// formatSimpleValueStr returns a simple value as string, or empty if not simple.
func formatSimpleValueStr(ctx *expandCtx, v any) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case bool:
		return fmt.Sprintf("%v", val)
	case int:
		return fmt.Sprintf("int(%d)", val)
	case int64:
		return fmt.Sprintf("int(%d)", val)
	case int32:
		return fmt.Sprintf("int(%d)", val)
	case float64:
		return fmt.Sprintf("float(%g)", val)
	case *ion.Decimal:
		return fmt.Sprintf("decimal(%s)", val.String())
	case string:
		return fmt.Sprintf("%q", val)
	case []byte:
		return fmt.Sprintf("blob(%d bytes)", len(val))
	case kfx.RawValue:
		return fmt.Sprintf("raw(%d bytes)", len(val))
	case kfx.SymbolValue:
		return fmt.Sprintf("symbol(%s)", kfx.KFXSymbol(val).String())
	case kfx.SymbolByNameValue:
		name := string(val)
		if css, ok := ctx.styleDefs[name]; ok {
			return fmt.Sprintf("symbol(%q) /* %s */", name, css)
		}
		return fmt.Sprintf("symbol(%q)", name)
	case kfx.ReadSymbolValue:
		symStr := string(val)
		if strings.HasPrefix(symStr, "$") {
			if id, err := strconv.Atoi(symStr[1:]); err == nil {
				return fmt.Sprintf("symbol(%s)", kfx.KFXSymbol(id).String())
			}
			return fmt.Sprintf("symbol(%s)", symStr)
		}
		if css, ok := ctx.styleDefs[symStr]; ok {
			return fmt.Sprintf("symbol(%q) /* %s */", symStr, css)
		}
		return fmt.Sprintf("symbol(%q)", symStr)
	default:
		return ""
	}
}

func isSimpleValueExpanded(v any) bool {
	switch v.(type) {
	case nil, bool, int, int64, int32, float64, string, *ion.Decimal,
		[]byte, kfx.RawValue, kfx.SymbolValue, kfx.SymbolByNameValue, kfx.ReadSymbolValue:
		return true
	default:
		return false
	}
}

func formatStructTree(tw *debug.TreeWriter, ctx *expandCtx, m map[kfx.KFXSymbol]any, depth int) {
	if len(m) == 0 {
		return
	}

	// Sort keys for deterministic output
	keys := make([]kfx.KFXSymbol, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		val := m[k]
		keyName := k.String()

		// Special handling for style-related keys
		switch k {
		case kfx.SymStyle, kfx.SymParentStyle:
			styleName := extractSymbolName(val)
			if css, ok := ctx.styleDefs[styleName]; ok {
				tw.Line(depth, "%s: symbol(%q) /* %s */", keyName, styleName, css)
			} else {
				tw.Line(depth, "%s: symbol(%q)", keyName, styleName)
			}
		case kfx.SymResourceName:
			resourceName := extractSymbolName(val)
			if info, ok := ctx.resourceDefs[resourceName]; ok {
				parts := []string{}
				if info.Location != "" {
					parts = append(parts, info.Location)
				}
				if info.Width > 0 && info.Height > 0 {
					parts = append(parts, fmt.Sprintf("%dx%d", info.Width, info.Height))
				}
				if info.MIME != "" {
					parts = append(parts, info.MIME)
				}
				if len(parts) > 0 {
					tw.Line(depth, "%s: symbol(%q) /* %s */", keyName, resourceName, strings.Join(parts, ", "))
				} else {
					tw.Line(depth, "%s: symbol(%q)", keyName, resourceName)
				}
			} else {
				tw.Line(depth, "%s: symbol(%q)", keyName, resourceName)
			}
		case kfx.SymContent:
			tw.Line(depth, "%s:", keyName)
			formatContentRefTree(tw, ctx, val, depth+1)
		case kfx.SymStyleEvents:
			formatStyleEventsTree(tw, ctx, keyName, val, depth)
		case kfx.SymContentList:
			formatContentListTree(tw, ctx, keyName, val, depth)
		default:
			if isSimpleValueExpanded(val) {
				tw.Line(depth, "%s: %s", keyName, formatSimpleValueStr(ctx, val))
			} else if list, ok := val.(kfx.ListValue); ok {
				formatListWithKeyTree(tw, ctx, keyName, []any(list), depth)
			} else if list, ok := val.([]any); ok {
				formatListWithKeyTree(tw, ctx, keyName, list, depth)
			} else {
				tw.Line(depth, "%s:", keyName)
				formatValueTree(tw, ctx, val, depth+1)
			}
		}
	}
}

func formatMapStringTree(tw *debug.TreeWriter, ctx *expandCtx, m map[string]any, depth int) {
	if len(m) == 0 {
		return
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		val := m[k]

		// Try to resolve "$NNN" keys to symbol names
		keyName := k
		var symID int
		if strings.HasPrefix(k, "$") {
			if id, err := strconv.Atoi(k[1:]); err == nil {
				keyName = kfx.KFXSymbol(id).String()
				symID = id
			}
		}

		// Special handling based on symbol ID
		switch kfx.KFXSymbol(symID) {
		case kfx.SymStyle, kfx.SymParentStyle:
			styleName := extractSymbolName(val)
			if css, ok := ctx.styleDefs[styleName]; ok {
				tw.Line(depth, "%s: symbol(%q) /* %s */", keyName, styleName, css)
			} else {
				tw.Line(depth, "%s: symbol(%q)", keyName, styleName)
			}
		case kfx.SymResourceName:
			resourceName := extractSymbolName(val)
			if info, ok := ctx.resourceDefs[resourceName]; ok {
				parts := []string{}
				if info.Location != "" {
					parts = append(parts, info.Location)
				}
				if info.Width > 0 && info.Height > 0 {
					parts = append(parts, fmt.Sprintf("%dx%d", info.Width, info.Height))
				}
				if info.MIME != "" {
					parts = append(parts, info.MIME)
				}
				if len(parts) > 0 {
					tw.Line(depth, "%s: symbol(%q) /* %s */", keyName, resourceName, strings.Join(parts, ", "))
				} else {
					tw.Line(depth, "%s: symbol(%q)", keyName, resourceName)
				}
			} else {
				tw.Line(depth, "%s: symbol(%q)", keyName, resourceName)
			}
		case kfx.SymContent:
			tw.Line(depth, "%s:", keyName)
			formatContentRefTree(tw, ctx, val, depth+1)
		case kfx.SymStyleEvents:
			formatStyleEventsTree(tw, ctx, keyName, val, depth)
		case kfx.SymContentList:
			formatContentListTree(tw, ctx, keyName, val, depth)
		default:
			if isSimpleValueExpanded(val) {
				tw.Line(depth, "%s: %s", keyName, formatSimpleValueStr(ctx, val))
			} else if list, ok := val.(kfx.ListValue); ok {
				formatListWithKeyTree(tw, ctx, keyName, []any(list), depth)
			} else if list, ok := val.([]any); ok {
				formatListWithKeyTree(tw, ctx, keyName, list, depth)
			} else {
				tw.Line(depth, "%s:", keyName)
				formatValueTree(tw, ctx, val, depth+1)
			}
		}
	}
}

func formatListTree(tw *debug.TreeWriter, ctx *expandCtx, items []any, depth int) {
	if len(items) == 0 {
		return
	}

	for i, item := range items {
		if isSimpleValueExpanded(item) {
			tw.Line(depth, "[%d]: %s", i, formatSimpleValueStr(ctx, item))
		} else {
			tw.Line(depth, "[%d]:", i)
			formatValueTree(tw, ctx, item, depth+1)
		}
	}
}

func formatListWithKeyTree(tw *debug.TreeWriter, ctx *expandCtx, key string, items []any, depth int) {
	tw.Line(depth, "%s: (%d)", key, len(items))
	for i, item := range items {
		if isSimpleValueExpanded(item) {
			tw.Line(depth+1, "[%d]: %s", i, formatSimpleValueStr(ctx, item))
		} else {
			tw.Line(depth+1, "[%d]:", i)
			formatValueTree(tw, ctx, item, depth+2)
		}
	}
}

// formatContentRefTree formats a content reference ($145) with text preview.
func formatContentRefTree(tw *debug.TreeWriter, ctx *expandCtx, v any, depth int) {
	m := toMapAny(v)
	if m == nil {
		tw.Line(depth, "%v", v)
		return
	}

	// Extract name and index for text preview lookup
	var contentName string
	var contentIndex int64 = -1

	if nameVal, ok := m["name"]; ok {
		contentName = extractSymbolName(nameVal)
	}
	if idxVal, ok := m["$403"]; ok {
		contentIndex = toInt64(idxVal)
	}

	// Get text preview if available
	var textPreview string
	if contentName != "" && contentIndex >= 0 {
		if info, ok := ctx.contentDefs[contentName]; ok {
			if int(contentIndex) < len(info.Texts) {
				textPreview = info.Texts[contentIndex]
			}
		}
	}

	// Print name
	if contentName != "" {
		tw.Line(depth, "name: %q", contentName)
	}

	// Print index with optional text preview
	if contentIndex >= 0 {
		if textPreview != "" {
			tw.Line(depth, "index ($403): %d /* %q */", contentIndex, textPreview)
		} else {
			tw.Line(depth, "index ($403): %d", contentIndex)
		}
	}

	// Print any other fields
	for k, val := range m {
		if k == "name" || k == "$403" {
			continue
		}
		keyName := k
		if strings.HasPrefix(k, "$") {
			if id, err := strconv.Atoi(k[1:]); err == nil {
				keyName = kfx.KFXSymbol(id).String()
			}
		}
		tw.Line(depth, "%s: %v", keyName, val)
	}
}

// formatStyleEventsTree formats style events ($142) with CSS expansion.
func formatStyleEventsTree(tw *debug.TreeWriter, ctx *expandCtx, key string, v any, depth int) {
	list := toListAny(v)
	if list == nil {
		tw.Line(depth, "%s: (empty)", key)
		return
	}

	tw.Line(depth, "%s: (%d)", key, len(list))

	for i, item := range list {
		m := toMapAny(item)
		if m == nil {
			tw.Line(depth+1, "[%d]: %v", i, item)
			continue
		}

		// Build a compact representation
		parts := []string{}

		// Offset ($143)
		if offset, ok := m["$143"]; ok {
			parts = append(parts, fmt.Sprintf("offset=%v", offset))
		}
		// Length ($144)
		if length, ok := m["$144"]; ok {
			parts = append(parts, fmt.Sprintf("len=%v", length))
		}
		// Style ($157)
		if styleVal, ok := m["$157"]; ok {
			styleName := extractSymbolName(styleVal)
			if styleName != "" {
				if css, ok := ctx.styleDefs[styleName]; ok {
					parts = append(parts, fmt.Sprintf("style=%q /* %s */", styleName, css))
				} else {
					parts = append(parts, fmt.Sprintf("style=%q", styleName))
				}
			}
		}
		// Link_to ($179)
		if linkVal, ok := m["$179"]; ok {
			linkName := extractSymbolName(linkVal)
			if linkName != "" {
				parts = append(parts, fmt.Sprintf("link_to=%q", linkName))
			}
		}
		// yj.display ($616)
		if displayVal, ok := m["$616"]; ok {
			displayName := extractSymbolName(displayVal)
			if displayName != "" {
				parts = append(parts, fmt.Sprintf("yj.display=%s", displayName))
			}
		}

		// Any other fields
		for k, val := range m {
			switch k {
			case "$143", "$144", "$157", "$179", "$616":
				continue
			}
			keyName := k
			if strings.HasPrefix(k, "$") {
				if id, err := strconv.Atoi(k[1:]); err == nil {
					keyName = kfx.KFXSymbol(id).String()
				}
			}
			parts = append(parts, fmt.Sprintf("%s=%v", keyName, val))
		}

		tw.Line(depth+1, "[%d]: %s", i, strings.Join(parts, ", "))
	}
}

// formatContentListTree formats content_list ($146) with expanded styles.
func formatContentListTree(tw *debug.TreeWriter, ctx *expandCtx, key string, v any, depth int) {
	list := toListAny(v)
	if list == nil {
		tw.Line(depth, "%s: (empty)", key)
		return
	}

	tw.Line(depth, "%s: (%d)", key, len(list))

	for i, item := range list {
		if isSimpleValueExpanded(item) {
			tw.Line(depth+1, "[%d]: %s", i, formatSimpleValueStr(ctx, item))
		} else {
			tw.Line(depth+1, "[%d]:", i)
			formatValueTree(tw, ctx, item, depth+2)
		}
	}
}

// StyleUsageInfo tracks which fragments use a particular style.
type StyleUsageInfo struct {
	// Fragments that reference this style via $157 (style field)
	DirectUsers []string
	// Fragments that reference this style via $142 (style_events)
	StyleEventUsers []string
}

// collectStyleUsage scans all fragments to find style references.
func collectStyleUsage(c *kfx.Container) map[string]*StyleUsageInfo {
	usage := make(map[string]*StyleUsageInfo)

	if c.Fragments == nil {
		return usage
	}

	// Scan all fragments for style references
	for _, frag := range c.Fragments.All() {
		fragID := formatFragmentID(c, frag)

		// Skip style fragments themselves
		if frag.FType == kfx.SymStyle {
			continue
		}

		// Recursively find all style references in this fragment
		directStyles, eventStyles := findAllStyleReferences(frag.Value)

		for _, styleName := range directStyles {
			if usage[styleName] == nil {
				usage[styleName] = &StyleUsageInfo{}
			}
			usage[styleName].DirectUsers = append(usage[styleName].DirectUsers, fragID)
		}

		for _, styleName := range eventStyles {
			if usage[styleName] == nil {
				usage[styleName] = &StyleUsageInfo{}
			}
			usage[styleName].StyleEventUsers = append(usage[styleName].StyleEventUsers, fragID)
		}
	}

	return usage
}

// findAllStyleReferences recursively finds all style references in a value.
// Returns (direct style references via $157, style event references via $142).
func findAllStyleReferences(v any) (directStyles, eventStyles []string) {
	seenDirect := make(map[string]bool)
	seenEvent := make(map[string]bool)

	var walk func(val any)
	walk = func(val any) {
		switch m := val.(type) {
		case kfx.StructValue:
			// Check for $157 (style) field
			if styleVal, ok := m[kfx.SymStyle]; ok {
				if name := extractSymbolName(styleVal); name != "" && !seenDirect[name] {
					seenDirect[name] = true
					directStyles = append(directStyles, name)
				}
			}
			// Check for $142 (style_events) field
			if eventsVal, ok := m[kfx.SymStyleEvents]; ok {
				for _, styleName := range extractStylesFromEventList(eventsVal) {
					if !seenEvent[styleName] {
						seenEvent[styleName] = true
						eventStyles = append(eventStyles, styleName)
					}
				}
			}
			// Recurse into all values
			for _, subVal := range m {
				walk(subVal)
			}

		case map[kfx.KFXSymbol]any:
			// Check for $157 (style) field
			if styleVal, ok := m[kfx.SymStyle]; ok {
				if name := extractSymbolName(styleVal); name != "" && !seenDirect[name] {
					seenDirect[name] = true
					directStyles = append(directStyles, name)
				}
			}
			// Check for $142 (style_events) field
			if eventsVal, ok := m[kfx.SymStyleEvents]; ok {
				for _, styleName := range extractStylesFromEventList(eventsVal) {
					if !seenEvent[styleName] {
						seenEvent[styleName] = true
						eventStyles = append(eventStyles, styleName)
					}
				}
			}
			// Recurse into all values
			for _, subVal := range m {
				walk(subVal)
			}

		case map[string]any:
			// Check for $157 (style) field
			if styleVal, ok := m["$157"]; ok {
				if name := extractSymbolName(styleVal); name != "" && !seenDirect[name] {
					seenDirect[name] = true
					directStyles = append(directStyles, name)
				}
			}
			// Check for $142 (style_events) field
			if eventsVal, ok := m["$142"]; ok {
				for _, styleName := range extractStylesFromEventList(eventsVal) {
					if !seenEvent[styleName] {
						seenEvent[styleName] = true
						eventStyles = append(eventStyles, styleName)
					}
				}
			}
			// Recurse into all values
			for _, subVal := range m {
				walk(subVal)
			}

		case []any:
			for _, item := range m {
				walk(item)
			}

		case kfx.ListValue:
			for _, item := range m {
				walk(item)
			}
		}
	}

	walk(v)
	return directStyles, eventStyles
}

// formatFragmentID returns a readable identifier for a fragment.
func formatFragmentID(c *kfx.Container, frag *kfx.Fragment) string {
	if frag.IsRoot() {
		return fmt.Sprintf("%s (root)", frag.FType.Name())
	}
	if frag.FIDName != "" {
		return fmt.Sprintf("%s:%s", frag.FType.Name(), frag.FIDName)
	}
	// Try to resolve FID to name
	if c.DocSymbolTable != nil {
		if name, ok := c.DocSymbolTable.FindByID(uint64(frag.FID)); ok && !strings.HasPrefix(name, "$") {
			return fmt.Sprintf("%s:%s", frag.FType.Name(), name)
		}
	}
	return fmt.Sprintf("%s:$%d", frag.FType.Name(), frag.FID)
}

// extractStylesFromEventList extracts style names from a list of style events.
func extractStylesFromEventList(v any) []string {
	list := toListAny(v)
	if list == nil {
		return nil
	}

	var styles []string
	seen := make(map[string]bool)

	for _, item := range list {
		// Each event item should have a $157 (style) field
		if m := toMapAny(item); m != nil {
			for key, val := range m {
				keyID := keyToSymbolID(key)
				if keyID == int(kfx.SymStyle) { // $157
					if styleName := extractSymbolName(val); styleName != "" && !seen[styleName] {
						seen[styleName] = true
						styles = append(styles, styleName)
					}
				}
			}
		}
	}
	return styles
}

// toMapAny converts various map types to map[string]any for uniform access.
func toMapAny(v any) map[string]any {
	switch m := v.(type) {
	case kfx.StructValue:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[fmt.Sprintf("$%d", k)] = val
		}
		return result
	case map[kfx.KFXSymbol]any:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[fmt.Sprintf("$%d", k)] = val
		}
		return result
	case map[string]any:
		return m
	default:
		return nil
	}
}

// toListAny converts various list types to []any.
func toListAny(v any) []any {
	switch l := v.(type) {
	case []any:
		return l
	case kfx.ListValue:
		return []any(l)
	default:
		return nil
	}
}

// keyToSymbolID extracts a symbol ID from a map key.
func keyToSymbolID(key string) int {
	if strings.HasPrefix(key, "$") {
		if id, err := strconv.Atoi(key[1:]); err == nil {
			return id
		}
	}
	return -1
}

// extractSymbolName extracts a style name from a symbol value.
func extractSymbolName(v any) string {
	switch s := v.(type) {
	case kfx.SymbolValue:
		return kfx.KFXSymbol(s).Name()
	case kfx.SymbolByNameValue:
		return string(s)
	case kfx.ReadSymbolValue:
		// ReadSymbolValue is already a string like "$320" or "style_name"
		str := string(s)
		if strings.HasPrefix(str, "$") {
			if id, err := strconv.Atoi(str[1:]); err == nil {
				return kfx.KFXSymbol(id).Name()
			}
		}
		return str
	case string:
		// Handle "$NNN" format
		if strings.HasPrefix(s, "$") {
			if id, err := strconv.Atoi(s[1:]); err == nil {
				return kfx.KFXSymbol(id).Name()
			}
		}
		return s
	default:
		return ""
	}
}

func dumpFragmentsByTypeWithUsage(c *kfx.Container, ftype kfx.KFXSymbol, styleUsage map[string]*StyleUsageInfo) (string, int) {
	var sb strings.Builder

	fmt.Fprintf(&sb, "=== KFX Fragments: %s ===\n\n", ftype)

	if c.Fragments == nil {
		sb.WriteString("(no fragments)\n")
		return sb.String(), 0
	}

	frags := c.Fragments.GetByType(ftype)
	if len(frags) == 0 {
		sb.WriteString("(no fragments)\n")
		return sb.String(), 0
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

	fmt.Fprintf(&sb, "### %s (%d fragments)\n\n", ftype, len(sortedFrags))

	for _, f := range sortedFrags {
		styleName := "" // Track style name for usage lookup

		if f.IsRoot() {
			sb.WriteString("  [root fragment]\n")
		} else {
			var fidID kfx.KFXSymbol
			var fidName string

			if f.FIDName != "" {
				fidName = f.FIDName
				styleName = fidName // Style name is the FIDName
				if len(c.LocalSymbols) > 0 {
					fidID = c.GetLocalSymbolID(f.FIDName)
					if fidID < 0 {
						if f.FID > 0 {
							fidID = f.FID
						} else {
							fmt.Fprintf(&sb, "  id: (unresolved) (%s)\n", f.FIDName)
							fmt.Fprintf(&sb, "  value: %s\n\n", kfx.FormatValueCompact(f.Value))
							continue
						}
					}
				} else if f.FID > 0 {
					fidID = f.FID
				} else {
					fmt.Fprintf(&sb, "  id: (unresolved) (%s)\n", f.FIDName)
					fmt.Fprintf(&sb, "  value: %s\n\n", kfx.FormatValueCompact(f.Value))
					continue
				}
			} else {
				fidID = f.FID
				if c.DocSymbolTable != nil {
					name, ok := c.DocSymbolTable.FindByID(uint64(fidID))
					if ok && !strings.HasPrefix(name, "$") {
						fidName = name
						styleName = fidName
					}
				}
			}

			if fidName != "" {
				fmt.Fprintf(&sb, "  id: $%d (%s)\n", fidID, fidName)
			} else {
				fmt.Fprintf(&sb, "  id: %s\n", fidID)
			}
		}
		fmt.Fprintf(&sb, "  value: %s\n", kfx.FormatValueCompact(f.Value))

		// Add CSS-like decoded output for style fragments
		if css := kfx.FormatStylePropsAsCSSMultiLine(f.Value); css != "" {
			sb.WriteString(css)
		}

		// Add usage information if available
		if styleName != "" && styleUsage != nil {
			if usage := styleUsage[styleName]; usage != nil {
				sb.WriteString("  Usage:\n")
				if len(usage.DirectUsers) > 0 {
					fmt.Fprintf(&sb, "    Direct ($157): %d fragment(s)\n", len(usage.DirectUsers))
					for _, user := range usage.DirectUsers {
						fmt.Fprintf(&sb, "      %s\n", user)
					}
				}
				if len(usage.StyleEventUsers) > 0 {
					fmt.Fprintf(&sb, "    Style events ($142): %d fragment(s)\n", len(usage.StyleEventUsers))
					for _, user := range usage.StyleEventUsers {
						fmt.Fprintf(&sb, "      %s\n", user)
					}
				}
			} else {
				sb.WriteString("  Usage: UNUSED (no references found)\n")
			}
		}

		sb.WriteString("\n")
	}

	return sb.String(), len(sortedFrags)
}

func dumpResources(container *kfx.Container, inPath, outDir string, overwrite bool) error {
	base := filepath.Base(inPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	dir := filepath.Dir(inPath)
	if outDir != "" {
		dir = outDir
	}
	outPath := filepath.Join(dir, stem+"-resources.zip")
	if _, err := os.Stat(outPath); err == nil {
		if !overwrite {
			return fmt.Errorf("output file already exists: %s", outPath)
		}
		if err := os.Remove(outPath); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	usedNames := make(map[string]int)
	written := 0
	for _, frag := range container.Fragments.GetByType(kfx.SymRawMedia) {
		blob, ok := asBlob(frag.Value)
		if !ok || len(blob) == 0 {
			continue
		}

		idName := ""
		if frag.FIDName != "" {
			idName = frag.FIDName
		} else if container.DocSymbolTable != nil {
			if n, ok := container.DocSymbolTable.FindByID(uint64(frag.FID)); ok {
				idName = n
			}
		}

		idPrefix := fmt.Sprintf("%d", frag.FID)
		if idName != "" {
			idPrefix += "_" + sanitizeFileComponent(idName)
		}

		ext := extFromFiletype(blob)
		entryName := idPrefix + ext
		if count := usedNames[entryName]; count > 0 {
			entryName = idPrefix + fmt.Sprintf("_%d", count+1) + ext
		}
		usedNames[idPrefix+ext]++

		w, err := zw.Create(entryName)
		if err != nil {
			return err
		}
		if _, err := w.Write(blob); err != nil {
			return err
		}
		written++
	}

	_, _ = fmt.Fprintf(os.Stderr, "resources: wrote %d file(s) into %s\n", written, outPath)
	return nil
}

func asBlob(v any) ([]byte, bool) {
	switch vv := v.(type) {
	case []byte:
		return vv, true
	case kfx.RawValue:
		return []byte(vv), true
	default:
		return nil, false
	}
}

func extFromFiletype(b []byte) string {
	kind, err := filetype.Match(b)
	if err == nil && kind != filetype.Unknown && kind.Extension != "" {
		return "." + kind.Extension
	}
	return ".bin"
}

func sanitizeFileComponent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_' || r == '.':
			return r
		default:
			return '_'
		}
	}, s)
}
