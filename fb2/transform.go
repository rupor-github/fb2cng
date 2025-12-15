package fb2

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"fbc/config"
)

// applyTextTransformations applies all requested text transformations
func applyTextTransformations(text string, cfg *config.TextTransformConfig) string {
	if cfg.Speech.Enable {
		text = transformSpeech(text, string(cfg.Speech.From), string(cfg.Speech.To))
	}
	if cfg.Dashes.Enable {
		text = transformDashes(text, string(cfg.Dashes.From), string(cfg.Dashes.To))
	}
	if cfg.Dialogue.Enable {
		text = transformDialogue(text, string(cfg.Dialogue.From), string(cfg.Dialogue.To))
	}
	return text
}

// transformSpeech normalizes direct speech if requested - legacy, from fb2mobi
func transformSpeech(text string, from, to string) string {
	cutIndex := 0
	for i, sym := range text {
		if i == 0 {
			if !strings.ContainsRune(from, sym) {
				break
			}
			cutIndex += utf8.RuneLen(sym)
		} else {
			if unicode.IsSpace(sym) {
				cutIndex += utf8.RuneLen(sym)
			} else {
				text = to + text[cutIndex:]
				break
			}
		}
	}
	return text
}

// transformDashes unifies dashes if requested - legacy, from fb2mobi
func transformDashes(text string, from, to string) string {
	runes := []rune(text)

	var b strings.Builder
	for i := range len(runes) {
		if i > 0 && unicode.IsSpace(runes[i-1]) &&
			i < len(runes)-1 && unicode.IsSpace(runes[i+1]) &&
			strings.ContainsRune(from, runes[i]) {

			b.WriteString(to)
			continue
		}
		b.WriteRune(runes[i])
	}

	return b.String()
}

// transformDialogue handles punctuation in dialogues if requested. Attenmpts to enforce line break after
// dash in accordance with rules of the Russian language
func transformDialogue(text string, from, to string) string {
	runes := []rune(text)

	var b strings.Builder
	leadingSpaces := -1
	for i := range len(runes) {
		if unicode.IsSpace(runes[i]) {
			leadingSpaces = i
			continue
		}
		if i > 0 && strings.ContainsRune(from, runes[i]) {
			b.WriteString(to)
			b.WriteRune(runes[i])
			leadingSpaces = -1
			continue
		}
		if leadingSpaces >= 0 {
			b.WriteString(string(runes[leadingSpaces:i]))
		}
		b.WriteRune(runes[i])
		leadingSpaces = -1
	}
	if leadingSpaces > 0 {
		b.WriteString(string(runes[leadingSpaces:]))
	}
	return b.String()
}
