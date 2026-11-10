package azw3

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/css"
	"fbc/state"
)

func prepareGenerationTest(t *testing.T, cfg *config.Config, defaultStyle []byte) (context.Context, *content.Content) {
	t.Helper()
	font, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fonts", "bookerly-regular_9_5.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 20"><path id="native-art" d="M0 0H40V20H0Z" fill="red"/></svg>`
	input := `<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
 <stylesheet type="text/css">
 @media amzn-kf8 and not amzn-et {
  @font-face {font-family: Probe; src: url('#font');} p {font-family: Probe;} .kf8-only {color: navy;}
 }
 @media amzn-et {
  @font-face {font-family: ET; src: url('#et-font.woff2');} .et-only {color: red;}
 }
 @media fbc-pdf { .pdf-only {color: green;} }
 @media amzn-mobi { .mobi-only {color: blue;} }
 </stylesheet>
 <description><title-info><genre>prose</genre><author><nickname>Author</nickname></author>
  <book-title>Generation test</book-title><lang>en</lang></title-info>
  <document-info><id>ce3d5d9a-ceab-4600-818f-f00860cc70d9</id></document-info></description>
 <body><section id="chapter"><title><p>Chapter</p></title>
  <p>Unicode café, 中文, 🙂. A note <a l:href="#note" type="note">17</a>.</p><image l:href="#art"/>
 </section></body><body name="notes"><section id="note"><title><p>17</p></title>
  <p><strong>Rich note</strong> with emphasis.</p></section></body>
 <binary id="font" content-type="font/ttf">` + base64.StdEncoding.EncodeToString(font) + `</binary>
 <binary id="et-font.woff2" content-type="font/woff2">d09GMg==</binary>
 <binary id="art" content-type="image/svg+xml">` + base64.StdEncoding.EncodeToString([]byte(svg)) + `</binary>
</FictionBook>`
	cfg.Document.Vignettes = config.VignettesConfig{}
	ctx := state.ContextWithEnv(t.Context())
	env := state.EnvFromContext(ctx)
	env.Cfg, env.Log, env.DefaultStyle = cfg, zap.NewNop(), defaultStyle
	c, err := content.Prepare(ctx, strings.NewReader(input), "generation.fb2", common.OutputFmtAzw3, env.Log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(c.WorkDir); err != nil {
			t.Error(err)
		}
	})
	return ctx, c
}

func TestGenerateAZW3(t *testing.T) {
	for _, compression := range []string{"none", "standard", "huffdic"} {
		for _, apnxMode := range []string{"none", "app", "eink"} {
			t.Run(compression+"/"+apnxMode, func(t *testing.T) {
				cfg, err := config.LoadConfiguration("")
				if err != nil {
					t.Fatal(err)
				}
				cfg.Document.AZW3.Compression, cfg.Document.AZW3.APNX = compression, apnxMode
				cfg.Document.PageMap.Enable, cfg.Document.PageMap.Size = true, 30
				cfg.Document.Footnotes.Mode = common.FootnotesModeFloatRenumbered
				cfg.Document.Footnotes.LabelTemplate = "label-{{.Format}}-{{.NoteNumber}}"
				cfg.Document.Footnotes.BacklinkTemplate = "back-{{.Format}}"
				cfg.Document.Metainformation.TitleTemplate = "title-{{.Format}}"
				cfg.Document.Metainformation.CreatorNameTemplate = "author-{{.Format}}"
				cfg.Document.TOCPage.AuthorsTemplate = "toc-{{.Format}}"
				cfg.Document.TOCPage.Placement = common.TOCPagePlacementBefore
				ctx, c := prepareGenerationTest(t, cfg, nil)
				c.Debug = true
				c.KindleEbook, c.ASIN = apnxMode == "eink", "B012345678"
				path := filepath.Join(t.TempDir(), "new", "chosen final name.azw3")
				result, err := Generate(ctx, c, path, false, &cfg.Document, zap.NewNop())
				if err != nil {
					t.Fatal(err)
				}
				if result.BookPath != path || c.OutputFormat != common.OutputFmtAzw3 {
					t.Fatal("generation lost its final path or real format identity")
				}
				data, err := os.ReadFile(result.BookPath)
				if err != nil {
					t.Fatal(err)
				}
				if string(data[60:68]) != "BOOKMOBI" || !bytes.Contains(data, []byte("title-azw3")) ||
					!bytes.Contains(data, []byte("author-azw3")) || !bytes.Contains(data, []byte("FONT")) {
					t.Fatal("native container, metadata identity or embedded font is missing")
				}
				record0 := int(binary.BigEndian.Uint32(data[78:]))
				wantCompression := map[string]uint16{"none": 1, "standard": 2, "huffdic": 17480}[compression]
				if binary.BigEndian.Uint16(data[record0:]) != wantCompression || binary.BigEndian.Uint32(data[record0+0x24:]) != 8 {
					t.Fatal("wrong compression or non-KF8 output")
				}
				raw, err := os.ReadFile(filepath.Join(c.WorkDir, "book.rawml"))
				if err != nil {
					t.Fatal(err)
				}
				for _, expected := range []string{
					"label-azw3-1", "back-azw3", "toc-azw3", "<aside", `epub:type="noteref"`,
					"kindle:flow:", "<svg", "native-art", "kf8-only",
				} {
					if !bytes.Contains(raw, []byte(expected)) {
						t.Errorf("native publication is missing %q", expected)
					}
				}
				for _, excluded := range []string{"et-only", "pdf-only", "mobi-only", "et-font.woff2", "@media amzn-"} {
					if bytes.Contains(raw, []byte(excluded)) {
						t.Errorf("non-KF8 CSS survived generation: %s", excluded)
					}
				}
				var pageData []byte
				if apnxMode == "none" {
					if result.APNXPath != "" {
						t.Fatal("APNX was published despite mode none")
					}
				} else {
					if result.APNXPath != outputTestAPNXPath(t, path, apnxMode) {
						t.Fatal("APNX was not placed using the final filename")
					}
					pageData, err = os.ReadFile(result.APNXPath)
					if err != nil {
						t.Fatal(err)
					}
					decoded := parseAPNX(t, pageData)
					kind := "PDOC"
					if c.KindleEbook {
						kind = "EBOK"
					}
					if decoded.Content["asin"] != c.ASIN || decoded.Content["cdeType"] != kind {
						t.Fatal("APNX lost Kindle metadata identity")
					}
				}
				assertNoBundleTemps(t, filepath.Dir(path))
				name := "generated-identity-" + compression + "-" + apnxMode
				artifacts := map[string][]byte{name + ".azw3": data, name + ".rawml": raw}
				if pageData != nil {
					artifacts[name+".apnx"] = pageData
				}
				writeProbeArtifacts(t, artifacts)
				if os.Getenv("AZW3_KINDLEUNPACK_TEST") == "1" {
					work := unpackProbe(t, name, data, pageData)
					assertUnpackedRaw(t, work, raw)
				}
			})
		}
	}
}

