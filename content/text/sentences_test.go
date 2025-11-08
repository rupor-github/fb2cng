package text

import (
	"slices"
	"strings"
	"testing"
	"unicode"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/text/language"
)

var paras = []string{
	`– Послушайте, Максим Максимыч! – сказал Печорин, приподнявшись, – ведь вы добрый человек, – а если отдадим дочь этому дикарю, он ее зарежет или продаст. Дело сделано, не надо только охотою портить; оставьте ее у меня, а у себя мою шпагу…`,
	`– Да покажите мне ее, – сказал я.`,
	`– Она за этой дверью; только я сам нынче напрасно хотел ее видеть: сидит в углу, закутавшись в покрывало, не говорит и не смотрит: пуглива, как дикая серна. Я нанял нашу духанщицу: она знает по-татарски, будет ходить за нею и приучит ее к мысли, что она моя, потому что она никому не будет принадлежать, кроме меня, – прибавил он, ударив кулаком по столу.`,
	`Я и в этом согласился… Что прикажете делать? Есть люди, с которыми непременно должно соглашаться.`,
	`– А что? – спросил я у Максима Максимыча, – в самом ли деле он приучил ее к себе, или она зачахла в неволе, с тоски по родине?`,
	`– Помилуйте, отчего же с тоски по родине? Из крепости видны были те же горы, что из аула, а этим дикарям больше ничего не надобно. Да притом Григорий Александрович каждый день дарил ей что-нибудь: первые дни она молча, гордо отталкивала подарки, которые тогда доставались духанщице и возбуждали ее красноречие. Ах, подарки! чего не сделает женщина за цветную тряпичку!.. Ну, да это в сторону… Долго бился с нею Григорий Александрович; между тем учился по-татарски, и она начинала понимать по-нашему. Мало-помалу она приучилась на него смотреть, сначала исподлобья, искоса, и всё грустила, напевала свои песни вполголоса, так что, бывало, и мне становилось грустно, когда слушал ее из соседней комнаты. Никогда не забуду одной сцены: шел я мимо и заглянул в окно; Бэла сидела на лежанке, повесив голову на грудь, а Григорий Александрович стоял перед нею.`,
	`– Послушай, моя пери, – говорил он, – ведь ты знаешь, что рано или поздно ты должна быть моею, – отчего же только мучишь меня? Разве ты любишь какого-нибудь чеченца? Если так, я тебя сейчас отпущу домой. – Она вздрогнула едва приметно и покачала головой. – Или, – продолжал он, – я тебе совершенно ненавистен? – Она вздохнула. – Или твоя вера запрещает полюбить меня? – Она побледнела и молчала. – Поверь мне, Аллах для всех племен один и тот же, и если он мне позволяет любить тебя, отчего же запретит тебе платить мне взаимностью? – Она посмотрела ему пристально в лицо, как будто пораженная этой новой мыслию; в глазах ее выразились недоверчивость и желание убедиться. Что за глаза! они так и сверкали, будто два угля. – Послушай, милая, добрая Бэла, – продолжал Печорин, – ты видишь, как я тебя люблю; я всё готов отдать, чтоб тебя развеселить: я хочу, чтоб ты была счастлива; а если ты снова будешь грустить, то я умру. Скажи, ты будешь веселей?`,
	`Она призадумалась, не спуская с него черных глаз своих, потом улыбнулась ласково и кивнула головой в знак согласия. Он взял ее руку и стал ее уговаривать, чтоб она его поцеловала; она слабо защищалась и только повторяла: «Поджалуста, поджалуста, не нада, не нада». Он стал настаивать; она задрожала, заплакала.`,
	`– Я твоя пленница, – говорила она, – твоя раба; конечно, ты можешь меня принудить, – и опять слезы.`,
	`Григорий Александрович ударил себя в лоб кулаком и выскочил в другую комнату. Я зашел к нему; он сложа руки прохаживался угрюмый взад и вперед.`,
}

