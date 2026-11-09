package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"
)

func TestWritePalmDB(t *testing.T) {
	records := [][]byte{[]byte("header"), []byte("text"), {0xe9, 0x8e, 0x0d, 0x0a}}
	var output bytes.Buffer
	if err := writePalmDB(context.Background(), &output, "FBC_probe", records); err != nil {
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
	if err := writePalmDB(context.Background(), &repeated, "FBC_probe", records); err != nil || !bytes.Equal(data, repeated.Bytes()) {
		t.Error("PalmDB layout is not deterministic")
	}
}

func TestWritePalmDBRejectsInvalidLayout(t *testing.T) {
	for _, tt := range []struct {
		name    string
		dbname  string
		records [][]byte
	}{
		{name: "empty name", records: [][]byte{{1}}},
		{name: "long name", dbname: "12345678901234567890123456789012", records: [][]byte{{1}}},
		{name: "non-ASCII name", dbname: "Книга", records: [][]byte{{1}}},
		{name: "NUL in name", dbname: "book\x00", records: [][]byte{{1}}},
		{name: "no records", dbname: "book"},
		{name: "too many records", dbname: "book", records: make([][]byte, math.MaxUint16+1)},
		{name: "empty record", dbname: "book", records: [][]byte{{1}, nil}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := writePalmDB(context.Background(), &output, tt.dbname, tt.records); err == nil || output.Len() != 0 {
				t.Error("invalid layout should fail before writing")
			}
		})
	}
}

type failingWriter struct {
	short bool
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
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := writePalmDB(context.Background(), tt.writer, "book", [][]byte{{1}})
			if !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	err := writePalmDB(ctx, &output, "book", [][]byte{{1}})
	if !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Error("cancellation should stop output before writing")
	}
}
