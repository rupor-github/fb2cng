package container

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/amazon-ion/ion-go/ion"

	"fbc/convert/kfx/model"
	"fbc/convert/kfx/symbols"
)

var ionBVM = []byte{0xE0, 0x01, 0x00, 0xEA}

type symtabImport struct {
	Name  string `ion:"name"`
	Ver   int64  `ion:"version"`
	MaxID int64  `ion:"max_id"`
}

type symtabDoc struct {
	Imports []symtabImport `ion:"imports"`
	Symbols []string       `ion:"symbols"`
}

type entityTableRow struct {
	ID     uint32
	Type   uint32
	Offset uint64
	Size   uint64
}

// Unpacked is a human-oriented view of a KFX container.
//
// It is intended for debug tooling (kfxdump) to compare with our own output.
// The Value of each Fragment is either decoded Ion (map/[]/scalars) or raw []byte
// for media fragments ($417/$418).
//
// NOTE: This is not a complete KFX implementation.
// It is just enough to inspect and diff KFX files.
//
// (linters are intentionally silenced: this package is internal debug plumbing.)
//
//nolint:revive // name is fine for debugging API
//nolint:stylecheck // name is fine for debugging API
//nolint:golint
type Unpacked struct {
	ContainerID         string
	DocumentSymbols     []byte
	FormatCapabilities  any
	ContainerInfo       map[string]any
	Fragments           []model.Fragment
	HeaderLen           uint32
	EntityTableOffset   int
	EntityTableLen      int
	DocSymbolsOffset    int
	FormatCapsOffset    int
	ContainerInfoOffset int
}

