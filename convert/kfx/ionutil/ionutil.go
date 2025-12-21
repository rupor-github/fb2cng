package ionutil

import (
	"bytes"

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
	Bytes []byte
	LST   ion.SymbolTable
}

// BuildProlog creates a local symbol table (importing the provided shared tables)
// and returns its binary encoding (BVM+LST) plus the resulting symbol table.
func BuildProlog(imports ...ion.SharedSymbolTable) (*Prolog, error) {
	// Build an immutable LST for later symbol lookups.
	lst := ion.NewSymbolTableBuilder(imports...).Build()

	buf := bytes.Buffer{}
	w := ion.NewBinaryWriter(&buf, imports...)
	// Do not encode any values, only finish the datagram.
	if err := w.Finish(); err != nil {
		return nil, err
	}

	return &Prolog{Bytes: buf.Bytes(), LST: lst}, nil
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
