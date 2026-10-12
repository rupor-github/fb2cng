package kfx

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"strings"
	"testing"

	"github.com/amazon-ion/ion-go/ion"
)

func nullImportTable(t testing.TB, field string, annotated bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := ion.NewBinaryWriter(&buf)
	_ = w.Annotation(ion.NewSymbolTokenFromString("$ion_symbol_table"))
	_ = w.BeginStruct()
	_ = w.FieldName(ion.NewSymbolTokenFromString("imports"))
	if field == "imports" {
		_ = w.WriteNullType(ion.SymbolType)
	} else {
		_ = w.BeginList()
		_ = w.BeginStruct()
		for _, name := range []string{"name", "version", "max_id"} {
			_ = w.FieldName(ion.NewSymbolTokenFromString(name))
			switch {
			case name == field:
				if annotated {
					_ = w.Annotation(ion.NewSymbolTokenFromString("$ion"))
				}
				typ := ion.IntType
				if name == "name" {
					typ = ion.StringType
				}
				_ = w.WriteNullType(typ)
			case name == "name":
				_ = w.WriteString("YJ_symbols")
			case name == "version":
				_ = w.WriteInt(10)
			default:
				_ = w.WriteInt(int64(LargestKnownSymbol))
			}
		}
		_ = w.EndStruct()
		_ = w.EndList()
	}
	if field == "" {
		_ = w.FieldName(ion.NewSymbolTokenFromString("symbols"))
		_ = w.BeginList()
		_ = w.WriteNullType(ion.StringType)
		_ = w.WriteString("local")
		_ = w.EndList()
	}
	_ = w.EndStruct()
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestIonReadersRejectNullImports(t *testing.T) {
	for _, test := range []struct {
		name      string
		field     string
		annotated bool
	}{
		{"null name", "name", false},
		{"null version", "version", false},
		{"annotated null name", "name", true},
		{"annotated null version", "version", true},
		{"null imports symbol", "imports", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := nullImportTable(t, test.field, test.annotated)
			for _, decoder := range []struct {
				name string
				run  func() error
			}{
				{"symbol table", func() error { _, err := DecodeSymbolTable(data); return err }},
				{"decode prolog", func() error {
					var value any
					return DecodeIon(data, []byte{0xd0}, &value)
				}},
				{"decode payload", func() error {
					var value any
					return DecodeIon(GetIonProlog(), append(bytes.Clone(data), 0xd0), &value)
				}},
			} {
				t.Run(decoder.name, func(t *testing.T) {
					if err := decoder.run(); err == nil {
						t.Fatal("accepted null symbol-table import")
					}
				})
			}
			for _, factory := range []struct {
				name string
				new  func() *IonReader
			}{
				{"reader prolog", func() *IonReader { return NewIonReader(data, []byte{0xd0}) }},
				{"reader payload", func() *IonReader {
					return NewIonReader(GetIonProlog(), append(bytes.Clone(data), 0xd0))
				}},
				{"reader local symbols", func() *IonReader {
					return NewIonReaderWithLocalSymbols(data, []byte{0xd0}, []string{"local"})
				}},
				{"reader bytes", func() *IonReader { return NewIonReaderBytes(append(bytes.Clone(data), 0xd0)) }},
			} {
				t.Run(factory.name, func(t *testing.T) {
					r := factory.new()
					if r.Next() || r.Err() == nil {
						t.Fatal("expected a reader error for null symbol-table import")
					}
					originalErr := r.Err()
					if r.Next() || r.Err() != originalErr {
						t.Fatal("reader continued after a parsing failure")
					}
				})
			}
		})
	}
}

