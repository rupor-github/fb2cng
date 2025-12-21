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
	Bytes []byte
	LST   ion.SymbolTable
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

	// ion-go only writes the LST when writing at least one top-level value.
	// We write a single null and then strip it, leaving a datagram with exactly
	// one value: the LST.
	buf := bytes.Buffer{}
	w := ion.NewBinaryWriterLST(&buf, lst)
	if err := w.WriteNull(); err != nil {
		return nil, err
	}
	if err := w.Finish(); err != nil {
		return nil, err
	}

	b := buf.Bytes()
	if len(b) == 0 || b[len(b)-1] != 0x0F {
		return nil, fmt.Errorf("unexpected prolog trailer")
	}
	b = b[:len(b)-1]

	return &Prolog{Bytes: b, LST: lst}, nil
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
