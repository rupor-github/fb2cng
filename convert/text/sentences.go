package text

import (
	"bytes"
	"compress/gzip"
	"embed"
	"io"
	"iter"
	"strings"
	"unicode"

	"github.com/neurosnap/sentences"
	"go.uber.org/zap"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

//go:embed sentences/*.gz
var modelFiles embed.FS

type Splitter struct {
	*sentences.DefaultSentenceTokenizer
}

func getCompressedModelData(name string) ([]byte, error) {
	data, err := modelFiles.ReadFile(name)
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

func tryLoadModel(name string) ([]byte, error) {
	fileName := "sentences/" + name + ".json.gz"
	data, err := getCompressedModelData(fileName)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func NewSplitter(lang language.Tag, log *zap.Logger) *Splitter {
	var data []byte
	var err error

	// Try language tag using display name
	name := strings.ToLower(display.English.Languages().Name(lang))
	data, err = tryLoadModel(name)
	if err == nil {
		model, err := sentences.LoadTraining(data)
		if err != nil {
			log.Warn("Unable to load sentences tokenizer data", zap.Stringer("tag", lang), zap.Error(err))
			return nil
		}
		return &Splitter{sentences.NewSentenceTokenizer(model)}
	}

	// Try base language tag
	base, confidence := lang.Base()
	if confidence != language.No {
		name = strings.ToLower(base.String())
		data, err = tryLoadModel(name)
		if err == nil {
			model, err := sentences.LoadTraining(data)
			if err != nil {
				log.Warn("Unable to load sentences tokenizer data", zap.Stringer("tag", lang), zap.Error(err))
				return nil
			}
			return &Splitter{sentences.NewSentenceTokenizer(model)}
		}
	} else {
		log.Warn("Unable to determine language base", zap.Stringer("tag", lang), zap.Stringer("base", base))
	}

	log.Warn("Unable to find suitable sentence tokenizer model, turning off sentence splitting", zap.Stringer("language", lang))
	return nil
}

// Split returns slice of sentences.
// For memory-efficient streaming, use Sentences iterator instead.
func (s *Splitter) Split(in string) []string {

	var sentences []string
	if s == nil {
		// sentenses tokenizer is off
		return append(sentences, in)
	}

	for _, sentence := range s.Tokenize(in) {
		sentences = append(sentences, sentence.Text)
	}

	// Sentences tokenizer has a funny way of working - sentence trailing
	// spaces belong to the next sentence. That puts off kepub viewer on Kobo
	// devices. I do not want to change external
	// "github.com/neurosnap/sentences" module - will do careful inplace
	// mockery right here instead.

	for i := range len(sentences) - 1 {
		for idx, sym := range sentences[i+1] {
			if !unicode.IsSpace(sym) {
				sentences[i] = sentences[i] + sentences[i+1][0:idx]
				sentences[i+1] = sentences[i+1][idx:]
				break
			}
		}
	}
	return sentences
}

// Sentences returns an iterator over sentences.
// This is more memory-efficient than Split for large texts as it doesn't
// allocate a slice for all sentences upfront. The iterator applies the same
// space-trimming logic as Split for Kobo device compatibility.
func (s *Splitter) Sentences(in string) iter.Seq[string] {
	return func(yield func(string) bool) {
		if s == nil {
			yield(in)
			return
		}

		sentences := s.Tokenize(in)
		if len(sentences) == 0 {
			return
		}

		// Process all sentences with space-trimming logic
		for i := 0; i < len(sentences)-1; i++ {
			text := sentences[i].Text

			// Sentences tokenizer has a funny way of working - sentence
			// trailing spaces belong to the next sentence. That puts off kepub
			// viewer on Kobo devices. I do not want to change external
			// "github.com/neurosnap/sentences" module - move leading spaces
			// from next sentence to current one here instead

			nextText := sentences[i+1].Text
			for idx, sym := range nextText {
				if !unicode.IsSpace(sym) {
					text = text + nextText[0:idx]
					sentences[i+1].Text = nextText[idx:]
					break
				}
			}
			if !yield(text) {
				return
			}
		}
		// Yield the last sentence
		if len(sentences) > 0 {
			yield(sentences[len(sentences)-1].Text)
		}
	}
}

// SplitWords returns slice of words.
// For memory-efficient streaming, use Words iterator instead.
func (*Splitter) SplitWords(in string, ignoreNBSP bool) []string {
	var (
		result = []string{}
		word   strings.Builder
	)
	for _, sym := range in {
		if isSeparator(sym, ignoreNBSP) {
			result = append(result, word.String())
			word.Reset()
			continue
		}
		word.WriteRune(sym)
	}
	return append(result, word.String())
}

// Words returns an iterator over words.
// This is more memory-efficient than SplitWords for large texts.
// The ignoreNBSP parameter determines whether NBSP (0xA0) is treated as a separator.
func (*Splitter) Words(in string, ignoreNBSP bool) iter.Seq[string] {
	return func(yield func(string) bool) {
		var word strings.Builder
		for _, sym := range in {
			if isSeparator(sym, ignoreNBSP) {
				if !yield(word.String()) {
					return
				}
				word.Reset()
				continue
			}
			word.WriteRune(sym)
		}
		yield(word.String())
	}
}

func isSeparator(r rune, ignoreNBSP bool) bool {
	if uint32(r) <= unicode.MaxLatin1 {
		switch r {
		// exclude NBSP from the list of white space separators for latin1 symbols
		case '\t', '\n', '\v', '\f', '\r', ' ', 0x85:
			return true
		case 0xA0: // NBSP
			return ignoreNBSP
		}
		return false
	}
	return unicode.IsSpace(r)
}
