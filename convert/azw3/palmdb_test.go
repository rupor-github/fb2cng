package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
	"time"
)

func TestWritePalmDB(t *testing.T) {
	records := [][]byte{[]byte("header"), []byte("text"), {0xe9, 0x8e, 0x0d, 0x0a}}
	var output bytes.Buffer
	if err := writePalmDB(context.Background(), &output, "FBC_probe", time.Time{}, records); err != nil {
		t.Fatal(err)
	}
	data := output.Bytes()
	if string(data[60:68]) != "BOOKMOBI" || binary.BigEndian.Uint16(data[76:]) != uint16(len(records)) {
		t.Fatal("incorrect PalmDB identity or record count")
	}
	for i, want := range records {
		start := binary.BigEndian.Uint32(data[78+8*i:])
		end := uint32(len(data))
		if i+1 < len(records) {
			end = binary.BigEndian.Uint32(data[78+8*(i+1):])
		}
		if !bytes.Equal(data[start:end], want) || binary.BigEndian.Uint32(data[82+8*i:]) != uint32(2*i) {
			t.Errorf("record %d data or UID mismatch", i)
		}
	}
	var repeated bytes.Buffer
	if err := writePalmDB(context.Background(), &repeated, "FBC_probe", time.Time{}, records); err != nil || !bytes.Equal(data, repeated.Bytes()) {
		t.Error("PalmDB layout is not deterministic")
	}
}

func TestWritePalmDBNameBytes(t *testing.T) {
	for _, test := range []struct{ name, raw string }{
		{"", ""},
		{"Straße Æ", "Stra\xdfe \xc6"},
		{"ƒŒœ", "\x83\x8c\x9c"},
		{strings.Repeat("ß", 31), strings.Repeat("\xdf", 31)},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := writePalmDB(t.Context(), &output, test.name, time.Time{}, [][]byte{{1}}); err != nil {
				t.Fatal(err)
			}
			want := make([]byte, 32)
			copy(want, test.raw)
			if !bytes.Equal(output.Bytes()[:32], want) {
				t.Fatalf("name field = %x, want %x", output.Bytes()[:32], want)
			}
		})
	}
}

func TestWritePalmDBRejectsInvalidLayout(t *testing.T) {
	for _, tt := range []struct {
		name    string
		dbname  string
		records [][]byte
	}{
		{name: "long name", dbname: "12345678901234567890123456789012", records: [][]byte{{1}}},
		{name: "unencodable name", dbname: "Книга", records: [][]byte{{1}}},
		{name: "long single-byte name", dbname: strings.Repeat("ß", 32), records: [][]byte{{1}}},
		{name: "NUL in name", dbname: "book\x00", records: [][]byte{{1}}},
		{name: "no records", dbname: "book"},
		{name: "too many records", dbname: "book", records: make([][]byte, math.MaxUint16+1)},
		{name: "empty record", dbname: "book", records: [][]byte{{1}, nil}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := writePalmDB(context.Background(), &output, tt.dbname, time.Time{}, tt.records); err == nil || output.Len() != 0 {
				t.Error("invalid layout should fail before writing")
			}
		})
	}
}

func TestPalmDBTimestamp(t *testing.T) {
	for _, tt := range []struct {
		name    string
		created time.Time
		want    uint32
		invalid bool
	}{
		{name: "deterministic probe"},
		{name: "Palm epoch", created: time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)},
		{name: "Unix epoch", created: time.Unix(0, 0), want: 2082844800},
		{name: "conversion time", created: time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC), want: 0xe7176640},
		{name: "last second", created: time.Unix(int64(math.MaxUint32)-2082844800, 0), want: math.MaxUint32},
		{name: "before Palm epoch", created: time.Unix(-2082844801, 0), invalid: true},
		{name: "after last second", created: time.Unix(int64(math.MaxUint32)-2082844800+1, 0), invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			err := writePalmDB(context.Background(), &output, "book", tt.created, [][]byte{{1}})
			if tt.invalid {
				if err == nil || output.Len() != 0 {
					t.Fatal("unrepresentable timestamp should fail before output")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, offset := range []int{36, 40} {
				if got := binary.BigEndian.Uint32(output.Bytes()[offset:]); got != tt.want {
					t.Errorf("timestamp at %d = %#x, want %#x", offset, got, tt.want)
				}
			}
		})
	}
}

type failingWriter struct {
	short bool
}

type recordFailingWriter struct {
	writes int
	short  bool
}

func (w *recordFailingWriter) Write(data []byte) (int, error) {
	w.writes++
	if w.writes == 1 {
		return len(data), nil // Header succeeds; the first payload write fails.
	}
	return failingWriter{short: w.short}.Write(data)
}

func (w failingWriter) Write(data []byte) (int, error) {
	if w.short {
		return len(data) - 1, nil
	}
	return 0, io.ErrClosedPipe
}

func TestWritePalmDBFailures(t *testing.T) {
	for _, tt := range []struct {
		name   string
		writer io.Writer
		want   error
	}{
		{name: "I/O error", writer: failingWriter{}, want: io.ErrClosedPipe},
		{name: "short write", writer: failingWriter{short: true}, want: io.ErrShortWrite},
		{name: "record I/O error", writer: &recordFailingWriter{}, want: io.ErrClosedPipe},
		{name: "record short write", writer: &recordFailingWriter{short: true}, want: io.ErrShortWrite},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := writePalmDB(context.Background(), tt.writer, "book", time.Time{}, [][]byte{{1}})
			if !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
			if strings.HasPrefix(tt.name, "record ") && (err == nil || !strings.Contains(err.Error(), "record 0")) {
				t.Error("payload failure lost the physical record number")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	err := writePalmDB(ctx, &output, "book", time.Time{}, [][]byte{{1}})
	if !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Error("cancellation should stop output before writing")
	}
}
