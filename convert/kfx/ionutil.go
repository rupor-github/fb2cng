package kfx

import (
	"bytes"
	"fmt"
	"io"

	"github.com/amazon-ion/ion-go/ion"
)

// Ion Binary Version Marker
var ionBVM = []byte{0xE0, 0x01, 0x00, 0xEA}

// sharedSymbolTable is the YJ_symbols shared symbol table for KFX.
var sharedSymbolTable = createSharedSymbolTable(int(LargestKnownSymbol))

// ionProlog is the Ion binary prolog with YJ_symbols import.
var ionProlog = createIonProlog()

// createSharedSymbolTable creates the YJ_symbols shared symbol table.
// Symbol names are $10, $11, etc. (after Ion system symbols which go 1-9).
func createSharedSymbolTable(maxID int) ion.SharedSymbolTable {
	systemSymCount := len(ion.V1SystemSymbolTable.Symbols())
	symbols := make([]string, 0, maxID)
	for i := systemSymCount + 1; i <= systemSymCount+maxID; i++ {
		symbols = append(symbols, fmt.Sprintf("$%d", i))
	}
	return ion.NewSharedSymbolTable("YJ_symbols", 10, symbols)
}

// createIonProlog creates the Ion binary prolog with YJ_symbols import.
func createIonProlog() []byte {
	buf := bytes.Buffer{}
	if err := ion.NewBinaryWriter(&buf, sharedSymbolTable).Finish(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// GetIonProlog returns the Ion prolog bytes for writing.
func GetIonProlog() []byte {
	return ionProlog
}

// GetSharedSymbolTable returns the YJ_symbols shared symbol table.
func GetSharedSymbolTable() ion.SharedSymbolTable {
	return sharedSymbolTable
}

// DecodeIon decodes Ion binary data into a Go value using YJ_symbols.
// The prolog should be from GetIonProlog() or from the document's symbol table.
func DecodeIon(prolog, data []byte, v any) error {
	// Strip BVM from data if present and prepend prolog
	ionData := StripIonBVM(data)
	// Create new slice to avoid modifying original prolog's underlying array
	combined := make([]byte, 0, len(prolog)+len(ionData))
	combined = append(combined, prolog...)
	combined = append(combined, ionData...)

	if err := ion.Unmarshal(combined, v, sharedSymbolTable); err != nil {
		return err
	}
	if val, ok := v.(interface{ Validate() error }); ok {
		return val.Validate()
	}
	return nil
}

// DecodeSymbolTable decodes an Ion symbol table from binary data.
func DecodeSymbolTable(data []byte) (ion.SymbolTable, error) {
	r := ion.NewReaderCat(bytes.NewReader(data), ion.NewCatalog(sharedSymbolTable))
	r.Next() // Advance to read the symbol table
	if err := r.Err(); err != nil {
		return nil, err
	}
	return r.SymbolTable(), nil
}

// EncodeIon encodes a Go value to Ion binary using YJ_symbols.
func EncodeIon(v any) ([]byte, error) {
	return ion.MarshalBinary(v, sharedSymbolTable)
}

// IonWriter wraps ion.Writer with KFX-specific helpers.
type IonWriter struct {
	buf    *bytes.Buffer
	writer ion.Writer
}

// NewIonWriter creates a new Ion binary writer with YJ_symbols.
func NewIonWriter() *IonWriter {
	buf := &bytes.Buffer{}
	w := ion.NewBinaryWriter(buf, sharedSymbolTable)
	return &IonWriter{
		buf:    buf,
		writer: w,
	}
}

// NewIonWriterLocalSymbols creates a new Ion binary writer with YJ_symbols and local symbols.
// The local symbols are added to a combined shared symbol table.
func NewIonWriterLocalSymbols(localSymbols []string) *IonWriter {
	buf := &bytes.Buffer{}

	// Create a combined symbol table that includes both YJ_symbols and local symbols
	combinedST := createCombinedSymbolTable(localSymbols)
	w := ion.NewBinaryWriter(buf, combinedST)
	return &IonWriter{
		buf:    buf,
		writer: w,
	}
}

// createCombinedSymbolTable creates a shared symbol table for writing fragment payloads.
// The IDs must match what KFX readers (kfxlib, sync2kindle) expect:
// - IDs 1-851: YJ_symbols ($10 to $860)
// - IDs 852+: local symbols
// Note: Ion system symbols (1-9) are NOT included because KFX readers don't count them.
func createCombinedSymbolTable(localSymbols []string) ion.SharedSymbolTable {
	// Get symbols from YJ_symbols (851 symbols: "$10" to "$860")
	baseSymbols := sharedSymbolTable.Symbols()

	// Build combined symbol list: YJ_symbols + local symbols
	allSymbols := make([]string, 0, len(baseSymbols)+len(localSymbols))

	// Add YJ_symbols (IDs 1-851 in this SharedSymbolTable)
	allSymbols = append(allSymbols, baseSymbols...)

	// Add local symbols (IDs 852+)
	allSymbols = append(allSymbols, localSymbols...)

	return ion.NewSharedSymbolTable("YJ_combined", 1, allSymbols)
}

// Bytes returns the serialized Ion binary data including prolog (BVM + symbol table import).
func (w *IonWriter) Bytes() ([]byte, error) {
	if err := w.writer.Finish(); err != nil {
		return nil, fmt.Errorf("finish ion writer: %w", err)
	}
	return w.buf.Bytes(), nil
}

// RawBytes returns the serialized Ion binary data without the prolog (no BVM, no symbol table).
// This is used for entity payloads which are stored in ENTY blocks.
func (w *IonWriter) RawBytes() ([]byte, error) {
	data, err := w.Bytes()
	if err != nil {
		return nil, err
	}
	return stripIonProlog(data), nil
}

// BytesWithBVM returns Ion data with BVM but without the symbol table import annotation.
// This is used for container_info and format_capabilities blobs which need BVM
// but rely on doc_symbol_table for symbol resolution.
func (w *IonWriter) BytesWithBVM() ([]byte, error) {
	data, err := w.Bytes()
	if err != nil {
		return nil, err
	}
	raw := stripIonProlog(data)
	return PrependIonBVM(raw), nil
}

// stripIonProlog removes the Ion BVM and symbol table annotation from the beginning of Ion data.
// It returns the raw Ion value(s) that follow the symbol table.
func stripIonProlog(data []byte) []byte {
	if len(data) < 4 {
		return data
	}
	// Check for Ion BVM (E0 01 00 EA)
	if data[0] != 0xE0 || data[1] != 0x01 || data[2] != 0x00 || data[3] != 0xEA {
		return data
	}
	pos := 4

	// Skip symbol table annotation wrapper if present
	// Format: EE <VarUInt length> <annot_count> <annot_symbols...> <content>
	// For symbol table: annot is $3 ($ion_symbol_table), content is a struct
	for pos < len(data) {
		typeByte := data[pos]
		typeCode := typeByte >> 4
		lenCode := typeByte & 0x0F

		// Check for annotation wrapper (type 0xE)
		if typeCode != 0xE {
			break
		}

		// Get the total length of this annotation wrapper
		var totalLen int
		var headerLen int
		if lenCode == 0xE {
			// VarUInt length follows
			length, lenBytes := readVarUInt(data[pos+1:])
			totalLen = int(length)
			headerLen = 1 + lenBytes
		} else {
			// Length is in the low nibble
			totalLen = int(lenCode)
			headerLen = 1
		}

		// Check if first annotation is $3 ($ion_symbol_table)
		// After the header, we have annot_length (VarUInt) then annot symbols
		annotStart := pos + headerLen
		if annotStart >= len(data) {
			break
		}

		// Read annotation count/length VarUInt
		annotSymLen, annotSymLenBytes := readVarUInt(data[annotStart:])
		firstAnnotPos := annotStart + annotSymLenBytes

		if firstAnnotPos >= len(data) {
			break
		}

		// Check if it's symbol $3 - Ion encodes symbol IDs as VarUInt
		// $3 would be encoded as 0x83 (high bit set = end, value = 3)
		if annotSymLen >= 1 && data[firstAnnotPos] == 0x83 {
			// This is $ion_symbol_table, skip the entire annotation wrapper
			pos += headerLen + totalLen
			continue
		}

		break
	}

	return data[pos:]
}

// readVarUInt reads a variable-length unsigned integer from Ion binary.
// Returns the value and the number of bytes consumed.
func readVarUInt(data []byte) (uint64, int) {
	var result uint64
	for i, b := range data {
		result = (result << 7) | uint64(b&0x7F)
		if b&0x80 != 0 {
			return result, i + 1
		}
	}
	return result, len(data)
}

// WriteSymbol writes a symbol value by name (e.g., "$409").
func (w *IonWriter) WriteSymbol(name string) error {
	return w.writer.WriteSymbolFromString(name)
}

// WriteSymbolID writes a symbol value by ID.
func (w *IonWriter) WriteSymbolID(id int) error {
	return w.writer.WriteSymbolFromString(fmt.Sprintf("$%d", id))
}

// WriteSymbolBySID writes a symbol value with explicit symbol ID.
// This is used for local symbols that have a fixed ID in the document symbol table.
func (w *IonWriter) WriteSymbolBySID(name string, sid KFXSymbol) error {
	tok := ion.SymbolToken{Text: &name, LocalSID: int64(sid)}
	return w.writer.WriteSymbol(tok)
}

// WriteSymbolField writes a field name by ID (for use in structs).
func (w *IonWriter) WriteSymbolField(id KFXSymbol) error {
	tok := ion.NewSymbolTokenFromString(fmt.Sprintf("$%d", id))
	return w.writer.FieldName(tok)
}

// WriteAnnotation adds an annotation by symbol ID.
func (w *IonWriter) WriteAnnotation(id KFXSymbol) error {
	tok := ion.NewSymbolTokenFromString(fmt.Sprintf("$%d", id))
	return w.writer.Annotation(tok)
}

// WriteInt writes an integer value.
func (w *IonWriter) WriteInt(v int64) error {
	return w.writer.WriteInt(v)
}

// WriteString writes a string value.
func (w *IonWriter) WriteString(v string) error {
	return w.writer.WriteString(v)
}

// WriteBlob writes a blob value.
func (w *IonWriter) WriteBlob(v []byte) error {
	return w.writer.WriteBlob(v)
}

// WriteFloat writes a float64 value.
func (w *IonWriter) WriteFloat(v float64) error {
	return w.writer.WriteFloat(v)
}

// WriteDecimal writes an arbitrary-precision decimal value.
func (w *IonWriter) WriteDecimal(v *ion.Decimal) error {
	return w.writer.WriteDecimal(v)
}

// WriteBool writes a boolean value.
func (w *IonWriter) WriteBool(v bool) error {
	return w.writer.WriteBool(v)
}

// WriteNull writes a null value.
func (w *IonWriter) WriteNull() error {
	return w.writer.WriteNull()
}

// BeginStruct starts a struct.
func (w *IonWriter) BeginStruct() error {
	return w.writer.BeginStruct()
}

// EndStruct ends a struct.
func (w *IonWriter) EndStruct() error {
	return w.writer.EndStruct()
}

// BeginList starts a list.
func (w *IonWriter) BeginList() error {
	return w.writer.BeginList()
}

// EndList ends a list.
func (w *IonWriter) EndList() error {
	return w.writer.EndList()
}

// WriteIntField writes a struct field with an integer value.
func (w *IonWriter) WriteIntField(fieldID KFXSymbol, value int64) error {
	if err := w.WriteSymbolField(fieldID); err != nil {
		return err
	}
	return w.WriteInt(value)
}

// WriteStringField writes a struct field with a string value.
func (w *IonWriter) WriteStringField(fieldID KFXSymbol, value string) error {
	if err := w.WriteSymbolField(fieldID); err != nil {
		return err
	}
	return w.WriteString(value)
}

// WriteSymbolFieldValue writes a struct field with a symbol value.
func (w *IonWriter) WriteSymbolFieldValue(fieldID KFXSymbol, valueID KFXSymbol) error {
	if err := w.WriteSymbolField(fieldID); err != nil {
		return err
	}
	return w.WriteSymbolID(int(valueID))
}

// WriteBlobField writes a struct field with a blob value.
func (w *IonWriter) WriteBlobField(fieldID KFXSymbol, value []byte) error {
	if err := w.WriteSymbolField(fieldID); err != nil {
		return err
	}
	return w.WriteBlob(value)
}

// IonReader wraps ion.Reader with KFX-specific helpers.
type IonReader struct {
	reader       ion.Reader
	localSymbols []string // Local symbols for resolving IDs 852+
}

// NewIonReader creates a new Ion reader from binary data with prolog.
func NewIonReader(prolog, data []byte) *IonReader {
	// Strip BVM from data if present and prepend prolog
	ionData := StripIonBVM(data)
	// Create new slice to avoid modifying original prolog's underlying array
	combined := make([]byte, 0, len(prolog)+len(ionData))
	combined = append(combined, prolog...)
	combined = append(combined, ionData...)
	r := ion.NewReaderCat(bytes.NewReader(combined), ion.NewCatalog(sharedSymbolTable))
	return &IonReader{reader: r}
}

// NewIonReaderWithLocalSymbols creates an Ion reader with prolog and local symbol support.
// This is used to read fragment payloads where symbol IDs 852+ need to be resolved
// to local symbol names. The prolog defines the symbol table context.
func NewIonReaderWithLocalSymbols(prolog, data []byte, localSymbols []string) *IonReader {
	// Strip BVM from data if present and prepend prolog
	ionData := StripIonBVM(data)
	combined := make([]byte, 0, len(prolog)+len(ionData))
	combined = append(combined, prolog...)
	combined = append(combined, ionData...)
	r := ion.NewReaderCat(bytes.NewReader(combined), ion.NewCatalog(sharedSymbolTable))
	return &IonReader{reader: r, localSymbols: localSymbols}
}

// NewIonReaderBytes creates a new Ion reader using the default prolog.
func NewIonReaderBytes(data []byte) *IonReader {
	return NewIonReader(ionProlog, data)
}

// Next advances to the next value.
func (r *IonReader) Next() bool {
	return r.reader.Next()
}

// Type returns the current value type.
func (r *IonReader) Type() ion.Type {
	return r.reader.Type()
}

// Err returns any error that occurred.
func (r *IonReader) Err() error {
	return r.reader.Err()
}

// StepIn steps into a container (struct, list, sexp).
func (r *IonReader) StepIn() error {
	return r.reader.StepIn()
}

// StepOut steps out of a container.
func (r *IonReader) StepOut() error {
	return r.reader.StepOut()
}

// IntValue returns the current value as int64.
func (r *IonReader) IntValue() (int64, error) {
	v, err := r.reader.Int64Value()
	if err != nil {
		return 0, err
	}
	if v == nil {
		return 0, nil
	}
	return *v, nil
}

// StringValue returns the current value as string.
func (r *IonReader) StringValue() (string, error) {
	v, err := r.reader.StringValue()
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", nil
	}
	return *v, nil
}

// BlobValue returns the current value as []byte.
func (r *IonReader) BlobValue() ([]byte, error) {
	return r.reader.ByteValue()
}

// BoolValue returns the current value as bool.
func (r *IonReader) BoolValue() (bool, error) {
	v, err := r.reader.BoolValue()
	if err != nil {
		return false, err
	}
	if v == nil {
		return false, nil
	}
	return *v, nil
}

// SymbolValue returns the current symbol value as string (e.g., "$409" or local symbol name).
func (r *IonReader) SymbolValue() (string, error) {
	tok, err := r.reader.SymbolValue()
	if err != nil {
		return "", err
	}

	// If we have local symbols and this is a local symbol ID (>= 852),
	// resolve it using our localSymbols slice instead of the Ion reader's symbol table.
	// This is needed because the payload was written with IDs 852+ but the doc symbol
	// table uses IDs 861+, causing a 9-ID offset mismatch.
	localStartID := int64(LargestKnownSymbol) + 1 // 852
	if r.localSymbols != nil && tok.LocalSID >= localStartID {
		idx := int(tok.LocalSID - localStartID)
		if idx >= 0 && idx < len(r.localSymbols) {
			return r.localSymbols[idx], nil
		}
	}

	// For non-local symbols, use the text from the Ion reader
	if tok.Text != nil {
		return *tok.Text, nil
	}
	if tok.LocalSID != ion.SymbolIDUnknown {
		return fmt.Sprintf("$%d", tok.LocalSID), nil
	}
	return "", fmt.Errorf("symbol has no text or SID")
}

// FieldName returns the current field name as string (e.g., "$409").
func (r *IonReader) FieldName() (string, error) {
	tok, err := r.reader.FieldName()
	if err != nil {
		return "", err
	}
	if tok == nil {
		return "", fmt.Errorf("no field name")
	}
	if tok.Text != nil {
		return *tok.Text, nil
	}
	if tok.LocalSID != ion.SymbolIDUnknown {
		return fmt.Sprintf("$%d", tok.LocalSID), nil
	}
	return "", fmt.Errorf("field name has no text or SID")
}

// Annotations returns the annotations on the current value as strings.
func (r *IonReader) Annotations() ([]string, error) {
	toks, err := r.reader.Annotations()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(toks))
	for _, tok := range toks {
		if tok.Text != nil {
			names = append(names, *tok.Text)
		} else if tok.LocalSID != ion.SymbolIDUnknown {
			names = append(names, fmt.Sprintf("$%d", tok.LocalSID))
		}
	}
	return names, nil
}

