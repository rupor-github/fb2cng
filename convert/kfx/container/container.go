package container

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/amazon-ion/ion-go/ion"

	"fbc/convert/kfx/ionutil"
	"fbc/convert/kfx/model"
)

const (
	signatureCONT = "CONT"
	signatureENTY = "ENTY"

	containerVersion = 2
	entityVersion    = 1
)

type containerHeader struct {
	Signature [4]byte
	Version   uint16
	HeaderLen uint32

	ContainerInfoOffset uint32
	ContainerInfoLength uint32
}

type entityHeader struct {
	Signature [4]byte
	Version   uint16
	HeaderLen uint32
}

type indexTableEntry struct {
	NumID, NumType uint32
	Offset, Size   uint64
}

// PackParams are inputs to Pack().
//
// This packer follows KFXInput's container layout (see kfxlib/kfx_container.py)
// closely because offsets/lengths are validated by the decoder.
type PackParams struct {
	ContainerID string

	KfxgenApplicationVersion string
	KfxgenPackageVersion     string

	// DocumentSymbols is the BVM+LST blob (local symbol table datagram).
	DocumentSymbols []byte
	// FormatCapabilities is the Ion value that will be stored annotated as $593.
	FormatCapabilities any

	// Prolog is the parsed prolog (doc symbols) used to serialize fragments.
	Prolog *ionutil.Prolog

	// Fragments are entity fragments (everything except container fragments $270,
	// $593, $ion_symbol_table). The $419 fragment MUST be included here.
	Fragments []model.Fragment
}

