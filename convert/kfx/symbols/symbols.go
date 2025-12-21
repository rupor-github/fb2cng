package symbols

import (
	"fmt"

	"github.com/amazon-ion/ion-go/ion"
)

const (
	// YJSymbolsName is the shared symbol table name used by KFX.
	YJSymbolsName = "YJ_symbols"
	// YJSymbolsVersion is the shared symbol table version used by Kindle/KFX.
	YJSymbolsVersion = 10
)

// SharedYJSymbols creates a shared symbol table compatible with the KFX tooling.
//
// KFX tends to use numeric symbol texts like "$270". Those are stored in the
// shared symbol table with an offset of Ion system symbols (maxID=10), so SID N
// maps to "$<N+10>".
func SharedYJSymbols(maxID uint64) ion.SharedSymbolTable {
	// KFX uses numeric symbol texts like "$538" and entity tables store numeric
	// IDs that correspond to those values (e.g. "$538" -> idnum 538).
	//
	// To achieve that, the shared symbol table must start at "$11" so that the
	// first imported symbol (global SID 11) resolves to text "$11".
	base := len(ion.V1SystemSymbolTable.Symbols())

	syms := make([]string, 0, maxID)
	for i := base + 1; i <= base+int(maxID); i++ {
		syms = append(syms, fmt.Sprintf("$%d", i))
	}
	return ion.NewSharedSymbolTable(YJSymbolsName, YJSymbolsVersion, syms)
}
