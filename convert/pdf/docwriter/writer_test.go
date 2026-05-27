package docwriter

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestFormatPrimitives(t *testing.T) {
	tests := []struct {
		name string
		obj  Object
		want string
	}{
		{name: "nil as null", obj: nil, want: "null"},
		{name: "null", obj: Null{}, want: "null"},
		{name: "true", obj: Bool(true), want: "true"},
		{name: "false", obj: Bool(false), want: "false"},
		{name: "integer", obj: Integer(42), want: "42"},
		{name: "number", obj: Number(303.3600), want: "303.36"},
		{name: "name", obj: Name("FlateDecode"), want: "/FlateDecode"},
		{name: "escaped name", obj: Name("A Name/With#Delimiters"), want: "/A#20Name#2FWith#23Delimiters"},
		{name: "hex string", obj: HexString([]byte{0xfe, 0xff, 0x00, 0x41}), want: "<FEFF0041>"},
		{name: "array", obj: Array{Integer(1), Name("Two"), Ref{ObjectNumber: 3}}, want: "[1 /Two 3 0 R]"},
		{name: "dict sorted", obj: Dict{"Type": Name("Page"), "Count": Integer(1)}, want: "<< /Count 1 /Type /Page >>"},
		{name: "ref", obj: Ref{ObjectNumber: 7, Generation: 2}, want: "7 2 R"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(tt.obj); got != tt.want {
				t.Errorf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUTF16TextString(t *testing.T) {
	if got := Format(UTF16TextString("AЖ")); got != "<FEFF00410416>" {
		t.Errorf("UTF16TextString() = %q, want %q", got, "<FEFF00410416>")
	}
}

func TestWriterSerializesStreamsAndXrefOffsets(t *testing.T) {
	w := NewWriter("1.4")
	if err := w.Object(1, Dict{"Pages": Ref{ObjectNumber: 2}, "Type": Name("Catalog")}); err != nil {
		t.Fatalf("object 1: %v", err)
	}
	if err := w.StreamObject(3, Dict{"Filter": Name("FlateDecode")}, []byte("hello")); err != nil {
		t.Fatalf("stream object: %v", err)
	}
	if err := w.Object(2, Dict{"Count": Integer(0), "Kids": Array{}, "Type": Name("Pages")}); err != nil {
		t.Fatalf("object 2: %v", err)
	}

	data, err := w.Finish(Trailer{Root: Ref{ObjectNumber: 1}})
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	text := string(data)

	for _, want := range []string{
		"%PDF-1.4\n",
		"3 0 obj\n<< /Filter /FlateDecode /Length 5 >>\nstream\nhello\nendstream\nendobj\n",
		"xref\n0 4\n",
		"trailer\n<< /Root 1 0 R /Size 4 >>\n",
		"%%EOF\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("PDF does not contain %q", want)
		}
	}

	assertXrefOffsets(t, data, 3)
}

func TestWriterRejectsInvalidObjectState(t *testing.T) {
	w := NewWriter("1.4")
	if err := w.Object(0, Null{}); err == nil {
		t.Fatal("expected invalid object number error")
	}
	if err := w.Object(1, Null{}); err != nil {
		t.Fatalf("object 1: %v", err)
	}
	if err := w.Object(1, Null{}); err == nil {
		t.Fatal("expected duplicate object number error")
	}
	if _, err := w.Finish(Trailer{Root: Ref{ObjectNumber: 1}}); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if err := w.Object(2, Null{}); err == nil {
		t.Fatal("expected closed writer error")
	}
	if _, err := w.Finish(Trailer{Root: Ref{ObjectNumber: 1}}); err == nil {
		t.Fatal("expected second finish error")
	}
}

func assertXrefOffsets(t *testing.T, data []byte, maxObject int) {
	t.Helper()

	matches := regexp.MustCompile(`(?m)^xref\n0 \d+\n((?:\d{10} \d{5} [nf] \n)+)`).FindSubmatch(data)
	if matches == nil {
		t.Fatalf("xref table not found in:\n%s", data)
	}

	lines := bytes.Split(bytes.TrimSuffix(matches[1], []byte("\n")), []byte("\n"))
	if len(lines) < maxObject+1 {
		t.Fatalf("xref has %d entries, want at least %d", len(lines), maxObject+1)
	}

	for id := 1; id <= maxObject; id++ {
		line := string(lines[id])
		offset, err := strconv.Atoi(line[:10])
		if err != nil {
			t.Fatalf("parse xref offset for object %d from %q: %v", id, line, err)
		}
		wantPrefix := fmt.Appendf(nil, "%d 0 obj\n", id)
		if !bytes.HasPrefix(data[offset:], wantPrefix) {
			t.Fatalf("xref offset for object %d = %d, bytes at offset start %q", id, offset, data[offset:min(len(data), offset+20)])
		}
	}
}
