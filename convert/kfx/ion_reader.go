package kfx

import (
	"bytes"
	"fmt"

	"github.com/amazon-ion/ion-go/ion"
)

// ion-go parses local symbol tables inside Next and panics on typed-null imports.
// Guard that dependency call so malformed input becomes a terminal reader error,
// while ordinary null document values keep their normal Ion semantics.
type guardedIonReader struct {
	ion.Reader
	err error
}

func newGuardedIonReader(data []byte) *guardedIonReader {
	return &guardedIonReader{Reader: ion.NewReaderCat(bytes.NewReader(data), ion.NewCatalog(sharedSymbolTable))}
}

func (r *guardedIonReader) Next() (ok bool) {
	if r.err != nil {
		return false
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			if err, isError := recovered.(error); isError {
				r.err = fmt.Errorf("invalid Ion input: %w", err)
			} else {
				r.err = fmt.Errorf("invalid Ion input: %v", recovered)
			}
			ok = false
		}
	}()
	return r.Reader.Next()
}

func (r *guardedIonReader) Err() error {
	if r.err != nil {
		return r.err
	}
	return r.Reader.Err()
}