// Unpack parses a single-file KFX "CONT" container produced by Kindle tooling.
func Unpack(data []byte) (*Unpacked, error) {
	if len(data) < binary.Size(containerHeader{}) {
		return nil, fmt.Errorf("kfx: too small")
	}

	var h containerHeader
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	if string(h.Signature[:]) != signatureCONT {
		return nil, fmt.Errorf("kfx: invalid signature %q", string(h.Signature[:]))
	}
	if h.Version != containerVersion {
		return nil, fmt.Errorf("kfx: unsupported version %d", h.Version)
	}
	if int(h.HeaderLen) > len(data) {
		return nil, fmt.Errorf("kfx: header_len out of range")
	}

	headerStart := binary.Size(containerHeader{})
	headerArea := data[:h.HeaderLen]
	bvmOffsets := findAllBVMOffsets(headerArea[headerStart:])
	if len(bvmOffsets) < 2 {
		return nil, fmt.Errorf("kfx: cannot locate document symbols")
	}
	for i := range bvmOffsets {
		bvmOffsets[i] += headerStart
	}

	// KFX stores several Ion BVM-prefixed blocks in the header area.
	// In both Kindle output and our own packer, the first one is document symbols.
	docSymbolsOffset := bvmOffsets[0]
	formatCapsOffset := bvmOffsets[1]

	containerInfoOffset := int(h.ContainerInfoOffset)
	containerInfoEnd := int(h.ContainerInfoOffset + h.ContainerInfoLength)
	if containerInfoOffset < 0 || containerInfoEnd > len(data) {
		return nil, fmt.Errorf("kfx: container_info out of range")
	}
	if containerInfoOffset < int(h.HeaderLen) {
		// OK. container_info should be within header area.
	}

	docSymbolsBytes := headerArea[docSymbolsOffset:formatCapsOffset]

	// Determine format_capabilities slice.
	formatCapsEnd := containerInfoOffset
	if formatCapsEnd <= formatCapsOffset {
		// Fall back to the next BVM, if present.
		if len(bvmOffsets) > 2 {
			formatCapsEnd = bvmOffsets[2]
		} else {
			formatCapsEnd = int(h.HeaderLen)
		}
	}
	formatCapsBytes := headerArea[formatCapsOffset:formatCapsEnd]

	lst, _, prologBytes, sys, err := buildDecodeContextFromDocSymbols(docSymbolsBytes)
	if err != nil {
		return nil, err
	}

	// Decode container_info.
	containerInfoBytes := data[containerInfoOffset:containerInfoEnd]
	ciFull, err := withProlog(prologBytes, containerInfoBytes)
	if err != nil {
		return nil, fmt.Errorf("kfx: container_info: %w", err)
	}
	var containerInfo map[string]any
	if err := sys.Unmarshal(ciFull, &containerInfo); err != nil {
		return nil, fmt.Errorf("kfx: decode container_info: %w", err)
	}

	containerID, _ := containerInfo["$409"].(string)

	// Decode format_capabilities.
	var formatCaps any
	fcFull, err := withProlog(prologBytes, formatCapsBytes)
	if err == nil {
		_ = sys.Unmarshal(fcFull, &formatCaps)
	}

	// Entity table is between the fixed header and doc symbols.
	entityTableOffset := headerStart
	entityTableLen := docSymbolsOffset - headerStart
	if entityTableLen < 0 {
		return nil, fmt.Errorf("kfx: entity_table out of range")
	}
	if entityTableLen%24 != 0 {
		return nil, fmt.Errorf("kfx: entity_table length %d not multiple of 24", entityTableLen)
	}

	rows := make([]entityTableRow, 0, entityTableLen/24)
	for i := 0; i < entityTableLen; i += 24 {
		b := headerArea[entityTableOffset+i : entityTableOffset+i+24]
		rows = append(rows, entityTableRow{
			ID:     binary.LittleEndian.Uint32(b[0:4]),
			Type:   binary.LittleEndian.Uint32(b[4:8]),
			Offset: binary.LittleEndian.Uint64(b[8:16]),
			Size:   binary.LittleEndian.Uint64(b[16:24]),
		})
	}

	entityDataOffset := int(h.HeaderLen)
	fragments := make([]model.Fragment, 0, len(rows))

	for _, r := range rows {
		idName := sidToText(lst, r.ID)
		typeName := sidToText(lst, r.Type)

		fid := idName
		if idName == "$348" {
			fid = typeName
		}

		start := entityDataOffset + int(r.Offset)
		end := start + int(r.Size)
		if start < entityDataOffset || end > len(data) || start > end {
			return nil, fmt.Errorf("kfx: entity %s/%s out of range", fid, typeName)
		}

		enty := data[start:end]
		if len(enty) < binary.Size(entityHeader{}) {
			return nil, fmt.Errorf("kfx: entity %s/%s too small", fid, typeName)
		}

		var eh entityHeader
		if err := binary.Read(bytes.NewReader(enty), binary.LittleEndian, &eh); err != nil {
			return nil, err
		}
		if string(eh.Signature[:]) != signatureENTY {
			return nil, fmt.Errorf("kfx: invalid entity signature %q", string(eh.Signature[:]))
		}
		if eh.Version != entityVersion {
			return nil, fmt.Errorf("kfx: unsupported entity version %d", eh.Version)
		}
		if int(eh.HeaderLen) > len(enty) {
			return nil, fmt.Errorf("kfx: entity header_len out of range")
		}
		payload := enty[eh.HeaderLen:]

		var v any
		switch typeName {
		case "$417", "$418":
			v = payload
		default:
			full, err := withProlog(prologBytes, payload)
			if err != nil {
				return nil, fmt.Errorf("kfx: fragment %s/%s: %w", fid, typeName, err)
			}
			if err := sys.Unmarshal(full, &v); err != nil {
				return nil, fmt.Errorf("kfx: decode fragment %s/%s: %w", fid, typeName, err)
			}
		}

		fragments = append(fragments, model.Fragment{FID: fid, FType: typeName, Value: v})
	}

	return &Unpacked{
		ContainerID:         containerID,
		DocumentSymbols:     docSymbolsBytes,
		FormatCapabilities:  formatCaps,
		ContainerInfo:       containerInfo,
		Fragments:           fragments,
		HeaderLen:           h.HeaderLen,
		EntityTableOffset:   entityTableOffset,
		EntityTableLen:      entityTableLen,
		DocSymbolsOffset:    docSymbolsOffset,
		FormatCapsOffset:    formatCapsOffset,
		ContainerInfoOffset: containerInfoOffset,
	}, nil
}

func findAllBVMOffsets(b []byte) []int {
	out := make([]int, 0, 8)
	for i := 0; ; {
		j := bytes.Index(b[i:], ionBVM)
		if j < 0 {
			break
		}
		pos := i + j
		out = append(out, pos)
		i = pos + 1
	}
	return out
}

func sidToText(st ion.SymbolTable, sid uint32) string {
	if st == nil {
		return fmt.Sprintf("$%d", sid)
	}
	if s, ok := st.FindByID(uint64(sid)); ok {
		return s
	}
	return fmt.Sprintf("$%d", sid)
}

func normalizeYJMaxID(maxID uint64) uint64 {
	// Known YJ_symbols table size from Kindle Previewer / KFXInput (b.jad): $10..$851.
	const yjMaxKnown = uint64(851)
	if maxID <= yjMaxKnown {
		return maxID
	}

	// Some producers write import.max_id including the 9 system symbols.
	sys := uint64(len(ion.V1SystemSymbolTable.Symbols()))
	if maxID > sys && maxID-sys == yjMaxKnown {
		return yjMaxKnown
	}

	// Clamp to what we know; otherwise local symbol IDs shift and kfxdump output becomes unstable.
	return yjMaxKnown
}

