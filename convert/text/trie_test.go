package text

import (
	"testing"
)

func checkValues(trie *trie, s string, v []int, t *testing.T) {
	value, ok := trie.getValue(s)
	values := value.([]int)
	if !ok {
		t.Fatalf("No value returned for string '%s'", s)
	}

	if len(values) != len(v) {
		t.Fatalf("Length mismatch: Values for '%s' should be %v, but got %v", s, v, values)
	}
	for i := range len(values) {
		if values[i] != v[i] {
			t.Fatalf("Content mismatch: Values for '%s' should be %v, but got %v", s, v, values)
		}
	}
}

func TestTrie(t *testing.T) {
	trie := newTrie()

	trie.addString("hello, world!")
	trie.addString("hello, there!")
	trie.addString("this is a sentence.")

	if !trie.contains("hello, world!") {
		t.Error("trie should contain 'hello, world!'")
	}
	if !trie.contains("hello, there!") {
		t.Error("trie should contain 'hello, there!'")
	}
	if !trie.contains("this is a sentence.") {
		t.Error("trie should contain 'this is a sentence.'")
	}
	if trie.contains("hello, Wisconsin!") {
		t.Error("trie should NOT contain 'hello, Wisconsin!'")
	}

	expectedSize := len("hello, ") + len("world!") + len("there!") + len("this is a sentence.")
	if trie.size() != expectedSize {
		t.Errorf("trie should contain %d nodes", expectedSize)
	}

	// insert an existing string-- should be no change
	trie.addString("hello, world!")
	if trie.size() != expectedSize {
		t.Errorf("trie should still contain only %d nodes after re-adding an existing member string", expectedSize)
	}

	// three strings in total
	if len(trie.members()) != 3 {
		t.Error("trie should contain exactly three member strings")
	}

	// remove a string-- should reduce the size by the number of unique characters in that string
	trie.remove("hello, world!")
	if trie.contains("hello, world!") {
		t.Error("trie should no longer contain the string 'hello, world!'")
	}

	expectedSize -= len("world!")
	if trie.size() != expectedSize {
		t.Errorf("trie should contain %d nodes after removing 'hello, world!'", expectedSize)
	}
}

func TestMultiFind(t *testing.T) {

	trie := newTrie()

	// these are part of the matches for the word 'hyphenation'
	trie.addString(`hyph`)
	trie.addString(`hen`)
	trie.addString(`hena`)
	trie.addString(`henat`)

	expected := []string{}
	expected = append(expected, `hyph`)
	found := trie.allSubstrings(`hyphenation`)
	if len(found) != len(expected) {
		t.Errorf("expected %v but found %v", expected, found)
	}

	expected = []string{`hen`, `hena`, `henat`}

	found = trie.allSubstrings(`henation`)
	if len(found) != len(expected) {
		t.Errorf("expected %v but found %v", expected, found)
	}
}

///////////////////////////////////////////////////////////////
// Trie tests

