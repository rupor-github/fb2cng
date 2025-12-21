package builders

import "github.com/amazon-ion/ion-go/ion"

type annotatedValue struct {
	Value       any               `ion:",omitempty"`
	Annotations []ion.SymbolToken `ion:",annotations"`
}

func annot(names ...string) []ion.SymbolToken {
	out := make([]ion.SymbolToken, 0, len(names))
	for _, n := range names {
		out = append(out, ion.NewSymbolTokenFromString(n))
	}
	return out
}