func TestGenerateAZW3FailuresPreserveOutputs(t *testing.T) {
	for _, failure := range []string{"compression", "APNX mode", "sidecar collision", "book collision", "canceled", "identity", "path"} {
		t.Run(failure, func(t *testing.T) {
			cfg, err := config.LoadConfiguration("")
			if err != nil {
				t.Fatal(err)
			}
			cfg.Document.AZW3.APNX, cfg.Document.PageMap.Enable = "eink", true
			ctx, c := prepareGenerationTest(t, cfg, nil)
			path := filepath.Join(t.TempDir(), "book.azw3")
			apnxPath := outputTestAPNXPath(t, path, "eink")
			writeOutputTestFile(t, path, []byte("old book"))
			writeOutputTestFile(t, apnxPath, []byte("old APNX"))
			overwrite := true
			switch failure {
			case "compression":
				cfg.Document.AZW3.Compression = "lzma"
			case "APNX mode":
				cfg.Document.AZW3.APNX = "auto"
			case "sidecar collision":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				overwrite = false
			case "book collision":
				overwrite = false
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "identity":
				c.OutputFormat = common.OutputFmtEpub3
			case "path":
				path += string(filepath.Separator)
			}
			result, err := Generate(ctx, c, path, overwrite, &cfg.Document, zap.NewNop())
			if err == nil || result != (Result{}) {
				t.Fatalf("expected generation failure, got %+v, %v", result, err)
			}
			if failure == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation identity was lost", err)
			}
			if failure != "sidecar collision" {
				assertOutputFile(t, filepath.Clean(path), []byte("old book"))
			}
			assertOutputFile(t, apnxPath, []byte("old APNX"))
			assertNoBundleTemps(t, filepath.Dir(filepath.Clean(path)))
		})
	}
}

func TestGenerateAZW3RequiresInputs(t *testing.T) {
	for _, configured := range []bool{false, true} {
		t.Run(fmt.Sprintf("configuration=%t", configured), func(t *testing.T) {
			var cfg *config.DocumentConfig
			if configured {
				cfg = &config.DocumentConfig{}
			}
			if _, err := Generate(t.Context(), nil, "book.azw3", false, cfg, zap.NewNop()); err == nil {
				t.Fatal("missing prepared content was accepted")
			}
		})
	}
}

func TestGenerateAZW3DefaultMedia(t *testing.T) {
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatal(err)
	}
	defaults, err := os.ReadFile(filepath.Join("..", "default.css"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, c := prepareGenerationTest(t, cfg, defaults)
	c.Debug = true
	path := filepath.Join(t.TempDir(), "default-media.azw3")
	if _, err := Generate(ctx, c, path, false, &cfg.Document, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(c.WorkDir, "book.stylesheet.css"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("@media")) || bytes.Contains(data, []byte("et-only")) {
		t.Fatal("default stylesheet media was not selected before native generation")
	}
	sheet := css.NewParser(nil).Parse(data, "")
	var margin string
	for _, rule := range sheet.RulesBySelector("p") {
		if value, found := rule.GetProperty("margin"); found {
			margin = value.Raw
		}
	}
	if margin != "0 -8pt 0.3em -8pt" {
		t.Errorf("KF8-only default paragraph margin = %q", margin)
	}
	more := sheet.RulesBySelector(".footnote-more")
	if len(more) != 1 {
		t.Fatalf("expected one non-ET footnote-more style, got %d", len(more))
	}
	if _, hidden := more[0].GetProperty("display"); hidden {
		t.Fatal("Enhanced Typesetting's hidden footnote-more style leaked into KF8")
	}
	if value, found := more[0].GetProperty("color"); !found || value.Raw != "gray" {
		t.Fatal("KF8 did not retain the visible non-ET footnote-more indicator")
	}
}
