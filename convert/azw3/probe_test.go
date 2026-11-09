package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func stringName(value uint32) string {
	return strconv.FormatUint(uint64(value), 10)
}

func probeLayout() headerLayout {
	return headerLayout{
		Title: "Native KF8 structural probe", UID: 0x01020304, Language: 9,
		TextLength: 1, TextRecords: 1, FirstNonText: 2, Skeletons: 2,
		FirstResource: missingRecord, FDST: missingRecord, FlowCount: 1,
		Navigation: missingRecord, Fragments: missingRecord, Guide: missingRecord,
		Metadata: []exthRecord{
			{ID: 100, Data: []byte("fb2cng")}, {ID: 113, Data: []byte("B000000000")},
			{ID: 501, Data: []byte("PDOC")}, {ID: 524, Data: []byte("en")},
		},
	}
}

// TestStandaloneStructuralProbe is deliberately not a publication compiler: it
// assembles one complete XHTML skeleton with no resources, fragments, TOC or guide.
// AZW3_PROBE_DIR optionally retains native bytes and expected content for independent
// KindleUnpack inspection. Ordinary tests do not require any external compiler/tool.
func TestStandaloneStructuralProbe(t *testing.T) {
	data, xhtml := buildStructuralProbe(t)
	first := binary.BigEndian.Uint32(data[78:])
	if binary.BigEndian.Uint32(data[first+0x24:]) != 8 || bytes.Contains(data, []byte("BOUNDARY")) {
		t.Error("structural probe is not standalone MOBI8")
	}
	writeProbeArtifacts(t, map[string][]byte{"structural.azw3": data, "expected.xhtml": xhtml})
}

func buildStructuralProbe(t *testing.T) ([]byte, []byte) {
	t.Helper()
	xhtml := []byte(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE html>` +
		`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Native KF8</title></head><body>` +
		strings.Repeat(`<p>Native KF8: Кириллица, café, 中文, 🙂 &amp; escaped text.</p>`, 600) + `</body></html>`)
	text, err := splitText(xhtml)
	if err != nil {
		t.Fatal(err)
	}
	index, err := encodeIndex([]indexTag{{ID: 1, Values: 1}, {ID: 6, Values: 2}}, []indexEntry{
		{Key: "SKEL0000000000", Values: map[byte][]uint32{1: {0}, 6: {0, uint32(len(xhtml))}}},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	layout := probeLayout()
	layout.TextLength, layout.TextRecords = uint32(len(xhtml)), uint16(len(text))
	layout.FirstNonText = uint32(1 + len(text))
	layout.Skeletons = layout.FirstNonText
	layout.FirstResource = uint32(1 + len(text) + len(index))
	header, err := encodeHeader(layout)
	if err != nil {
		t.Fatal(err)
	}
	records := append([][]byte{header}, text...)
	records = append(records, index...)
	records = append(records, []byte{0xe9, 0x8e, 0x0d, 0x0a})
	var output bytes.Buffer
	if err := writePalmDB(context.Background(), &output, "Native_KF8_probe", records); err != nil {
		t.Fatal(err)
	}
	return output.Bytes(), xhtml
}

func TestStandaloneKindleUnpack(t *testing.T) {
	if os.Getenv("AZW3_KINDLEUNPACK_TEST") != "1" {
		t.Skip("set AZW3_KINDLEUNPACK_TEST=1 to run the independent decoder")
	}
	data, expected := buildStructuralProbe(t)
	work := t.TempDir()
	input := filepath.Join(work, "structural.azw3")
	if err := os.WriteFile(input, data, 0644); err != nil {
		t.Fatal(err)
	}
	wrapper, err := filepath.Abs("../../testdata/run_kindleunpack.sh")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, wrapper, "-r", "-d", input, filepath.Join(work, "unpacked"))
	command.Dir = work
	output, err := command.CombinedOutput()
	writeProbeArtifacts(t, map[string][]byte{"kindleunpack.log": output})
	if err != nil {
		t.Fatalf("KindleUnpack failed: %v\n%s", err, output)
	}
	for _, line := range strings.Split(string(output), "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "warning") || strings.Contains(lower, "error") || strings.Contains(lower, "traceback") {
			t.Fatalf("unexpected KindleUnpack diagnostic: %s", line)
		}
	}
	for _, name := range []string{"structural.rawml", "OEBPS/Text/part0000.xhtml"} {
		got, err := os.ReadFile(filepath.Join(work, "unpacked", "mobi8", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, expected) {
			t.Errorf("independent reconstruction changed %s", name)
		}
	}
}

func writeProbeArtifacts(t *testing.T, artifacts map[string][]byte) {
	t.Helper()
	if directory := os.Getenv("AZW3_PROBE_DIR"); directory != "" {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		for name, contents := range artifacts {
			if err := os.WriteFile(filepath.Join(directory, name), contents, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