func TestIonReadersPreserveNullValues(t *testing.T) {
	var buf bytes.Buffer
	w := ion.NewBinaryWriter(&buf, GetSharedSymbolTable())
	_ = w.BeginStruct()
	for _, field := range []struct {
		name string
		typ  ion.Type
	}{
		{"name", ion.StringType}, {"version", ion.IntType}, {"imports", ion.SymbolType}, {"symbols", ion.ListType},
	} {
		_ = w.FieldName(ion.NewSymbolTokenFromString(field.name))
		_ = w.WriteNullType(field.typ)
	}
	_ = w.EndStruct()
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"name": nil, "version": nil, "imports": nil, "symbols": nil}
	var decoded map[string]any
	if err := DecodeIon(GetIonProlog(), buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decoded null fields changed: got %v, want %v", decoded, want)
	}
	r := NewIonReaderWithLocalSymbols(GetIonProlog(), buf.Bytes(), []string{"local"})
	if !r.Next() {
		t.Fatalf("missing null-containing struct: %v", r.Err())
	}
	value, err := r.ReadValue()
	if err != nil || !reflect.DeepEqual(value, want) {
		t.Fatalf("read null fields changed: got %v, err=%v", value, err)
	}
}

func TestIonReaderRejectsNullImportAfterValue(t *testing.T) {
	for _, field := range []string{"name", "version"} {
		t.Run(field, func(t *testing.T) {
			data := append([]byte{0x21, 12}, nullImportTable(t, field, false)...)
			r := NewIonReaderBytes(data)
			if !r.Next() {
				t.Fatalf("missing initial value: %v", r.Err())
			}
			value, err := r.IntValue()
			if err != nil || value != 12 {
				t.Fatalf("initial value changed: %d, err=%v", value, err)
			}
			if r.Next() || r.Err() == nil {
				t.Fatal("accepted a subsequent null import")
			}
		})
	}
}

func TestIonReadersPreserveNullableLocalSymbols(t *testing.T) {
	data := nullImportTable(t, "", false)
	table, err := DecodeSymbolTable(data)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"", "local"}; !reflect.DeepEqual(table.Symbols(), want) {
		t.Fatalf("nullable local symbols changed: got %v, want %v", table.Symbols(), want)
	}
	r := NewIonReader(data, []byte{0x0f})
	if !r.Next() || !r.IsNull() {
		t.Fatalf("missing null document value: %v", r.Err())
	}
	value, err := r.ReadValue()
	if err != nil || value != nil {
		t.Fatalf("null document value changed: %v, err=%v", value, err)
	}
}

func TestReadContainerRejectsNullImportsInPayloads(t *testing.T) {
	c := NewContainer()
	c.FormatCapabilities = strings.Repeat("x", 128)
	if err := c.Fragments.Add(NewRootFragment(SymMetadata, strings.Repeat("x", 128))); err != nil {
		t.Fatal(err)
	}
	valid, err := c.WriteContainer()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadContainer(valid); err != nil {
		t.Fatalf("invalid original container: %v", err)
	}
	var header containerHeader
	if err := binary.Read(bytes.NewReader(valid), binary.LittleEndian, &header); err != nil {
		t.Fatal(err)
	}
	var info containerInfo
	if err := DecodeIon(GetIonProlog(), valid[header.InfoOffset:header.InfoOffset+header.InfoSize], &info); err != nil {
		t.Fatal(err)
	}
	var entry indexTableEntry
	if err := entry.readFrom(bytes.NewReader(valid[info.IndexTabOff:])); err != nil {
		t.Fatal(err)
	}
	start := int(header.Size) + int(entry.Offset)
	var entity entityHeader
	if err := binary.Read(bytes.NewReader(valid[start:]), binary.LittleEndian, &entity); err != nil {
		t.Fatal(err)
	}
	for _, region := range []struct {
		name         string
		offset, size int
	}{
		{"format capabilities", info.FCapabOff, info.FCapabLen},
		{"entity payload", start + int(entity.Size), int(entry.Size) - int(entity.Size)},
	} {
		for _, field := range []string{"name", "version"} {
			t.Run(region.name+"/"+field, func(t *testing.T) {
				replacement := append(nullImportTable(t, field, false), 0xd0)
				if len(replacement) > region.size {
					t.Fatal("replacement exceeds original payload length")
				}
				data := bytes.Clone(valid)
				payload := data[region.offset : region.offset+region.size]
				clear(payload)
				copy(payload, replacement)
				if _, err := ReadContainer(data); err == nil || !strings.Contains(err.Error(), "invalid Ion input") {
					t.Fatalf("expected an Ion parsing error, got %v", err)
				}
			})
		}
	}
}
