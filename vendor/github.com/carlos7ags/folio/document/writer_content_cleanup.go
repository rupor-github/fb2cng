// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"

	"github.com/carlos7ags/folio/core"
)

// cleanContentStreams removes redundant operators from page content
// streams (ISO 32000-1 §7.8). Two patterns are recognized in this
// first cut:
//
//   - Empty `q` ... `Q` graphics-state save/restore pairs. A `q`
//     followed by nothing but whitespace and another `Q` saves and
//     restores state for no observable effect; both operators are
//     dropped.
//   - Identity `1 0 0 1 0 0 cm` matrix concatenations (§8.3.3 — the
//     identity matrix). Concatenating the identity with the current
//     transformation matrix is a no-op; the operator is dropped.
//
// Eligibility is restricted to streams referenced from a page
// dictionary's /Contents entry. Other streams (font programs,
// XObjects, ICC profiles, image data, ToUnicode CMaps) are left
// alone — they have different syntactic conventions and the safe
// rewrites listed above only apply to content streams.
//
// The size-regression guard reverts any rewrite whose cleaned payload
// is not strictly smaller than the original. Streams whose payload
// is already FlateDecode-compressed at this point are skipped — the
// content cleanup operates on the raw operator text and would need
// an inflate before any rewrite. The recompression pass runs after
// cleanup, so cleaning before compression is the natural order.
//
// Encrypted documents are refused upstream; the defensive guard at
// the top of this method matches the contract of the other passes.
func (w *Writer) cleanContentStreams() {
	if len(w.objects) == 0 {
		return
	}
	if w.encryptor != nil {
		return
	}

	// Identify content stream object numbers by walking page dicts.
	contentNums := contentStreamObjectNumbers(w.objects)
	if len(contentNums) == 0 {
		return
	}

	for _, obj := range w.objects {
		if !contentNums[obj.ObjectNumber] {
			continue
		}
		stream, ok := obj.Object.(*core.PdfStream)
		if !ok {
			continue
		}
		if stream.Dict.Get("Filter") != nil {
			// Already-compressed content stream. Cleanup needs the
			// raw operator text; defer until decompression — out of
			// scope for this pass.
			continue
		}
		cleanOneContentStream(stream)
	}
}

// cleanOneContentStream attempts to remove redundant operators from a
// single content stream's payload. Mutates the stream in place on
// commit; leaves it untouched when the cleaned payload is not
// strictly smaller (size-regression guard).
func cleanOneContentStream(stream *core.PdfStream) {
	original := stream.Data
	if len(original) == 0 {
		return
	}

	cleaned, err := sizeRegressionGuard(original, func() ([]byte, error) {
		return cleanContentStreamBytes(original), nil
	})
	if err != nil {
		return
	}
	if len(cleaned) >= len(original) {
		return
	}
	stream.Data = cleaned
}

// contentStreamObjectNumbers walks the registered objects and returns
// the set of object numbers reachable as page /Contents references.
// Per ISO 32000-1 §7.7.3.3, /Contents is either a single stream or
// an array of streams; both forms are handled.
func contentStreamObjectNumbers(objects []IndirectObject) map[int]bool {
	out := make(map[int]bool)
	for _, obj := range objects {
		dict, ok := obj.Object.(*core.PdfDictionary)
		if !ok {
			continue
		}
		if !isPageDict(dict) {
			continue
		}
		contents := dict.Get("Contents")
		if contents == nil {
			continue
		}
		collectContentRefs(contents, out)
	}
	return out
}

// isPageDict reports whether a dictionary has /Type /Page. We do not
// look at /Subtype because Page dicts do not carry one.
func isPageDict(d *core.PdfDictionary) bool {
	t, ok := d.Get("Type").(*core.PdfName)
	return ok && t.Value == "Page"
}

// collectContentRefs walks the value of a /Contents entry, adding
// every PdfIndirectReference's target to the set. The entry can be
// a single reference or an array of references per §7.7.3.3.
func collectContentRefs(value core.PdfObject, set map[int]bool) {
	switch v := value.(type) {
	case *core.PdfIndirectReference:
		set[v.Num()] = true
	case *core.PdfArray:
		for _, elem := range v.All() {
			if ref, ok := elem.(*core.PdfIndirectReference); ok {
				set[ref.Num()] = true
			}
		}
	}
}

// cleanContentStreamBytes returns the input with redundant operators
// removed. The current rules:
//
//   - Drop every `1 0 0 1 0 0 cm` operator (identity transform).
//   - Drop every `q` ... `Q` pair whose body is whitespace-only.
//
// The function is byte-faithful for everything else: comments,
// strings, hex strings, names, and operands are preserved verbatim.
// Multiple cleanup passes are applied iteratively until a fixed point
// is reached (a Q-cleaning pass can expose a previously-nested empty
// q/Q whose body was just `q Q`).
func cleanContentStreamBytes(data []byte) []byte {
	const maxPasses = 8
	current := data
	for i := 0; i < maxPasses; i++ {
		next := cleanContentStreamOnePass(current)
		if bytes.Equal(next, current) {
			break
		}
		current = next
	}
	return current
}

