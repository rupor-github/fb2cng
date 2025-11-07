package debug

import (
	"fmt"
	"strconv"
	"strings"
)

type TreeWriter struct {
	w *strings.Builder
}

func NewTreeWriter() *TreeWriter {
	return &TreeWriter{
		w: &strings.Builder{},
	}
}

func (tw TreeWriter) String() string {
	return tw.w.String()
}

func (tw TreeWriter) Line(depth int, format string, args ...any) {
	for range depth {
		tw.w.WriteString("  ")
	}
	fmt.Fprintf(tw.w, format, args...)
	tw.w.WriteByte('\n')
}

func (tw TreeWriter) TextBlock(depth int, label, value string) {
	for range depth {
		tw.w.WriteString("  ")
	}
	tw.w.WriteString(label)
	tw.w.WriteString(": ")
	tw.w.WriteString(encodeText(value))
	tw.w.WriteByte('\n')
}

func encodeText(raw string) string {
	if raw == "" {
		return raw
	}
	return strconv.Quote(raw)
}
