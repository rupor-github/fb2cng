package dumputil

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/amazon-ion/ion-go/ion"

	"fbc/convert/kfx"
	"fbc/utils/debug"
)

// DumpStorylineTxt writes expanded storyline fragments to <stem>-storyline.txt.
func DumpStorylineTxt(container *kfx.Container, inPath, outDir string, overwrite bool) error {
	styleDefs := collectStyleDefinitions(container)
	contentDefs := collectContentDefinitions(container)
	resourceDefs := collectResourceDefinitions(container)
	structureDefs := collectStructureDefinitions(container)

	dump, count := dumpStorylineFragmentsExpanded(container, styleDefs, contentDefs, resourceDefs, structureDefs)
	dump += "\n"
	if err := WriteOutput(inPath, outDir, "-storyline.txt", []byte(dump), overwrite); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stderr, "storyline: wrote %d fragment(s) into output\n", count)
	return nil
}

// ContentInfo holds text snippets for a content fragment.
type ContentInfo struct {
	Name  string   // Content fragment name
	Texts []string // Text snippets (truncated)
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

// SymStructure is the KFX symbol for "structure" fragment type ($608).
// Used by KDF files where storyline content_list references structure fragments by name
// instead of containing inline content structs.
const SymStructure kfx.KFXSymbol = 608

// expandCtx holds all the context needed for expanding storyline values.
type expandCtx struct {
	c             *kfx.Container
	styleDefs     map[string]string
	contentDefs   map[string]*ContentInfo
	resourceDefs  map[string]*ResourceInfo
	structureDefs map[string]any // structure fragment ID -> parsed value (KDF only)
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

		css := kfx.FormatStylePropsAsCSS(frag.Value)
		if css == "" {
			css = "(empty)"
		}
		defs[styleName] = css
	}
	return defs
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

// collectStructureDefinitions builds a map of structure fragment name -> parsed value.
// This is used by KDF files where storyline content_list references structure fragments
// by their string ID (e.g., "i18") instead of containing inline content structs.
func collectStructureDefinitions(c *kfx.Container) map[string]any {
	defs := make(map[string]any)
	if c.Fragments == nil {
		return defs
	}

	for _, frag := range c.Fragments.GetByType(SymStructure) {
		name := frag.FIDName
		if name == "" {
			if c.DocSymbolTable != nil {
				if n, ok := c.DocSymbolTable.FindByID(uint64(frag.FID)); ok && !strings.HasPrefix(n, "$") {
					name = n
				}
			}
		}
		if name == "" {
			continue
		}
		defs[name] = frag.Value
	}
	return defs
}

func dumpStorylineFragmentsExpanded(c *kfx.Container, styleDefs map[string]string, contentDefs map[string]*ContentInfo, resourceDefs map[string]*ResourceInfo, structureDefs map[string]any) (string, int) {
	tw := debug.NewTreeWriter()

	ctx := &expandCtx{
		c:             c,
		styleDefs:     styleDefs,
		contentDefs:   contentDefs,
		resourceDefs:  resourceDefs,
		structureDefs: structureDefs,
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

			if fidName != "" && fidID > 0 {
				tw.Line(1, "id=$%d (%s)", fidID, fidName)
			} else if fidName != "" {
				tw.Line(1, "id=%s", fidName)
			} else {
				tw.Line(1, "id=$%d", fidID)
			}
		}

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

	keys := make([]kfx.KFXSymbol, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		val := m[k]
		keyName := k.String()

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

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		val := m[k]

		keyName := k
		var symID int
		if strings.HasPrefix(k, "$") {
			if id, err := strconv.Atoi(k[1:]); err == nil {
				keyName = kfx.KFXSymbol(id).String()
				symID = id
			}
		}

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
// In KFX, this is a struct with name/index fields referencing a content fragment.
// In KDF, this may be a direct inline string.
func formatContentRefTree(tw *debug.TreeWriter, ctx *expandCtx, v any, depth int) {
	// KDF: content ($145) is a direct inline string.
	if s, ok := v.(string); ok {
		tw.Line(depth, "%s", truncateText(s, 120))
		return
	}

	m := toMapAny(v)
	if m == nil {
		tw.Line(depth, "%v", v)
		return
	}

	var contentName string
	var contentIndex int64 = -1

	if nameVal, ok := m["name"]; ok {
		contentName = extractSymbolName(nameVal)
	}
	if idxVal, ok := m["$403"]; ok {
		contentIndex = toInt64(idxVal)
	}

	var textPreview string
	if contentName != "" && contentIndex >= 0 {
		if info, ok := ctx.contentDefs[contentName]; ok {
			if int(contentIndex) < len(info.Texts) {
				textPreview = info.Texts[contentIndex]
			}
		}
	}

	if contentName != "" {
		tw.Line(depth, "name: %q", contentName)
	}

	if contentIndex >= 0 {
		if textPreview != "" {
			tw.Line(depth, "index ($403): %d /* %q */", contentIndex, textPreview)
		} else {
			tw.Line(depth, "index ($403): %d", contentIndex)
		}
	}

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

		parts := []string{}

		if offset, ok := m["$143"]; ok {
			parts = append(parts, fmt.Sprintf("offset=%v", offset))
		}
		if length, ok := m["$144"]; ok {
			parts = append(parts, fmt.Sprintf("len=%v", length))
		}
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
		if linkVal, ok := m["$179"]; ok {
			linkName := extractSymbolName(linkVal)
			if linkName != "" {
				parts = append(parts, fmt.Sprintf("link_to=%q", linkName))
			}
		}
		if displayVal, ok := m["$616"]; ok {
			displayName := extractSymbolName(displayVal)
			if displayName != "" {
				parts = append(parts, fmt.Sprintf("yj.display=%s", displayName))
			}
		}

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
// In KDF files, content_list items may be string references to structure ($608) fragments
// instead of inline struct values. These are resolved through structureDefs.
func formatContentListTree(tw *debug.TreeWriter, ctx *expandCtx, key string, v any, depth int) {
	list := toListAny(v)
	if list == nil {
		tw.Line(depth, "%s: (empty)", key)
		return
	}

	tw.Line(depth, "%s: (%d)", key, len(list))

	for i, item := range list {
		// Resolve string references to structure fragments (KDF).
		resolved := resolveStructureRef(ctx, item)

		if isSimpleValueExpanded(resolved) {
			tw.Line(depth+1, "[%d]: %s", i, formatSimpleValueStr(ctx, resolved))
		} else {
			tw.Line(depth+1, "[%d]:", i)
			formatValueTree(tw, ctx, resolved, depth+2)
		}
	}
}

// resolveStructureRef resolves a string reference to a structure fragment value.
// If item is a string and matches a structure fragment ID, returns the fragment's value.
// Otherwise returns the item unchanged.
func resolveStructureRef(ctx *expandCtx, item any) any {
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
