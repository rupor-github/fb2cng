package main

import (
	"testing"

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
		name       string
		args       []string
		wantSource string
		wantFormat common.OutputFmt
	}{
		{
			name:       "format flag before source",
			args:       []string{"-ow", "--to", "md", "/mnt/d/test/_Test.fb2", "/mnt/d"},
			wantSource: "/mnt/d/test/_Test.fb2",
			wantFormat: common.OutputFmtMd,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSource, gotFormat := preparseConvertInvocation(tt.args)
			if gotSource != tt.wantSource || gotFormat != tt.wantFormat {
				t.Fatalf("preparseConvertInvocation() = (%q, %s), want (%q, %s)", gotSource, gotFormat, tt.wantSource, tt.wantFormat)
			}
		})
	}
}