func TestTrieValues(t *testing.T) {
	trie := newTrie()

	str := "hyphenation"
	hyp := []int{0, 3, 0, 0, 2, 5, 4, 2, 0, 2, 0}

	hyphStr := "hy3phe2n5a4t2io2n"

	// test addition using separate string and vector
	trie.addValue(str, hyp)
	if !trie.contains(str) {
		t.Error("value trie should contain the word 'hyphenation'")
	}

	if trie.size() != len(str) {
		t.Errorf("value trie should have %d nodes (the number of characters in 'hyphenation')", len(str))
	}

	if len(trie.members()) != 1 {
		t.Error("value trie should have only one member string")
	}

	trie.remove(str)
	if trie.contains(str) {
		t.Errorf("value trie should no longer contain the word '%s'", str)
	}
	if trie.size() != 0 {
		t.Error("value trie should have a node count of zero")
	}

	// test with an interspersed string of the form TeX's patterns use
	trie.addPatternString(hyphStr)
	if !trie.contains(str) {
		t.Errorf("value trie should now contain the word '%s'", str)
	}
	if trie.size() != len(str) {
		t.Errorf("value trie should consist of %d nodes, instead has %d", len(str), trie.size())
	}
	if len(trie.members()) != 1 {
		t.Error("value trie should have only one member string")
	}

	mem := trie.members()
	if mem[0] != str {
		t.Errorf("Expected first member string to be '%s', got '%s'", str, mem[0])
	}

	checkValues(trie, `hyphenation`, hyp, t)

	trie.remove(`hyphenation`)
	if trie.size() != 0 {
		t.Fail()
	}

	// test prefix values
	prefixedStr := `5emnix` // this is actually a string from the en_US TeX hyphenation trie
	purePrefixedStr := `emnix`
	values := []int{5, 0, 0, 0, 0, 0}
	trie.addValue(purePrefixedStr, values)

	if trie.size() != len(purePrefixedStr) {
		t.Errorf("Size of trie after adding '%s' should be %d, was %d", purePrefixedStr,
			len(purePrefixedStr), trie.size())
	}

	checkValues(trie, `emnix`, values, t)

	trie.remove(`emnix`)
	if trie.size() != 0 {
		t.Fail()
	}

	trie.addPatternString(prefixedStr)

	if trie.size() != len(purePrefixedStr) {
		t.Errorf("Size of trie after adding '%s' should be %d, was %d", prefixedStr, len(purePrefixedStr),
			trie.size())
	}

	checkValues(trie, `emnix`, values, t)
}

func TestMultiFindValue(t *testing.T) {
	trie := newTrie()

	// these are part of the matches for the word 'hyphenation'
	trie.addPatternString(`hy3ph`)
	trie.addPatternString(`he2n`)
	trie.addPatternString(`hena4`)
	trie.addPatternString(`hen5at`)

	v1 := []int{0, 3, 0, 0}
	v2 := []int{0, 2, 0}
	v3 := []int{0, 0, 0, 4}
	v4 := []int{0, 0, 5, 0, 0}

	expectStr := []string{`hyph`}
	expectVal := []any{v1}

	found, values := trie.allSubstringsAndValues(`hyphenation`)
	if len(found) != len(expectStr) {
		t.Errorf("expected %v but found %v", expectStr, found)
	}
	if len(values) != len(expectVal) {
		t.Errorf("Length mismatch: expected %v but found %v", expectVal, values)
	}
	for i := 0; i < len(found); i++ {
		if found[i] != expectStr[i] {
			t.Errorf("Strings content mismatch: expected %v but found %v", expectStr, found)
			break
		}
	}
	for i := range len(values) {
		ev := expectVal[i].([]int)
		fv := values[i].([]int)
		if len(ev) != len(fv) {
			t.Errorf("Value length mismatch: expected %v but found %v", ev, fv)
			break
		}
		for j := range len(ev) {
			if ev[j] != fv[j] {
				t.Errorf("Value mismatch: expected %v but found %v", ev, fv)
				break
			}
		}
	}

	expectStr = []string{`hen`, `hena`, `henat`}
	expectVal = []any{v2, v3, v4}

	found, values = trie.allSubstringsAndValues(`henation`)
	if len(found) != len(expectStr) {
		t.Errorf("expected %v but found %v", expectStr, found)
	}
	if len(values) != len(expectVal) {
		t.Errorf("Length mismatch: expected %v but found %v", expectVal, values)
	}
	for i := 0; i < len(found); i++ {
		if found[i] != expectStr[i] {
			t.Errorf("Strings content mismatch: expected %v but found %v", expectStr, found)
			break
		}
	}
	for i := 0; i < len(values); i++ {
		ev := expectVal[i].([]int)
		fv := values[i].([]int)
		if len(ev) != len(fv) {
			t.Errorf("Value length mismatch: expected %v but found %v", ev, fv)
			break
		}
		for i := 0; i < len(ev); i++ {
			if ev[i] != fv[i] {
				t.Errorf("Value mismatch: expected %v but found %v", ev, fv)
				break
			}
		}
	}
}

