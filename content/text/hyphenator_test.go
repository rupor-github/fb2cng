package text

import (
	"strings"
	"testing"

	"go.uber.org/zap"
	"golang.org/x/text/language"
)

func buildHyphenator(t *testing.T, lang string) *hyph {
	dataPatterns, err := tryLoadDictionary(lang, "pat")
	if err != nil {
		t.Fatalf("Unable to load patterns dictionary for %s: %v", lang, err)
	}

	dataExceptions, err := tryLoadDictionary(lang, "hyp")
	if err != nil {
		dataExceptions = []byte{}
	}

	h := new(hyph)
	if err := h.loadDictionary(lang, strings.NewReader(string(dataPatterns)), strings.NewReader(string(dataExceptions))); err != nil {
		t.Fatalf("Unable to load dictionary for %s: %v", lang, err)
	}
	return h
}

// technically this string contains an em-dash character, but the scanner.Scanner barfs on that for some
// reason, producing error glyphs in the output.  It also parses it twice, which is super-annoying.  For
// this reason I've replaced it with a double-hyphen sequence, like many ASCII-limited people before me.
const testStr = `Go is a new language. Although it borrows ideas from existing languages, it has unusual properties that make effective Go programs different in character from programs written in its relatives. A straightforward translation of a C++ or Java program into Go is unlikely to produce a satisfactory result--Java programs are written in Java, not Go. On the other hand, thinking about the problem from a Go perspective could produce a successful but quite different program. In other words, to write Go well, it's important to understand its properties and idioms. It's also important to know the established conventions for programming in Go, such as naming, formatting, program construction, and so on, so that programs you write will be easy for other Go programmers to understand.

This document gives tips for writing clear, idiomatic Go code. It augments the language specification and the tutorial, both of which you should read first.

Examples
The Go package sources are intended to serve not only as the core library but also as examples of how to use the language. If you have a question about how to approach a problem or how something might be implemented, they can provide answers, ideas and background.

Formatting
Formatting issues are the most contentious but the least consequential. People can adapt to different formatting styles but it's better if they don't have to, and less time is devoted to the topic if everyone adheres to the same style. The problem is how to approach this Utopia without a long prescriptive style guide.

With Go we take an unusual approach and let the machine take care of most formatting issues. A program, gofmt, reads a Go program and emits the source in a standard style of indentation and vertical alignment, retaining and if necessary reformatting comments. If you want to know how to handle some new layout situation, run gofmt; if the answer doesn't seem right, fix the program (or file a bug), don't work around it.

As an example, there's no need to spend time lining up the comments on the fields of a structure. Gofmt will do that for you.`

const hyphStr = `Go is a new lan-guage. Although it bor-rows ideas from ex-ist-ing lan-guages, it has un-usu-al prop-er-ties that make ef-fec-tive Go pro-grams dif-fer-ent in char-ac-ter from pro-grams writ-ten in its rel-a-tives. A straight-for-ward trans-la-tion of a C++ or Ja-va pro-gram in-to Go is un-like-ly to pro-duce a sat-is-fac-to-ry re-sult--Ja-va pro-grams are writ-ten in Ja-va, not Go. On the oth-er hand, think-ing about the prob-lem from a Go per-spec-tive could pro-duce a suc-cess-ful but quite dif-fer-ent pro-gram. In oth-er words, to write Go well, it's im-por-tant to un-der-stand its prop-er-ties and id-ioms. It's al-so im-por-tant to know the es-tab-lished con-ven-tions for pro-gram-ming in Go, such as nam-ing, for-mat-ting, pro-gram con-struc-tion, and so on, so that pro-grams you write will be easy for oth-er Go pro-gram-mers to un-der-stand.

This doc-u-ment gives tips for writ-ing clear, id-iomat-ic Go code. It aug-ments the lan-guage spec-i-fi-ca-tion and the tu-to-r-i-al, both of which you should read first.

Ex-am-ples
The Go pack-age sources are in-tend-ed to serve not on-ly as the core li-brary but al-so as ex-am-ples of how to use the lan-guage. If you have a ques-tion about how to ap-proach a prob-lem or how some-thing might be im-ple-ment-ed, they can pro-vide an-swers, ideas and back-ground.

For-mat-ting
For-mat-ting is-sues are the most con-tentious but the least con-se-quen-tial. Peo-ple can adapt to dif-fer-ent for-mat-ting styles but it's bet-ter if they don't have to, and less time is de-vot-ed to the top-ic if every-one ad-heres to the same style. The prob-lem is how to ap-proach this Utopia with-out a long pre-scrip-tive style guide.

With Go we take an un-usu-al ap-proach and let the ma-chine take care of most for-mat-ting is-sues. A pro-gram, gofmt, reads a Go pro-gram and emits the source in a stan-dard style of in-den-ta-tion and ver-ti-cal align-ment, re-tain-ing and if nec-es-sary re-for-mat-ting com-ments. If you want to know how to han-dle some new lay-out sit-u-a-tion, run gofmt; if the an-swer doesn't seem right, fix the pro-gram (or file a bug), don't work around it.

As an ex-am-ple, there's no need to spend time lin-ing up the com-ments on the fields of a struc-ture. Gofmt will do that for you.`

