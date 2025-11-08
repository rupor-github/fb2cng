// hyphenator provides TeX-style hyphenation for multiple languages (forked
// from github.com/AlanQuatermain/go-hyphenator and modified).
package text

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"strings"
	"text/scanner"
	"unicode/utf8"

	"go.uber.org/zap"
	"golang.org/x/text/language"
)

//go:embed dictionaries/*.gz
var dictionaryFiles embed.FS

type Hyphenator struct {
	*hyph
}

// Some languages require additional specification.
var langMap = map[string]string{
	"de":    "de-1901",
	"de-de": "de-1901",
	"de-at": "de-1996",
	"de-ch": "de-ch-1901",
	"el":    "el-monoton",
	"el-gr": "el-monoton",
	"en":    "en-us",
	"mn":    "mn-cyrl",
	"sh":    "sh-latn",
	"sr":    "sr-cyrl",
	"zh":    "zh-latn-pinyin",
}

func getCompressedDictionaryData(name string) ([]byte, error) {
	data, err := dictionaryFiles.ReadFile(name)
	if err != nil {
		return nil, err
	}
	r, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func tryLoadDictionary(name, suffix string) ([]byte, error) {
	fileName := fmt.Sprintf("dictionaries/hyph-%s.%s.txt.gz", name, suffix)
	data, err := getCompressedDictionaryData(fileName)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// NewHyphenator loads hyphenation dictionary for specified language
func NewHyphenator(lang language.Tag, log *zap.Logger) *Hyphenator {
	var langName string

	// Try language tag
	name := strings.ToLower(lang.String())
	dataPatterns, err := tryLoadDictionary(name, "pat")
	if err == nil {
		langName = name
	}

	// Try mapped language tag
	if langName == "" {
		if mapped, ok := langMap[name]; ok {
			dataPatterns, err = tryLoadDictionary(mapped, "pat")
			if err == nil {
				langName = mapped
			}
		}
	}

	// Try base language tag
	if langName == "" {
		base, confidence := lang.Base()
		if confidence != language.No {
			name = strings.ToLower(base.String())
			dataPatterns, err = tryLoadDictionary(name, "pat")
			if err == nil {
				langName = name
			}
		} else {
			log.Warn("Unable to determine language base", zap.Stringer("tag", lang), zap.Stringer("base", base))
		}
	}

	// Try mapped base language tag
	if langName == "" && name != "" {
		if mapped, ok := langMap[name]; ok {
			dataPatterns, err = tryLoadDictionary(mapped, "pat")
			if err == nil {
				langName = mapped
			}
		}
	}

	if langName == "" {
		log.Warn("Unable to find suitable hyphenation dictionary, turning off hyphenation", zap.Stringer("language", lang))
		return nil
	}

	// Try to load exceptions dictionary (optional)
	dataExceptions, err := tryLoadDictionary(langName, "hyp")
	if err != nil {
		log.Debug("No exceptions dictionary found, leaving empty", zap.Stringer("tag", lang), zap.String("name", langName))
		dataExceptions = []byte{}
	}

	h := &hyph{}
	if err = h.loadDictionary(langName, strings.NewReader(string(dataPatterns)), strings.NewReader(string(dataExceptions))); err != nil {
		log.Warn("Unable to load hyphenation dictionary", zap.Stringer("tag", lang), zap.Error(err))
		return nil
	}
	return &Hyphenator{h}
}

// Hyphenate inserts soft-hyphens into words in string.
func (h *Hyphenator) Hyphenate(in string) string {
	if h == nil {
		return in
	}
	return h.hyphString(in, SOFTHYPHEN)
}

// hyph struct wraps actual implementation.
type hyph struct {
	patterns   *trie
	exceptions map[string]string
	language   string
}

// loadDictionary imports hyphenation patterns and exceptions from provided input streams.
func (h *hyph) loadDictionary(language string, patterns, exceptions io.Reader) error {

	if h.language != language {
		h.patterns = nil
		h.exceptions = nil
		h.language = language
	}

	if h.patterns != nil && h.patterns.size() != 0 {
		// looks like it's already been set up
		return nil
	}

	h.patterns = newTrie()
	h.exceptions = make(map[string]string, 20)

	if err := h.loadPatterns(patterns); err != nil {
		return err
	}
	return h.loadExceptions(exceptions)
}

func (h *hyph) loadPatterns(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		h.patterns.addPatternString(scanner.Text())
	}
	return scanner.Err()
}

func (h *hyph) loadExceptions(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		str := scanner.Text()
		key := strings.ReplaceAll(str, `-`, ``)
		h.exceptions[key] = str
	}
	return scanner.Err()
}

func (h *hyph) hyphenateWord(s, hyphen string) string {

	testStr := `.` + s + `.`
	v := make([]int, utf8.RuneCountInString(testStr))

	vIndex := 0
	for pos := range testStr {
		t := testStr[pos:]
		strs, values := h.patterns.allSubstringsAndValues(t)
		for i := range len(values) {
			str := strs[i]
			val := values[i].([]int)

			diff := len(val) - utf8.RuneCountInString(str)
			vs := v[vIndex-diff:]

			for i := range len(val) {
				if val[i] > vs[i] {
					vs[i] = val[i]
				}
			}
		}
		vIndex++
	}

	var outstr string

	// trim the values for the beginning and ending dots
	markers := v[1 : len(v)-1]
	mIndex := 0
	u := make([]byte, 4)
	for _, ch := range s {
		l := utf8.EncodeRune(u, ch)
		outstr += string(u[0:l])
		// don't hyphenate between (or after) first two and the last two characters of a string
		if 1 <= mIndex && mIndex < len(markers)-2 {
			// hyphens are inserted on odd values, skipped on even ones
			if markers[mIndex]%2 != 0 {
				outstr += hyphen
			}
		}
		mIndex++
	}

	return outstr
}

// hyphenate string.
func (h *hyph) hyphString(s, hyphen string) string {

	var sc scanner.Scanner
	sc.Init(strings.NewReader(s))
	sc.Mode = scanner.ScanIdents
	sc.Whitespace = 0

	var outstr string

	tok := sc.Scan()
	for tok != scanner.EOF {
		switch tok {
		case scanner.Ident:
			// a word (or part thereof) to hyphenate
			t := sc.TokenText()

			// try the exceptions first
			exc := h.exceptions[t]
			if len(exc) != 0 {
				if hyphen != `-` {
					exc = strings.ReplaceAll(exc, `-`, hyphen)
				}
				outstr += exc
			} else {
				// not an exception, hyphenate normally
				outstr += h.hyphenateWord(sc.TokenText(), hyphen)
			}
		default:
			// A Unicode rune to append to the output
			p := make([]byte, utf8.UTFMax)
			l := utf8.EncodeRune(p, tok)
			outstr += string(p[0:l])
		}

		tok = sc.Scan()
	}
	return outstr
}
