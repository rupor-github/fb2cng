package docwriter

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Object is a serializable PDF object primitive.
type Object interface {
	writePDF(*bytes.Buffer)
}

// Raw is preformatted PDF syntax. Use it only at package boundaries where a
// higher-level primitive is not available yet.
type Raw string

func (r Raw) writePDF(buf *bytes.Buffer) {
	buf.WriteString(string(r))
}

// Null is the PDF null object.
type Null struct{}

func (Null) writePDF(buf *bytes.Buffer) {
	buf.WriteString("null")
}

// Bool is a PDF boolean object.
type Bool bool

func (b Bool) writePDF(buf *bytes.Buffer) {
	if b {
		buf.WriteString("true")
		return
	}
	buf.WriteString("false")
}

// Number is a PDF numeric object.
type Number float64

func (n Number) writePDF(buf *bytes.Buffer) {
	buf.WriteString(FormatNumber(float64(n)))
}

// Integer is a PDF integer object.
type Integer int

func (i Integer) writePDF(buf *bytes.Buffer) {
	buf.WriteString(strconv.Itoa(int(i)))
}

// Name is a PDF name object without the leading slash.
type Name string

func (n Name) writePDF(buf *bytes.Buffer) {
	buf.WriteByte('/')
	writeName(buf, string(n))
}

// HexString is a PDF hexadecimal string object.
type HexString []byte

func (h HexString) writePDF(buf *bytes.Buffer) {
	buf.WriteByte('<')
	enc := make([]byte, hex.EncodedLen(len(h)))
	hex.Encode(enc, h)
	buf.Write(bytes.ToUpper(enc))
	buf.WriteByte('>')
}

// Array is a PDF array object.
type Array []Object

func (a Array) writePDF(buf *bytes.Buffer) {
	buf.WriteByte('[')
	for i, obj := range a {
		if i > 0 {
			buf.WriteByte(' ')
		}
		writeObject(buf, obj)
	}
	buf.WriteByte(']')
}

// Dict is a PDF dictionary object. Keys are PDF names without leading slashes.
type Dict map[string]Object

func (d Dict) writePDF(buf *bytes.Buffer) {
	if len(d) == 0 {
		buf.WriteString("<< >>")
		return
	}

	keys := make([]string, 0, len(d))
	for key := range d {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	buf.WriteString("<<")
	for _, key := range keys {
		buf.WriteByte(' ')
		Name(key).writePDF(buf)
		buf.WriteByte(' ')
		writeObject(buf, d[key])
	}
	buf.WriteString(" >>")
}

// Ref is an indirect object reference.
type Ref struct {
	ObjectNumber int
	Generation   int
}

func (r Ref) writePDF(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "%d %d R", r.ObjectNumber, r.Generation)
}

// UTF16TextString returns a UTF-16BE hexadecimal PDF string with BOM.
func UTF16TextString(s string) HexString {
	words := utf16.Encode([]rune(s))
	data := make([]byte, 2, 2+len(words)*2)
	data[0] = 0xfe
	data[1] = 0xff
	for _, word := range words {
		data = append(data, byte(word>>8), byte(word))
	}
	return HexString(data)
}

// Format serializes one object into PDF syntax.
func Format(obj Object) string {
	var buf bytes.Buffer
	writeObject(&buf, obj)
	return buf.String()
}

// FormatNumber formats a number using a stable compact decimal form.
func FormatNumber(n float64) string {
	formatted := strconv.FormatFloat(n, 'f', 4, 64)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	if formatted == "-0" {
		return "0"
	}
	return formatted
}

type Trailer struct {
	Root Ref
	Info *Ref
}

// Writer serializes indirect objects into a PDF document with a classic xref
// table and trailer.
type Writer struct {
	version string
	buf     bytes.Buffer
	offsets map[int]int
	closed  bool
}

