package kfx

import (
	"testing"

	"fbc/css"
)

func TestSelectorStyleName(t *testing.T) {
	tests := []struct {
		name     string
		selector css.Selector
		want     string
	}{
		// Element-only selectors — unchanged
		{
			name:     "element h1",
			selector: css.Selector{Raw: "h1", Element: "h1"},
			want:     "h1",
		},
		{
			name:     "element p",
			selector: css.Selector{Raw: "p", Element: "p"},
			want:     "p",
		},

		// Class-only selectors that DO NOT collide with HTML tags — unchanged
		{
			name:     "class .chapter-title-header",
			selector: css.Selector{Raw: ".chapter-title-header", Class: "chapter-title-header"},
			want:     "chapter-title-header",
		},
		{
			name:     "class .emphasis",
			selector: css.Selector{Raw: ".emphasis", Class: "emphasis"},
			want:     "emphasis",
		},

		// Class-only selectors that collide with HTML tags — prefixed with "."
		{
			name:     "class .h1 collides with element h1",
			selector: css.Selector{Raw: ".h1", Class: "h1"},
			want:     ".h1",
		},
		{
			name:     "class .p collides with element p",
			selector: css.Selector{Raw: ".p", Class: "p"},
			want:     ".p",
		},
		{
			name:     "class .sub collides with element sub",
			selector: css.Selector{Raw: ".sub", Class: "sub"},
			want:     ".sub",
		},
		{
			name:     "class .div collides with element div",
			selector: css.Selector{Raw: ".div", Class: "div"},
			want:     ".div",
		},

		// Element+class selectors — NOT prefixed (element qualifies the class)
		{
			name:     "element+class h1.special",
			selector: css.Selector{Raw: "h1.special", Element: "h1", Class: "special"},
			want:     "special",
		},
		{
			name:     "element+class div.h1 — class h1 qualified by element div",
			selector: css.Selector{Raw: "div.h1", Element: "div", Class: "h1"},
			want:     "h1", // Element is present, so no collision guard
		},

		// Descendant selectors
		{
			name: "descendant p code",
			selector: css.Selector{
				Raw:     "p code",
				Element: "code",
				Ancestor: &css.Selector{
					Raw:     "p",
					Element: "p",
				},
			},
			want: "p--code",
		},
		{
			name: "descendant .h1 code — ancestor class collides",
			selector: css.Selector{
				Raw:     ".h1 code",
				Element: "code",
				Ancestor: &css.Selector{
					Raw:   ".h1",
					Class: "h1",
				},
			},
			want: ".h1--code",
		},

		// Pseudo-element selectors
		{
			name:     "element h1::before",
			selector: css.Selector{Raw: "h1::before", Element: "h1", Pseudo: css.PseudoBefore},
			want:     "h1--before",
		},
		{
			name:     "class .h1::after — collision + pseudo",
			selector: css.Selector{Raw: ".h1::after", Class: "h1", Pseudo: css.PseudoAfter},
			want:     ".h1--after",
		},
		{
			name:     "class .emphasis::before — no collision + pseudo",
			selector: css.Selector{Raw: ".emphasis::before", Class: "emphasis", Pseudo: css.PseudoBefore},
			want:     "emphasis--before",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectorStyleName(tt.selector)
			if got != tt.want {
				t.Errorf("selectorStyleName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSelectorDescendantName(t *testing.T) {
	tests := []struct {
		name     string
		selector css.Selector
		want     string
	}{
		{
			name:     "element only",
			selector: css.Selector{Raw: "code", Element: "code"},
			want:     "code",
		},
		{
			name:     "class only",
			selector: css.Selector{Raw: ".foo", Class: "foo"},
			want:     "foo",
		},
		{
			name:     "element+class",
			selector: css.Selector{Raw: "h2.section-title-header", Element: "h2", Class: "section-title-header"},
			want:     "h2.section-title-header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectorDescendantName(tt.selector)
			if got != tt.want {
				t.Errorf("selectorDescendantName() = %q, want %q", got, tt.want)
			}
		})
	}
}
