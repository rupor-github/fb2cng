package kfx

import "strings"

// StyleListEntry captures a single stylelist rule row.
type StyleListEntry struct {
	Key   string
	Class string
}

// IgnorablePattern matches entries that should be skipped during mapping.
type IgnorablePattern struct {
	Tag   string
	Style string
	Value string
	Unit  string
}

type styleListKey struct {
	property                string
	isMeasure               string
	existingUnit            string
	newUnit                 string
	allowWritingModeConvert string
	sourceIsWrapper         string
	sourceIsContainer       string
	sourceIsInline          string
}

type styleListRule struct {
	key  styleListKey
	rule propertyMergeRule
}

var styleListRules []styleListRule

func init() {
	styleListRules = buildStyleListRules(defaultStyleListEntries)
}

func buildStyleListRules(entries []StyleListEntry) []styleListRule {
	rules := make([]styleListRule, 0, len(entries))
	for _, e := range entries {
		if rule, ok := mergeRuleForClass(e.Class); ok {
			rules = append(rules, styleListRule{
				key:  parseStyleListKey(e.Key),
				rule: rule,
			})
		}
	}
	return rules
}

func parseStyleListKey(key string) styleListKey {
	parts := strings.Split(key, ",")
	if len(parts) > 8 {
		parts = parts[:8]
	}
	for len(parts) < 8 {
		parts = append(parts, "*")
	}
	for i := range parts {
		if parts[i] == "" {
			parts[i] = "*"
		}
	}
	return styleListKey{
		property:                parts[0],
		isMeasure:               parts[1],
		existingUnit:            parts[2],
		newUnit:                 parts[3],
		allowWritingModeConvert: parts[4],
		sourceIsWrapper:         parts[5],
		sourceIsContainer:       parts[6],
		sourceIsInline:          parts[7],
	}
}

func (key styleListKey) matches(actual styleListKey) bool {
	return key.matchesField(key.property, actual.property) &&
		key.matchesField(key.isMeasure, actual.isMeasure) &&
		key.matchesField(key.existingUnit, actual.existingUnit) &&
		key.matchesField(key.newUnit, actual.newUnit) &&
		key.matchesField(key.allowWritingModeConvert, actual.allowWritingModeConvert) &&
		key.matchesField(key.sourceIsWrapper, actual.sourceIsWrapper) &&
		key.matchesField(key.sourceIsContainer, actual.sourceIsContainer) &&
		key.matchesField(key.sourceIsInline, actual.sourceIsInline)
}

func (key styleListKey) matchesField(expected, actual string) bool {
	if expected == "" || expected == "*" {
		return true
	}
	return strings.EqualFold(expected, actual)
}

func mergeRuleForClass(class string) (propertyMergeRule, bool) {
	switch class {
	case "com.amazon.yj.style.merger.rules.YJHorizontalPositionRuleMerger":
		return propertyMergeRule{"horizontal-position", mergeHorizontalPosition}, true
	case "com.amazon.yj.style.merger.rules.YJCumulativeRuleMerger":
		return propertyMergeRule{"cumulative", mergeCumulative}, true
	case "com.amazon.yj.style.merger.rules.YJCumulativeInSameContainerRuleMerger":
		// KP3's YJCumulativeInSameContainerRuleMerger validates that both styles come from
		// the same container before accumulating. Container identity tracking is now handled
		// in StyleContext.PushBlock (marginOrigins) and StyleContext.handleContainerAwareMargins.
		// The merge function itself uses override semantics because by the time we reach here,
		// same-container margins have already been filtered or accumulated correctly.
		return propertyMergeRule{"cumulative-same-container", mergeOverride}, true
	case "com.amazon.yj.style.merger.rules.YJOverrideMaximumRuleMerger":
		return propertyMergeRule{"override-maximum", mergeOverrideMaximum}, true
	case "com.amazon.yj.style.merger.rules.YJRelativeRuleMerger":
		return propertyMergeRule{"relative", mergeRelative}, true
	case "com.amazon.yj.style.merger.rules.YJBaselineStyleRuleMerger":
		return propertyMergeRule{"baseline-style", mergeBaselineStyle}, true
	case "com.amazon.yj.style.merger.rules.YJOverridingRuleMerger":
		return propertyMergeRule{"override", mergeOverride}, true
	}
	return propertyMergeRule{}, false
}
