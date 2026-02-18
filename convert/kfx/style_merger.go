package kfx

import (
	"maps"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/amazon-ion/ion-go/ion"
)

// propertyMergeRule mirrors stylelist behavior for a property.
type propertyMergeRule struct {
	name string
	fn   func(existing, incoming any) (any, bool)
}

type mergeContext struct {
	allowWritingModeConvert bool
	sourceIsWrapper         bool
	sourceIsContainer       bool
	sourceIsInline          bool
}

var (
	mergeContextInline = mergeContext{allowWritingModeConvert: true, sourceIsInline: true}
	// mergeContextClassOverride is used when applying CSS class styles over tag defaults.
	// Uses allowWritingModeConvert=false to trigger YJOverridingRuleMerger for margins
	// instead of YJOverrideMaximumRuleMerger. This ensures class-level margin values
	// override tag-level defaults (matching CSS specificity behavior).
	mergeContextClassOverride = mergeContext{allowWritingModeConvert: false, sourceIsInline: true}
)

func mergePropertyWithRules(dst map[KFXSymbol]any, sym KFXSymbol, incoming any, ctx mergeContext, tracer *StyleTracer) {
	existing, has := dst[sym]
	if !has {
		dst[sym] = incoming
		return
	}
	merged, keep := mergeStyleProperty(sym, existing, incoming, ctx, tracer)
	if keep {
		dst[sym] = merged
	} else if has {
		delete(dst, sym)
	}
}

func mergeAllWithRules(dst, src map[KFXSymbol]any, ctx mergeContext, tracer *StyleTracer) {
	for sym, val := range src {
		mergePropertyWithRules(dst, sym, val, ctx, tracer)
	}
}

// mergeAllOverride performs simple CSS cascade merge: later values override earlier ones.
// This is used when merging CSS rules that produce the same style name (e.g., ".cite" and
// "blockquote.cite" both producing "cite"). Unlike mergeAllWithRules, this doesn't use
// stylelist rules which are designed for runtime style merging, not CSS cascade behavior.
func mergeAllOverride(dst, src map[KFXSymbol]any) {
	maps.Copy(dst, src)
}

func mergeStyleProperty(sym KFXSymbol, existing, incoming any, ctx mergeContext, tracer *StyleTracer) (any, bool) {
	if existing == nil {
		return incoming, true
	}
	rule := selectMergeRule(sym, existing, incoming, ctx)
	merged, ok := rule.fn(existing, incoming)
	if tracer != nil {
		tracer.TraceMerge(traceSymbolName(sym), rule.name, existing, incoming, merged)
	}
	return merged, ok
}

func selectMergeRule(sym KFXSymbol, existing, incoming any, ctx mergeContext) propertyMergeRule {
	// Special cases not covered by stylelist or with custom merge logic
	switch sym {
	case SymBackgroundRepeat:
		return propertyMergeRule{"background-repeat", mergeBackgroundRepeat}
	case SymLayoutHints:
		return propertyMergeRule{"cumulative", mergeLayoutHints}
	case SymKeepLinesTogether:
		return propertyMergeRule{"keep-lines-together", mergeKeepLinesTogether}
	}

	// Look up merge rule from stylelist data
	actual := buildStyleListActual(sym, existing, incoming, ctx)
	for _, rule := range styleListRules {
		if rule.key.matches(actual) {
			return rule.rule
		}
	}

	// Stylelist has catch-all rules "*,false" and "*,true" -> override
	// so this default should rarely be reached
	return propertyMergeRule{"override", mergeOverride}
}

func buildStyleListActual(sym KFXSymbol, existing, incoming any, ctx mergeContext) styleListKey {
	actual := styleListKey{
		property:                sym.Name(),
		isMeasure:               "false",
		existingUnit:            "*",
		newUnit:                 "*",
		allowWritingModeConvert: boolString(ctx.allowWritingModeConvert),
		sourceIsWrapper:         boolString(ctx.sourceIsWrapper),
		sourceIsContainer:       boolString(ctx.sourceIsContainer),
		sourceIsInline:          boolString(ctx.sourceIsInline),
	}

	if _, eunit, eok := measureParts(existing); eok {
		actual.isMeasure = "true"
		actual.existingUnit = eunit.Name()
		if _, iunit, iok := measureParts(incoming); iok {
			actual.newUnit = iunit.Name()
		}
	}

	return actual
}

