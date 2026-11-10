package convert

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cli "github.com/urfave/cli/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"fbc/config"
)

const azw3IntegrationID = "bdd401d4-f999-4a19-b69d-54a86631bfe7"

const azw3IntegrationInput = `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
 <description><title-info><book-title>AZW3 test</book-title><lang>en</lang></title-info>
 <document-info><id>` + azw3IntegrationID + `</id></document-info></description>
 <body><section id="chapter"><title><p>Chapter</p></title><p>Unicode café, 中文, 🙂.</p></section></body>
</FictionBook>`

func azw3Command() *cli.Command {
	return &cli.Command{
		Name: "convert", Action: Run,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "to"}, &cli.StringFlag{Name: "output-file"},
			&cli.BoolFlag{Name: "ebook"}, &cli.StringFlag{Name: "asin"},
			&cli.BoolFlag{Name: "overwrite"}, &cli.StringFlag{Name: "compression"},
		},
	}
}

func readAZW3TestHeader(t *testing.T, data []byte) ([]byte, map[uint32][]byte) {
	t.Helper()
	if len(data) < 94 || string(data[60:68]) != "BOOKMOBI" {
		t.Fatal("output is not a native PalmDB book")
	}
	start, end := int(binary.BigEndian.Uint32(data[78:])), int(binary.BigEndian.Uint32(data[86:]))
	if start < 94 || end < start+280 || end > len(data) {
		t.Fatal("invalid native book record directory")
	}
	header := data[start:end]
	if string(header[16:20]) != "MOBI" || binary.BigEndian.Uint32(header[0x24:]) != 8 {
		t.Fatal("output is not standalone KF8")
	}
	at := 16 + int(binary.BigEndian.Uint32(header[20:]))
	if at+12 > len(header) || string(header[at:at+4]) != "EXTH" {
		t.Fatal("native metadata is missing")
	}
	count := binary.BigEndian.Uint32(header[at+8:])
	at += 12
	fields := make(map[uint32][]byte)
	for range count {
		if at+8 > len(header) {
			t.Fatal("truncated EXTH field")
		}
		tag, size := binary.BigEndian.Uint32(header[at:]), int(binary.BigEndian.Uint32(header[at+4:]))
		if size < 8 || at+size > len(header) {
			t.Fatal("invalid EXTH field length")
		}
		fields[tag] = header[at+8 : at+size]
		at += size
	}
	return header, fields
}

