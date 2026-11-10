package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	rand "math/rand/v2"
	"strings"
	"testing"
)

// Decode the format independently of the encoder's hash table and token choices.
// In particular, a back-reference may copy bytes it has just appended.
func decodePalmDOC(t testing.TB, encoded []byte) []byte {
	t.Helper()
	var result []byte
	for pos := 0; pos < len(encoded); {
		code := encoded[pos]
		pos++
		switch {
		case code >= 1 && code <= 8:
			if len(encoded)-pos < int(code) {
				t.Fatal("truncated PalmDOC literal run")
			}
			result = append(result, encoded[pos:pos+int(code)]...)
			pos += int(code)
		case code < 0x80:
			result = append(result, code)
		case code >= 0xc0:
			result = append(result, ' ', code^0x80)
		default:
			if pos == len(encoded) {
				t.Fatal("truncated PalmDOC back-reference")
			}
			value := uint16(code)<<8 | uint16(encoded[pos])
			pos++
			distance, length := int(value>>3&0x7ff), int(value&7)+3
			if distance == 0 || distance > len(result) {
				t.Fatal("PalmDOC back-reference points outside its record")
			}
			for range length {
				result = append(result, result[len(result)-distance])
			}
		}
		if len(result) > textRecordSize {
			t.Fatal("PalmDOC record expands past 4096 bytes")
		}
	}
	return result
}

func TestCompressPalmDOC(t *testing.T) {
	for _, tt := range []struct {
		name string
		data []byte
		want []byte
	}{
		{name: "empty", data: []byte{}, want: []byte{}},
		{name: "direct literals", data: []byte{0, 9, 10, 13, '!', '~', 0x7f}, want: []byte{0, 9, 10, 13, '!', '~', 0x7f}},
		{name: "eight escaped bytes", data: []byte{1, 2, 3, 4, 5, 6, 7, 8}, want: []byte{8, 1, 2, 3, 4, 5, 6, 7, 8}},
		{
			name: "nine escaped bytes", data: []byte{0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88},
			want: []byte{8, 0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 1, 0x88},
		},
		{name: "space pairs", data: []byte(" @ A _ \x7f"), want: []byte{0xc0, 0xc1, 0xdf, 0xff}},
		{name: "space without pair", data: []byte(" ? "), want: []byte(" ? ")},
		{name: "minimum match", data: []byte("abcabc"), want: []byte{'a', 'b', 'c', 0x80, 0x18}},
		{name: "overlapping match", data: []byte("abcabcabc"), want: []byte{'a', 'b', 'c', 0x80, 0x1b}},
		{name: "maximum match", data: bytes.Repeat([]byte{'a'}, 11), want: []byte{'a', 0x80, 0x0f}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := compressPalmDOC(tt.data)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("compressed bytes = %x, want %x", got, tt.want)
			}
			if decoded := decodePalmDOC(t, got); !bytes.Equal(decoded, tt.data) {
				t.Errorf("round trip = %x, want %x", decoded, tt.data)
			}
		})
	}
}

func TestPalmDOCWindowLimit(t *testing.T) {
	for _, distance := range []int{2047, 2048} {
		t.Run(fmt.Sprint(distance), func(t *testing.T) {
			input := []byte("ABC" + strings.Repeat("x", distance-3) + "ABC")
			compressed := compressPalmDOC(input)
			if !bytes.Equal(decodePalmDOC(t, compressed), input) {
				t.Fatal("window-boundary round trip changed bytes")
			}
			if distance == 2047 && !bytes.HasSuffix(compressed, binary.BigEndian.AppendUint16(nil, 0xbff8)) {
				t.Error("maximum-distance back-reference was not emitted")
			}
			if distance == 2048 && !bytes.HasSuffix(compressed, []byte("ABC")) {
				t.Error("encoder referenced data outside the 2047-byte window")
			}
		})
	}
}