func boolString(val bool) string {
	if val {
		return "true"
	}
	return "false"
}

// mergeOverride always prefers the incoming value.
func mergeOverride(existing, incoming any) (any, bool) {
	return incoming, true
}

// mergeCumulative sums numeric measures; for non-numeric inputs it falls back to incoming.
func mergeCumulative(existing, incoming any) (any, bool) {
	existingList := listFromAny(existing)
	incomingList := listFromAny(incoming)
	if len(existingList) > 0 || len(incomingList) > 0 {
		seen := make(map[string]bool, len(existingList)+len(incomingList))
		merged := make([]any, 0, len(existingList)+len(incomingList))
		for _, val := range existingList {
			key := encodeStyleValue(val)
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, val)
		}
		for _, val := range incomingList {
			key := encodeStyleValue(val)
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, val)
		}
		return merged, true
	}
	if merged, ok := mergeMeasure(existing, incoming, func(ev, iv float64) float64 { return ev + iv }); ok {
		return merged, true
	}

	ev, eok := numericFromAny(existing)
	iv, iok := numericFromAny(incoming)
	if eok && iok {
		return ev + iv, true
	}
	return incoming, true
}

func mergeRelative(existing, incoming any) (any, bool) {
	ev, eunit, eok := measureParts(existing)
	iv, iunit, iok := measureParts(incoming)
	if !eok || !iok {
		return incoming, true
	}
	if eunit == iunit {
		switch eunit {
		case SymUnitPercent:
			return DimensionValue(ev*iv/100, eunit), true
		case SymUnitEm:
			return DimensionValue(ev*iv, eunit), true
		default:
			return incoming, true
		}
	}
	switch iunit {
	case SymUnitPercent:
		return DimensionValue(ev*iv/100, eunit), true
	case SymUnitEm:
		return DimensionValue(ev*iv, eunit), true
	default:
		return incoming, true
	}
}

// mergeOverrideMaximum picks the value with the greater numeric magnitude; when
// comparison fails it returns the incoming value.
func mergeOverrideMaximum(existing, incoming any) (any, bool) {
	if winner, ok := pickMeasureByMagnitude(existing, incoming); ok {
		return winner, true
	}

	ev, eok := numericFromAny(existing)
	iv, iok := numericFromAny(incoming)
	if eok && iok {
		if iv >= ev {
			return incoming, true
		}
		return existing, true
	}
	return incoming, true
}

// mergeBaselineStyle prefers a non-zero/non-empty incoming value.
func mergeBaselineStyle(existing, incoming any) (any, bool) {
	if incoming == nil {
		return existing, true
	}
	if isSymbol(incoming, SymNormal) && existing != nil && !isSymbol(existing, SymNormal) {
		return existing, true
	}
	return incoming, true
}

func mergeKeepLinesTogether(existing, incoming any) (any, bool) {
	em, eok := convertToSymbolMap(existing)
	im, iok := convertToSymbolMap(incoming)

	switch {
	case !eok && !iok:
		return incoming, true
	case !eok:
		return incoming, true
	case !iok:
		return existing, true
	}

	merged := make(map[KFXSymbol]any, len(em)+len(im)+1)
	maps.Copy(merged, em)
	maps.Copy(merged, im)
	return merged, true
}

func mergeBackgroundRepeat(existing, incoming any) (any, bool) {
	if existing == nil {
		return incoming, true
	}
	if incoming == nil {
		return existing, true
	}

	rank := func(v any) int {
		sym, ok := symbolIDFromAny(v)
		if !ok {
			return -1
		}
		switch sym {
		case SymBackgroundRepeat:
			return 3
		default:
			if rx, ok := symbolIDFromString("repeat_x"); ok && sym == rx {
				return 2
			}
			if ry, ok := symbolIDFromString("repeat_y"); ok && sym == ry {
				return 2
			}
			if nr, ok := symbolIDFromString("no_repeat"); ok && sym == nr {
				return 1
			}
		}
		return 0
	}

	er := rank(existing)
	ir := rank(incoming)
	if er >= ir {
		return existing, true
	}
	return incoming, true
}

// mergeHorizontalPosition collapses float/clear semantics:
// - if either side is nil, return the other
// - if equal, keep as-is
// - otherwise return both
func mergeHorizontalPosition(existing, incoming any) (any, bool) {
	if existing == nil {
		return incoming, true
	}
	if incoming == nil {
		return existing, true
	}

	existingSym, eok := symbolIDFromAny(existing)
	incomingSym, iok := symbolIDFromAny(incoming)
	if eok && iok {
		if existingSym == incomingSym {
			return incoming, true
		}
		return SymBoth, true
	}
	if existing == incoming {
		return existing, true
	}
	return incoming, true
}

