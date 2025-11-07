package debug

import (
	"strings"
	"testing"
)

func TestNewTreeWriter(t *testing.T) {
	tw := NewTreeWriter()
	if tw == nil {
		t.Fatal("NewTreeWriter() returned nil")
	}
	if tw.w == nil {
		t.Error("TreeWriter builder is nil")
	}
}

func TestTreeWriter_String(t *testing.T) {
	tw := NewTreeWriter()
	if tw.String() != "" {
		t.Error("Expected empty string from new TreeWriter")
	}

	tw.w.WriteString("test content")
	if tw.String() != "test content" {
		t.Errorf("String() = %q, want %q", tw.String(), "test content")
	}
}

func TestTreeWriter_Line(t *testing.T) {
	tests := []struct {
		name   string
		depth  int
		format string
		args   []any
		want   string
	}{
		{
			name:   "no depth",
			depth:  0,
			format: "test",
			args:   nil,
			want:   "test\n",
		},
		{
			name:   "depth 1",
			depth:  1,
			format: "indented",
			args:   nil,
			want:   "  indented\n",
		},
		{
			name:   "depth 2",
			depth:  2,
			format: "double indent",
			args:   nil,
			want:   "    double indent\n",
		},
		{
			name:   "with formatting",
			depth:  1,
			format: "value: %d",
			args:   []any{42},
			want:   "  value: 42\n",
		},
		{
			name:   "multiple args",
			depth:  0,
			format: "%s = %d",
			args:   []any{"count", 5},
			want:   "count = 5\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tw := NewTreeWriter()
			tw.Line(tt.depth, tt.format, tt.args...)
			got := tw.String()
			if got != tt.want {
				t.Errorf("Line() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTreeWriter_TextBlock(t *testing.T) {
	tests := []struct {
		name  string
		depth int
		label string
		value string
		want  string
	}{
		{
			name:  "no depth empty value",
			depth: 0,
			label: "field",
			value: "",
			want:  "field: \n",
		},
		{
			name:  "no depth with value",
			depth: 0,
			label: "text",
			value: "hello world",
			want:  "text: \"hello world\"\n",
		},
		{
			name:  "depth 1 with value",
			depth: 1,
			label: "content",
			value: "test",
			want:  "  content: \"test\"\n",
		},
		{
			name:  "depth 2 with value",
			depth: 2,
			label: "nested",
			value: "data",
			want:  "    nested: \"data\"\n",
		},
		{
			name:  "value with quotes",
			depth: 0,
			label: "quoted",
			value: "he said \"hello\"",
			want:  "quoted: \"he said \\\"hello\\\"\"\n",
		},
		{
			name:  "value with newline",
			depth: 0,
			label: "multiline",
			value: "line1\nline2",
			want:  "multiline: \"line1\\nline2\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tw := NewTreeWriter()
			tw.TextBlock(tt.depth, tt.label, tt.value)
			got := tw.String()
			if got != tt.want {
				t.Errorf("TextBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncodeText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "simple text",
			input: "hello",
			want:  `"hello"`,
		},
		{
			name:  "with spaces",
			input: "hello world",
			want:  `"hello world"`,
		},
		{
			name:  "with quotes",
			input: `say "hi"`,
			want:  `"say \"hi\""`,
		},
		{
			name:  "with newline",
			input: "line1\nline2",
			want:  `"line1\nline2"`,
		},
		{
			name:  "with tab",
			input: "col1\tcol2",
			want:  `"col1\tcol2"`,
		},
		{
			name:  "with backslash",
			input: `path\to\file`,
			want:  `"path\\to\\file"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeText(tt.input)
			if got != tt.want {
				t.Errorf("encodeText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTreeWriter_MultipleOperations(t *testing.T) {
	tw := NewTreeWriter()
	tw.Line(0, "Root")
	tw.Line(1, "Child 1")
	tw.TextBlock(2, "field", "value")
	tw.Line(1, "Child 2")
	tw.TextBlock(1, "data", "test")

	got := tw.String()
	want := "Root\n  Child 1\n    field: \"value\"\n  Child 2\n  data: \"test\"\n"

	if got != want {
		t.Errorf("Multiple operations:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestTreeWriter_ComplexTree(t *testing.T) {
	tw := NewTreeWriter()
	tw.Line(0, "document")
	tw.TextBlock(1, "title", "My Document")
	tw.Line(1, "sections")
	tw.Line(2, "section id=%d", 1)
	tw.TextBlock(3, "name", "Introduction")
	tw.TextBlock(3, "content", "This is the intro")
	tw.Line(2, "section id=%d", 2)
	tw.TextBlock(3, "name", "Body")

	result := tw.String()
	if !strings.Contains(result, "document\n") {
		t.Error("Missing document line")
	}
	if !strings.Contains(result, "  title: \"My Document\"\n") {
		t.Error("Missing title line")
	}
	if !strings.Contains(result, "    section id=1\n") {
		t.Error("Missing section 1 line")
	}
	if !strings.Contains(result, "      name: \"Introduction\"\n") {
		t.Error("Missing name line")
	}
}