func TestRunAZW3(t *testing.T) {
	for _, compression := range []string{"none", "standard", "huffdic"} {
		for _, mode := range []string{"none", "app", "eink"} {
			t.Run(compression+"/"+mode, func(t *testing.T) {
				ctx, env := setupTestEnv(t)
				core, logs := observer.New(zap.DebugLevel)
				env.Log = zap.New(core)
				env.Cfg.Document.AZW3.Compression, env.Cfg.Document.AZW3.APNX = compression, mode
				env.Cfg.Document.PageMap.Enable = true
				env.Cfg.Document.Metainformation.TitleTemplate = "meta-{{.Format}}"
				env.Cfg.Document.Images.Screen.Width, env.Cfg.Document.Images.Screen.Height = 200, 300
				root := t.TempDir()
				source := filepath.Join(root, "input.fb2")
				output := filepath.Join(root, "out", "chosen final.azw3")
				if err := os.WriteFile(source, []byte(azw3IntegrationInput), 0644); err != nil {
					t.Fatal(err)
				}
				reporter := config.ReporterConfig{DestinationTemplate: filepath.Join(root, "report.zip")}
				var err error
				env.Rpt, err = reporter.Prepare()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if env.Rpt != nil {
						if err := env.Rpt.Close(); err != nil {
							t.Error(err)
						}
					}
				})
				args := []string{"convert", "--to", "azw3", "--output-file", output, "--asin", "b012345678"}
				kind := "PDOC"
				if mode == "eink" {
					args, kind = append(args, "--ebook"), "EBOK"
				}
				args = append(args, source)
				if err := azw3Command().Run(ctx, args); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				header, fields := readAZW3TestHeader(t, data)
				wantCompression := map[string]uint16{"none": 1, "standard": 2, "huffdic": 17480}[compression]
				if binary.BigEndian.Uint16(header) != wantCompression || string(fields[503]) != "meta-azw3" ||
					string(fields[113]) != "B012345678" || string(fields[504]) != "B012345678" || string(fields[501]) != kind {
					t.Fatal("CLI dispatch lost native compression or format/Kindle metadata identity")
				}
				if len(fields[201]) != 4 || len(fields[202]) != 4 {
					t.Fatal("Kindle cover and thumbnail policy was not applied to AZW3")
				}
				starts := logs.FilterMessage("AZW3 generation starting").All()
				if len(starts) != 1 || starts[0].ContextMap()["output"] != output || starts[0].ContextMap()["format"] != "azw3" {
					t.Fatal("native generation did not run once using the final filename and AZW3 identity")
				}
				if logs.FilterMessage("KFX compression selected").Len() != 0 || logs.FilterMessage("EPUB generation starting").Len() != 0 {
					t.Fatal("AZW3 was routed through an unrelated output adapter")
				}
				var pageData []byte
				if mode != "none" {
					stem := strings.TrimSuffix(filepath.Base(output), ".azw3")
					parent := filepath.Dir(output)
					if mode == "eink" {
						parent = filepath.Join(parent, stem+".sdr")
					}
					pageData, err = os.ReadFile(filepath.Join(parent, stem+".apnx"))
					if err != nil {
						t.Fatal(err)
					}
					length := int(binary.BigEndian.Uint32(pageData[8:]))
					var identity map[string]string
					if err := json.Unmarshal(pageData[12:12+length], &identity); err != nil {
						t.Fatal(err)
					}
					if identity["cdeType"] != kind || identity["asin"] != "B012345678" ||
						identity["contentGuid"] != fmt.Sprintf("%08x", binary.BigEndian.Uint32(header[0x20:])) {
						t.Fatal("published APNX does not identify the published book")
					}
				}
				if err := env.Rpt.Close(); err != nil {
					t.Fatal(err)
				}
				env.Rpt = nil
				report, err := zip.OpenReader(reporter.Destination())
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := report.Close(); err != nil {
						t.Error(err)
					}
				}()
				foundBook, foundAPNX, foundSummary := false, false, false
				for _, file := range report.File {
					if file.Name == "result-"+azw3IntegrationID+".azw3" || file.Name == "result-"+azw3IntegrationID+".apnx" ||
						strings.HasSuffix(file.Name, "/book.azw3.txt") {
						r, err := file.Open()
						if err != nil {
							t.Fatal(err)
						}
						payload, err := io.ReadAll(r)
						if err := errors.Join(err, r.Close()); err != nil {
							t.Fatal(err)
						}
						switch {
						case strings.HasSuffix(file.Name, ".azw3"):
							foundBook = bytes.Equal(payload, data)
						case strings.HasSuffix(file.Name, ".apnx"):
							foundAPNX = pageData != nil && bytes.Equal(payload, pageData)
						default:
							foundSummary = bytes.Contains(payload, []byte("format=azw3\n")) &&
								bytes.Contains(payload, []byte("compression_source=default\n"))
						}
					}
				}
				if !foundBook || foundAPNX != (mode != "none") || !foundSummary {
					t.Fatalf("report contents: book=%t, APNX=%t, summary=%t", foundBook, foundAPNX, foundSummary)
				}
			})
		}
	}
}

func TestRunAZW3CompressionOverride(t *testing.T) {
	for _, test := range []struct {
		mode string
		kind uint16
	}{
		{"none", 1}, {"standard", 2}, {"huffdic", 17480},
	} {
		t.Run(test.mode, func(t *testing.T) {
			ctx, env := setupTestEnv(t)
			core, logs := observer.New(zap.DebugLevel)
			env.Log = zap.New(core)
			env.Cfg.Document.AZW3.Compression = "huffdic"
			if test.mode == "huffdic" {
				env.Cfg.Document.AZW3.Compression = "none"
			}
			env.Cfg.Document.AZW3.APNX = "app"
			env.Cfg.Document.PageMap.Enable = true
			env.Cfg.Document.Images.Screen.Width, env.Cfg.Document.Images.Screen.Height = 200, 300
			root := t.TempDir()
			source, output := filepath.Join(root, "input.fb2"), filepath.Join(root, "chosen.azw3")
			if err := os.WriteFile(source, []byte(azw3IntegrationInput), 0644); err != nil {
				t.Fatal(err)
			}
			args := []string{"convert", "--to", "azw3", "--compression", test.mode, "--output-file", output, source}
			if err := azw3Command().Run(ctx, args); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			header, _ := readAZW3TestHeader(t, data)
			if binary.BigEndian.Uint16(header) != test.kind || env.Cfg.Document.AZW3.CompressionSource != "cli" {
				t.Fatal("native output did not use the CLI compression override")
			}
			if _, err := os.Stat(filepath.Join(root, "chosen.apnx")); err != nil {
				t.Fatal("compression override changed config-only APNX publication", err)
			}
			entries := logs.FilterMessage("AZW3 compression selected").All()
			if len(entries) != 1 || entries[0].ContextMap()["compression"] != test.mode || entries[0].ContextMap()["source"] != "cli" {
				t.Fatal("missing native CLI compression/source diagnostic", entries)
			}
		})
	}
}