func buildDecodeContextFromDocSymbols(docSymbols []byte) (ion.SymbolTable, ion.SharedSymbolTable, []byte, ion.System, error) {
	var ds symtabDoc
	// Case A: our own packer stores a single Ion value (no embedded LST).
	if err := ion.Unmarshal(docSymbols, &ds); err == nil && len(ds.Imports) > 0 {
		yjMax := uint64(0)
		for _, imp := range ds.Imports {
			if imp.Name == symbols.YJSymbolsName {
				yjMax = uint64(imp.MaxID)
				break
			}
		}
		if yjMax == 0 {
			return nil, nil, nil, ion.System{}, fmt.Errorf("kfx: no %s import in document symbols", symbols.YJSymbolsName)
		}
		yjMax = normalizeYJMaxID(yjMax)
		yjSST := symbols.SharedYJSymbols(yjMax)
		lstb := ion.NewSymbolTableBuilder(yjSST)
		for _, s := range ds.Symbols {
			_, _ = lstb.Add(s)
		}
		lst := lstb.Build()
		prologBytes, err := ion.MarshalBinaryLST(nil, lst)
		if err != nil {
			return nil, nil, nil, ion.System{}, err
		}
		if len(prologBytes) == 0 || prologBytes[len(prologBytes)-1] != 0x0F {
			return nil, nil, nil, ion.System{}, fmt.Errorf("kfx: unexpected prolog trailer")
		}
		prologBytes = prologBytes[:len(prologBytes)-1]
		sys := ion.System{Catalog: ion.NewCatalog(yjSST)}
		return lst, yjSST, prologBytes, sys, nil
	}

	// Case B: Kindle/KFXInput stores a full symbol-table datagram.
	r := ion.NewReaderBytes(docSymbols)
	for r.Next() {
		// drain
	}
	if err := r.Err(); err != nil {
		return nil, nil, nil, ion.System{}, fmt.Errorf("kfx: decode document symbols: %w", err)
	}
	st := r.SymbolTable()
	if st == nil {
		return nil, nil, nil, ion.System{}, fmt.Errorf("kfx: missing symbol table")
	}

	yjImportMax := uint64(0)
	for _, imp := range st.Imports() {
		if imp != nil && imp.Name() == symbols.YJSymbolsName {
			yjImportMax = imp.MaxID()
			break
		}
	}
	if yjImportMax == 0 {
		return nil, nil, nil, ion.System{}, fmt.Errorf("kfx: no %s import in document symbols", symbols.YJSymbolsName)
	}

	// ion-go interprets import.max_id as "symbol count" (Ion spec), but KFX stores it
	// including the 9 system symbols (KFXInput compensates by subtracting 9).
	const yjSymbolCount = uint64(842) // "$10".."$851"
	sysCount := uint64(len(ion.V1SystemSymbolTable.Symbols()))
	yjCount := yjImportMax
	if yjCount > yjSymbolCount && yjCount > sysCount {
		yjCount -= sysCount
	}
	if yjCount > yjSymbolCount {
		yjCount = yjSymbolCount
	}

	yjSST := symbols.SharedYJSymbols(sysCount + yjCount)
	lst := ion.NewLocalSymbolTable([]ion.SharedSymbolTable{yjSST}, st.Symbols())

	prologBytes, err := ion.MarshalBinaryLST(nil, lst)
	if err != nil {
		return nil, nil, nil, ion.System{}, err
	}
	if len(prologBytes) == 0 || prologBytes[len(prologBytes)-1] != 0x0F {
		return nil, nil, nil, ion.System{}, fmt.Errorf("kfx: unexpected prolog trailer")
	}
	prologBytes = prologBytes[:len(prologBytes)-1]

	sys := ion.System{Catalog: ion.NewCatalog(yjSST)}
	return lst, yjSST, prologBytes, sys, nil
}

func withProlog(prolog, payload []byte) ([]byte, error) {
	if len(payload) < len(ionBVM) || !bytes.Equal(payload[:len(ionBVM)], ionBVM) {
		return nil, fmt.Errorf("missing BVM")
	}

	out := make([]byte, 0, len(prolog)+len(payload)-len(ionBVM))
	out = append(out, prolog...)
	out = append(out, payload[len(ionBVM):]...)
	return out, nil
}
