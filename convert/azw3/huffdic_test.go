package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The test decoder builds a bit-by-bit code tree from the wire length table,
// independently of the encoder's trie, canonical-code assignment and fast lookup.
type testHuffDecoder struct {
	Codes   map[uint64][]byte
	Phrases [][]byte
	Huff    []byte
}

func newTestHuffDecoder(t testing.TB, records [][]byte) *testHuffDecoder {
	t.Helper()
	if len(records) < 2 || len(records[0]) < 24 || string(records[0][:4]) != "HUFF" {
		t.Fatal("missing HUFF/CDIC records")
	}
	huff := records[0]
	fast, lengths := int(binary.BigEndian.Uint32(huff[8:])), int(binary.BigEndian.Uint32(huff[12:]))
	for _, pair := range [][3]int{
		{fast, int(binary.BigEndian.Uint32(huff[16:])), 1024},
		{lengths, int(binary.BigEndian.Uint32(huff[20:])), 256},
	} {
		if pair[0] < 24 || pair[1] < 24 || pair[0]+pair[2] > len(huff) || pair[1]+pair[2] > len(huff) {
			t.Fatal("HUFF table points outside its record")
		}
		for i := 0; i < pair[2]; i += 4 {
			if binary.BigEndian.Uint32(huff[pair[0]+i:]) != binary.LittleEndian.Uint32(huff[pair[1]+i:]) {
				t.Fatal("HUFF big/little-endian tables disagree")
			}
		}
	}
	var phrases [][]byte
	declared := -1
	for _, cdic := range records[1:] {
		if len(cdic) < 16 || string(cdic[:4]) != "CDIC" || binary.BigEndian.Uint32(cdic[4:]) != 16 {
			t.Fatal("invalid CDIC header")
		}
		total, bits := int(binary.BigEndian.Uint32(cdic[8:])), binary.BigEndian.Uint32(cdic[12:])
		if declared >= 0 && declared != total || bits > 16 {
			t.Fatal("inconsistent CDIC dictionary size")
		}
		declared = total
		n := min(1<<bits, total-len(phrases))
		if n <= 0 || 16+2*n > len(cdic) {
			t.Fatal("CDIC record exceeds declared phrase count")
		}
		for i := range n {
			offset := 16 + int(binary.BigEndian.Uint16(cdic[16+2*i:]))
			if offset < 16+2*n || offset+2 > len(cdic) {
				t.Fatal("CDIC phrase overlaps offsets or record end")
			}
			value := binary.BigEndian.Uint16(cdic[offset:])
			size := int(value & 0x7fff)
			if value&0x8000 == 0 || size == 0 || offset+2+size > len(cdic) {
				t.Fatal("expected a bounded, nonempty terminal CDIC phrase")
			}
			phrases = append(phrases, cdic[offset+2:offset+2+size])
		}
	}
	if len(phrases) != declared {
		t.Fatal("incomplete CDIC dictionary")
	}
	decoder := &testHuffDecoder{Codes: make(map[uint64][]byte), Phrases: phrases, Huff: huff}
	base := 0
	for length := 1; length <= 32; length++ {
		entry := huff[lengths+8*(length-1):]
		minimum, maximum := binary.BigEndian.Uint32(entry), binary.BigEndian.Uint32(entry[4:])
		if minimum == 0 && maximum == 0 {
			continue
		}
		count := int64(maximum) - int64(minimum) + 1 - int64(base)
		if count <= 0 {
			continue
		}
		if base+int(count) > len(phrases) {
			t.Fatal("HUFF length class exceeds CDIC dictionary")
		}
		for i := range int(count) {
			code := maximum - uint32(base+i)
			decoder.Codes[uint64(length)<<32|uint64(code)] = phrases[base+i]
		}
		base += int(count)
	}
	if len(decoder.Codes) != len(phrases) {
		t.Fatal("HUFF codes do not cover every CDIC phrase")
	}
	for prefix := range 256 {
		value := binary.BigEndian.Uint32(huff[fast+4*prefix:])
		length := value & 31
		if length == 0 || length <= 8 && value&0x80 == 0 {
			t.Fatal("invalid HUFF fast entry")
		}
		if value&0x80 != 0 {
			code := uint32(prefix) >> (8 - length)
			if _, found := decoder.Codes[uint64(length)<<32|uint64(code)]; !found {
				t.Fatal("fast HUFF entry names an absent code")
			}
		}
	}
	return decoder
}