// cleanContentStreamOnePass walks the byte stream once, dropping
// identity-cm and empty-q/Q occurrences. Returns the rewritten bytes.
func cleanContentStreamOnePass(data []byte) []byte {
	tokens := scanContentTokens(data)
	if len(tokens) == 0 {
		return data
	}

	// Build a set of token indices to drop.
	drop := make(map[int]bool)

	// Pass A: identity cm. Pattern is six numeric operand tokens
	// (1 0 0 1 0 0) followed by an "cm" operator token.
	for i := 6; i < len(tokens); i++ {
		if tokens[i].kind != tokenOperator {
			continue
		}
		if !bytes.Equal(tokens[i].slice(data), []byte("cm")) {
			continue
		}
		if isIdentityCmOperands(data, tokens[i-6:i]) {
			for j := i - 6; j <= i; j++ {
				drop[j] = true
			}
		}
	}

	// Pass B: empty q/Q. A `q` operator followed (after only
	// whitespace, no other tokens) by a `Q` operator — drop both.
	for i := 0; i < len(tokens)-1; i++ {
		if drop[i] {
			continue
		}
		if tokens[i].kind != tokenOperator {
			continue
		}
		if !bytes.Equal(tokens[i].slice(data), []byte("q")) {
			continue
		}
		// Find the next non-dropped token.
		next := -1
		for j := i + 1; j < len(tokens); j++ {
			if drop[j] {
				continue
			}
			next = j
			break
		}
		if next < 0 {
			continue
		}
		if tokens[next].kind != tokenOperator {
			continue
		}
		if !bytes.Equal(tokens[next].slice(data), []byte("Q")) {
			continue
		}
		drop[i] = true
		drop[next] = true
	}

	if len(drop) == 0 {
		return data
	}

	// Build the output by skipping every byte range that belongs to a
	// dropped token plus the trailing whitespace separator that
	// followed the token.
	var buf bytes.Buffer
	buf.Grow(len(data))
	pos := 0
	for i, tok := range tokens {
		if !drop[i] {
			continue
		}
		// Emit the run from the previous cursor up to (but not
		// including) this dropped token's start.
		buf.Write(data[pos:tok.start])
		// Skip the token itself plus any contiguous whitespace that
		// followed it (so dropping `q\n` does not leave a stray
		// blank line). Stop at any non-whitespace byte.
		pos = tok.end
		for pos < len(data) && isContentWhitespace(data[pos]) {
			pos++
		}
	}
	buf.Write(data[pos:])
	return buf.Bytes()
}

// isIdentityCmOperands reports whether six operand tokens are exactly
// the literal sequence 1 0 0 1 0 0 — the identity transformation
// matrix per §8.3.3.
func isIdentityCmOperands(data []byte, operands []contentToken) bool {
	want := []string{"1", "0", "0", "1", "0", "0"}
	for i, tok := range operands {
		if tok.kind != tokenNumber {
			return false
		}
		if string(tok.slice(data)) != want[i] {
			return false
		}
	}
	return true
}

// contentTokenKind names the lexical class of a content-stream token.
type contentTokenKind int

const (
	tokenOperator contentTokenKind = iota // bare keyword (q, Q, Tj, cm, ...)
	tokenNumber                           // numeric literal (1, -2.5, .42)
	tokenString                           // (...) or <hex>
	tokenName                             // /Name
	tokenArray                            // [...]
	tokenDict                             // <<...>>
)

// contentToken records the byte range and lexical kind of a single
// token in a content stream. The slice helper extracts the bytes.
type contentToken struct {
	start, end int
	kind       contentTokenKind
}

// slice returns the token's bytes from the source data.
func (t contentToken) slice(data []byte) []byte { return data[t.start:t.end] }

