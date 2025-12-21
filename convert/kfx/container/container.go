package container

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/amazon-ion/ion-go/ion"

	"fbc/convert/kfx"
	"fbc/convert/kfx/ionutil"
)

const (
	signatureCONT = "CONT"
	signatureENTY = "ENTY"

	containerVersion = 2
	entityVersion    = 1
)

type containerHeader struct {
	Signature  [4]byte
	Version    uint16
	HeaderSize uint32

	InfoOffset uint32
	InfoSize   uint32
}

type entityHeader struct {
	Signature [4]byte
	Version   uint16
	Size      uint32
}

type indexTableEntry struct {
	NumID, NumType uint32
	Offset, Size   uint64
}

// PackParams are inputs to Pack(). This is intentionally low-level: higher
// layers should build fragments and decide symbol tables.
type PackParams struct {
	ContainerID string

	// ContainerInfo is the Ion $270 value (unannotated) that will be stored at InfoOffset.
	ContainerInfo any
	// DocumentSymbols is the BVM+LST blob (local symbol table datagram).
	DocumentSymbols []byte
	// FormatCapabilities is the Ion $593 value.
	FormatCapabilities any

	// Prolog is the parsed prolog (doc symbols) used to serialize fragments.
	Prolog *ionutil.Prolog

	Fragments []kfx.Fragment
}

// Pack creates a single-file KFX "CONT" container.
//
// NOTE: This is a scaffolding packer. It writes a structurally-correct container
// (header + container_info + kfxgen_info + doc symbols + format capabilities +
// entity index + entities). Producing a *valid* KFX book is handled by fragment
// builders and strict field choices.
func Pack(p *PackParams) ([]byte, error) {
	if p.Prolog == nil {
		return nil, fmt.Errorf("missing prolog")
	}
	if len(p.DocumentSymbols) == 0 {
		return nil, fmt.Errorf("missing document symbols")
	}

	// Serialize container_info and format_capabilities as standalone Ion values.
	containerInfoBytes, err := ionutil.MarshalPayload(p.ContainerInfo, p.Prolog)
	if err != nil {
		return nil, fmt.Errorf("marshal container_info: %w", err)
	}
	formatCapabilitiesBytes, err := ionutil.MarshalPayload(p.FormatCapabilities, p.Prolog)
	if err != nil {
		return nil, fmt.Errorf("marshal format_capabilities: %w", err)
	}

	// Build entity payloads.
	// KFX entity payload is BVM + (annotated fragment value).
	type packedEntity struct {
		idNum   uint32
		typeNum uint32
		data    []byte
	}

	st := p.Prolog.LST
	packed := make([]packedEntity, 0, len(p.Fragments))
	for _, fr := range p.Fragments {
		ann := []ion.SymbolToken{
			ion.NewSymbolTokenFromString(fr.FID),
			ion.NewSymbolTokenFromString(fr.FType),
		}
		payload, err := ionutil.MarshalAnnotatedPayload(fr.Value, ann, p.Prolog)
		if err != nil {
			return nil, fmt.Errorf("marshal fragment %s/%s: %w", fr.FID, fr.FType, err)
		}

		id, ok := st.FindByName(fr.FID)
		if !ok {
			return nil, fmt.Errorf("fid symbol not in doc LST: %q", fr.FID)
		}
		typ, ok := st.FindByName(fr.FType)
		if !ok {
			return nil, fmt.Errorf("ftype symbol not in doc LST: %q", fr.FType)
		}

		// KFX index table uses numeric IDs *excluding* the system table.
		id -= ion.V1SystemSymbolTable.MaxID()
		typ -= ion.V1SystemSymbolTable.MaxID()

		packed = append(packed, packedEntity{
			idNum:   uint32(id),
			typeNum: uint32(typ),
			data:    payload,
		})
	}

	// Stable sort makes output deterministic.
	sort.Slice(packed, func(i, j int) bool {
		if packed[i].typeNum != packed[j].typeNum {
			return packed[i].typeNum < packed[j].typeNum
		}
		return packed[i].idNum < packed[j].idNum
	})

	// Serialize entities (ENTY + payload).
	// TODO: add entity_info when needed.
	entitiesBuf := bytes.Buffer{}
	index := make([]indexTableEntry, 0, len(packed))

	for _, e := range packed {
		start := uint64(entitiesBuf.Len())

		eh := entityHeader{}
		copy(eh.Signature[:], []byte(signatureENTY))
		eh.Version = entityVersion
		eh.Size = uint32(binary.Size(eh))
		if err := binary.Write(&entitiesBuf, binary.LittleEndian, &eh); err != nil {
			return nil, err
		}

		if _, err := entitiesBuf.Write(e.data); err != nil {
			return nil, err
		}

		index = append(index, indexTableEntry{
			NumID:   e.idNum,
			NumType: e.typeNum,
			Offset:  start,
			Size:    uint64(binary.Size(eh)) + uint64(len(e.data)),
		})
	}

	indexBuf := bytes.Buffer{}
	for _, it := range index {
		if err := binary.Write(&indexBuf, binary.LittleEndian, &it); err != nil {
			return nil, err
		}
	}

	// Build kfxgen_info JSON payload (kfxlib expects JSON array of {key,value}).
	// TODO: include kfxgen_payload_sha1 and kfxgen_acr when container_info offsets are fixed.
	kfxgenInfo := []map[string]any{
		{"key": "appVersion", "value": "fb2cng"},
		{"key": "buildVersion", "value": "dev"},
	}
	kfxgenInfoBytes, err := json.Marshal(kfxgenInfo)
	if err != nil {
		return nil, err
	}

	out := bytes.Buffer{}

	h := containerHeader{}
	copy(h.Signature[:], []byte(signatureCONT))
	h.Version = containerVersion
	h.HeaderSize = uint32(binary.Size(h))

	// Header is immediately followed by container info.
	h.InfoOffset = h.HeaderSize
	h.InfoSize = uint32(len(containerInfoBytes))

	if err := binary.Write(&out, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	if _, err := out.Write(containerInfoBytes); err != nil {
		return nil, err
	}
	if _, err := out.Write(kfxgenInfoBytes); err != nil {
		return nil, err
	}
	if _, err := out.Write(p.DocumentSymbols); err != nil {
		return nil, err
	}
	if _, err := out.Write(formatCapabilitiesBytes); err != nil {
		return nil, err
	}
	if _, err := out.Write(indexBuf.Bytes()); err != nil {
		return nil, err
	}
	if _, err := out.Write(entitiesBuf.Bytes()); err != nil {
		return nil, err
	}

	_ = sha1.Sum(out.Bytes())
	return out.Bytes(), nil
}

// Write writes the packed container to w.
func Write(w io.Writer, data []byte) error {
	_, err := w.Write(data)
	return err
}
