package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	cli "github.com/urfave/cli/v3"

	"fbc/common"
)

func TestPreparseInvocationCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantName  string
		wantIndex int
	}{
		{
			name:      "global flags before command",
			args:      []string{"-d", "-c", "build/test.yaml", "convert", "book.fb2"},
			wantName:  "convert",
			wantIndex: 3,
		},
		{
			name:      "equals config flag",
			args:      []string{"--config=build/test.yaml", "dumpconfig"},
			wantName:  "dumpconfig",
			wantIndex: 1,
		},
		{
			name:      "no command",
			args:      []string{"--debug", "--config", "build/test.yaml"},
			wantName:  "",
			wantIndex: -1,
		},
		{
			name:     "single dash long config",
			args:     []string{"-config", "build/test.conf", "-debug", "convert", "book.fb2"},
			wantName: "convert", wantIndex: 3,
		},
		{
			name:     "double dash config alias",
			args:     []string{"--c", "build/test.conf", "convert", "book.fb2"},
			wantName: "convert", wantIndex: 2,
		},
		{
			name:     "equals single dash long config",
			args:     []string{"-config=build/test.conf", "convert", "book.fb2"},
			wantName: "convert", wantIndex: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotIndex := preparseInvocationCommand(tt.args)
			if gotName != tt.wantName || gotIndex != tt.wantIndex {
				t.Fatalf("preparseInvocationCommand() = (%q, %d), want (%q, %d)", gotName, gotIndex, tt.wantName, tt.wantIndex)
			}
		})
	}
}

func TestPreparseConvertInvocation(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantSource    string
		wantFormat    common.OutputFmt
		wantOutputDir string
	}{
		{
			name:          "format flag before source",
			args:          []string{"-ow", "--to", "md", "/mnt/d/test/_Test.fb2", "/mnt/d"},
			wantSource:    "/mnt/d/test/_Test.fb2",
			wantFormat:    common.OutputFmtMd,
			wantOutputDir: "/mnt/d",
		},
		{
			name:       "equals format flag",
			args:       []string{"--to=pdf", "book.fb2"},
			wantSource: "book.fb2",
			wantFormat: common.OutputFmtPdf,
		},
		{
			name:       "skip value flags",
			args:       []string{"--asin", "ABCDEFGHIJ", "--output-file", "out.md", "book.fb2"},
			wantSource: "book.fb2",
			wantFormat: common.OutputFmtEpub2,
		},
		{
			name:       "explicit argument separator",
			args:       []string{"--to", "txt", "--", "-odd-name.fb2"},
			wantSource: "-odd-name.fb2",
			wantFormat: common.OutputFmtTxt,
		},
		{
			name:          "compression before format",
			args:          []string{"--kfx-compression", "lzma", "--to", "azw8", "/mnt/d/test/book.fb2", "/mnt/d/output"},
			wantSource:    "/mnt/d/test/book.fb2",
			wantFormat:    common.OutputFmtAzw8,
			wantOutputDir: "/mnt/d/output",
		},
		{
			name:       "compression after format",
			args:       []string{"--to", "kfx", "--kfx-compression", "none", "book.fb2"},
			wantSource: "book.fb2",
			wantFormat: common.OutputFmtKfx,
		},
		{
			name:       "equals compression flag",
			args:       []string{"--kfx-compression=lzma", "--to=azw8", "book.fb2"},
			wantSource: "book.fb2",
			wantFormat: common.OutputFmtAzw8,
		},
		{
			name:       "exact output file",
			args:       []string{"--output-file", "/tmp/my books/result.kfx", "--kfx-compression", "lzma", "book.fb2"},
			wantSource: "book.fb2", wantFormat: common.OutputFmtEpub2, wantOutputDir: filepath.FromSlash("/tmp/my books"),
		},
		{
			name:       "equals exact output file",
			args:       []string{"--to=azw8", "--output-file=results/result.azw8", "book.fb2"},
			wantSource: "book.fb2", wantFormat: common.OutputFmtAzw8, wantOutputDir: "results",
		},
		{
			name:       "short exact output file",
			args:       []string{"-o", "results/result.epub", "book.fb2"},
			wantSource: "book.fb2", wantFormat: common.OutputFmtEpub2, wantOutputDir: "results",
		},
		{
			name:       "equals short exact output file",
			args:       []string{"-o=results/result.epub", "book.fb2"},
			wantSource: "book.fb2", wantFormat: common.OutputFmtEpub2, wantOutputDir: "results",
		},
		{
			name:       "destination after separator",
			args:       []string{"--to=txt", "--", "-odd-name.fb2", "results"},
			wantSource: "-odd-name.fb2", wantFormat: common.OutputFmtTxt, wantOutputDir: "results",
		},
		{
			name:       "equals value flag does not consume source",
			args:       []string{"--asin=ABCDEFGHIJ", "--force-zip-cp=windows-1251", "book.fb2", "results"},
			wantSource: "book.fb2", wantFormat: common.OutputFmtEpub2, wantOutputDir: "results",
		},
		{
			name:       "exact output file after source",
			args:       []string{"--to", "txt", "book.fb2", "--output-file", "results/chosen.txt"},
			wantSource: "book.fb2", wantFormat: common.OutputFmtTxt, wantOutputDir: "results",
		},
		{
			name:       "equals output alias after source",
			args:       []string{"book.fb2", "--to=azw8", "-o=results/chosen.azw8"},
			wantSource: "book.fb2", wantFormat: common.OutputFmtAzw8, wantOutputDir: "results",
		},
		{
			name:       "value flags between source and destination",
			args:       []string{"book.fb2", "--to", "kfx", "--kfx-compression", "lzma", "results"},
			wantSource: "book.fb2", wantFormat: common.OutputFmtKfx, wantOutputDir: "results",
		},
		{
			name:       "boolean flags between source and destination",
			args:       []string{"book.fb2", "--overwrite", "results", "--to=pdf", "--no-cover"},
			wantSource: "book.fb2", wantFormat: common.OutputFmtPdf, wantOutputDir: "results",
		},
		{
			name:       "separator after source keeps earlier positional",
			args:       []string{"book.fb2", "--to=txt", "--", "--literal-destination"},
			wantSource: "book.fb2", wantFormat: common.OutputFmtTxt, wantOutputDir: "--literal-destination",
		},
		{
			name:       "last format wins after positionals",
			args:       []string{"--to=kfx", "book.fb2", "results", "--to=azw8"},
			wantSource: "book.fb2", wantFormat: common.OutputFmtAzw8, wantOutputDir: "results",
		},
		{
			name:       "invalid last format uses conversion fallback",
			args:       []string{"--to=kfx", "book.fb2", "results", "--to=invalid"},
			wantSource: "book.fb2", wantFormat: common.OutputFmtEpub2, wantOutputDir: "results",
		},
		{
			name:       "single dash is positional",
			args:       []string{"-", "--to=txt", "results"},
			wantSource: "-", wantFormat: common.OutputFmtTxt, wantOutputDir: "results",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSource, gotFormat, gotOutputDir := preparseConvertInvocation(tt.args)
			wantOutputDir := tt.wantOutputDir
			if wantOutputDir == "" {
				wantOutputDir = "."
			}
			if gotSource != tt.wantSource || gotFormat != tt.wantFormat || gotOutputDir != wantOutputDir {
				t.Fatalf("preparseConvertInvocation() = (%q, %s, %q), want (%q, %s, %q)",
					gotSource, gotFormat, gotOutputDir, tt.wantSource, tt.wantFormat, wantOutputDir)
			}
		})
	}
}

