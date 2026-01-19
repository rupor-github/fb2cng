package kfx

import (
	"testing"
)

func TestNormalizingWriter_Basic(t *testing.T) {
	nw := newNormalizingWriter()
	nw.WriteString("Hello")
	nw.WriteString(" ")
	nw.WriteString("World")

	if got := nw.String(); got != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", got)
	}
}

func TestNormalizingWriter_CollapseWhitespace(t *testing.T) {
	nw := newNormalizingWriter()
	nw.WriteString("Hello   \t\n  World")

	if got := nw.String(); got != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", got)
	}
}

func TestNormalizingWriter_TrimLeadingWhitespace(t *testing.T) {
	nw := newNormalizingWriter()
	nw.WriteString("  \n\t  Hello")

	if got := nw.String(); got != "Hello" {
		t.Errorf("expected 'Hello', got %q", got)
	}
}

func TestNormalizingWriter_TrimTrailingWhitespace(t *testing.T) {
	nw := newNormalizingWriter()
	nw.WriteString("Hello  \n\t  ")

	if got := nw.String(); got != "Hello" {
		t.Errorf("expected 'Hello', got %q", got)
	}
}

func TestNormalizingWriter_WriteRaw(t *testing.T) {
	nw := newNormalizingWriter()
	nw.WriteString("Line1")
	nw.WriteRaw("\n")
	nw.WriteString("Line2")

	if got := nw.String(); got != "Line1\nLine2" {
		t.Errorf("expected 'Line1\\nLine2', got %q", got)
	}
}

func TestNormalizingWriter_SuppressSpaceAfterRaw(t *testing.T) {
	// This tests the fix for whitespace after structural breaks in titles.
	// When we have:
	//   <p>First para</p>
	//   <p>\n  <emphasis>\n    <sub>Second para</sub></emphasis></p>
	// The whitespace ("\n  ", "\n    ") should NOT produce a leading space.
	nw := newNormalizingWriter()
	nw.WriteString("First para")
	nw.WriteRaw("\n")
	// Simulate whitespace-only text segments from XML parsing
	nw.WriteString("\n  ")        // whitespace before <emphasis>
	nw.WriteString("\n    ")      // whitespace before <sub>
	nw.WriteString("Second para") // actual content

	expected := "First para\nSecond para"
	if got := nw.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestNormalizingWriter_SuppressMultipleWhitespaceSegmentsAfterRaw(t *testing.T) {
	// More thorough test: multiple WriteString calls with whitespace after WriteRaw
	nw := newNormalizingWriter()
	nw.WriteString("A")
	nw.WriteRaw("\n")
	nw.WriteString(" ")  // single space
	nw.WriteString("  ") // multiple spaces
	nw.WriteString("\t") // tab
	nw.WriteString("\n") // newline
	nw.WriteString("B")

	expected := "A\nB"
	if got := nw.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestNormalizingWriter_PendingSpaceAfterWriteRawContent(t *testing.T) {
	// After writing content following WriteRaw, whitespace should work normally again
	nw := newNormalizingWriter()
	nw.WriteString("A")
	nw.WriteRaw("\n")
	nw.WriteString("B")  // this clears suppressNextSpace
	nw.WriteString("  ") // this should set pendingSpace (not suppressed)
	nw.WriteString("C")

	expected := "A\nB C"
	if got := nw.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestNormalizingWriter_PreserveWhitespace(t *testing.T) {
	nw := newNormalizingWriter()
	nw.SetPreserveWhitespace(true)
	nw.WriteString("  code  \n  with  spaces  ")
	nw.SetPreserveWhitespace(false)
	nw.WriteString(" normal")

	expected := "  code  \n  with  spaces   normal"
	if got := nw.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestNormalizingWriter_RuneCount(t *testing.T) {
	nw := newNormalizingWriter()
	nw.WriteString("Привет") // 6 Cyrillic characters

	if got := nw.RuneCount(); got != 6 {
		t.Errorf("expected rune count 6, got %d", got)
	}

	nw.WriteString(" мир") // space + 3 Cyrillic characters

	if got := nw.RuneCount(); got != 10 {
		t.Errorf("expected rune count 10, got %d", got)
	}
}

func TestNormalizingWriter_RuneCountWithRaw(t *testing.T) {
	nw := newNormalizingWriter()
	nw.WriteString("Line1") // 5 chars
	nw.WriteRaw("\n")       // 1 char
	nw.WriteString("  ")    // suppressed, 0 chars written
	nw.WriteString("Line2") // 5 chars

	if got := nw.RuneCount(); got != 11 {
		t.Errorf("expected rune count 11, got %d", got)
	}
}

func TestNormalizingWriter_MultiParagraphTitle(t *testing.T) {
	// Simulates the exact scenario from the bug:
	// <title>
	//   <p>Надстрочник и подстрочник.<a>[18]</a></p>
	//   <p><emphasis><sub>Подстрочник в заголовке.</sub></emphasis></p>
	//   <p><sup><emphasis>Надстрочник в заголовке.</emphasis></sup></p>
	// </title>
	nw := newNormalizingWriter()

	// First paragraph
	nw.WriteString("Надстрочник и подстрочник.")
	nw.WriteString("[18]")

	// Newline between paragraphs
	nw.WriteRaw("\n")

	// Second paragraph with nested XML whitespace
	nw.WriteString("\n          ")   // before <emphasis>
	nw.WriteString("\n            ") // before <sub>
	nw.WriteString("Подстрочник в заголовке.")
	nw.WriteString("\n          ") // after </sub>
	nw.WriteString("\n        ")   // after </emphasis>

	// Newline between paragraphs
	nw.WriteRaw("\n")

	// Third paragraph with nested XML whitespace
	nw.WriteString("\n          ")   // before <sup>
	nw.WriteString("\n            ") // before <emphasis>
	nw.WriteString("Надстрочник в заголовке.")
	nw.WriteString("\n          ") // after </emphasis>
	nw.WriteString("\n        ")   // after </sup>

	expected := "Надстрочник и подстрочник.[18]\nПодстрочник в заголовке.\nНадстрочник в заголовке."
	if got := nw.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
