package ionutil

import (
	"bytes"
	"fmt"

	"github.com/amazon-ion/ion-go/ion"
)

var ionBVM = []byte{0xE0, 0x01, 0x00, 0xEA}

// Prolog is a binary Ion datagram that consists of:
// - binary version marker (BVM)
// - local symbol table (LST)
//
// KFX stores this prolog separately as "document symbols" and then stores each
// fragment payload as BVM + value (without LST).
type Prolog struct {
	// Bytes is the BVM+LST prefix used to strip the prolog from MarshalBinaryLST output.
	Bytes []byte
	// DocSymbols is the BVM+IonValue blob stored in the container as document symbols.
	DocSymbols []byte
	LST        ion.SymbolTable
}

// BuildProlog creates a KFX "document symbols" datagram (BVM + local symbol
// table) that imports the provided shared tables and defines the requested local
// symbols.
//
// KFX stores this datagram separately (container_info.$415/$416) and fragment
// payloads are encoded as: BVM + value (no embedded symbol table).
func BuildProlog(localSymbols []string, imports ...ion.SharedSymbolTable) (*Prolog, error) {
	lstb := ion.NewSymbolTableBuilder(imports...)
	for _, s := range localSymbols {
		_, _ = lstb.Add(s)
	}
	lst := lstb.Build()

	// MarshalBinaryLST is what MarshalPayload relies on; using it here guarantees
	// that prolog.Bytes is exactly the prefix of MarshalBinaryLST output.
	b, err := ion.MarshalBinaryLST(nil, lst)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 || b[len(b)-1] != 0x0F {
		return nil, fmt.Errorf("unexpected prolog trailer")
	}
	b = b[:len(b)-1]

	p := &Prolog{Bytes: b, LST: lst}

	// Build document symbols payload as a single annotated value ($ion_symbol_table)
	// with import max_id including system symbols (kfxlib expects this).
	type importEntry struct {
		Name  string `ion:"name"`
		Ver   int64  `ion:"version"`
		MaxID int64  `ion:"max_id"`
	}
	type symtab struct {
		Imports []importEntry `ion:"imports"`
		Symbols []string      `ion:"symbols"`
	}

	maxID := int64(lst.MaxID())
	// Imported YJ_symbols are numeric "$10".."$851"; local symbols start after that.
	// kfxlib adjusts import.max_id by subtracting system symbol count (9).
	if len(imports) > 0 {
		// imports[0] is YJ_symbols; its max_id should be system+851 (=860) for v10.
		// With ion-go system max_id=9, this is 9+842=851.
		maxID = int64(len(ion.V1SystemSymbolTable.Symbols())) + 842
	}

	ds := symtab{
		Imports: []importEntry{{Name: "YJ_symbols", Ver: 10, MaxID: maxID}},
		Symbols: localSymbols,
	}
	doc, err := MarshalAnnotatedPayload(ds, []ion.SymbolToken{ion.NewSymbolTokenFromString("$ion_symbol_table")}, p)
	if err != nil {
		return nil, fmt.Errorf("marshal document symbols: %w", err)
	}
	p.DocSymbols = doc

	return p, nil
}

// MarshalPayload encodes v as a KFX fragment payload: BVM + value bytes.
// It does that by encoding (BVM+LST+value) with ion-go and stripping the prolog.
func MarshalPayload(v any, prolog *Prolog) ([]byte, error) {
	full, err := ion.MarshalBinaryLST(v, prolog.LST)
	if err != nil {
		return nil, err
	}

	// full = BVM + LST + value
	// prolog.Bytes = BVM + LST
	val := full[len(prolog.Bytes):]

	out := make([]byte, 0, len(ionBVM)+len(val))
	out = append(out, ionBVM...)
	out = append(out, val...)
	return out, nil
}

// MarshalAnnotatedPayload is MarshalPayload, but wraps the top-level value with Ion annotations.
func MarshalAnnotatedPayload(v any, annotations []ion.SymbolToken, prolog *Prolog) ([]byte, error) {
	buf := bytes.Buffer{}
	w := ion.NewBinaryWriterLST(&buf, prolog.LST)
	if err := w.Annotations(annotations...); err != nil {
		return nil, err
	}
	if err := ion.MarshalTo(w, v); err != nil {
		return nil, err
	}
	if err := w.Finish(); err != nil {
		return nil, err
	}

	full := buf.Bytes()
	val := full[len(prolog.Bytes):]

	out := make([]byte, 0, len(ionBVM)+len(val))
	out = append(out, ionBVM...)
	out = append(out, val...)
	return out, nil
}
