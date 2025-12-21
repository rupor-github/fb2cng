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
// KFX uses numeric symbol texts like "$270". In Kindle's YJ_symbols table v10,
// the imported symbol with global SID N resolves to text "$N".
//
// maxSymbol is the highest numeric "$<n>" symbol text to include (e.g. 851).
func SharedYJSymbols(maxSymbol uint64) ion.SharedSymbolTable {
	base := uint64(len(ion.V1SystemSymbolTable.Symbols()))
	if maxSymbol <= base {
		return ion.NewSharedSymbolTable(YJSymbolsName, YJSymbolsVersion, nil)
	}

	syms := make([]string, 0, maxSymbol-base)
	for i := base + 1; i <= maxSymbol; i++ {
		syms = append(syms, fmt.Sprintf("$%d", i))
	}
	return ion.NewSharedSymbolTable(YJSymbolsName, YJSymbolsVersion, syms)
}