func TestArtifactTemplateValuesLongFlagNames(t *testing.T) {
	root := &cli.Command{Name: "fbc"}
	values := artifactTemplateValues(time.Now(), root, []string{
		"-config", "build/test.conf", "-debug", "convert", "-to", "azw8",
		"-output-file", "results/chosen.azw8", "-kfx-compression", "lzma", "book.fb2",
	})
	if values.Command != "convert" || values.Source != "book.fb2" ||
		values.Format != common.OutputFmtAzw8 || values.OutputDir != "results" {
		t.Fatalf("long-name flag context: %+v", values)
	}
}

func TestArtifactTemplateValuesOutputDir(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		wantDir string
	}{
		{"destination", []string{"book.fb2", "results"}, "results"},
		{"exact file", []string{"--output-file", "exact/result.azw8", "book.fb2"}, "exact"},
		{"default", []string{"book.fb2"}, "."},
		{"exact file after source", []string{"book.fb2", "--output-file", "exact/result.azw8"}, "exact"},
		{"equals alias after source", []string{"book.fb2", "-o=exact/result.azw8"}, "exact"},
		{"value flag between positionals", []string{"book.fb2", "--kfx-compression", "lzma", "results"}, "results"},
		{"format between positionals", []string{"book.fb2", "--to", "azw8", "results"}, "results"},
		{"boolean between positionals", []string{"book.fb2", "--overwrite", "results"}, "results"},
		{"separator after source", []string{"book.fb2", "--", "--literal-destination"}, "--literal-destination"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := &cli.Command{Name: "fbc"}
			args := append([]string{"-d", "convert", "--to", "azw8", "--kfx-compression", "lzma"}, test.args...)
			values := artifactTemplateValues(time.Now(), root, args)
			if values.OutputDir != test.wantDir || values.Source != "book.fb2" || values.Format != common.OutputFmtAzw8 {
				t.Fatalf("root context: %+v", values)
			}
			app := &cli.Command{Name: "fbc", Commands: []*cli.Command{{
				Name: "convert", Flags: []cli.Flag{
					&cli.StringFlag{Name: "to"}, &cli.StringFlag{Name: "output-file", Aliases: []string{"o"}},
					&cli.StringFlag{Name: "kfx-compression"}, &cli.BoolFlag{Name: "overwrite"},
				},
				Action: func(_ context.Context, cmd *cli.Command) error {
					values := artifactTemplateValues(time.Now(), cmd, nil)
					if values.OutputDir == "" {
						values.OutputDir = "."
					}
					if values.OutputDir != test.wantDir || values.Source != "book.fb2" || values.Format != common.OutputFmtAzw8 {
						t.Fatalf("parsed context: %+v", values)
					}
					return nil
				},
			}}}
			args = append([]string{"fbc", "convert", "--to", "azw8"}, test.args...)
			if err := app.Run(t.Context(), args); err != nil {
				t.Fatal(err)
			}
		})
	}
}
