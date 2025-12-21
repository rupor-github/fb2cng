package symbols

// This file intentionally mirrors *string* symbol values from Kindle Previewer
// (see com.amazon.kaf.c.b in b.jad) and KFXInput conventions.

const (
	IonSymbolTableAnnotation = "$ion_symbol_table"
	IonSharedSymbolTableAnno = "$ion_shared_symbol_table"
	IonSystemSymbolTableName = "$ion"
	IonVersionMarker         = "$ion_1_0"

	FieldName    = "name"
	FieldVersion = "version"
	FieldImports = "imports"
	FieldSymbols = "symbols"
	FieldMaxID   = "max_id"
)