// scanContentTokens lexes a content stream into tokens. The lexer
// recognizes only the syntactic shapes needed to identify operator
// boundaries safely; deeper analysis (operand types, string contents)
// is left to higher-level callers.
//
// The lexer skips:
//
//   - Whitespace (space, tab, LF, CR, FF, NUL per §7.2.3).
//   - Comments (% to end of line per §7.2.4).
//
// And it recognizes the literal forms:
//
//   - Numbers: optional sign + digits with optional decimal point.
//   - Names: / followed by regular characters.
//   - Strings: balanced parenthesis-delimited bytes with escape
//     handling so `(\))` is one token.
//   - Hex strings: <...>.
//   - Arrays: [...] (whole bracket span is one token).
//   - Dictionaries: <<...>> (whole span is one token).
//   - Operators: any other run of regular characters not matched above.
//
// The lexer is faithful: a token's bytes can be extracted with
// data[token.start:token.end] and concatenating all tokens with the
// surviving whitespace produces the original stream.
func scanContentTokens(data []byte) []contentToken {
	var tokens []contentToken
	i := 0
	for i < len(data) {
		// Skip whitespace.
		for i < len(data) && isContentWhitespace(data[i]) {
			i++
		}
		if i >= len(data) {
			break
		}
		// Skip comments.
		if data[i] == '%' {
			for i < len(data) && data[i] != '\n' && data[i] != '\r' {
				i++
			}
			continue
		}
		start := i
		switch {
		case data[i] == '(':
			i = skipString(data, i)
			tokens = append(tokens, contentToken{start, i, tokenString})
		case data[i] == '<' && i+1 < len(data) && data[i+1] == '<':
			i = skipDict(data, i)
			tokens = append(tokens, contentToken{start, i, tokenDict})
		case data[i] == '<':
			i = skipHexString(data, i)
			tokens = append(tokens, contentToken{start, i, tokenString})
		case data[i] == '[':
			i = skipArray(data, i)
			tokens = append(tokens, contentToken{start, i, tokenArray})
		case data[i] == '/':
			i++
			for i < len(data) && isRegularChar(data[i]) {
				i++
			}
			tokens = append(tokens, contentToken{start, i, tokenName})
		case isNumberStart(data, i):
			i = skipNumber(data, i)
			tokens = append(tokens, contentToken{start, i, tokenNumber})
		default:
			// Bare keyword — the operator.
			for i < len(data) && isRegularChar(data[i]) {
				i++
			}
			if i == start {
				// Unrecognized byte; skip to avoid infinite loop.
				i++
				continue
			}
			tokens = append(tokens, contentToken{start, i, tokenOperator})
			// Inline image (§8.9.7): BI begins an inline image, ID
			// introduces its raw byte payload, EI terminates it. The
			// bytes between ID and EI are arbitrary image data and
			// MUST NOT be lexed as operators or operands; doing so
			// would let cleanup match patterns inside the image.
			if string(data[start:i]) == "BI" {
				i = skipInlineImage(data, i)
			}
		}
	}
	return tokens
}

// isContentWhitespace reports the white-space bytes per ISO 32000-1
// §7.2.3 Table 1: NUL, HT, LF, FF, CR, SP.
func isContentWhitespace(b byte) bool {
	return b == 0 || b == '\t' || b == '\n' || b == '\f' || b == '\r' || b == ' '
}

// isRegularChar reports the regular characters per §7.2.3 Table 2:
// anything that is not whitespace and not one of the delimiters
// ()<>[]{}/%.
func isRegularChar(b byte) bool {
	if isContentWhitespace(b) {
		return false
	}
	switch b {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return false
	}
	return true
}

// isNumberStart reports whether the byte at i could begin a numeric
// literal: a digit, a decimal point, or a sign followed by either.
func isNumberStart(data []byte, i int) bool {
	if i >= len(data) {
		return false
	}
	b := data[i]
	if b >= '0' && b <= '9' {
		return true
	}
	if b == '.' {
		return i+1 < len(data) && data[i+1] >= '0' && data[i+1] <= '9'
	}
	if b == '+' || b == '-' {
		if i+1 >= len(data) {
			return false
		}
		next := data[i+1]
		if next >= '0' && next <= '9' {
			return true
		}
		if next == '.' {
			return i+2 < len(data) && data[i+2] >= '0' && data[i+2] <= '9'
		}
	}
	return false
}

// skipNumber advances past a numeric literal starting at i.
func skipNumber(data []byte, i int) int {
	if data[i] == '+' || data[i] == '-' {
		i++
	}
	for i < len(data) {
		b := data[i]
		if (b >= '0' && b <= '9') || b == '.' {
			i++
			continue
		}
		break
	}
	return i
}

// skipString advances past a (...) literal string handling escapes
// and balanced nested parentheses per §7.3.4.2.
func skipString(data []byte, i int) int {
	i++ // skip opening (
	depth := 1
	for i < len(data) && depth > 0 {
		switch data[i] {
		case '\\':
			// Escape sequence — skip the next byte.
			if i+1 < len(data) {
				i += 2
				continue
			}
			i++
		case '(':
			depth++
			i++
		case ')':
			depth--
			i++
		default:
			i++
		}
	}
	return i
}

// skipHexString advances past a <...> hex string per §7.3.4.3.
func skipHexString(data []byte, i int) int {
	i++ // skip opening <
	for i < len(data) && data[i] != '>' {
		i++
	}
	if i < len(data) {
		i++ // skip closing >
	}
	return i
}

