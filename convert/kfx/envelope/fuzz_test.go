package envelope

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/amazon-ion/ion-go/ion"
)

const maxFuzzSize = 64 * 1024

func nullImportProlog(tb testing.TB, field string, typ ion.Type) []byte {
	tb.Helper()
	var buf bytes.Buffer
	w := ion.NewBinaryWriter(&buf)
	_ = w.Annotation(ion.NewSymbolTokenFromString("$ion_symbol_table"))
	_ = w.BeginStruct()
	_ = w.FieldName(ion.NewSymbolTokenFromString("imports"))
	_ = w.BeginList()
	_ = w.BeginStruct()
	for _, name := range []string{"name", "version", "max_id"} {
		_ = w.FieldName(ion.NewSymbolTokenFromString(name))
		switch {
		case name == field:
			_ = w.WriteNullType(typ)
		case name == "name":
			_ = w.WriteString("ProtectedData")
		case name == "version":
			_ = w.WriteInt(1)
		default:
			_ = w.WriteInt(64)
		}
	}
	_ = w.EndStruct()
	_ = w.EndList()
	_ = w.EndStruct()
	if err := w.Finish(); err != nil {
		tb.Fatal(err)
	}
	return buf.Bytes()
}

func replaceTestProlog(tb testing.TB, data []byte, offset uint64, length int, replacement []byte) []byte {
	tb.Helper()
	if len(replacement) > length {
		tb.Fatal("replacement prolog exceeds original length")
	}
	// Pad with zero-length Ion NOPs to preserve all envelope offsets.
	b := bytes.Clone(data)
	section := b[offset : offset+uint64(length)]
	clear(section)
	copy(section, replacement)
	return b
}

func malformedIonSeeds() [][]byte {
	deep := []byte{0x20}
	for range 20 {
		if len(deep) < 14 {
			deep = append([]byte{0xb0 | byte(len(deep))}, deep...)
		} else {
			deep = append([]byte{0xbe, byte(len(deep)) | 0x80}, deep...)
		}
	}
	return [][]byte{
		{},
		{0xe0, 1, 0, 0xea},
		{0xe0, 1, 0, 0xea, 0xae, 0x7f, 0x7f, 0x7f, 0x7f, 0xff}, // Oversized truncated blob.
		{0xe0, 1, 0, 0xea, 0xae, 0x00},                         // Truncated scalar length.
		append([]byte{0xe0, 1, 0, 0xea}, deep...),
	}
}

func FuzzIonReader(f *testing.F) {
	data, err := datagram(func(w ion.Writer) error {
		_ = w.Annotation(ion.NewSymbolTokenFromString(indexType))
		_ = w.BeginList()
		_ = w.BeginList()
		for _, n := range []int64{41, 100, 0, 17} {
			_ = w.WriteInt(n)
		}
		_ = w.EndList()
		return w.EndList()
	})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	f.Add(data[:len(data)-1])
	for _, seed := range malformedIonSeeds() {
		f.Add(seed)
	}
	prolog, err := datagram(func(ion.Writer) error { return nil })
	if err != nil {
		f.Fatal(err)
	}
	for _, field := range []string{"name", "version", "max_id"} {
		typ := ion.IntType
		if field == "name" {
			typ = ion.StringType
		}
		f.Add(replaceTestProlog(f, data, 0, len(prolog), nullImportProlog(f, field, typ)))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxFuzzSize {
			t.Skip()
		}
		_, _ = readIndex(t.Context(), data)
	})
}

func FuzzInspect(f *testing.F) {
	prolog, err := datagram(func(ion.Writer) error { return nil })
	if err != nil {
		f.Fatal(err)
	}
	for _, mode := range []Mode{Plaintext, LZMA} {
		data, info, err := Encode(f.Context(), bytes.Repeat([]byte("CONT\x00世界\xff"), 512), mode)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
		f.Add(data[:len(data)-1])
		badOffset := bytes.Clone(data)
		binary.BigEndian.PutUint64(badOffset[len(badOffset)-8:], ^uint64(0))
		f.Add(badOffset)
		for _, offset := range []uint64{8, 8 + info.FrameOffset, 8 + info.IndexOffset} {
			for _, field := range []string{"name", "version", "max_id"} {
				typ := ion.IntType
				if field == "name" {
					typ = ion.StringType
				}
				f.Add(replaceTestProlog(f, data, offset, len(prolog), nullImportProlog(f, field, typ)))
			}
			for _, seed := range malformedIonSeeds()[2:] {
				f.Add(replaceTestProlog(f, data, offset, len(prolog), seed))
			}
		}
	}
	for _, name := range []string{"plaintext", "lzma", "lzma64"} {
		text, err := os.ReadFile("testdata/" + name + ".hex")
		if err != nil {
			f.Fatal(err)
		}
		data, err := hex.DecodeString(strings.TrimSpace(string(text)))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxFuzzSize {
			t.Skip()
		}
		e, err := Inspect(t.Context(), data)
		if err != nil || e.info.DecodedSize > maxFuzzSize {
			return
		}
		decoded, err := e.Decode(t.Context())
		if err != nil {
			// Inspection checks page headers; malformed compressed bodies can still fail decoding.
			return
		}
		encoded, _, err := Encode(t.Context(), decoded, e.info.Mode)
		if err != nil {
			t.Fatalf("re-encode decoded envelope: %v", err)
		}
		roundTrip, err := Inspect(t.Context(), encoded)
		if err != nil {
			t.Fatalf("inspect re-encoded envelope: %v", err)
		}
		got, err := roundTrip.Decode(t.Context())
		if err != nil || !bytes.Equal(got, decoded) {
			t.Fatalf("round-trip changed decoded bytes: %v", err)
		}
	})
}
