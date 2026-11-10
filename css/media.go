package css

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	parse "github.com/tdewolff/parse/v2"
	csslex "github.com/tdewolff/parse/v2/css"
	"go.uber.org/zap"
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
func ResolveMedia(data []byte, ctx MediaContext, log *zap.Logger) ([]byte, error) {
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
				query := MediaQuery{Raw: strings.TrimSpace(string(data[token.end:tokens[open].start]))}
				result := query.Evaluate(ctx)
				result.Log(query.Raw, ctx, true, log)
				matches, known = result.Matches, result.Resolved
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