// skipArray advances past a [...] array. The bracket span is treated
// as one opaque token by the cleanup pass; we do not need to descend
// into operand-position arrays (TJ argument arrays).
func skipArray(data []byte, i int) int {
	i++ // skip opening [
	depth := 1
	for i < len(data) && depth > 0 {
		switch data[i] {
		case '[':
			depth++
		case ']':
			depth--
		case '(':
			i = skipString(data, i)
			continue
		case '<':
			if i+1 < len(data) && data[i+1] == '<' {
				i = skipDict(data, i)
			} else {
				i = skipHexString(data, i)
			}
			continue
		}
		i++
	}
	return i
}

// skipInlineImage advances past the dictionary, ID introducer, raw
// image bytes, and EI terminator of an inline image (§8.9.7). The
// caller has already consumed the leading BI keyword and `i` points
// at the byte just after BI.
//
// Algorithm:
//
//  1. Skip the inline image dictionary (key/value pairs) until the
//     ID keyword. Keys and values are tokenized normally so a value
//     that happens to look like ID does not terminate prematurely.
//  2. After ID, the spec mandates exactly one whitespace byte before
//     the raw image data starts. Some producers emit "\r\n" (two
//     bytes); we tolerate both.
//  3. Scan forward for "EI" preceded by whitespace and not embedded
//     inside a longer keyword. Anything in between is opaque image
//     bytes — do NOT tokenize.
//
// On EOF or malformed input the function returns the original
// position so the outer lexer can recover gracefully (the inline
// image is treated as already-consumed; the cleanup pass will leave
// it alone because no operators were emitted from inside it).
func skipInlineImage(data []byte, i int) int {
	// Step 1: scan for the ID keyword.
	for i < len(data) {
		// Skip whitespace.
		for i < len(data) && isContentWhitespace(data[i]) {
			i++
		}
		if i >= len(data) {
			return i
		}
		// Skip comments — same as the main lexer.
		if data[i] == '%' {
			for i < len(data) && data[i] != '\n' && data[i] != '\r' {
				i++
			}
			continue
		}
		// A name (/Foo) or hex string (<...>) or string ((...)) or
		// number — skip via the existing helpers.
		switch {
		case data[i] == '/':
			i++
			for i < len(data) && isRegularChar(data[i]) {
				i++
			}
			continue
		case data[i] == '(':
			i = skipString(data, i)
			continue
		case data[i] == '<' && i+1 < len(data) && data[i+1] == '<':
			i = skipDict(data, i)
			continue
		case data[i] == '<':
			i = skipHexString(data, i)
			continue
		case data[i] == '[':
			i = skipArray(data, i)
			continue
		case isNumberStart(data, i):
			i = skipNumber(data, i)
			continue
		}
		// Otherwise: a keyword. If it's ID, we're done with the dict.
		start := i
		for i < len(data) && isRegularChar(data[i]) {
			i++
		}
		if i == start {
			i++ // unrecognized byte; advance to avoid infinite loop
			continue
		}
		if string(data[start:i]) == "ID" {
			break
		}
		// Other keyword inside an inline-image dict is unexpected but
		// not fatal; keep scanning for ID.
	}
	if i >= len(data) {
		return i
	}
	// Step 2: a single whitespace byte (sometimes CRLF) before image data.
	if i < len(data) && (data[i] == '\r' || data[i] == '\n' || data[i] == ' ' || data[i] == '\t') {
		if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
			i += 2
		} else {
			i++
		}
	}
	// Step 3: scan for EI preceded by whitespace.
	for i < len(data)-1 {
		if data[i] == 'E' && data[i+1] == 'I' {
			// Check the preceding byte is whitespace (so we are at
			// a token boundary, not inside a longer identifier).
			prevOK := i == 0 || isContentWhitespace(data[i-1])
			// Check the following byte is whitespace or EOF (so EI
			// is a complete token).
			nextOK := i+2 >= len(data) || isContentWhitespace(data[i+2])
			if prevOK && nextOK {
				return i + 2
			}
		}
		i++
	}
	return len(data)
}

// skipDict advances past a <<...>> dictionary literal.
func skipDict(data []byte, i int) int {
	i += 2 // skip opening <<
	depth := 1
	for i < len(data) && depth > 0 {
		if i+1 < len(data) && data[i] == '<' && data[i+1] == '<' {
			depth++
			i += 2
			continue
		}
		if i+1 < len(data) && data[i] == '>' && data[i+1] == '>' {
			depth--
			i += 2
			continue
		}
		switch data[i] {
		case '(':
			i = skipString(data, i)
			continue
		case '<':
			i = skipHexString(data, i)
			continue
		case '[':
			i = skipArray(data, i)
			continue
		}
		i++
	}
	return i
}