var paras1 = []string{
	`– Послушайте, Максим Максимыч! – сказал Печорин, приподнявшись, – ведь вы добрый человек, – а если отдадим дочь этому дикарю, он ее зарежет или продаст. Дело сделано, не надо только охотою портить; оставьте ее у меня, а у себя мою шпагу…`,
	`– Да покажите мне ее, – сказал я.`,
	`– Она за этой дверью; только я сам нынче напрасно хотел ее видеть: сидит в углу, закутавшись в покрывало, не говорит и не смотрит: пуглива, как дикая серна. Я нанял нашу духанщицу: она знает по-татарски, будет ходить за нею и приучит ее к мысли, что она моя, потому что она никому не будет принадлежать, кроме меня, – прибавил он, ударив кулаком по столу.`,
	`Я и в этом согласился… Что прикажете делать? Есть люди, с которыми непременно должно соглашаться.`,
	`– А что? – спросил я у Максима Максимыча, – в самом ли деле он приучил ее к себе, или она зачахла в неволе, с тоски по родине?`,
	`– Помилуйте, отчего же с тоски по родине? Из крепости видны были те же горы, что из аула, а этим дикарям больше ничего не надобно. Да притом Григорий Александрович каждый день дарил ей что-нибудь: первые дни она молча, гордо отталкивала подарки, которые тогда доставались духанщице и возбуждали ее красноречие. Ах, подарки! чего не сделает женщина за цветную тряпичку!.. Ну, да это в сторону… Долго бился с нею Григорий Александрович; между тем учился по-татарски, и она начинала понимать по-нашему. Мало-помалу она приучилась на него смотреть, сначала исподлобья, искоса, и всё грустила, напевала свои песни вполголоса, так что, бывало, и мне становилось грустно, когда слушал ее из соседней комнаты. Никогда не забуду одной сцены: шел я мимо и заглянул в окно; Бэла сидела на лежанке, повесив голову на грудь, а Григорий Александрович стоял перед нею.`,
	`– Послушай, моя пери, – говорил он, – ведь ты знаешь, что рано или поздно ты должна быть моею, – отчего же только мучишь меня? Разве ты любишь какого-нибудь чеченца? Если так, я тебя сейчас отпущу домой. – Она вздрогнула едва приметно и покачала головой. – Или, – продолжал он, – я тебе совершенно ненавистен? – Она вздохнула. – Или твоя вера запрещает полюбить меня? – Она побледнела и молчала. – Поверь мне, Аллах для всех племен один и тот же, и если он мне позволяет любить тебя, отчего же запретит тебе платить мне взаимностью? – Она посмотрела ему пристально в лицо, как будто пораженная этой новой мыслию; в глазах ее выразились недоверчивость и желание убедиться. Что за глаза! они так и сверкали, будто два угля. – Послушай, милая, добрая Бэла, – продолжал Печорин, – ты видишь, как я тебя люблю; я всё готов отдать, чтоб тебя развеселить: я хочу, чтоб ты была счастлива; а если ты снова будешь грустить, то я умру. Скажи, ты будешь веселей?`,
	`Она призадумалась, не спуская с него черных глаз своих, потом улыбнулась ласково и кивнула головой в знак согласия. Он взял ее руку и стал ее уговаривать, чтоб она его поцеловала; она слабо защищалась и только повторяла: «Поджалуста, поджалуста, не нада, не нада». Он стал настаивать; она задрожала, заплакала.`,
	`– Я твоя пленница, – говорила она, – твоя раба; конечно, ты можешь меня принудить, – и опять слезы.`,
	`Григорий Александрович ударил себя в лоб кулаком и выскочил в другую комнату. Я зашел к нему; он сложа руки прохаживался угрюмый взад и вперед.`,
}

