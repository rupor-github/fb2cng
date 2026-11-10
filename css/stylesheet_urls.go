package css

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	parse "github.com/tdewolff/parse/v2"
	csslex "github.com/tdewolff/parse/v2/css"
)

// RewriteStylesheetURLs changes only URL tokens and quoted @import destinations.
// Unlike the renderer-oriented AST, it preserves selectors, declaration order,
// repeated properties, shorthand/longhand precedence, at-rules and literal text.
func RewriteStylesheetURLs(data []byte, rewrite func(string) string) ([]byte, error) {
	lexer := csslex.NewLexer(parse.NewInputBytes(data))
	var output bytes.Buffer
	importURL, functionURL := false, false
	for {
		kind, value := lexer.Next()
		if kind == csslex.ErrorToken {
			if !errors.Is(lexer.Err(), io.EOF) {
				return nil, fmt.Errorf("tokenize stylesheet URLs: %w", lexer.Err())
			}
			return output.Bytes(), nil
		}
		text := string(value)
		switch kind {
		case csslex.WhitespaceToken, csslex.CommentToken:
		case csslex.AtKeywordToken:
			importURL = strings.EqualFold(text, "@import")
		case csslex.FunctionToken:
			functionURL = strings.EqualFold(text, "url(")
			importURL = false
		case csslex.URLToken:
			start, end := strings.IndexByte(text, '('), strings.LastIndexByte(text, ')')
			if start < 0 || end <= start {
				// Preserve malformed authored CSS for the reader rather than
				// inventing a destination or changing warning-only recovery.
				importURL, functionURL = false, false
				break
			}
			url := strings.TrimSpace(text[start+1 : end])
			quoted := len(url) >= 2 && (url[0] == '\'' || url[0] == '"') && url[len(url)-1] == url[0]
			if quoted {
				url = url[1 : len(url)-1]
			}
			original := decodeCSSURL(url)
			if replacement := rewrite(original); replacement != original {
				if !quoted && csslex.IsURLUnquoted([]byte(replacement)) {
					text = "url(" + replacement + ")"
				} else {
					text = `url("` + cssEscapeDoubleQuoted(replacement) + `")`
				}
			}
			importURL, functionURL = false, false
		case csslex.StringToken:
			if (importURL || functionURL) && len(text) >= 2 && text[len(text)-1] == text[0] {
				original := decodeCSSURL(text[1 : len(text)-1])
				if replacement := rewrite(original); replacement != original {
					text = `"` + cssEscapeDoubleQuoted(replacement) + `"`
				}
			}
			importURL, functionURL = false, false
		default:
			importURL, functionURL = false, false
		}
		output.WriteString(text) // bytes.Buffer writes cannot fail.
	}
}

func decodeCSSURL(value string) string {
	var output strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 == len(value) {
			output.WriteByte(value[i])
			continue
		}
		i++
		var code rune
		digits := 0
		for digits < 6 && i < len(value) {
			ch := value[i]
			var digit byte
			switch {
			case ch >= '0' && ch <= '9':
				digit = ch - '0'
			case ch >= 'a' && ch <= 'f':
				digit = ch - 'a' + 10
			case ch >= 'A' && ch <= 'F':
				digit = ch - 'A' + 10
			default:
				goto decoded
			}
			code = code*16 + rune(digit)
			digits++
			i++
		}
	decoded:
		if digits > 0 {
			if code == 0 || !utf8.ValidRune(code) {
				code = utf8.RuneError
			}
			output.WriteRune(code)
			if i < len(value) && strings.ContainsRune(" \t\r\n\f", rune(value[i])) {
				if value[i] == '\r' && i+1 < len(value) && value[i+1] == '\n' {
					i++
				}
			} else {
				i--
			}
		} else if value[i] != '\n' && value[i] != '\r' && value[i] != '\f' {
			output.WriteByte(value[i])
		} else if value[i] == '\r' && i+1 < len(value) && value[i+1] == '\n' {
			i++
		}
	}
	return output.String()
}