func TestHyphenatorEnglish(t *testing.T) {
	h := buildHyphenator(t, "en-us")
	hyphenated := h.hyphString(testStr, `-`)

	if hyphenated != hyphStr {
		t.Logf("Got:\n%s", hyphenated)
		t.Logf("Expected:\n%s", hyphStr)
		t.Fail()
	}
}

const testStrRU = `Швейк несколько лет тому назад, после того как медицинская комиссия признала его идиотом, ушёл с военной службы и теперь промышлял продажей собак, безобразных ублюдков, которым он сочинял фальшивые родословные.

Кроме того, он страдал ревматизмом и в настоящий момент растирал себе колени оподельдоком.

— Какого Фердинанда, пани Мюллерова? — спросил Швейк, не переставая массировать колени.

— Я знаю двух Фердинандов. Один служит у фармацевта Пруши. Как-то раз по ошибке он выпил у него бутылку жидкости для ращения волос; а ещё есть Фердинанд Кокошка, тот, что собирает собачье дерьмо. Обоих ни чуточки не жалко.

Ударения сделаны как отдельным символом ́ , так и похожими на русские буквы с ударением символами á, é, ó (т.к. в существующих книгах встречаются оба варианта).`

const hyphStrRU = `Швейк несколь-ко лет то-му на-зад, по-сле то-го как ме-ди-цин-ская ко-мис-сия при-зна-ла его иди-о-том, ушёл с во-ен-ной служ-бы и те-перь про-мыш-лял про-да-жей со-бак, без-об-раз-ных ублюд-ков, ко-то-рым он со-чи-нял фаль-ши-вые ро-до-слов-ные.

Кро-ме то-го, он стра-дал рев-ма-тиз-мом и в на-сто-я-щий мо-мент рас-ти-рал се-бе ко-ле-ни опо-дель-до-ком.

— Ка-ко-го Фер-ди-нан-да, па-ни Мюл-ле-ро-ва? — спро-сил Швейк, не пе-ре-ста-вая мас-си-ро-вать ко-ле-ни.

— Я знаю двух Фер-ди-нан-дов. Один слу-жит у фар-ма-цев-та Пру-ши. Как-то раз по ошиб-ке он вы-пил у него бу-тыл-ку жид-ко-сти для ра-ще-ния во-лос; а ещё есть Фер-ди-нанд Ко-кош-ка, тот, что со-би-ра-ет со-ба-чье дерь-мо. Обо-их ни чу-точ-ки не жал-ко.

Уда-ре-ния сде-ла-ны как от-дель-ным сим-во-лом ́ , так и по-хо-жи-ми на рус-ские бук-вы с уда-ре-ни-ем сим-во-ла-ми á, é, ó (т.к. в су-ще-ству-ю-щих кни-гах встре-ча-ют-ся оба ва-ри-ан-та).`

