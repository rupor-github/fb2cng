package kfx

import (
	"testing"

	"go.uber.org/zap"

	"fbc/css"
)

func TestParseCSSContent(t *testing.T) {
	tests := []struct {
		name     string
		input    css.Value
		expected string
	}{
		{
			name:     "double quoted string",
			input:    css.Value{Raw: `"["`},
			expected: "[",
		},
		{
			name:     "single quoted string",
			input:    css.Value{Raw: `']'`},
			expected: "]",
		},
		{
			name:     "complex content",
			input:    css.Value{Raw: `">>> "`},
			expected: ">>> ",
		},
		{
			name:     "none keyword",
			input:    css.Value{Raw: "none"},
			expected: "",
		},
		{
			name:     "normal keyword",
			input:    css.Value{Raw: "normal"},
			expected: "",
		},
		{
			name:     "empty value",
			input:    css.Value{Raw: ""},
			expected: "",
		},
		{
			name:     "unquoted value (attr function)",
			input:    css.Value{Raw: "attr(data-before)"},
			expected: "", // Not supported
		},
		{
			name:     "counter function",
			input:    css.Value{Raw: "counter(section)"},
			expected: "", // Not supported
		},
		{
			name:     "whitespace around quotes",
			input:    css.Value{Raw: `  "text"  `},
			expected: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCSSContent(tt.input)
			if result != tt.expected {
				t.Errorf("parseCSSContent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractPseudoContent(t *testing.T) {
	log := zap.NewNop()
	parser := css.NewParser(log)

	tests := []struct {
		name          string
		cssText       string
		checkClass    string
		wantBefore    string
		wantAfter     string
		wantWarnings  int
		wantHasPseudo bool
	}{
		{
			name: "before and after",
			cssText: `
				.note::before { content: "["; }
				.note::after { content: "]"; }
			`,
			checkClass:    "note",
			wantBefore:    "[",
			wantAfter:     "]",
			wantWarnings:  0,
			wantHasPseudo: true,
		},
		{
			name: "only before",
			cssText: `
				.prefix::before { content: ">>> "; }
			`,
			checkClass:    "prefix",
			wantBefore:    ">>> ",
			wantAfter:     "",
			wantWarnings:  0,
			wantHasPseudo: true,
		},
		{
			name: "only after",
			cssText: `
				.suffix::after { content: " <<<"; }
			`,
			checkClass:    "suffix",
			wantBefore:    "",
			wantAfter:     " <<<",
			wantWarnings:  0,
			wantHasPseudo: true,
		},
		{
			name: "pseudo with unsupported properties",
			cssText: `
				.styled::before {
					content: "!";
					color: red;
					font-weight: bold;
				}
			`,
			checkClass:    "styled",
			wantBefore:    "!",
			wantAfter:     "",
			wantWarnings:  2, // color and font-weight
			wantHasPseudo: true,
		},
		{
			name: "pseudo without content",
			cssText: `
				.empty::before { color: red; }
			`,
			checkClass:    "empty",
			wantBefore:    "",
			wantAfter:     "",
			wantWarnings:  0, // No content = no warning (rule is just ignored)
			wantHasPseudo: false,
		},
		{
			name: "pseudo with none content",
			cssText: `
				.none::before { content: none; }
			`,
			checkClass:    "none",
			wantBefore:    "",
			wantAfter:     "",
			wantWarnings:  0,
			wantHasPseudo: false,
		},
		{
			name:          "no pseudo elements",
			cssText:       `.normal { font-weight: bold; }`,
			checkClass:    "normal",
			wantBefore:    "",
			wantAfter:     "",
			wantWarnings:  0,
			wantHasPseudo: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sheet := parser.Parse([]byte(tt.cssText))
			sr := NewStyleRegistry()
			warnings := sr.extractPseudoContent(sheet)

			if len(warnings) != tt.wantWarnings {
				t.Errorf("got %d warnings, want %d: %v", len(warnings), tt.wantWarnings, warnings)
			}

			if sr.HasPseudoContent() != tt.wantHasPseudo {
				t.Errorf("HasPseudoContent() = %v, want %v", sr.HasPseudoContent(), tt.wantHasPseudo)
			}

			pc := sr.GetPseudoContentForClass(tt.checkClass)
			if tt.wantBefore == "" && tt.wantAfter == "" {
				if pc != nil {
					t.Errorf("expected no pseudo content for %q, got %+v", tt.checkClass, pc)
				}
			} else {
				if pc == nil {
					t.Fatalf("expected pseudo content for %q, got nil", tt.checkClass)
				}
				if pc.Before != tt.wantBefore {
					t.Errorf("Before = %q, want %q", pc.Before, tt.wantBefore)
				}
				if pc.After != tt.wantAfter {
					t.Errorf("After = %q, want %q", pc.After, tt.wantAfter)
				}
			}
		})
	}
}

func TestExtractPseudoContent_NilSheet(t *testing.T) {
	sr := NewStyleRegistry()
	warnings := sr.extractPseudoContent(nil)

	if warnings != nil {
		t.Errorf("expected nil warnings for nil sheet, got %v", warnings)
	}
	if sr.HasPseudoContent() {
		t.Error("expected no pseudo content for nil sheet")
	}
}

func TestRegisterPseudoContentByName(t *testing.T) {
	sr := NewStyleRegistry()

	// Register ::before content
	sr.registerPseudoContentByName("link-footnote--before", "[")

	// Register ::after content for same base class
	sr.registerPseudoContentByName("link-footnote--after", "]")

	pc := sr.GetPseudoContentForClass("link-footnote")
	if pc == nil {
		t.Fatal("expected pseudo content")
	}

	if pc.Before != "[" {
		t.Errorf("Before = %q, want %q", pc.Before, "[")
	}
	if pc.After != "]" {
		t.Errorf("After = %q, want %q", pc.After, "]")
	}
}

func TestRegisterPseudoContentByName_InvalidSuffix(t *testing.T) {
	sr := NewStyleRegistry()

	// Name without --before or --after suffix should be ignored
	sr.registerPseudoContentByName("link-footnote", "text")

	if sr.HasPseudoContent() {
		t.Error("expected no pseudo content for invalid suffix")
	}
}

func TestGetPseudoContent_MultipleClasses(t *testing.T) {
	sr := NewStyleRegistry()
	sr.registerPseudoContentByName("note--before", "NOTE: ")
	sr.registerPseudoContentByName("warning--before", "WARNING: ")

	// Test GetPseudoContent with space-separated classes
	pc := sr.GetPseudoContent("other note more")
	if pc == nil {
		t.Fatal("expected to find pseudo content for 'note' class")
	}
	if pc.Before != "NOTE: " {
		t.Errorf("Before = %q, want %q", pc.Before, "NOTE: ")
	}

	// First matching class wins
	pc = sr.GetPseudoContent("warning note")
	if pc == nil {
		t.Fatal("expected to find pseudo content")
	}
	if pc.Before != "WARNING: " {
		t.Errorf("Before = %q, want %q (first matching class)", pc.Before, "WARNING: ")
	}
}

func TestGetPseudoContent_NoMatch(t *testing.T) {
	sr := NewStyleRegistry()
	sr.registerPseudoContentByName("note--before", "NOTE: ")

	pc := sr.GetPseudoContent("unknown-class other")
	if pc != nil {
		t.Errorf("expected nil for non-matching classes, got %+v", pc)
	}
}

func TestGetPseudoContent_NilRegistry(t *testing.T) {
	sr := &StyleRegistry{} // No pseudoContent map

	pc := sr.GetPseudoContent("any-class")
	if pc != nil {
		t.Errorf("expected nil for nil pseudoContent map, got %+v", pc)
	}

	pc = sr.GetPseudoContentForClass("any-class")
	if pc != nil {
		t.Errorf("expected nil for nil pseudoContent map, got %+v", pc)
	}
}

func TestPseudoContentWarningsIntegration(t *testing.T) {
	log := zap.NewNop()

	// CSS with pseudo-element that has unsupported properties
	cssData := []byte(`
		.link-footnote {
			font-size: 0.8em;
		}
		.link-footnote::before {
			content: "[";
			color: blue;
		}
		.link-footnote::after {
			content: "]";
			font-style: italic;
			text-decoration: underline;
		}
	`)

	registry, warnings := parseAndCreateRegistry(cssData, nil, log)

	// Should have warnings for color, font-style, and text-decoration
	warningCount := 0
	for _, w := range warnings {
		if contains(w, "pseudo-element") && contains(w, "will be ignored") {
			warningCount++
		}
	}

	if warningCount != 3 {
		t.Errorf("expected 3 pseudo-element warnings, got %d (total warnings: %v)", warningCount, warnings)
	}

	// But the content should still be extracted
	pc := registry.GetPseudoContentForClass("link-footnote")
	if pc == nil {
		t.Fatal("expected pseudo content despite warnings")
	}
	if pc.Before != "[" {
		t.Errorf("Before = %q, want %q", pc.Before, "[")
	}
	if pc.After != "]" {
		t.Errorf("After = %q, want %q", pc.After, "]")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