func dialoguesTransform(text string) string {
	from := "‐‑−–—―"
	to := " "

	// handle punctuation in dialogues if requested. Allows to enforce line break after
	// dash in accordance with Russian rules
	var (
		b             strings.Builder
		runes         = []rune(text)
		leadingSpaces = -1
	)

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

func TestNewSplitter(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	t.Run("Russian language", func(t *testing.T) {
		tok := NewSplitter(language.Russian, logger)
		if tok == nil {
			t.Fatal("Expected tokenizer for Russian, got nil")
		}
	})

	t.Run("English language", func(t *testing.T) {
		tok := NewSplitter(language.English, logger)
		if tok == nil {
			t.Fatal("Expected tokenizer for English, got nil")
		}
	})

	t.Run("Unsupported language", func(t *testing.T) {
		tok := NewSplitter(language.Afrikaans, logger)
		if tok != nil {
			t.Fatal("Expected nil for unsupported language")
		}
	})
}

func TestSplit(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	t.Run("Nil tokenizer", func(t *testing.T) {
		var tok *Splitter
		result := tok.Split("This is a test. This is another test.")
		if len(result) != 1 {
			t.Errorf("Expected 1 sentence with nil tokenizer, got %d", len(result))
		}
		if result[0] != "This is a test. This is another test." {
			t.Errorf("Expected original text, got %q", result[0])
		}
	})

	t.Run("Simple English sentences", func(t *testing.T) {
		tok := NewSplitter(language.English, logger)
		if tok == nil {
			t.Skip("English tokenizer not available")
		}
		text := "This is a test. This is another test."
		result := tok.Split(text)
		if len(result) != 2 {
			t.Errorf("Expected 2 sentences, got %d", len(result))
		}
	})

	t.Run("Trailing spaces handling", func(t *testing.T) {
		tok := NewSplitter(language.English, logger)
		if tok == nil {
			t.Skip("English tokenizer not available")
		}
		text := "First sentence.  Second sentence."
		result := tok.Split(text)
		if len(result) < 2 {
			t.Skip("Not enough sentences to test space handling")
		}
		for i, sent := range result[:len(result)-1] {
			if len(sent) > 0 && sent[len(sent)-1] != '.' && !strings.HasSuffix(sent, ". ") && !strings.HasSuffix(sent, ".  ") {
				t.Logf("Sentence %d: %q", i, sent)
			}
		}
	})

	t.Run("Single sentence", func(t *testing.T) {
		tok := NewSplitter(language.English, logger)
		if tok == nil {
			t.Skip("English tokenizer not available")
		}
		text := "This is a single sentence"
		result := tok.Split(text)
		if len(result) != 1 {
			t.Errorf("Expected 1 sentence, got %d", len(result))
		}
		if result[0] != text {
			t.Errorf("Expected %q, got %q", text, result[0])
		}
	})

	t.Run("Empty string", func(t *testing.T) {
		tok := NewSplitter(language.English, logger)
		if tok == nil {
			t.Skip("English tokenizer not available")
		}
		result := tok.Split("")
		if len(result) != 0 {
			t.Errorf("Expected 0 sentences for empty string, got %d", len(result))
		}
	})
}

func TestSplitWords(t *testing.T) {
	tok := &Splitter{}

	t.Run("Simple words", func(t *testing.T) {
		result := tok.SplitWords("Hello world test", false)
		expected := []string{"Hello", "world", "test"}
		if !slices.Equal(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("Words with punctuation", func(t *testing.T) {
		result := tok.SplitWords("Hello, world!", false)
		expected := []string{"Hello,", "world!"}
		if !slices.Equal(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("Multiple spaces", func(t *testing.T) {
		result := tok.SplitWords("Hello  world", false)
		expected := []string{"Hello", "", "world"}
		if !slices.Equal(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("With NBSP (ignoreNBSP=false)", func(t *testing.T) {
		text := "Hello\u00A0world"
		result := tok.SplitWords(text, false)
		expected := []string{"Hello\u00A0world"}
		if !slices.Equal(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("With NBSP (ignoreNBSP=true)", func(t *testing.T) {
		text := "Hello\u00A0world"
		result := tok.SplitWords(text, true)
		expected := []string{"Hello", "world"}
		if !slices.Equal(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("Empty string", func(t *testing.T) {
		result := tok.SplitWords("", false)
		expected := []string{""}
		if !slices.Equal(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("Only spaces", func(t *testing.T) {
		result := tok.SplitWords("   ", false)
		expected := []string{"", "", "", ""}
		if !slices.Equal(result, expected) {
			t.Errorf("Expected %v, got %v", expected, result)
		}
	})

	t.Run("Various whitespace characters", func(t *testing.T) {
		result := tok.SplitWords("Hello\t\n\vworld", false)
		if len(result) < 2 {
			t.Errorf("Expected at least 2 parts, got %d", len(result))
		}
	})
}

func TestIsSeparator(t *testing.T) {
	tests := []struct {
		name       string
		r          rune
		ignoreNBSP bool
		want       bool
	}{
		{"space", ' ', false, true},
		{"tab", '\t', false, true},
		{"newline", '\n', false, true},
		{"vertical tab", '\v', false, true},
		{"form feed", '\f', false, true},
		{"carriage return", '\r', false, true},
		{"NEL", 0x85, false, true},
		{"NBSP ignoreNBSP=false", 0xA0, false, false},
		{"NBSP ignoreNBSP=true", 0xA0, true, true},
		{"regular char", 'a', false, false},
		{"unicode space", '\u2003', false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSeparator(tt.r, tt.ignoreNBSP)
			if got != tt.want {
				t.Errorf("isSeparator(%q, %v) = %v, want %v", tt.r, tt.ignoreNBSP, got, tt.want)
			}
		})
	}
}

func TestSentencesIterator(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	t.Run("Nil tokenizer", func(t *testing.T) {
		var tok *Splitter
		text := "This is a test. This is another test."
		var result []string
		for s := range tok.Sentences(text) {
			result = append(result, s)
		}
		if len(result) != 1 || result[0] != text {
			t.Errorf("Expected single sentence with original text, got %v", result)
		}
	})

	t.Run("Compare with Split", func(t *testing.T) {
		tok := NewSplitter(language.English, logger)
		if tok == nil {
			t.Skip("English tokenizer not available")
		}
		text := "First sentence. Second sentence. Third sentence."

		sliceResult := tok.Split(text)
		var iterResult []string
		for s := range tok.Sentences(text) {
			iterResult = append(iterResult, s)
		}

		if !slices.Equal(sliceResult, iterResult) {
			t.Errorf("Iterator and slice results differ:\nSlice: %v\nIter:  %v", sliceResult, iterResult)
		}
	})

	t.Run("Empty string", func(t *testing.T) {
		tok := NewSplitter(language.English, logger)
		if tok == nil {
			t.Skip("English tokenizer not available")
		}
		var result []string
		for s := range tok.Sentences("") {
			result = append(result, s)
		}
		if len(result) != 0 {
			t.Errorf("Expected no sentences for empty string, got %v", result)
		}
	})

	t.Run("Early termination", func(t *testing.T) {
		tok := NewSplitter(language.English, logger)
		if tok == nil {
			t.Skip("English tokenizer not available")
		}
		text := "First sentence. Second sentence. Third sentence."
		count := 0
		for range tok.Sentences(text) {
			count++
			if count == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("Expected to stop at 2 sentences, got %d", count)
		}
	})
}

func TestWordsIterator(t *testing.T) {
	tok := &Splitter{}

	t.Run("Compare with SplitWords", func(t *testing.T) {
		text := "Hello world test"
		sliceResult := tok.SplitWords(text, false)
		var iterResult []string
		for w := range tok.Words(text, false) {
			iterResult = append(iterResult, w)
		}
		if !slices.Equal(sliceResult, iterResult) {
			t.Errorf("Iterator and slice results differ:\nSlice: %v\nIter:  %v", sliceResult, iterResult)
		}
	})

	t.Run("NBSP handling", func(t *testing.T) {
		text := "Hello\u00A0world"

		var resultIgnore []string
		for w := range tok.Words(text, true) {
			resultIgnore = append(resultIgnore, w)
		}
		expectedIgnore := []string{"Hello", "world"}
		if !slices.Equal(resultIgnore, expectedIgnore) {
			t.Errorf("Expected %v with ignoreNBSP=true, got %v", expectedIgnore, resultIgnore)
		}

		var resultKeep []string
		for w := range tok.Words(text, false) {
			resultKeep = append(resultKeep, w)
		}
		expectedKeep := []string{"Hello\u00A0world"}
		if !slices.Equal(resultKeep, expectedKeep) {
			t.Errorf("Expected %v with ignoreNBSP=false, got %v", expectedKeep, resultKeep)
		}
	})

	t.Run("Early termination", func(t *testing.T) {
		text := "one two three four five"
		count := 0
		for range tok.Words(text, false) {
			count++
			if count == 3 {
				break
			}
		}
		if count != 3 {
			t.Errorf("Expected to stop at 3 words, got %d", count)
		}
	})

	t.Run("Empty string", func(t *testing.T) {
		var result []string
		for w := range tok.Words("", false) {
			result = append(result, w)
		}
		expected := []string{""}
		if !slices.Equal(result, expected) {
			t.Errorf("Expected %v for empty string, got %v", expected, result)
		}
	})
}

func TestDialogueTransformation(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	tok := NewSplitter(language.Russian, logger)

	t.Run("Using slices", func(t *testing.T) {
		res := [][][]string{}

		for i, par := range paras {
			sents := tok.Split(par)
			t.Logf("Paragraph %d: %d sentences", i, len(sents))
			ss := [][]string{}
			for j, sent := range sents {
				t.Logf("Sentence %d: '%s'", j, sent)
				words := tok.SplitWords(sent, false)
				for k, word := range words {
					t.Logf("Word %d: '%s'", k, word)
				}
				ss = append(ss, words)
			}
			res = append(res, ss)
		}

		res1 := [][][]string{}

		for i, par := range paras1 {
			t.Logf("Paragraph %d: '%s'", i, par)
			sents := tok.Split(dialoguesTransform(par))
			t.Logf("Paragraph %d: %d sentences", i, len(sents))
			ss := [][]string{}
			for j, sent := range sents {
				t.Logf("Sentence %d: '%s'", j, sent)
				words := tok.SplitWords(sent, false)
				for k, word := range words {
					t.Logf("Word %d: '%s'", k, word)
				}
				ss = append(ss, words)
			}
			res1 = append(res1, ss)
		}

		if len(res) != len(res1) {
			t.Fatalf("Different number of paragraphs: %d != %d", len(res), len(res1))
		}
		for i, ss := range res {
			if len(ss) != len(res1[i]) {
				t.Fatalf("Different number of sentences in paragraph %d: %d != %d", i, len(ss), len(res1[i]))
			}
			for j, ws := range ss {
				if len(ws) != len(res1[i][j]) {
					t.Fatalf("Different number of words in sentence %d of paragraph %d: %d != %d", j, i, len(ws), len(res1[i][j]))
				}
				if !slices.Equal(ws, res1[i][j]) {
					t.Fatalf("Different words in sentence %d of paragraph %d", j, i)
				}
			}
		}
	})

	t.Run("Using iterators", func(t *testing.T) {
		res := [][][]string{}

		for i, par := range paras {
			ss := [][]string{}
			var sentCount int
			for sent := range tok.Sentences(par) {
				t.Logf("Sentence %d: '%s'", sentCount, sent)
				var words []string
				var wordCount int
				for word := range tok.Words(sent, false) {
					t.Logf("Word %d: '%s'", wordCount, word)
					words = append(words, word)
					wordCount++
				}
				ss = append(ss, words)
				sentCount++
			}
			t.Logf("Paragraph %d: %d sentences", i, sentCount)
			res = append(res, ss)
		}

		res1 := [][][]string{}

		for i, par := range paras1 {
			t.Logf("Paragraph %d: '%s'", i, par)
			ss := [][]string{}
			var sentCount int
			for sent := range tok.Sentences(dialoguesTransform(par)) {
				t.Logf("Sentence %d: '%s'", sentCount, sent)
				var words []string
				var wordCount int
				for word := range tok.Words(sent, false) {
					t.Logf("Word %d: '%s'", wordCount, word)
					words = append(words, word)
					wordCount++
				}
				ss = append(ss, words)
				sentCount++
			}
			t.Logf("Paragraph %d: %d sentences", i, sentCount)
			res1 = append(res1, ss)
		}

		if len(res) != len(res1) {
			t.Fatalf("Different number of paragraphs: %d != %d", len(res), len(res1))
		}
		for i, ss := range res {
			if len(ss) != len(res1[i]) {
				t.Fatalf("Different number of sentences in paragraph %d: %d != %d", i, len(ss), len(res1[i]))
			}
			for j, ws := range ss {
				if len(ws) != len(res1[i][j]) {
					t.Fatalf("Different number of words in sentence %d of paragraph %d: %d != %d", j, i, len(ws), len(res1[i][j]))
				}
				if !slices.Equal(ws, res1[i][j]) {
					t.Fatalf("Different words in sentence %d of paragraph %d", j, i)
				}
			}
		}
	})

	t.Run("Compare slice and iterator results", func(t *testing.T) {
		sliceRes := [][][]string{}
		for _, par := range paras {
			sents := tok.Split(par)
			ss := [][]string{}
			for _, sent := range sents {
				words := tok.SplitWords(sent, false)
				ss = append(ss, words)
			}
			sliceRes = append(sliceRes, ss)
		}

		iterRes := [][][]string{}
		for _, par := range paras {
			ss := [][]string{}
			for sent := range tok.Sentences(par) {
				var words []string
				for word := range tok.Words(sent, false) {
					words = append(words, word)
				}
				ss = append(ss, words)
			}
			iterRes = append(iterRes, ss)
		}

		if len(sliceRes) != len(iterRes) {
			t.Fatalf("Different number of paragraphs: slice=%d, iter=%d", len(sliceRes), len(iterRes))
		}
		for i, ss := range sliceRes {
			if len(ss) != len(iterRes[i]) {
				t.Fatalf("Different number of sentences in paragraph %d: slice=%d, iter=%d", i, len(ss), len(iterRes[i]))
			}
			for j, ws := range ss {
				if len(ws) != len(iterRes[i][j]) {
					t.Fatalf("Different number of words in sentence %d of paragraph %d: slice=%d, iter=%d", j, i, len(ws), len(iterRes[i][j]))
				}
				if !slices.Equal(ws, iterRes[i][j]) {
					t.Fatalf("Different words in sentence %d of paragraph %d:\nslice: %v\niter:  %v", j, i, ws, iterRes[i][j])
				}
			}
		}
		t.Log("Slice and iterator results are identical")
	})
}
