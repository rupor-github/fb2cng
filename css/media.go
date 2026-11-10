package css

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	parse "github.com/tdewolff/parse/v2"
	csslex "github.com/tdewolff/parse/v2/css"
)

type mediaToken struct {
	kind       csslex.TokenType
	start, end int
	text       string
}

// ResolveMedia selects output-specific media blocks without parsing/reformatting
// their contents. Matching blocks are unwrapped in source order; nonmatching ones
// are removed, including nested fonts/resources. Unknown reader-dependent queries
// remain conditional. Ordinary selectors, declarations and their order are intact.
func ResolveMedia(data []byte, ctx MediaContext) ([]byte, error) {
	lexer := csslex.NewLexer(parse.NewInputBytes(data))
	var tokens []mediaToken
	var stack []int
	blocks := make(map[int]int)
	offset := 0
	for {
		kind, value := lexer.Next()
		if kind == csslex.ErrorToken {
			if !errors.Is(lexer.Err(), io.EOF) {
				return nil, fmt.Errorf("tokenize CSS media: %w", lexer.Err())
			}
			break
		}
		token := mediaToken{kind: kind, start: offset, end: offset + len(value), text: string(value)}
		offset = token.end
		switch kind {
		case csslex.LeftBraceToken:
			if len(stack) >= 128 {
				return nil, errors.New("CSS media block nesting exceeds 128 levels")
			}
			stack = append(stack, len(tokens))
		case csslex.RightBraceToken:
			if len(stack) == 0 {
				return nil, errors.New("CSS media has an unmatched closing brace")
			}
			blocks[stack[len(stack)-1]] = len(tokens)
			stack = stack[:len(stack)-1]
		}
		tokens = append(tokens, token)
	}
	if len(stack) > 0 || offset != len(data) {
		return nil, errors.New("CSS media has an incomplete block or token")
	}
	// bytes.Buffer writes cannot fail; preserve source slices verbatim.
	var output bytes.Buffer
	var resolve func(first, last, start, end int)
	resolve = func(first, last, start, end int) {
		copied := start
		for i := first; i < last; i++ {
			token := tokens[i]
			if token.kind == csslex.LeftBraceToken {
				i = blocks[i] // A normal declaration block is opaque.
				continue
			}
			if token.kind != csslex.AtKeywordToken {
				continue
			}
			open := i + 1
			depth := 0
			for open < last {
				kind := tokens[open].kind
				if depth == 0 && (kind == csslex.LeftBraceToken || kind == csslex.SemicolonToken) {
					break
				}
				switch kind {
				case csslex.LeftParenthesisToken, csslex.FunctionToken, csslex.LeftBracketToken:
					depth++
				case csslex.RightParenthesisToken, csslex.RightBracketToken:
					depth--
				}
				open++
			}
			if open == last || tokens[open].kind == csslex.SemicolonToken {
				i = open
				continue
			}
			close := blocks[open]
			name := strings.ToLower(token.text)
			if name != "@media" && name != "@supports" && name != "@layer" && name != "@container" && name != "@document" && name != "@scope" {
				i = close // Fonts and other declaration at-rules are opaque too.
				continue
			}
			matches, known := true, false
			if name == "@media" {
				matches, known = matchMediaList(tokens[i+1:open], ctx)
			}
			output.Write(data[copied:token.start])
			if !known {
				output.Write(data[token.start:tokens[open].end])
			}
			if !known || matches {
				resolve(open+1, close, tokens[open].end, tokens[close].start)
			}
			if !known {
				output.Write(data[tokens[close].start:tokens[close].end])
			}
			copied, i = tokens[close].end, close
		}
		output.Write(data[copied:end])
	}
	resolve(0, len(tokens), 0, len(data))
	return output.Bytes(), nil
}

func matchMediaList(tokens []mediaToken, ctx MediaContext) (bool, bool) {
	var query []mediaToken
	unknown, depth := false, 0
	for i, token := range tokens {
		if token.kind == csslex.WhitespaceToken || token.kind == csslex.CommentToken {
			continue
		}
		switch token.kind {
		case csslex.LeftParenthesisToken, csslex.FunctionToken, csslex.LeftBracketToken:
			depth++
		case csslex.RightParenthesisToken, csslex.RightBracketToken:
			depth--
		}
		if token.kind != csslex.CommaToken || depth != 0 {
			query = append(query, token)
		}
		if token.kind == csslex.CommaToken && depth == 0 || i == len(tokens)-1 {
			matches, known := matchMediaCondition(query, ctx)
			if matches && known {
				return true, true // Comma-separated queries are alternatives.
			}
			unknown = unknown || !known
			query = nil
		}
	}
	if len(query) > 0 { // The final significant token can precede whitespace.
		matches, known := matchMediaCondition(query, ctx)
		if matches && known {
			return true, true
		}
		unknown = unknown || !known
	}
	return false, !unknown
}

func matchMediaCondition(tokens []mediaToken, ctx MediaContext) (bool, bool) {
	if len(tokens) == 0 {
		return false, false
	}
	ident := func(i int, value string) bool {
		return i < len(tokens) && tokens[i].kind == csslex.IdentToken && strings.EqualFold(tokens[i].text, value)
	}
	index, negate := 0, ident(0, "not")
	if negate || ident(0, "only") {
		index++
	}
	unknown, rejected, first := false, false, true
	for index < len(tokens) {
		if !first {
			if !ident(index, "and") {
				return false, false // Preserve unsupported boolean syntax for the reader.
			}
			index++
		}
		termNot := !first && ident(index, "not")
		if termNot {
			index++
		}
		if index == len(tokens) {
			return false, false
		}
		atom, atomKnown := "", false
		switch tokens[index].kind {
		case csslex.IdentToken:
			atom = strings.ToLower(tokens[index].text)
			index++
		case csslex.LeftParenthesisToken:
			start, depth := index, 1
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
				return false, false
			}
			if index == start+3 && tokens[start+1].kind == csslex.IdentToken {
				atom = strings.ToLower(tokens[start+1].text)
			}
		default:
			return false, false
		}
		switch atom {
		case "amzn-kf8", "amzn-et", "amzn-mobi", "fbc-pdf", "all", "screen", "print":
			atomKnown = true
		}
		if atomKnown {
			rejected = rejected || !(MediaQuery{Type: atom, Negated: termNot}).Evaluate(ctx)
		} else {
			unknown = true
		}
		first = false
	}
	if first || unknown && !rejected {
		return false, false
	}
	// Leading 'not' negates the whole conjunction; 'and not' negates one term.
	return !rejected != negate, true
}