func mergeLayoutHints(existing, incoming any) (any, bool) {
	existingList := listFromAny(existing)
	incomingList := listFromAny(incoming)

	if len(existingList) == 0 {
		return incomingList, true
	}
	if len(incomingList) == 0 {
		return existingList, true
	}

	seen := make(map[string]bool, len(existingList)+len(incomingList))
	merged := make([]any, 0, len(existingList)+len(incomingList))

	for _, val := range existingList {
		key := encodeStyleValue(val)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, val)
	}
	for _, val := range incomingList {
		key := encodeStyleValue(val)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, val)
	}
	return merged, true
}

func mergeMeasure(existing, incoming any, combine func(float64, float64) float64) (any, bool) {
	ev, eunit, eok := measureParts(existing)
	iv, iunit, iok := measureParts(incoming)
	if eok && iok && eunit == iunit {
		return DimensionValue(combine(ev, iv), eunit), true
	}
	return nil, false
}

func pickMeasureByMagnitude(existing, incoming any) (any, bool) {
	ev, eunit, eok := measureParts(existing)
	iv, iunit, iok := measureParts(incoming)
	if eok && iok && eunit == iunit {
		if iv >= ev {
			return incoming, true
		}
		return existing, true
	}
	return nil, false
}

func measureParts(v any) (float64, KFXSymbol, bool) {
	sv, ok := toStructValue(v)
	if !ok {
		return 0, 0, false
	}

	rawVal, hasVal := sv[SymValue]
	rawUnit, hasUnit := sv[SymUnit]
	if !hasVal || !hasUnit {
		return 0, 0, false
	}

	unit, ok := symbolIDFromAny(rawUnit)
	if !ok {
		return 0, 0, false
	}

	if dec, ok := rawVal.(*ion.Decimal); ok {
		return decimalToFloat64(dec), unit, true
	}

	if num, ok := numericFromAny(rawVal); ok {
		return num, unit, true
	}

	return 0, 0, false
}

func numericFromAny(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint64:
		return float64(x), true
	case *ion.Decimal:
		return decimalToFloat64(x), true
	}
	return 0, false
}

func decimalToFloat64(d *ion.Decimal) float64 {
	if d == nil {
		return 0
	}
	coeff, exp := d.CoEx()
	bf := new(big.Float).SetInt(coeff)
	if exp != 0 {
		pow := new(big.Float).SetFloat64(math.Pow10(int(exp)))
		bf.Mul(bf, pow)
	}
	f, _ := bf.Float64()
	return f
}

func symbolIDFromAny(v any) (KFXSymbol, bool) {
	switch val := v.(type) {
	case KFXSymbol:
		return val, true
	case SymbolValue:
		return KFXSymbol(val), true
	case ReadSymbolValue:
		return symbolIDFromString(string(val))
	case string:
		return symbolIDFromString(val)
	}
	return 0, false
}

func symbolIDFromString(val string) (KFXSymbol, bool) {
	if sym, ok := yjSymbolIDs[val]; ok {
		return sym, true
	}
	if after, ok := strings.CutPrefix(val, "$"); ok {
		if id, err := strconv.Atoi(after); err == nil {
			return KFXSymbol(id), true
		}
	}
	return 0, false
}

func toStructValue(v any) (StructValue, bool) {
	switch s := v.(type) {
	case StructValue:
		return s, true
	case map[KFXSymbol]any:
		return StructValue(s), true
	}
	return nil, false
}

func convertToSymbolMap(v any) (map[KFXSymbol]any, bool) {
	switch m := v.(type) {
	case map[KFXSymbol]any:
		return m, true
	case StructValue:
		return map[KFXSymbol]any(m), true
	case bool:
		if m {
			return map[KFXSymbol]any{SymKeepLinesTogether: true}, true
		}
	}
	return nil, false
}

func listFromAny(v any) []any {
	switch l := v.(type) {
	case []any:
		return l
	case ListValue:
		return []any(l)
	}
	return nil
}

func isSymbol(val any, target KFXSymbol) bool {
	if sym, ok := symbolIDFromAny(val); ok {
		return sym == target
	}
	return false
}
