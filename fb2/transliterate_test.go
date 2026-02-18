package fb2

import (
	"sync"
	"testing"
)

func TestTransliterate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Cyrillic title",
			input:    "Война и мир",
			expected: "Voina i mir",
		},
		{
			name:     "Cyrillic author name",
			input:    "Лев Николаевич Толстой",
			expected: "Lev Nikolaevich Tolstoi",
		},
		{
			name:     "All uppercase Cyrillic",
			input:    "ВОЙНА",
			expected: "VOINA",
		},
		{
			name:     "ASCII text unchanged",
			input:    "Test Book",
			expected: "Test Book",
		},
		{
			name:     "Mixed case ASCII",
			input:    "The Great Gatsby",
			expected: "The Great Gatsby",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Single word",
			input:    "Книга",
			expected: "Kniga",
		},
		{
			name:     "Lowercase Cyrillic",
			input:    "война",
			expected: "voina",
		},
		{
			name:     "German umlaut",
			input:    "Günter Grass",
			expected: "Gunter Grass",
		},
		{
			name:     "French accents",
			input:    "Café Résumé",
			expected: "Cafe Resume",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Transliterate(tt.input)
			if result != tt.expected {
				t.Errorf("Transliterate(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestTransliterateRace verifies that concurrent calls to Transliterate and
// Slugify do not race on shared global state. Run with -race to detect.
func TestTransliterateRace(t *testing.T) {
	const goroutines = 10
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Half the goroutines call Transliterate (which needs case-preserved output)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				got := Transliterate("Война и мир")
				if got != "Voina i mir" {
					t.Errorf("Transliterate race: got %q, want %q", got, "Voina i mir")
					return
				}
			}
		}()
	}

	// The other half call Slugify (which expects lowercase output)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				got := Slugify("Война и мир")
				if got != "voina-i-mir" {
					t.Errorf("Slugify race: got %q, want %q", got, "voina-i-mir")
					return
				}
			}
		}()
	}

	wg.Wait()
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Cyrillic title",
			input:    "Война и мир",
			expected: "voina-i-mir",
		},
		{
			name:     "ASCII with spaces",
			input:    "Test Book",
			expected: "test-book",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
