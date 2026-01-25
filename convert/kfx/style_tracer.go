package kfx

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/amazon-ion/ion-go/ion"
)

// StyleTracer records style resolution and assignment operations for debugging.
// When enabled (via non-empty workDir), it captures detailed information about how
// styles are resolved, merged, and assigned to content elements.
//
// The trace is written to a file when Flush() is called, typically at the end
// of KFX generation. The trace file is placed in the working directory so it
// gets included in the debug report archive.
type StyleTracer struct {
	enabled  bool
	workDir  string
	entries  []traceEntry
	sections map[string]int // section name -> entry count for summary
}

type traceEntry struct {
	operation string // "register", "resolve", "merge", "assign", etc.
	styleName string
	details   string
}

// NewStyleTracer creates a new tracer. If workDir is empty, tracing is disabled.
func NewStyleTracer(workDir string) *StyleTracer {
	return &StyleTracer{
		workDir:  workDir,
		enabled:  workDir != "",
		sections: make(map[string]int),
	}
}

// IsEnabled returns true if tracing is active.
func (t *StyleTracer) IsEnabled() bool {
	if t == nil {
		return false
	}
	return t.enabled
}

// TraceRegister logs when a style is registered in the registry.
func (t *StyleTracer) TraceRegister(name string, props map[KFXSymbol]any) {
	if !t.IsEnabled() {
		return
	}
	t.entries = append(t.entries, traceEntry{
		operation: "REGISTER",
		styleName: name,
		details:   traceFormatProperties(props),
	})
	t.sections["registered"]++
}

// TraceInheritance logs inheritance resolution for a style.
func (t *StyleTracer) TraceInheritance(name string, parent string, finalProps map[KFXSymbol]any) {
	if !t.IsEnabled() {
		return
	}
	parentInfo := "(no parent)"
	if parent != "" {
		parentInfo = "inherits from " + parent
	}
	t.entries = append(t.entries, traceEntry{
		operation: "INHERIT",
		styleName: name,
		details:   parentInfo + "\n" + traceFormatProperties(finalProps),
	})
	t.sections["inherited"]++
}

// TraceAssign logs when a style is assigned to content.
// styleSpec is the original style specification (e.g., "p poem verse") used for resolution.
// If styleSpec is empty, the style was assigned directly without resolution.
func (t *StyleTracer) TraceAssign(contentType string, contentID string, styleName string, location string, styleSpec string) {
	if !t.IsEnabled() {
		return
	}
	var details strings.Builder
	details.WriteString(fmt.Sprintf("to %s %q at %s", contentType, contentID, location))
	if styleSpec != "" {
		details.WriteString(fmt.Sprintf("\n  from spec: %s", styleSpec))
	}
	t.entries = append(t.entries, traceEntry{
		operation: "ASSIGN",
		styleName: styleName,
		details:   details.String(),
	})
	t.sections["assigned"]++
}

// TracePostProcess logs post-processing enhancements applied to a style.
func (t *StyleTracer) TracePostProcess(name string, enhancement string, props map[KFXSymbol]any) {
	if !t.IsEnabled() {
		return
	}
	t.entries = append(t.entries, traceEntry{
		operation: "POSTPROC",
		styleName: name,
		details:   enhancement + "\n" + traceFormatProperties(props),
	})
	t.sections["postprocessed"]++
}

// TraceCSSConvert logs CSS rule to KFX style conversion.
func (t *StyleTracer) TraceCSSConvert(selector string, props map[KFXSymbol]any) {
	if !t.IsEnabled() {
		return
	}
	t.entries = append(t.entries, traceEntry{
		operation: "CSS->KFX",
		styleName: selector,
		details:   traceFormatProperties(props),
	})
	t.sections["css_converted"]++
}

// TraceNormalize logs normalization of a wrapper's CSS map.
func (t *StyleTracer) TraceNormalize(wrapper string, original map[string]string, normalized map[string]string) {
	if !t.IsEnabled() {
		return
	}
	t.entries = append(t.entries, traceEntry{
		operation: "NORMALIZE",
		styleName: wrapper,
		details:   "original: " + traceFormatCSS(original) + "\nnormalized: " + traceFormatCSS(normalized),
	})
	t.sections["normalized"]++
}

// TraceMap logs HTML key → KFX property emissions, including transformer info.
func (t *StyleTracer) TraceMap(key string, transformer string, emitted map[KFXSymbol]any) {
	if !t.IsEnabled() {
		return
	}
	info := key
	if transformer != "" {
		info = key + " via " + transformer
	}
	t.entries = append(t.entries, traceEntry{
		operation: "MAP",
		styleName: info,
		details:   traceFormatProperties(emitted),
	})
	t.sections["mapped"]++
}

// TraceMerge logs stylelist-driven merges for a property.
func (t *StyleTracer) TraceMerge(prop string, rule string, existing any, incoming any, result any) {
	if !t.IsEnabled() {
		return
	}
	t.entries = append(t.entries, traceEntry{
		operation: "MERGE",
		styleName: prop,
		details:   fmt.Sprintf("rule=%s existing=%v incoming=%v result=%v", rule, traceFormatValue(existing), traceFormatValue(incoming), traceFormatValue(result)),
	})
	t.sections["merged"]++
}

