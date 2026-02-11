package kfx

import (
	"strconv"
	"strings"

	"fbc/css"
)

func firstNonEmpty(primary string, rest ...string) string {
	if primary != "" {
		return primary
	}
	for _, v := range rest {
		if v != "" {
			return v
		}
	}
	return ""
}

func splitStyleTokens(val string) []string {
	val = strings.ReplaceAll(val, ",", " ")
	return strings.Fields(val)
}

func parsePercent(val string) (float64, bool) {
	val = strings.TrimSpace(val)
	val = strings.TrimSuffix(val, "%")
	if val == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(val, 64)
	return f, err == nil
}

func positionKeywordPercent(token string) (float64, bool) {
	switch strings.ToLower(token) {
	case "left", "top":
		return 0, true
	case "center":
		return 50, true
	case "right", "bottom":
		return 100, true
	}
	return 0, false
}

func symbolForDecoration(prop string) KFXSymbol {
	switch prop {
	case "underline":
		return SymUnderline
	case "overline":
		return SymOverline
	case "strikethrough":
		return SymStrikethrough
	}
	return SymUnderline
}

func symbolIDOr(name string) KFXSymbol {
	if sym, ok := symbolIDFromString(name); ok {
		return sym
	}
	return SymbolUnknown
}

func parseIntValue(cssVal css.Value, rawVal string) (int, bool) {
	switch {
	case cssVal.IsNumeric():
		return int(cssVal.Value), true
	case cssVal.Keyword != "":
		if n, err := strconv.Atoi(cssVal.Keyword); err == nil {
			return n, true
		}
	case rawVal != "":
		if n, err := strconv.Atoi(rawVal); err == nil {
			return n, true
		}
	}
	return 0, false
}

func splitCSV(val string) []string {
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func firstOrEmpty(values []string, idx int) string {
	if idx < len(values) {
		return values[idx]
	}
	return ""
}

func ConvertBackgroundRepeat(css css.Value) (KFXSymbol, bool) {
	val := strings.ToLower(firstNonEmpty(css.Keyword, css.Raw))
	switch val {
	case "repeat-x":
		if sym, ok := symbolIDFromString("repeat_x"); ok {
			return sym, true
		}
	case "repeat-y":
		if sym, ok := symbolIDFromString("repeat_y"); ok {
			return sym, true
		}
	case "no-repeat":
		if sym, ok := symbolIDFromString("no_repeat"); ok {
			return sym, true
		}
	case "repeat":
		if sym, ok := symbolIDFromString("background_repeat"); ok {
			return sym, true
		}
	}
	if sym, ok := symbolIDFromString(strings.ReplaceAll(val, "-", "_")); ok {
		return sym, true
	}
	return 0, false
}