func TestRunAZW3OutputTemplate(t *testing.T) {
	ctx, env := setupTestEnv(t)
	env.Cfg.Document.OutputNameTemplate = "{{.Format}}-{{.Title}}"
	env.Cfg.Document.Images.Screen.Width, env.Cfg.Document.Images.Screen.Height = 200, 300
	root := t.TempDir()
	source, destination := filepath.Join(root, "source.fb2"), filepath.Join(root, "output")
	if err := os.WriteFile(source, []byte(azw3IntegrationInput), 0644); err != nil {
		t.Fatal(err)
	}
	if err := azw3Command().Run(ctx, []string{"convert", "--to", "AZW3", source, destination}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "azw3-AZW3 test.azw3"))
	if err != nil {
		t.Fatal("output name template did not receive native format identity", err)
	}
	readAZW3TestHeader(t, data)
}

func TestRunAZW3FailurePreservesBundle(t *testing.T) {
	for _, failure := range []string{"book exists", "sidecar exists", "generation", "canceled"} {
		t.Run(failure, func(t *testing.T) {
			ctx, env := setupTestEnv(t)
			env.Cfg.Document.AZW3.APNX, env.Cfg.Document.PageMap.Enable = "app", true
			env.Cfg.Document.Images.Screen.Width, env.Cfg.Document.Images.Screen.Height = 200, 300
			root := t.TempDir()
			source, output, apnx := filepath.Join(root, "input.fb2"), filepath.Join(root, "book.azw3"), filepath.Join(root, "book.apnx")
			if err := os.WriteFile(source, []byte(azw3IntegrationInput), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(apnx, []byte("old APNX"), 0644); err != nil {
				t.Fatal(err)
			}
			if failure != "sidecar exists" {
				if err := os.WriteFile(output, []byte("old book"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"convert", "--to", "azw3", "--output-file", output}
			switch failure {
			case "generation":
				args = append(args, "--overwrite")
				env.Cfg.Document.AZW3.Compression = "lzma"
			case "canceled":
				args = append(args, "--overwrite")
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()
				env.Log = env.Log.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
					if entry.Message == "AZW3 generation starting" {
						cancel()
					}
					return nil
				}))
			}
			args = append(args, source)
			err := azw3Command().Run(ctx, args)
			if err == nil || failure == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("expected conversion failure, got %v", err)
			}
			if failure != "sidecar exists" {
				data, err := os.ReadFile(output)
				if err != nil || string(data) != "old book" {
					t.Fatal("failed conversion changed the existing book", err)
				}
			} else if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("sidecar collision left a partially published book")
			}
			data, err := os.ReadFile(apnx)
			if err != nil || string(data) != "old APNX" {
				t.Fatal("failed conversion changed the existing APNX", err)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".fbc-") {
					t.Fatalf("staging file left behind: %s", entry.Name())
				}
			}
		})
	}
}

func TestRunAZW3OverwriteAndAPNXSuppression(t *testing.T) {
	for _, pages := range []bool{false, true} {
		t.Run(fmt.Sprintf("page-map=%t", pages), func(t *testing.T) {
			ctx, env := setupTestEnv(t)
			env.Cfg.Document.AZW3.APNX, env.Cfg.Document.PageMap.Enable = "app", pages
			env.Cfg.Document.Images.Screen.Width, env.Cfg.Document.Images.Screen.Height = 200, 300
			core, logs := observer.New(zap.DebugLevel)
			env.Log = zap.New(core)
			root := t.TempDir()
			source, output, apnx := filepath.Join(root, "input.fb2"), filepath.Join(root, "book.azw3"), filepath.Join(root, "book.apnx")
			for path, data := range map[string]string{source: azw3IntegrationInput, output: "old book", apnx: "old APNX"} {
				if err := os.WriteFile(path, []byte(data), 0644); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"convert", "--to", "azw3", "--overwrite", "--output-file", output, source}
			if err := azw3Command().Run(ctx, args); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			readAZW3TestHeader(t, data)
			pageData, err := os.ReadFile(apnx)
			if err != nil {
				t.Fatal(err)
			}
			if replaced := string(pageData) != "old APNX"; replaced != pages {
				t.Fatal("requested APNX was not replaced, or suppressed APNX was touched")
			}
			wantWarnings := 0
			if !pages {
				wantWarnings = 1
			}
			if logs.FilterMessage("APNX requested but page maps are disabled; omitting sidecar").Len() != wantWarnings {
				t.Fatal("APNX suppression warning was lost")
			}
		})
	}
}