// TraceAutoCreate logs when a style is auto-created because it wasn't defined in CSS.
// This helps identify unknown classes from FB2 that may need CSS definitions.
func (t *StyleTracer) TraceAutoCreate(name string, inferredParent string) {
	if !t.IsEnabled() {
		return
	}
	parentInfo := "(no parent)"
	if inferredParent != "" {
		parentInfo = "inferred parent: " + inferredParent
	}
	t.entries = append(t.entries, traceEntry{
		operation: "AUTOCREATE",
		styleName: name,
		details:   parentInfo,
	})
	t.sections["auto_created"]++
}

// TraceMarginCollapse logs a margin collapse operation during post-processing.
// This tracks the CSS margin collapsing algorithm applied after content generation.
//
// Parameters:
//   - collapseType: "first-child", "last-child", "sibling", or "empty" (self-collapse)
//   - nodeID: content entry identifier (EID or descriptive string)
//   - beforeMT, beforeMB: margins before collapse (nil = no margin)
//   - afterMT, afterMB: margins after collapse (nil = no margin)
//   - containerInfo: description of the container context
func (t *StyleTracer) TraceMarginCollapse(collapseType, nodeID string, beforeMT, beforeMB, afterMT, afterMB *float64, containerInfo string) {
	if !t.IsEnabled() {
		return
	}

	var details strings.Builder
	details.WriteString(fmt.Sprintf("before: mt=%s mb=%s", formatMarginPtr(beforeMT), formatMarginPtr(beforeMB)))
	details.WriteString(fmt.Sprintf("\n  after: mt=%s mb=%s", formatMarginPtr(afterMT), formatMarginPtr(afterMB)))
	if containerInfo != "" {
		details.WriteString(fmt.Sprintf("\n  container: %s", containerInfo))
	}

	t.entries = append(t.entries, traceEntry{
		operation: "COLLAPSE",
		styleName: fmt.Sprintf("%s @ %s", collapseType, nodeID),
		details:   details.String(),
	})
	t.sections["collapse_"+collapseType]++
}

// TraceStyleVariant logs creation of a style variant with modified margins.
// This tracks when applyCollapsedMargins() creates new style variants.
//
// Parameters:
//   - originalStyle: name of the original style before margin modification
//   - newStyle: name of the new/reused style after margin modification
//   - nodeID: content entry identifier
//   - marginTop, marginBottom: final margin values (nil = removed)
func (t *StyleTracer) TraceStyleVariant(originalStyle, newStyle, nodeID string, marginTop, marginBottom *float64) {
	if !t.IsEnabled() {
		return
	}

	var details strings.Builder
	if originalStyle == newStyle {
		details.WriteString("(reused existing style)")
	} else {
		details.WriteString(fmt.Sprintf("original: %s", originalStyle))
	}
	details.WriteString(fmt.Sprintf("\n  margins: mt=%s mb=%s", formatMarginPtr(marginTop), formatMarginPtr(marginBottom)))

	t.entries = append(t.entries, traceEntry{
		operation: "VARIANT",
		styleName: newStyle,
		details:   details.String(),
	})
	if originalStyle == newStyle {
		t.sections["style_reused"]++
	} else {
		t.sections["style_created"]++
	}
}

// formatMarginPtr formats a margin pointer for trace output.
func formatMarginPtr(m *float64) string {
	if m == nil {
		return "nil"
	}
	return fmt.Sprintf("%.5flh", *m)
}

// TraceMarginAccumulate logs margin accumulation decisions in container-aware handling.
// This helps debug YJCumulativeInSameContainerRuleMerger behavior.
//
// Parameters:
//   - marginType: "margin-left" or "margin-right"
//   - styleName: the style contributing the margin
//   - action: "skip" (same container), "accumulate" (different container), or "set" (first value)
//   - existing: existing margin value (nil if none)
//   - incoming: new margin value being applied
//   - result: final margin value after action
//   - containerPath: container stack with positions and flags
func (t *StyleTracer) TraceMarginAccumulate(marginType, styleName, action string, existing, incoming, result any, containerPath string) {
	if !t.IsEnabled() {
		return
	}

	var details strings.Builder
	details.WriteString(fmt.Sprintf("action: %s", action))
	if existing != nil {
		details.WriteString(fmt.Sprintf(", existing: %s", traceFormatValue(existing)))
	}
	details.WriteString(fmt.Sprintf(", incoming: %s", traceFormatValue(incoming)))
	details.WriteString(fmt.Sprintf(", result: %s", traceFormatValue(result)))
	if containerPath != "(no containers)" {
		details.WriteString(fmt.Sprintf("\n  containers: %s", containerPath))
	}

	t.entries = append(t.entries, traceEntry{
		operation: "MARGIN",
		styleName: fmt.Sprintf("%s via %s", marginType, styleName),
		details:   details.String(),
	})
	t.sections["margin_"+action]++
}