func TestPalmDOCRecordRoundTrip(t *testing.T) {
	random := make([]byte, textRecordSize)
	rng := rand.New(rand.NewPCG(1, 2))
	for i := range random {
		random[i] = byte(rng.Uint32())
	}
	allBytes := make([]byte, 256)
	for i := range allBytes {
		allBytes[i] = byte(i)
	}
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{name: "random binary", data: random},
		{name: "all byte values", data: allBytes},
		{name: "all zeros", data: make([]byte, textRecordSize)},
		{name: "repeated byte", data: bytes.Repeat([]byte{0xff}, textRecordSize)},
		{name: "multilingual", data: []byte(strings.Repeat("Кириллица, café, 中文, 🙂 &amp; text. ", 50))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := bytes.Clone(tt.data)
			encoded := compressPalmDOC(tt.data)
			if !bytes.Equal(decodePalmDOC(t, encoded), tt.data) || !bytes.Equal(before, tt.data) {
				t.Error("compression changed input or lost bytes")
			}
			if !bytes.Equal(encoded, compressPalmDOC(tt.data)) {
				t.Error("compression is not deterministic")
			}
			name := "palmdoc-" + strings.ReplaceAll(tt.name, " ", "-")
			writeProbeArtifacts(t, map[string][]byte{name + ".raw": tt.data, name + ".bin": encoded})
		})
	}
	input := []byte(strings.Repeat("<p>Кириллица, café, 中文, 🙂 &amp; text.</p>", 300))
	records, err := splitText(input)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []byte
	compressedSize := 0
	for _, record := range records {
		encoded := compressPalmDOC(record.Data)
		compressedSize += len(encoded)
		plain := decodePalmDOC(t, encoded)
		if !bytes.Equal(plain, record.Data) {
			t.Error("compression changed bytes at a fixed record boundary")
		}
		decoded = append(decoded, plain...)
	}
	if !bytes.Equal(input, decoded) || compressedSize >= len(input)/2 {
		t.Errorf("record round trip or compression failed: %d -> %d bytes", len(input), compressedSize)
	}
}

func FuzzPalmDOC(f *testing.F) {
	for _, seed := range [][]byte{
		{}, {0, 1, 8, 9, 0x7f, 0x80, 0xc0, 0xff}, []byte(" @ A _ \x7f"),
		bytes.Repeat([]byte{'a'}, textRecordSize), []byte("Кириллица, café, 中文, 🙂"),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > textRecordSize {
			t.Skip()
		}
		compressed := compressPalmDOC(input)
		if !bytes.Equal(decodePalmDOC(t, compressed), input) {
			t.Fatal("PalmDOC round trip changed bytes")
		}
	})
}

func BenchmarkCompressPalmDOC(b *testing.B) {
	data := []byte(strings.Repeat("<p>Кириллица, café, 中文, 🙂 &amp; text.</p>", 100))[:textRecordSize]
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		compressPalmDOC(data)
	}
}

func TestTextCompressionModes(t *testing.T) {
	input := []byte(strings.Repeat("<p>Кириллица, café, 中文, 🙂 &amp; text.</p>", 150))
	before := bytes.Clone(input)
	chunks, err := splitText(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		mode string
		wire uint16
	}{
		{"", 2}, {"none", 1}, {"standard", 2}, {"huffdic", 17480},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			result, err := compileTextRecords(context.Background(), chunks, tt.mode)
			if err != nil {
				t.Fatal(err)
			}
			if result.Type != tt.wire || len(result.Records) != len(chunks) {
				t.Fatal("wrong compression header identity or text-record count")
			}
			var decoder *testHuffDecoder
			if tt.wire == 17480 {
				decoder = newTestHuffDecoder(t, result.Dictionary)
			} else if len(result.Dictionary) != 0 {
				t.Fatal("non-HUFF compression allocated dictionary records")
			}
			for i, record := range result.Records {
				overlap := len(chunks[i].Overlap)
				if record[len(record)-1] != byte(overlap) ||
					!bytes.Equal(record[len(record)-overlap-1:len(record)-1], chunks[i].Overlap) {
					t.Fatal("compression changed UTF-8 overlap trailers")
				}
				decoded := record[:len(record)-overlap-1]
				if tt.wire == 2 {
					decoded = decodePalmDOC(t, decoded)
				} else if decoder != nil {
					decoded = decoder.unpackFast32(t, decoded)
				}
				if !bytes.Equal(decoded, chunks[i].Data) {
					t.Fatal("compression changed decoded source bytes")
				}
			}
			if !bytes.Equal(before, input) {
				t.Fatal("compression or trailer insertion modified frozen source bytes")
			}
		})
	}
	for _, mode := range []string{"c0", "c1", "c2", "huffcdic", "lzma", "Standard"} {
		if _, err := compileTextRecords(context.Background(), chunks, mode); err == nil {
			t.Errorf("invalid compression mode %q accepted", mode)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := compileTextRecords(ctx, chunks, "invalid"); !errors.Is(err, context.Canceled) {
		t.Fatal("compression mode selection ignored context cancellation")
	}
}