func TestTrieEmptyStrings(t *testing.T) {
	trie := newTrie()

	trie.addString("")
	if trie.size() != 0 {
		t.Error("adding empty string should not change trie size")
	}

	trie.addValue("", []int{1, 2, 3})
	if trie.size() != 0 {
		t.Error("adding empty string with value should not change trie size")
	}

	if trie.contains("") {
		t.Error("trie should not contain empty string")
	}

	isEmpty := trie.remove("")
	if !isEmpty {
		t.Error("removing empty string from empty trie should return true (trie is empty)")
	}

	_, ok := trie.getValue("")
	if ok {
		t.Error("getValue on empty string should return false")
	}
}

func TestTrieUnicodeEdgeCases(t *testing.T) {
	trie := newTrie()

	emoji := "hello😀world"
	trie.addString(emoji)
	if !trie.contains(emoji) {
		t.Error("trie should contain emoji string")
	}

	combining := "café"
	trie.addString(combining)
	if !trie.contains(combining) {
		t.Error("trie should contain string with combining characters")
	}

	cyrillic := "Привет"
	trie.addString(cyrillic)
	if !trie.contains(cyrillic) {
		t.Error("trie should contain Cyrillic string")
	}

	chinese := "你好世界"
	trie.addString(chinese)
	if !trie.contains(chinese) {
		t.Error("trie should contain Chinese string")
	}
}

func TestTrieSingleCharacter(t *testing.T) {
	trie := newTrie()

	trie.addString("a")
	if !trie.contains("a") {
		t.Error("trie should contain single character")
	}

	if trie.size() != 1 {
		t.Errorf("trie size should be 1, got %d", trie.size())
	}

	trie.addValue("b", []int{5})
	val, ok := trie.getValue("b")
	if !ok {
		t.Error("should retrieve value for single character")
	}
	if v := val.([]int); len(v) != 1 || v[0] != 5 {
		t.Errorf("expected [5], got %v", v)
	}
}

func TestTriePatternEdgeCases(t *testing.T) {
	trie := newTrie()

	trie.addPatternString("12abc34")
	if !trie.contains("abc") {
		t.Error("should extract 'abc' from pattern with consecutive digits")
	}

	trie.addPatternString("xyz9")
	if !trie.contains("xyz") {
		t.Error("should extract 'xyz' from pattern with trailing digit")
	}

	trie.addPatternString("5start")
	if !trie.contains("start") {
		t.Error("should extract 'start' from pattern with leading digit")
	}

	val, ok := trie.getValue("abc")
	if !ok {
		t.Error("should have value for pattern with consecutive digits")
	}
	expected := []int{1, 2, 3, 4}
	if v := val.([]int); len(v) != len(expected) {
		t.Errorf("expected length %d, got %d", len(expected), len(v))
	}
}

func TestTrieGetValueNonExistent(t *testing.T) {
	trie := newTrie()
	trie.addString("hello")

	_, ok := trie.getValue("world")
	if ok {
		t.Error("getValue should return false for non-existent string")
	}

	_, ok = trie.getValue("helloworld")
	if ok {
		t.Error("getValue should return false for longer non-existent string")
	}
}

func TestTrieRemoveNonExistent(t *testing.T) {
	trie := newTrie()
	trie.addString("hello")

	initialSize := trie.size()
	trie.remove("world")

	if trie.size() != initialSize {
		t.Error("removing non-existent string should not change size")
	}
}

func TestTrieOverwriteValue(t *testing.T) {
	trie := newTrie()

	trie.addValue("test", []int{1, 2, 3})
	val1, _ := trie.getValue("test")

	trie.addValue("test", []int{4, 5, 6})
	val2, _ := trie.getValue("test")

	v1 := val1.([]int)
	v2 := val2.([]int)

	if len(v2) != 3 || v2[0] != 4 || v2[1] != 5 || v2[2] != 6 {
		t.Errorf("value should be overwritten, got %v", v2)
	}

	if trie.size() != len("test") {
		t.Error("overwriting value should not change trie size")
	}

	if v1[0] == v2[0] {
		t.Error("values should be different after overwrite")
	}
}