// Exercise the wire fast table using finite uint32 normalization, as in a native
// reader, in addition to the independent bit-by-bit decoder above. Python's
// arbitrary-width arithmetic alone would not catch overflowing empty thresholds.
func (decoder *testHuffDecoder) unpackFast32(t testing.TB, data []byte) []byte {
	t.Helper()
	huff := decoder.Huff
	fast, lengths := int(binary.BigEndian.Uint32(huff[8:])), int(binary.BigEndian.Uint32(huff[12:]))
	var result []byte
	for pos := 0; pos < 8*len(data); {
		var lookahead uint32
		for bit := range 32 {
			lookahead <<= 1
			at := pos + bit
			if at < 8*len(data) {
				lookahead |= uint32(data[at/8] >> (7 - at%8) & 1)
			}
		}
		entry := binary.BigEndian.Uint32(huff[fast+4*int(lookahead>>24):])
		length := entry & 31
		maximum := uint32(((uint64(entry>>8) + 1) << (32 - length)) - 1)
		if entry&0x80 == 0 {
			for {
				if length == 0 || length > 32 {
					t.Fatal("HUFF fast decoding ran past the length table")
				}
				table := huff[lengths+8*int(length-1):]
				minimum := binary.BigEndian.Uint32(table) << (32 - length)
				if lookahead >= minimum {
					maximum = uint32(((uint64(binary.BigEndian.Uint32(table[4:])) + 1) << (32 - length)) - 1)
					break
				}
				length++
			}
		}
		if pos+int(length) > 8*len(data) {
			break
		}
		index := (maximum - lookahead) >> (32 - length)
		if int(index) >= len(decoder.Phrases) {
			t.Fatal("HUFF fast decoding selected an absent CDIC phrase")
		}
		result = append(result, decoder.Phrases[index]...)
		if len(result) > textRecordSize {
			t.Fatal("HUFF fast decoding exceeds record bounds")
		}
		pos += int(length)
	}
	return result
}

func (decoder *testHuffDecoder) unpack(t testing.TB, data []byte) []byte {
	t.Helper()
	var result []byte
	var code uint32
	var length uint
	for _, value := range data {
		for bit := 7; bit >= 0; bit-- {
			code = code<<1 | uint32(value>>bit&1)
			length++
			if phrase, ok := decoder.Codes[uint64(length)<<32|uint64(code)]; ok {
				result = append(result, phrase...)
				code, length = 0, 0
			}
			if length > 32 || len(result) > textRecordSize {
				t.Fatal("HUFF stream exceeds record bounds")
			}
		}
	}
	if code != 0 || length >= 8 {
		t.Fatal("HUFF stream ends with invalid padding")
	}
	return result
}

func TestHuffdicRoundTrip(t *testing.T) {
	allBytes := make([]byte, 256)
	for i := range allBytes {
		allBytes[i] = byte(i)
	}
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{"single byte", []byte{0}},
		{"all bytes", allBytes},
		{"repetitive", bytes.Repeat([]byte{0x80}, textRecordSize)},
		{"multilingual", []byte(strings.Repeat("<p>Кириллица, café, 中文, 🙂 &amp; text.</p>", 50))},
		{"short markup", []byte("<p>Hi</p>")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := bytes.Clone(tt.data)
			chunks := []textRecord{{Data: tt.data}}
			dictionary, err := buildHuffDictionary(context.Background(), chunks)
			if err != nil {
				t.Fatal(err)
			}
			encoded := dictionary.compress(tt.data)
			decoder := newTestHuffDecoder(t, dictionary.Records)
			if got := decoder.unpack(t, encoded); !bytes.Equal(got, tt.data) || !bytes.Equal(before, tt.data) {
				t.Error("HUFF/CDIC changed bytes during compression or decoding")
			}
			if !bytes.Equal(decoder.unpackFast32(t, encoded), tt.data) {
				t.Error("HUFF/CDIC does not decode with 32-bit fast lookup arithmetic")
			}
			again, err := buildHuffDictionary(context.Background(), chunks)
			if err != nil || !reflect.DeepEqual(again.Codes, dictionary.Codes) || !reflect.DeepEqual(again.Records, dictionary.Records) {
				t.Fatal("HUFF/CDIC dictionary is not deterministic")
			}
			artifacts := map[string][]byte{"huffdic-" + tt.name + ".raw": tt.data, "huffdic-" + tt.name + ".bin": encoded}
			for i, record := range dictionary.Records {
				artifacts[fmt.Sprintf("huffdic-%s-dict-%d.bin", tt.name, i)] = record
			}
			writeProbeArtifacts(t, artifacts)
		})
	}
}

