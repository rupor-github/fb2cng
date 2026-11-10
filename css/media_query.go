package css

import (
	"errors"
	"io"
	"slices"
	"strings"

	parse "github.com/tdewolff/parse/v2"
	csslex "github.com/tdewolff/parse/v2/css"
	"go.uber.org/zap"
)

// MediaResult separates a compiled-rendering decision from whether a condition
// can be fully resolved without reader/device information. Unsupported atoms keep
// the compiled renderers' false fallback; negation and alternatives still apply.
// CSS-preserving adapters must retain unresolved conditions for the reader.
type MediaResult struct {
	Matches     bool
	Resolved    bool
	Unsupported []string
}

// Evaluate is the single evaluator used by both parsed stylesheets and native
// CSS selection. It never infers hardware dimensions or changes output profiles.
func (mq MediaQuery) Evaluate(ctx MediaContext) MediaResult {
	lexer := csslex.NewLexer(parse.NewInputString(mq.Raw))
	var tokens []mediaToken
	for {
		kind, value := lexer.Next()
		if kind == csslex.ErrorToken {
			if !errors.Is(lexer.Err(), io.EOF) {
				return unsupportedMedia("invalid media query tokens")
			}
			break
		}
		if kind != csslex.WhitespaceToken && kind != csslex.CommentToken {
			tokens = append(tokens, mediaToken{kind: kind, text: string(value)})
		}
	}
	if len(tokens) == 0 {
		return unsupportedMedia("empty media query")
	}
	result := MediaResult{Resolved: true}
	start, depth := 0, 0
	for i, token := range tokens {
		switch token.kind {
		case csslex.LeftParenthesisToken, csslex.FunctionToken, csslex.LeftBracketToken:
			depth++
		case csslex.RightParenthesisToken, csslex.RightBracketToken:
			depth--
		}
		if depth < 0 {
			return unsupportedMedia("unbalanced media query delimiters")
		}
		if token.kind == csslex.CommaToken && depth == 0 {
			result = combineMedia(result, evaluateMediaCondition(tokens[start:i], ctx), true)
			start = i + 1
		}
	}
	if depth != 0 {
		return unsupportedMedia("unbalanced media query delimiters")
	}
	return combineMedia(result, evaluateMediaCondition(tokens[start:], ctx), true)
}

func evaluateMediaCondition(tokens []mediaToken, ctx MediaContext) MediaResult {
	if len(tokens) == 0 {
		return unsupportedMedia("empty media alternative")
	}
	ident := func(i int, value string) bool {
		return i < len(tokens) && tokens[i].kind == csslex.IdentToken && strings.EqualFold(tokens[i].text, value)
	}
	index, negate := 0, ident(0, "not")
	if negate || ident(0, "only") {
		index++
	}
	result := MediaResult{Matches: true, Resolved: true}
	first := true
	for index < len(tokens) {
		if !first {
			if !ident(index, "and") {
				return unsupportedMedia("unsupported media syntax: " + mediaConditionText(tokens))
			}
			index++
		}
		termNot := !first && ident(index, "not")
		if termNot {
			index++
		}
		if index == len(tokens) {
			return unsupportedMedia("incomplete media condition")
		}
		start, atom := index, ""
		switch tokens[index].kind {
		case csslex.IdentToken:
			atom = strings.ToLower(tokens[index].text)
			index++
		case csslex.LeftParenthesisToken:
			depth := 1
			index++
			for index < len(tokens) && depth > 0 {
				switch tokens[index].kind {
				case csslex.LeftParenthesisToken, csslex.FunctionToken:
					depth++
				case csslex.RightParenthesisToken:
					depth--
				}
				index++
			}
			if depth != 0 {
				return unsupportedMedia("unbalanced media feature")
			}
			if index == start+3 && tokens[start+1].kind == csslex.IdentToken {
				atom = strings.ToLower(tokens[start+1].text)
			}
		default:
			return unsupportedMedia("unsupported media syntax: " + mediaConditionText(tokens))
		}
		term := MediaResult{Resolved: true}
		switch atom {
		case "amzn-kf8":
			term.Matches = ctx.KF8
		case "amzn-et":
			term.Matches = ctx.ET
		case "fbc-pdf":
			term.Matches = ctx.FBCPDF
		case "all", "screen":
			term.Matches = true
		case "amzn-mobi", "print":
			// Preserve existing profiles: no MOBI7 target, PDF is screen-styled.
		default:
			term = unsupportedMedia(mediaConditionText(tokens[start:index]))
		}
		if termNot {
			term.Matches = !term.Matches
		}
		result = combineMedia(result, term, false)
		first = false
	}
	if first {
		return unsupportedMedia("incomplete media condition")
	}
	if negate {
		result.Matches = !result.Matches
	}
	return result
}

func combineMedia(left, right MediaResult, alternative bool) MediaResult {
	result := MediaResult{Resolved: left.Resolved && right.Resolved, Unsupported: slices.Clone(left.Unsupported)}
	if alternative {
		result.Matches = left.Matches || right.Matches
		result.Resolved = result.Resolved || left.Resolved && left.Matches || right.Resolved && right.Matches
	} else {
		result.Matches = left.Matches && right.Matches
		result.Resolved = result.Resolved || left.Resolved && !left.Matches || right.Resolved && !right.Matches
	}
	for _, condition := range right.Unsupported {
		if !slices.Contains(result.Unsupported, condition) {
			result.Unsupported = append(result.Unsupported, condition)
		}
	}
	return result
}

func unsupportedMedia(condition string) MediaResult {
	return MediaResult{Unsupported: []string{condition}}
}

func mediaConditionText(tokens []mediaToken) string {
	var text strings.Builder
	for _, token := range tokens {
		text.WriteString(token.text)
	}
	return text.String()
}

// Log records the actual consumer decision, including false-fallback matching in
// compiled renderers versus preservation of unresolved reader-dependent queries.
func (result MediaResult) Log(query string, ctx MediaContext, preserveUnresolved bool, log *zap.Logger) {
	if log == nil {
		return
	}
	action := "skipped"
	if preserveUnresolved && !result.Resolved {
		action = "preserved for reader"
	} else if result.Matches {
		action = "matched"
	}
	message := "CSS media query evaluated"
	if len(result.Unsupported) > 0 {
		message = "Unsupported CSS media condition"
	}
	policy := "unsupported atoms are false"
	if preserveUnresolved {
		policy = "preserve unresolved queries for reader"
	}
	log.Debug(message, zap.String("query", query), zap.String("action", action),
		zap.Bool("matched", result.Matches), zap.Bool("resolved", result.Resolved), zap.Strings("unsupported", result.Unsupported),
		zap.String("unsupported_policy", policy),
		zap.Bool("kf8", ctx.KF8), zap.Bool("et", ctx.ET), zap.Bool("fbc_pdf", ctx.FBCPDF),
		zap.Bool("screen", true), zap.Bool("print", false))
}