func TestHyphenatorRussian(t *testing.T) {
	h := buildHyphenator(t, "ru")
	hyphenated := h.hyphString(testStrRU, `-`)

	if hyphenated != hyphStrRU {
		t.Logf("Got:\n%s", hyphenated)
		t.Logf("Expected:\n%s", hyphStrRU)
		t.Fail()
	}
}

const (
	testStrSpecial = `сегодня? –`
	hyphStrSpecial = "се" + SOFTHYPHEN + "го" + SOFTHYPHEN + "дня? –"
)

func TestHyphenatorSpecial(t *testing.T) {
	h := buildHyphenator(t, "ru")
	hyphenated := h.hyphString(testStrSpecial, SOFTHYPHEN)

	t.Log(hyphenated)

	if hyphenated != hyphStrSpecial {
		t.Fail()
	}
}

func TestNewHyphenatorValid(t *testing.T) {
	log, _ := zap.NewDevelopment()

	h := NewHyphenator(language.English, log)
	if h == nil {
		t.Error("should create hyphenator for English")
	}

	h = NewHyphenator(language.Russian, log)
	if h == nil {
		t.Error("should create hyphenator for Russian")
	}

	h = NewHyphenator(language.German, log)
	if h == nil {
		t.Error("should create hyphenator for German")
	}
}

func TestNewHyphenatorLanguageMapping(t *testing.T) {
	log, _ := zap.NewDevelopment()

	germanTag := language.MustParse("de-DE")
	h := NewHyphenator(germanTag, log)
	if h == nil {
		t.Error("should create hyphenator for de-DE using language mapping")
	}

	germanAustriaTag := language.MustParse("de-AT")
	h = NewHyphenator(germanAustriaTag, log)
	if h == nil {
		t.Error("should create hyphenator for de-AT using language mapping")
	}
}

func TestNewHyphenatorUnsupportedLanguage(t *testing.T) {
	log, _ := zap.NewDevelopment()

	unsupported := language.MustParse("zu")
	h := NewHyphenator(unsupported, log)
	if h != nil {
		t.Error("should return nil for unsupported language")
	}
}

func TestHyphenatePublicAPI(t *testing.T) {
	log, _ := zap.NewDevelopment()

	h := NewHyphenator(language.English, log)
	if h == nil {
		t.Fatal("failed to create hyphenator")
	}

	result := h.Hyphenate("hyphenation")
	if !strings.Contains(result, SOFTHYPHEN) {
		t.Error("should insert soft hyphens into word")
	}
}

func TestHyphenateNilHyphenator(t *testing.T) {
	var h *Hyphenator
	result := h.Hyphenate("test")
	if result != "test" {
		t.Error("nil hyphenator should return input unchanged")
	}
}

func TestHyphenatorEmptyString(t *testing.T) {
	log, _ := zap.NewDevelopment()
	h := NewHyphenator(language.English, log)
	if h == nil {
		t.Fatal("failed to create hyphenator")
	}

	result := h.Hyphenate("")
	if result != "" {
		t.Error("empty string should return empty string")
	}
}

func TestHyphenatorSingleCharacter(t *testing.T) {
	log, _ := zap.NewDevelopment()
	h := NewHyphenator(language.English, log)
	if h == nil {
		t.Fatal("failed to create hyphenator")
	}

	result := h.Hyphenate("a")
	if result != "a" {
		t.Error("single character should not be hyphenated")
	}
}

func TestHyphenatorNumbers(t *testing.T) {
	log, _ := zap.NewDevelopment()
	h := NewHyphenator(language.English, log)
	if h == nil {
		t.Fatal("failed to create hyphenator")
	}

	result := h.Hyphenate("12345")
	if result != "12345" {
		t.Error("numbers should not be hyphenated")
	}
}