// IsNull returns true if the current value is null.
func (r *IonReader) IsNull() bool {
	return r.reader.IsNull()
}

// SymbolTable returns the reader's current symbol table.
func (r *IonReader) SymbolTable() ion.SymbolTable {
	return r.reader.SymbolTable()
}

// ReadValue reads any Ion value into a generic representation.
func (r *IonReader) ReadValue() (any, error) {
	if r.reader.IsNull() {
		return nil, nil
	}

	switch r.reader.Type() {
	case ion.BoolType:
		return r.BoolValue()
	case ion.IntType:
		return r.IntValue()
	case ion.FloatType:
		v, err := r.reader.FloatValue()
		if err != nil {
			return nil, err
		}
		if v == nil {
			return nil, nil
		}
		return *v, nil
	case ion.DecimalType:
		v, err := r.reader.DecimalValue()
		if err != nil {
			return nil, err
		}
		if v == nil {
			return nil, nil
		}
		// Return as *ion.Decimal to preserve type information for debugging
		return v, nil
	case ion.TimestampType:
		v, err := r.reader.TimestampValue()
		if err != nil {
			return nil, err
		}
		if v == nil {
			return nil, nil
		}
		return v.GetDateTime(), nil
	case ion.StringType:
		return r.StringValue()
	case ion.SymbolType:
		s, err := r.SymbolValue()
		if err != nil {
			return nil, err
		}
		return ReadSymbolValue(s), nil
	case ion.BlobType, ion.ClobType:
		return r.reader.ByteValue()
	case ion.ListType:
		return r.readList()
	case ion.StructType:
		return r.readStruct()
	case ion.SexpType:
		return r.readSexp()
	default:
		return nil, fmt.Errorf("unsupported ion type: %v", r.reader.Type())
	}
}