func TestHuffdicMultipleCDICRecords(t *testing.T) {
	for _, shape := range []string{"uniform", "gapped lengths"} {
		t.Run(shape, func(t *testing.T) {
			n := huffMaxPhrases
			if shape == "gapped lengths" {
				n = 4090 // One length-1 code, one length-10 code, and 4088 length-13 codes.
			}
			phrases := make([]huffPhrase, n)
			codes, order := make([]huffCode, len(phrases)), make([]int, len(phrases))
			cursor := uint64(1) << 32
			for i := range phrases {
				phrases[i].Data = []byte{byte(i)}
				if i >= 256 {
					phrases[i].Data = []byte{0xff, byte(i >> 8), byte(i)}
				}
				length := uint8(12)
				if shape == "gapped lengths" {
					length = 13
					if i == 0 {
						length = 1
					} else if i == 1 {
						length = 10
					}
				}
				cursor -= uint64(1) << (32 - length)
				codes[i], order[i] = huffCode{Bits: uint32(cursor >> (32 - length)), Length: length}, i
			}
			records, err := encodeHuffRecords(phrases, codes, order)
			if err != nil {
				t.Fatal(err)
			}
			if len(records) != 5 {
				t.Fatal("expected HUFF plus four CDIC records")
			}
			dictionary := &huffDictionary{Tree: newHuffTrie(phrases), Codes: codes, Records: records}
			input := bytes.Repeat([]byte{0xff, 0x0e, 0xfe, 0, 1, 0x7f}, 300)
			decoder := newTestHuffDecoder(t, records)
			encoded := dictionary.compress(input)
			if !bytes.Equal(decoder.unpack(t, encoded), input) || !bytes.Equal(decoder.unpackFast32(t, encoded), input) {
				t.Error("HUFF dictionary indexes do not span CDIC record boundaries")
			}
			name := "huffdic-multiple-cdic-" + strings.ReplaceAll(shape, " ", "-")
			artifacts := map[string][]byte{name + ".raw": input, name + ".bin": encoded}
			for i, record := range records {
				artifacts[fmt.Sprintf("%s-dict-%d.bin", name, i)] = record
			}
			writeProbeArtifacts(t, artifacts)
		})
	}
}

func TestHuffCodeLengthLimit(t *testing.T) {
	phrases := make([]huffPhrase, 256)
	a, b := uint64(1), uint64(1)
	for i := range 40 {
		phrases[i].Frequency = a
		a, b = b, a+b
	}
	lengths := huffCodeLengths(phrases)
	var kraft uint64
	for _, length := range lengths {
		if length == 0 || length > huffCodeLimit {
			t.Fatal("HUFF code length exceeds wire bounds")
		}
		kraft += uint64(1) << (huffCodeLimit - length)
	}
	if kraft != 1<<huffCodeLimit || slices.Max(lengths) < 8 {
		t.Fatal("HUFF code tree is incomplete or permits spurious zero-padding symbols")
	}
}

func TestHuffdicCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := buildHuffDictionary(ctx, []textRecord{{Data: []byte("text")}}); !errors.Is(err, context.Canceled) {
		t.Fatal("HUFF dictionary training ignored cancellation")
	}
}

func FuzzHuffdic(f *testing.F) {
	for _, seed := range [][]byte{{0}, {0xff, 0, 1, 8, 127}, []byte("Кириллица, café, 中文, 🙂"), bytes.Repeat([]byte{'x'}, 4096)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) == 0 || len(input) > textRecordSize {
			t.Skip()
		}
		dictionary, err := buildHuffDictionary(context.Background(), []textRecord{{Data: input}})
		if err != nil {
			t.Fatal(err)
		}
		decoder := newTestHuffDecoder(t, dictionary.Records)
		if !bytes.Equal(decoder.unpack(t, dictionary.compress(input)), input) {
			t.Fatal("HUFF/CDIC round trip changed bytes")
		}
		if !bytes.Equal(decoder.unpackFast32(t, dictionary.compress(input)), input) {
			t.Fatal("HUFF/CDIC fast 32-bit round trip changed bytes")
		}
	})
}