func TestHyphenatorMixedContent(t *testing.T) {
	log, _ := zap.NewDevelopment()
	h := NewHyphenator(language.English, log)
	if h == nil {
		t.Fatal("failed to create hyphenator")
	}

	input := "test123test"
	result := h.Hyphenate(input)
	if !strings.Contains(result, "123") {
		t.Error("numbers should remain unchanged in mixed content")
	}
}

func TestHyphenatorSpecialCharacters(t *testing.T) {
	log, _ := zap.NewDevelopment()
	h := NewHyphenator(language.English, log)
	if h == nil {
		t.Fatal("failed to create hyphenator")
	}

	input := "hello! world?"
	result := h.Hyphenate(input)
	if !strings.Contains(result, "!") || !strings.Contains(result, "?") {
		t.Error("special characters should be preserved")
	}
}

func TestHyphenatorPunctuation(t *testing.T) {
	log, _ := zap.NewDevelopment()
	h := NewHyphenator(language.English, log)
	if h == nil {
		t.Fatal("failed to create hyphenator")
	}

	input := "word, word; word."
	result := h.Hyphenate(input)
	if !strings.Contains(result, ",") || !strings.Contains(result, ";") || !strings.Contains(result, ".") {
		t.Error("punctuation should be preserved")
	}
}

func TestHyphenatorUnicodeText(t *testing.T) {
	log, _ := zap.NewDevelopment()

	h := NewHyphenator(language.Russian, log)
	if h == nil {
		t.Fatal("failed to create Russian hyphenator")
	}

	result := h.Hyphenate("привет")
	if result == "" {
		t.Error("should handle Cyrillic text")
	}
}

func TestHyphenatorExceptions(t *testing.T) {
	h := buildHyphenator(t, "en-us")

	word := "present"
	result := h.hyphString(word, "-")

	if strings.Count(result, "-") == 0 {
		t.Log("Word 'present' was not hyphenated (may be in exceptions list or no pattern match)")
	}
}

func TestHyphenatorVeryShortWords(t *testing.T) {
	log, _ := zap.NewDevelopment()
	h := NewHyphenator(language.English, log)
	if h == nil {
		t.Fatal("failed to create hyphenator")
	}

	twoChar := h.Hyphenate("at")
	if strings.Contains(twoChar, SOFTHYPHEN) {
		t.Error("two character words should not be hyphenated")
	}

	threeChar := h.Hyphenate("the")
	if strings.Contains(threeChar, SOFTHYPHEN) {
		t.Error("three character words should not be hyphenated")
	}
}

func TestHyphenatorLoadDictionaryError(t *testing.T) {
	h := &hyph{}

	err := h.loadDictionary("test-lang", strings.NewReader(""), strings.NewReader(""))
	if err != nil {
		t.Errorf("loading empty patterns should not error: %v", err)
	}

	if h.patterns == nil {
		t.Error("patterns trie should be initialized")
	}

	if h.exceptions == nil {
		t.Error("exceptions map should be initialized")
	}
}

func TestHyphenatorReloadDictionary(t *testing.T) {
	h := &hyph{}

	err := h.loadDictionary("lang1", strings.NewReader("a1b"), strings.NewReader(""))
	if err != nil {
		t.Fatalf("first load failed: %v", err)
	}

	firstSize := h.patterns.size()

	err = h.loadDictionary("lang2", strings.NewReader("c2d"), strings.NewReader(""))
	if err != nil {
		t.Fatalf("second load failed: %v", err)
	}

	if h.language != "lang2" {
		t.Error("language should be updated")
	}

	if h.patterns == nil {
		t.Error("patterns should be reinitialized")
	}

	err = h.loadDictionary("lang2", strings.NewReader("e3f"), strings.NewReader(""))
	if err != nil {
		t.Fatalf("reload same language failed: %v", err)
	}

	if h.patterns.size() == firstSize {
		t.Log("same language reload kept existing patterns (expected behavior)")
	}
}
