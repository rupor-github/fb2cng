package envelope

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/amazon-ion/ion-go/ion"
)

func TestRoundTrip(t *testing.T) {
	for _, length := range []int{1, PageSize - 1, PageSize, PageSize + 1, 2*PageSize + 17} {
		for _, mode := range []Mode{Plaintext, LZMA} {
			t.Run(fmt.Sprintf("%s/%d", mode, length), func(t *testing.T) {
				inner := make([]byte, length)
				for j := range inner {
					inner[j] = byte(j % 251)
				}
				data, info, err := Encode(t.Context(), inner, mode)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode != mode || info.DecodedSize != uint64(length) || len(info.Pages) != (length+PageSize-1)/PageSize {
					t.Fatalf("incorrect metadata: %+v", info)
				}
				if data[len(data)-9] != 0x28 || binary.BigEndian.Uint64(data[len(data)-8:]) != info.IndexOffset {
					t.Fatal("incorrect fixed-width trailer")
				}
				for _, offset := range []uint64{8, 8 + info.FrameOffset, 8 + info.IndexOffset} {
					if !bytes.Equal(data[offset:offset+4], []byte{0xe0, 1, 0, 0xea}) {
						t.Fatal("missing independent prolog")
					}
				}
				e, err := Inspect(t.Context(), data)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := e.Decode(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(decoded, inner) {
					t.Fatal("round-trip changed opaque bytes")
				}
			})
		}
	}
}

func TestReferenceFixtures(t *testing.T) {
	for _, fixture := range []struct {
		name                  string
		repetitions, pageSize int
	}{
		{"plaintext", 0, PageSize}, {"lzma", 210, PageSize}, {"lzma64", 3000, MaxPageSize},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			text, err := os.ReadFile("testdata/" + fixture.name + ".hex")
			if err != nil {
				t.Fatal(err)
			}
			data, err := hex.DecodeString(strings.TrimSpace(string(text)))
			if err != nil {
				t.Fatal(err)
			}
			e, err := Inspect(t.Context(), data)
			if err != nil {
				t.Fatal(err)
			}
			got, err := e.Decode(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			want := []byte("CONT\x00reference\xff\x80")
			if fixture.repetitions != 0 {
				want = append(bytes.Repeat([]byte("Привет, 世界!\x00"), fixture.repetitions), want...)
			}
			if e.Info().PageSize != uint64(fixture.pageSize) {
				t.Fatal("incorrect reference page size")
			}
			if !bytes.Equal(got, want) {
				t.Fatal("reference fixture did not decode byte-exactly")
			}
		})
	}
}

func TestRejectMalformed(t *testing.T) {
	valid, info, err := Encode(t.Context(), []byte("Unicode 世界 and binary\x00\xff"), LZMA)
	if err != nil {
		t.Fatal(err)
	}
	for j := range len(valid) {
		if _, err := Inspect(t.Context(), valid[:j]); err == nil {
			t.Fatalf("accepted truncated input at %d", j)
		}
	}
	for _, test := range []struct {
		name   string
		mutate func([]byte)
	}{
		{"trailer", func(b []byte) { b[len(b)-9] = 0x21 }},
		{"index-overflow", func(b []byte) { binary.BigEndian.PutUint64(b[len(b)-8:], ^uint64(0)) }},
		{"frame-prolog", func(b []byte) { b[8+info.FrameOffset] = 0 }},
		{"index-prolog", func(b []byte) { b[8+info.IndexOffset] = 0 }},
		{"filter", func(b []byte) { p := bytes.Index(b, []byte{0, 0x5d, 0, 0, 0x80, 0}); b[p] = 1 }},
		{"unknown-size", func(b []byte) {
			p := bytes.Index(b, []byte{0, 0x5d, 0, 0, 0x80, 0})
			binary.LittleEndian.PutUint64(b[p+6:], ^uint64(0))
		}},
		{"encrypted-page", func(b []byte) {
			p := bytes.Index(b, []byte{0x81, 0xc6})
			b[p+1] = 0xc5 // EncryptedPage@2.0, preserving serialized lengths.
		}},
		{"wrong-page-size", func(b []byte) {
			p := bytes.Index(b, []byte{0x8d, 0x22, 0x10, 0})
			b[p+2] = 0x20
		}},
		{"algorithm", func(b []byte) {
			p := bytes.Index(b, []byte("LZMA"))
			b[p] = 'X'
		}},
		{"compression-annotation", func(b []byte) {
			p := bytes.Index(b, []byte{0x81, 0xc8})
			b[p+1] = 0xc7
		}},
		{"symbols", func(b []byte) {
			p := bytes.Index(b, []byte("ProtectedData"))
			b[p] = 'X'
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := bytes.Clone(valid)
			test.mutate(b)
			if _, err := Inspect(t.Context(), b); err == nil {
				t.Fatal("accepted malformed envelope")
			}
		})
	}
	e, err := Inspect(t.Context(), valid)
	if err != nil {
		t.Fatal(err)
	}
	e.pages[0] = e.pages[0][:14]
	if _, err := e.Decode(t.Context()); err == nil {
		t.Fatal("accepted truncated LZMA stream")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := Encode(ctx, []byte("x"), LZMA); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := e.Decode(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestIonBoundsRejectHugeScalarAndDeepNesting(t *testing.T) {
	// A huge, truncated scalar must be rejected before ion-go allocates it.
	huge := []byte{0xe0, 1, 0, 0xea, 0xae, 0x7f, 0x7f, 0x7f, 0x7f, 0xff}
	if _, err := newReader(t.Context(), huge); err == nil {
		t.Fatal("accepted oversized scalar")
	}
	value := []byte{0x20}
	for range 20 {
		// Each short list contains the previous encoded value; switch to VarUInt
		// length once needed. Lengths here remain below 128.
		value = append([]byte{0xbe, byte(len(value)) | 0x80}, value...)
	}
	deep := append([]byte{0xe0, 1, 0, 0xea}, value...)
	if _, err := newReader(t.Context(), deep); err == nil {
		t.Fatal("accepted excessive nesting")
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
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		_, _ = readIndex(t.Context(), data)
	})
}

func FuzzInspect(f *testing.F) {
	data, _, err := Encode(context.Background(), []byte("CONT test"), Plaintext)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		e, err := Inspect(t.Context(), data)
		if err == nil && e.info.DecodedSize <= 65536 {
			_, _ = e.Decode(t.Context())
		}
	})
}

func BenchmarkEncode(b *testing.B) {
	inner := bytes.Repeat([]byte("Привет, 世界!\n"), 1024)
	b.SetBytes(int64(len(inner)))
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := Encode(b.Context(), inner, LZMA); err != nil {
			b.Fatal(err)
		}
	}
}