// Flush writes the trace to a file and clears the buffer.
// Returns the path to the trace file, or empty string if tracing is disabled.
func (t *StyleTracer) Flush() string {
	if !t.IsEnabled() || len(t.entries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("=== KFX Style Trace ===\n\n")

	// Write summary
	sb.WriteString("Summary:\n")
	for section, count := range t.sections {
		sb.WriteString(fmt.Sprintf("  %s: %d\n", section, count))
	}
	sb.WriteString("\n")

	// Write detailed trace
	sb.WriteString("Detailed Trace:\n")
	sb.WriteString(strings.Repeat("-", 80) + "\n")

	for i, entry := range t.entries {
		sb.WriteString(fmt.Sprintf("[%04d] %s: %s\n", i+1, entry.operation, entry.styleName))
		if entry.details != "" {
			for _, line := range strings.Split(entry.details, "\n") {
				sb.WriteString("       " + line + "\n")
			}
		}
		sb.WriteString("\n")
	}

	// Write to file
	tracePath := filepath.Join(t.workDir, "style-trace.txt")
	if err := os.WriteFile(tracePath, []byte(sb.String()), 0644); err != nil {
		return ""
	}

	// Clear entries after flush
	t.entries = nil
	t.sections = make(map[string]int)

	return tracePath
}

// traceFormatProperties formats KFX properties for trace output.
func traceFormatProperties(props map[KFXSymbol]any) string {
	if len(props) == 0 {
		return "(no properties)"
	}

	// Sort keys for consistent output
	keys := make([]KFXSymbol, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	var parts []string
	for _, k := range keys {
		v := props[k]
		name := traceSymbolName(k)
		parts = append(parts, fmt.Sprintf("%s: %v", name, traceFormatValue(v)))
	}
	return strings.Join(parts, ", ")
}

// traceFormatCSS formats a CSS property map for trace output.
func traceFormatCSS(props map[string]string) string {
	if len(props) == 0 {
		return "(none)"
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, props[k]))
	}
	return strings.Join(parts, ", ")
}

// traceFormatValue formats a property value for display.
func traceFormatValue(v any) string {
	switch val := v.(type) {
	case KFXSymbol:
		return fmt.Sprintf("symbol(%s)", traceSymbolName(val))
	case SymbolValue:
		return fmt.Sprintf("symbol(%s)", traceSymbolName(KFXSymbol(val)))
	case ReadSymbolValue:
		return fmt.Sprintf("symbol(%s)", string(val))
	case uint16:
		// KFXSymbol is uint16, check if it's a known symbol
		if name, ok := yjSymbolNames[KFXSymbol(val)]; ok {
			return fmt.Sprintf("symbol(%s)", name)
		}
		return fmt.Sprintf("int(%d)", val)
	case int64:
		// Check if this int64 is a known symbol value
		if name, ok := yjSymbolNames[KFXSymbol(val)]; ok {
			return fmt.Sprintf("symbol(%s)", name)
		}
		return fmt.Sprintf("int(%d)", val)
	case int:
		// Check if this int is a known symbol value
		if name, ok := yjSymbolNames[KFXSymbol(val)]; ok {
			return fmt.Sprintf("symbol(%s)", name)
		}
		return fmt.Sprintf("int(%d)", val)
	case float64:
		// Check if this float64 represents a symbol value (no fractional part)
		if val == float64(int64(val)) {
			if name, ok := yjSymbolNames[KFXSymbol(int64(val))]; ok {
				return fmt.Sprintf("symbol(%s)", name)
			}
		}
		return fmt.Sprintf("float(%v)", val)
	case *ion.Decimal:
		return fmt.Sprintf("decimal(%s)", val.String())
	case StructValue:
		if unit, ok := val[SymUnit]; ok {
			if value, ok := val[SymValue]; ok {
				return fmt.Sprintf("%v%s", traceFormatValue(value), unitSuffix(unit))
			}
		}
		return fmt.Sprintf("%v", val)
	case []any:
		var parts []string
		for _, item := range val {
			parts = append(parts, traceFormatValue(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// traceSymbolName returns the name for a KFX symbol.
func traceSymbolName(sym KFXSymbol) string {
	if name, ok := yjSymbolNames[sym]; ok {
		return name
	}
	return fmt.Sprintf("$%d", sym)
}

// unitSuffix returns the CSS-like suffix for a unit symbol.
func unitSuffix(unit any) string {
	var u KFXSymbol
	switch v := unit.(type) {
	case KFXSymbol:
		u = v
	case SymbolValue:
		u = KFXSymbol(v)
	default:
		return ""
	}
	switch u {
	case SymUnitEm:
		return "em"
	case SymUnitPercent:
		return "%"
	case SymUnitLh:
		return "lh"
	case SymUnitPx:
		return "px"
	case SymUnitPt:
		return "pt"
	case SymUnitRem:
		return "rem"
	default:
		return fmt.Sprintf("($%d)", u)
	}
}