// NewWriter creates a PDF writer and writes the file header immediately.
func NewWriter(version string) *Writer {
	if version == "" {
		version = "1.4"
	}
	w := &Writer{
		version: version,
		offsets: make(map[int]int),
	}
	fmt.Fprintf(&w.buf, "%%PDF-%s\n", w.version)
	// Binary marker recommended by the PDF specification.
	w.buf.WriteString("%\xE2\xE3\xCF\xD3\n")
	return w
}

// Object writes an indirect object.
func (w *Writer) Object(id int, obj Object) error {
	if err := w.beginObject(id); err != nil {
		return err
	}
	writeObject(&w.buf, obj)
	w.buf.WriteString("\nendobj\n")
	return nil
}

// StreamObject writes an indirect stream object. /Length is added or replaced
// with the byte length of data.
func (w *Writer) StreamObject(id int, dict Dict, data []byte) error {
	if err := w.beginObject(id); err != nil {
		return err
	}

	streamDict := cloneDict(dict)
	streamDict["Length"] = Integer(len(data))
	streamDict.writePDF(&w.buf)
	w.buf.WriteString("\nstream\n")
	w.buf.Write(data)
	w.buf.WriteString("\nendstream\nendobj\n")
	return nil
}

// Finish appends the xref table, trailer and EOF marker, then returns the
// complete PDF bytes.
func (w *Writer) Finish(trailer Trailer) ([]byte, error) {
	if w.closed {
		return nil, errors.New("pdf writer is already closed")
	}
	if trailer.Root.ObjectNumber <= 0 {
		return nil, errors.New("trailer root reference is required")
	}

	startXref := w.buf.Len()
	maxID := 0
	for id := range w.offsets {
		maxID = max(maxID, id)
	}

	fmt.Fprintf(&w.buf, "xref\n0 %d\n", maxID+1)
	w.buf.WriteString("0000000000 65535 f \n")
	for id := 1; id <= maxID; id++ {
		if offset, ok := w.offsets[id]; ok {
			fmt.Fprintf(&w.buf, "%010d 00000 n \n", offset)
			continue
		}
		w.buf.WriteString("0000000000 65535 f \n")
	}

	trailerDict := Dict{
		"Root": trailer.Root,
		"Size": Integer(maxID + 1),
	}
	if trailer.Info != nil {
		trailerDict["Info"] = *trailer.Info
	}
	w.buf.WriteString("trailer\n")
	trailerDict.writePDF(&w.buf)
	fmt.Fprintf(&w.buf, "\nstartxref\n%d\n%%%%EOF\n", startXref)

	w.closed = true
	return w.buf.Bytes(), nil
}

func (w *Writer) beginObject(id int) error {
	if w.closed {
		return errors.New("pdf writer is already closed")
	}
	if id <= 0 {
		return fmt.Errorf("invalid object number: %d", id)
	}
	if _, exists := w.offsets[id]; exists {
		return fmt.Errorf("duplicate object number: %d", id)
	}

	w.offsets[id] = w.buf.Len()
	fmt.Fprintf(&w.buf, "%d 0 obj\n", id)
	return nil
}

func writeObject(buf *bytes.Buffer, obj Object) {
	if obj == nil {
		Null{}.writePDF(buf)
		return
	}
	obj.writePDF(buf)
}

func cloneDict(dict Dict) Dict {
	result := make(Dict, len(dict)+1)
	maps.Copy(result, dict)
	return result
}

func writeName(buf *bytes.Buffer, name string) {
	for _, b := range []byte(name) {
		if isRegularNameByte(b) {
			buf.WriteByte(b)
			continue
		}
		fmt.Fprintf(buf, "#%02X", b)
	}
}

func isRegularNameByte(b byte) bool {
	if b <= 0x20 || b >= 0x7f {
		return false
	}
	switch b {
	case '#', '%', '(', ')', '/', '<', '>', '[', ']', '{', '}':
		return false
	default:
		return true
	}
}