func (r *IonReader) readList() ([]any, error) {
	if err := r.reader.StepIn(); err != nil {
		return nil, err
	}
	var items []any
	for r.reader.Next() {
		v, err := r.ReadValue()
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	if err := r.reader.StepOut(); err != nil {
		return nil, err
	}
	return items, r.reader.Err()
}

func (r *IonReader) readStruct() (map[string]any, error) {
	if err := r.reader.StepIn(); err != nil {
		return nil, err
	}
	m := make(map[string]any)
	for r.reader.Next() {
		fieldName, err := r.FieldName()
		if err != nil {
			return nil, err
		}
		v, err := r.ReadValue()
		if err != nil {
			return nil, err
		}
		m[fieldName] = v
	}
	if err := r.reader.StepOut(); err != nil {
		return nil, err
	}
	return m, r.reader.Err()
}

func (r *IonReader) readSexp() ([]any, error) {
	return r.readList()
}

// VarUInt reads a variable-length unsigned integer from a reader.
func VarUInt(r io.Reader) (uint64, int, error) {
	var result uint64
	var bytesRead int
	for {
		var b [1]byte
		n, err := r.Read(b[:])
		if err != nil {
			return 0, bytesRead, err
		}
		bytesRead += n
		result = (result << 7) | uint64(b[0]&0x7F)
		if b[0]&0x80 != 0 {
			return result, bytesRead, nil
		}
	}
}

// WriteVarUInt writes a variable-length unsigned integer to a writer.
func WriteVarUInt(w io.Writer, v uint64) (int, error) {
	if v == 0 {
		return w.Write([]byte{0x80})
	}

	var buf [10]byte
	n := 0
	for v > 0 {
		buf[n] = byte(v & 0x7F)
		v >>= 7
		n++
	}

	written := 0
	for i := n - 1; i >= 0; i-- {
		b := buf[i]
		if i == 0 {
			b |= 0x80
		}
		nw, err := w.Write([]byte{b})
		written += nw
		if err != nil {
			return written, err
		}
	}
	return written, nil
}

// HasIonBVM checks if data starts with Ion Binary Version Marker.
func HasIonBVM(data []byte) bool {
	bvmLen := len(ionBVM)
	return len(data) >= bvmLen && bytes.Equal(data[:bvmLen], ionBVM)
}

// StripIonBVM removes the Ion BVM from the beginning of data if present.
func StripIonBVM(data []byte) []byte {
	if HasIonBVM(data) {
		return data[len(ionBVM):]
	}
	return data
}

// PrependIonBVM adds the Ion BVM to the beginning of data if not present.
func PrependIonBVM(data []byte) []byte {
	if HasIonBVM(data) {
		return data
	}
	bvmLen := len(ionBVM)
	result := make([]byte, len(data)+bvmLen)
	copy(result, ionBVM)
	copy(result[bvmLen:], data)
	return result
}
