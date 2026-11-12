package convert

import (
	"reflect"
	"regexp"
	"testing"

	"go.uber.org/zap"

	"fbc/css"
)

func TestDefaultDropcapMediaProfiles(t *testing.T) {
	// Check preserved CSS too: the compiled-renderer subset does not evaluate sibling selectors.
	followingParagraph := regexp.MustCompile(`p\.has-dropcap\s*\+\s*p\s*\{\s*clear:\s*left;\s*\}`)
	for _, tt := range []struct {
		name   string
		media  css.MediaContext
		native bool
	}{
		{name: "AZW3", media: css.MediaContext{KF8: true}, native: true},
		{name: "KFX and AZW8", media: css.MediaContext{KF8: true, ET: true}},
		{name: "PDF", media: css.MediaContext{KF8: true, ET: true, FBCPDF: true}},
		{name: "EPUB and Kobo", media: css.MediaContext{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, err := css.ResolveMedia(defaultStylesheet, tt.media, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			sheet := css.NewParser(nil).Parse(data, "")
			properties := func(selector string) map[string]string {
				result := make(map[string]string)
				for _, rule := range sheet.RulesBySelector(selector) {
					for name, value := range rule.Properties {
						result[name] = value.Raw
					}
				}
				return result
			}
			wantParagraph := map[string]string{"text-indent": "0", "margin": "0"}
			wantInitial := map[string]string{
				"float": "left", "font-size": "3.2em", "line-height": "1", "font-weight": "bold", "padding-right": "0.1em",
			}
			if tt.native {
				wantParagraph["margin"] = "0 -10pt 0.3em -10pt"
				wantInitial["font-size"] = "4.5em"
				wantInitial["line-height"] = "normal"
				wantInitial["margin-top"] = "-0.3em"
				wantInitial["margin-bottom"] = "-0.45em"
				wantInitial["margin-right"] = "0"
				for _, selector := range []string{"body", "p"} {
					if value, ok := properties(selector)["line-height"]; ok {
						t.Errorf("%s line-height = %q; retain reader-controlled paragraph spacing", selector, value)
					}
				}
			}
			if got := properties("p.has-dropcap"); !reflect.DeepEqual(got, wantParagraph) {
				t.Errorf("drop-cap paragraph properties = %v, want %v", got, wantParagraph)
			}
			if got := properties("p.has-dropcap .dropcap"); !reflect.DeepEqual(got, wantInitial) {
				t.Errorf("drop-cap initial properties = %v, want %v", got, wantInitial)
			}
			if followingParagraph.Match(data) {
				t.Error("do not force the following paragraph below a short opener's initial")
			}
		})
	}
}