// Pack creates a single-file KFX "CONT" container.
//
// It matches KFXInput's serializer order:
// header -> entity_table -> doc_symbols -> format_capabilities -> container_info -> kfxgen_info -> entity_data
func Pack(p *PackParams) ([]byte, error) {
	if p.Prolog == nil {
		return nil, fmt.Errorf("missing prolog")
	}
	if len(p.DocumentSymbols) == 0 {
		return nil, fmt.Errorf("missing document symbols")
	}

	st := p.Prolog.LST

	formatCapsBytes, err := ionutil.MarshalAnnotatedPayload(
		p.FormatCapabilities,
		[]ion.SymbolToken{ion.NewSymbolTokenFromString("$593")},
		p.Prolog,
	)
	if err != nil {
		return nil, fmt.Errorf("marshal format_capabilities: %w", err)
	}

	entityInfoBytes, err := ionutil.MarshalPayload(map[string]any{"$410": int64(0), "$411": int64(0)}, p.Prolog)
	if err != nil {
		return nil, fmt.Errorf("marshal entity_info: %w", err)
	}

	// Build entity_data + entity_table.
	entityData := bytes.Buffer{}
	entityTable := bytes.Buffer{}
	entityOffset := uint64(0)

	addTableRow := func(idNum, typeNum uint32, off uint64, size uint64) error {
		if err := binary.Write(&entityTable, binary.LittleEndian, idNum); err != nil {
			return err
		}
		if err := binary.Write(&entityTable, binary.LittleEndian, typeNum); err != nil {
			return err
		}
		if err := binary.Write(&entityTable, binary.LittleEndian, off); err != nil {
			return err
		}
		if err := binary.Write(&entityTable, binary.LittleEndian, size); err != nil {
			return err
		}
		return nil
	}

	for _, fr := range p.Fragments {
		// NOTE: For KFXInput compatibility we avoid storing IonAnnotations inside
		// entity payloads. Fragment identity is solely from entity table (id/type).
		idName := fr.FID

		id, ok := st.FindByName(idName)
		if !ok {
			return nil, fmt.Errorf("id symbol not in doc LST: %q", idName)
		}
		typ, ok := st.FindByName(fr.FType)
		if !ok {
			return nil, fmt.Errorf("type symbol not in doc LST: %q", fr.FType)
		}

		var payload []byte
		switch fr.FType {
		case "$417", "$418":
			b, ok := fr.Value.([]byte)
			if !ok {
				return nil, fmt.Errorf("raw fragment %s must be []byte", fr.FType)
			}
			payload = b
		default:
			payload, err = ionutil.MarshalPayload(fr.Value, p.Prolog)
			if err != nil {
				return nil, fmt.Errorf("marshal fragment %s/%s: %w", fr.FID, fr.FType, err)
			}
		}

		// Build ENTY.
		enty := bytes.Buffer{}
		eh := entityHeader{}
		copy(eh.Signature[:], []byte(signatureENTY))
		eh.Version = entityVersion
		eh.HeaderLen = 0
		if err := binary.Write(&enty, binary.LittleEndian, &eh); err != nil {
			return nil, err
		}
		if _, err := enty.Write(entityInfoBytes); err != nil {
			return nil, err
		}

		// Patch header_len.
		entyBytes := enty.Bytes()
		hdrLen := uint32(len(entyBytes))
		binary.LittleEndian.PutUint32(entyBytes[6:10], hdrLen)

		if _, err := enty.Write(payload); err != nil {
			return nil, err
		}

		serialized := enty.Bytes()
		if _, err := entityData.Write(serialized); err != nil {
			return nil, err
		}

		if err := addTableRow(uint32(id), uint32(typ), entityOffset, uint64(len(serialized))); err != nil {
			return nil, err
		}
		entityOffset += uint64(len(serialized))
	}

	// Header with placeholders.
	out := bytes.Buffer{}
	h := containerHeader{}
	copy(h.Signature[:], []byte(signatureCONT))
	h.Version = containerVersion
	if err := binary.Write(&out, binary.LittleEndian, &h); err != nil {
		return nil, err
	}

	// container_info (unannotated IonStruct), with offsets pointing at blocks.
	containerInfo := map[string]any{
		"$409": p.ContainerID,
		"$410": int64(0),
		"$411": int64(0),
		"$412": int64(4096),
		"$413": int64(out.Len()),
		"$414": int64(entityTable.Len()),
	}

	// entity_table
	if _, err := out.Write(entityTable.Bytes()); err != nil {
		return nil, err
	}

	// doc_symbols
	containerInfo["$415"] = int64(out.Len())
	containerInfo["$416"] = int64(len(p.DocumentSymbols))
	if _, err := out.Write(p.DocumentSymbols); err != nil {
		return nil, err
	}

	// format_capabilities
	containerInfo["$594"] = int64(out.Len())
	containerInfo["$595"] = int64(len(formatCapsBytes))
	if _, err := out.Write(formatCapsBytes); err != nil {
		return nil, err
	}

	// container_info block at the end of header area
	containerInfoBytes, err := ionutil.MarshalPayload(containerInfo, p.Prolog)
	if err != nil {
		return nil, fmt.Errorf("marshal container_info: %w", err)
	}
	containerInfoOffset := out.Len()
	if _, err := out.Write(containerInfoBytes); err != nil {
		return nil, err
	}

	// kfxgen_info JSON (must be within header area)
	kfxgenInfo := []map[string]any{
		{"key": "kfxgen_package_version", "value": p.KfxgenPackageVersion},
		{"key": "kfxgen_application_version", "value": p.KfxgenApplicationVersion},
		{"key": "kfxgen_payload_sha1", "value": fmt.Sprintf("%x", sha1.Sum(entityData.Bytes()))},
		{"key": "kfxgen_acr", "value": p.ContainerID},
	}
	kfxgenInfoBytes, err := json.Marshal(kfxgenInfo)
	if err != nil {
		return nil, err
	}
	if _, err := out.Write(kfxgenInfoBytes); err != nil {
		return nil, err
	}

	headerLen := out.Len()

	// Patch header.
	buf := out.Bytes()
	binary.LittleEndian.PutUint32(buf[6:10], uint32(headerLen))
	binary.LittleEndian.PutUint32(buf[10:14], uint32(containerInfoOffset))
	binary.LittleEndian.PutUint32(buf[14:18], uint32(len(containerInfoBytes)))

	// entity_data
	if _, err := out.Write(entityData.Bytes()); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// Write writes the packed container to w.
func Write(w io.Writer, data []byte) error {
	_, err := w.Write(data)
	return err
}
