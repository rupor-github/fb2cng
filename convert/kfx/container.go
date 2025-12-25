package kfx

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/amazon-ion/ion-go/ion"
)

// Container format constants
const (
	ContainerSignature = "CONT"
	EntitySignature    = "ENTY"
	ContainerVersion   = 2    // We always write v2
	EntityVersion      = 1    // Entity version
	DefaultChunkSize   = 4096 // Default chunk size
	MinContainerLen    = 18   // Minimum CONT header length
	MinEntityLen       = 10   // Minimum ENTY header length
	EntityDirEntrySize = 24   // Size of entity directory entry
)

// containerHeader is the fixed-size CONT header.
type containerHeader struct {
	Signature  [4]byte
	Version    uint16
	Size       uint32
	InfoOffset uint32
	InfoSize   uint32
}

func (c *containerHeader) Validate() error {
	if !bytes.Equal(c.Signature[:], []byte(ContainerSignature)) {
		return fmt.Errorf("wrong signature for KFX container: % X", c.Signature[:])
	}
	if c.Version > ContainerVersion {
		return fmt.Errorf("unsupported KFX container version: %d", c.Version)
	}
	if c.Size < MinContainerLen {
		return fmt.Errorf("invalid KFX container header size: %d", c.Size)
	}
	return nil
}

// containerInfo is the Ion struct at container_info offset.
type containerInfo struct {
	ContainerID string `ion:"$409"`
	ComprType   int    `ion:"$410"`
	DRMScheme   int    `ion:"$411"`
	ChunkSize   int    `ion:"$412"`
	IndexTabOff int    `ion:"$413"`
	IndexTabLen int    `ion:"$414"`
	DocSymOff   int    `ion:"$415"`
	DocSymLen   int    `ion:"$416"`
	FCapabOff   int    `ion:"$594"`
	FCapabLen   int    `ion:"$595"`
}

func (c *containerInfo) Validate() error {
	if c.ComprType != 0 {
		return fmt.Errorf("unsupported KFX container compression type: %d", c.ComprType)
	}
	if c.DRMScheme != 0 {
		return fmt.Errorf("unsupported KFX container DRM: %d", c.DRMScheme)
	}
	return nil
}

// entityHeader is the fixed-size ENTY header.
type entityHeader struct {
	Signature [4]byte
	Version   uint16
	Size      uint32
}

func (e *entityHeader) Validate() error {
	if !bytes.Equal(e.Signature[:], []byte(EntitySignature)) {
		return fmt.Errorf("wrong signature for KFX entity: % X", e.Signature[:])
	}
	if e.Version > EntityVersion {
		return fmt.Errorf("unsupported KFX entity version: %d", e.Version)
	}
	if e.Size < MinEntityLen {
		return fmt.Errorf("invalid KFX entity header size: %d", e.Size)
	}
	return nil
}

// entityInfo is the Ion struct in entity header.
type entityInfo struct {
	ComprType int `ion:"$410"`
	DRMScheme int `ion:"$411"`
}

func (e *entityInfo) Validate() error {
	if e.ComprType != 0 {
		return fmt.Errorf("unsupported KFX entity compression type: %d", e.ComprType)
	}
	if e.DRMScheme != 0 {
		return fmt.Errorf("unsupported KFX entity DRM: %d", e.DRMScheme)
	}
	return nil
}

// indexTableEntry is a single entity directory entry.
type indexTableEntry struct {
	NumID, NumType uint32
	Offset, Size   uint64
}

func (e *indexTableEntry) readFrom(r io.Reader) error {
	return binary.Read(r, binary.LittleEndian, e)
}

// Container represents a parsed KFX container.
type Container struct {
	Version            uint16
	ContainerID        string
	CompressionType    int
	DRMScheme          int
	ChunkSize          int
	GeneratorApp       string
	GeneratorPkg       string
	ContainerFormat    string // "KFX main", "KFX metadata", etc.
	Fragments          *FragmentList
	DocSymbolTable     ion.SymbolTable
	FormatCapabilities any // $593 value if present
}

// NewContainer creates a new empty container.
func NewContainer() *Container {
	return &Container{
		Version:   ContainerVersion,
		ChunkSize: DefaultChunkSize,
		Fragments: NewFragmentList(),
	}
}

// ReadContainer parses a KFX container from bytes.
func ReadContainer(data []byte) (*Container, error) {
	if len(data) < MinContainerLen {
		return nil, fmt.Errorf("container too small: %d bytes", len(data))
	}

	// Read fixed header
	var header containerHeader
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("read container header: %w", err)
	}
	if err := header.Validate(); err != nil {
		return nil, err
	}

	c := NewContainer()
	c.Version = header.Version

	// Read container_info
	if header.InfoSize == 0 {
		return nil, errors.New("no container_info in KFX")
	}

	var contInfo containerInfo
	if err := DecodeIon(GetIonProlog(), data[header.InfoOffset:header.InfoOffset+header.InfoSize], &contInfo); err != nil {
		return nil, fmt.Errorf("decode container_info: %w", err)
	}

	c.ContainerID = contInfo.ContainerID
	c.CompressionType = contInfo.ComprType
	c.DRMScheme = contInfo.DRMScheme
	c.ChunkSize = contInfo.ChunkSize

	// Read document symbol table
	if contInfo.DocSymLen == 0 {
		return nil, errors.New("no document symbols found, unsupported KFX type")
	}

	docSymData := data[contInfo.DocSymOff : contInfo.DocSymOff+contInfo.DocSymLen]
	docSymTab, err := DecodeSymbolTable(docSymData)
	if err != nil {
		return nil, fmt.Errorf("decode document symbol table: %w", err)
	}
	c.DocSymbolTable = docSymTab

	// Use doc symbol table as prolog for entity decoding
	lstProlog := docSymData

	// Read entity directory and entities
	if contInfo.IndexTabLen > 0 {
		indexTabReader := bytes.NewReader(data[contInfo.IndexTabOff : contInfo.IndexTabOff+contInfo.IndexTabLen])
		for {
			var entry indexTableEntry
			if err := entry.readFrom(indexTabReader); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, fmt.Errorf("read entity directory entry: %w", err)
			}

			// Validate entry bounds
			entityStart := uint64(header.Size) + entry.Offset
			entityEnd := entityStart + entry.Size
			if entityEnd > uint64(len(data)) {
				return nil, fmt.Errorf("entity out of bounds: offset=%d size=%d", entry.Offset, entry.Size)
			}

			entityData := data[entityStart:entityEnd]
			frag, err := c.parseEntity(entityData, lstProlog, int(entry.NumType), int(entry.NumID))
			if err != nil {
				return nil, fmt.Errorf("parse entity type=%d id=%d: %w", entry.NumType, entry.NumID, err)
			}

			if err := c.Fragments.Add(frag); err != nil {
				// Duplicate fragment - skip (as per KFXInput behavior)
				continue
			}
		}
	}

	// Parse kfxgen metadata (JSON-like blob after container_info)
	metaStart := header.InfoOffset + header.InfoSize
	if metaStart < header.Size {
		metaData := data[metaStart:header.Size]
		c.parseKfxgenMetadata(metaData)
	}

	// Determine container format
	c.classifyContainerFormat()

	return c, nil
}

// parseEntity parses a single ENTY record.
func (c *Container) parseEntity(data, lstProlog []byte, typeNum, idNum int) (*Fragment, error) {
	if len(data) < MinEntityLen {
		return nil, fmt.Errorf("entity too small: %d bytes", len(data))
	}

	// Read entity header (10 bytes: 4 sig + 2 version + 4 size)
	var header entityHeader
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("read entity header: %w", err)
	}
	if err := header.Validate(); err != nil {
		return nil, err
	}

	// Entity_info starts at byte 10 (fixed header size), not unsafe.Sizeof
	const fixedHeaderSize = 10

	// Read entity_info
	var entInfo entityInfo
	if err := DecodeIon(lstProlog, data[fixedHeaderSize:header.Size], &entInfo); err != nil {
		return nil, fmt.Errorf("decode entity_info: %w", err)
	}

	// Payload is after entity header
	payload := data[header.Size:]

	// Determine fragment type and id
	ftype := typeNum
	fid := idNum

	// If idNum is $348 (Null), this is a root fragment
	if idNum == SymNull {
		fid = ftype
	}

	// Parse payload
	var value any
	if RAW_FRAGMENT_TYPES[ftype] {
		// Raw fragment - store as bytes
		value = RawValue(payload)
	} else {
		// Ion-encoded fragment - read as generic value
		r := NewIonReader(lstProlog, payload)
		if !r.Next() {
			return nil, errors.New("no value in entity payload")
		}
		var err error
		value, err = r.ReadValue()
		if err != nil {
			return nil, fmt.Errorf("read ion value: %w", err)
		}
	}

	return &Fragment{
		FType: ftype,
		FID:   fid,
		Value: value,
	}, nil
}

// parseKfxgenMetadata parses the JSON-like kfxgen metadata blob.
func (c *Container) parseKfxgenMetadata(data []byte) {
	// Remove 0x1B bytes and decode as ASCII
	cleaned := bytes.ReplaceAll(data, []byte{0x1B}, []byte{})
	text := string(cleaned)

	// Convert pseudo-JSON to valid JSON
	// Pattern: {key: "value", key: "value"}
	re := regexp.MustCompile(`(\w+)\s*:`)
	text = re.ReplaceAllString(text, `"$1":`)

	var items []map[string]string
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		return // Ignore errors
	}

	for _, item := range items {
		key := item["key"]
		value := item["value"]
		switch key {
		case "appVersion", "kfxgen_application_version":
			c.GeneratorApp = value
		case "buildVersion", "kfxgen_package_version":
			c.GeneratorPkg = value
		case "kfxgen_acr":
			if c.ContainerID == "" {
				c.ContainerID = value
			}
		}
	}
}

// classifyContainerFormat determines the container format type.
func (c *Container) classifyContainerFormat() {
	// Check for main container types
	mainTypes := []int{SymStoryline, SymSection, SymDocumentData} // $259, $260, $538
	for _, t := range mainTypes {
		if len(c.Fragments.GetByType(t)) > 0 {
			c.ContainerFormat = "KFX main"
			return
		}
	}

	// Check for metadata container types
	metaTypes := []int{SymMetadata, SymContEntityMap, SymBookMetadata} // $258, $419, $490
	for _, t := range metaTypes {
		if len(c.Fragments.GetByType(t)) > 0 {
			c.ContainerFormat = "KFX metadata"
			return
		}
	}

	// Check for attachable
	if len(c.Fragments.GetByType(SymRawMedia)) > 0 {
		c.ContainerFormat = "KFX attachable"
		return
	}

	c.ContainerFormat = "KFX unknown"
}

// WriteContainer serializes a container to bytes.
func (c *Container) WriteContainer() ([]byte, error) {
	// Build entity directory and entity payloads
	var entityDir bytes.Buffer
	var entityPayloads bytes.Buffer

	fragments := c.Fragments.SortedByType()
	for _, frag := range fragments {
		// Skip container-level fragments (except $419)
		if CONTAINER_FRAGMENT_TYPES[frag.FType] && frag.FType != SymContEntityMap {
			continue
		}

		// Serialize entity
		entityData, err := c.serializeEntity(frag)
		if err != nil {
			return nil, fmt.Errorf("serialize entity %s: %w", frag, err)
		}

		// Add to directory
		// Entry: id_idnum (u32), type_idnum (u32), offset (u64), length (u64)
		var entry [EntityDirEntrySize]byte

		// For root fragments, use $348 (Null) as id placeholder
		idNum := frag.FID
		if frag.IsRoot() {
			idNum = SymNull // $348
		}

		WriteLittleEndianU32(entry[0:4], uint32(idNum))
		WriteLittleEndianU32(entry[4:8], uint32(frag.FType))
		WriteLittleEndianU64(entry[8:16], uint64(entityPayloads.Len()))
		WriteLittleEndianU64(entry[16:24], uint64(len(entityData)))
		entityDir.Write(entry[:])

		entityPayloads.Write(entityData)
	}

	// Build doc symbol table blob
	docSymBlob, err := c.buildDocSymbolTable()
	if err != nil {
		return nil, fmt.Errorf("build doc symbol table: %w", err)
	}

	// Build format capabilities blob (for v2)
	var fCapabBlob []byte
	if c.FormatCapabilities != nil {
		fCapabBlob, err = c.buildFormatCapabilities()
		if err != nil {
			return nil, fmt.Errorf("build format capabilities: %w", err)
		}
	}

	// Build kfxgen metadata
	payloadSHA1 := sha1.Sum(entityPayloads.Bytes())
	kfxgenMeta := c.buildKfxgenMetadata(hex.EncodeToString(payloadSHA1[:]))

	// Calculate header layout
	// Fixed header: 18 bytes
	// Then: entity directory, doc symbol table, format capabilities, container_info, kfxgen metadata
	entityDirOffset := uint32(18)
	docSymOffset := entityDirOffset + uint32(entityDir.Len())
	fCapabOffset := docSymOffset + uint32(len(docSymBlob))

	// Build container_info with correct offsets
	containerInfo, err := c.buildContainerInfoWithOffsets(
		entityDirOffset, uint32(entityDir.Len()),
		docSymOffset, uint32(len(docSymBlob)),
		fCapabOffset, uint32(len(fCapabBlob)),
	)
	if err != nil {
		return nil, fmt.Errorf("build container_info: %w", err)
	}

	// Now calculate final offsets
	containerInfoOffset := fCapabOffset + uint32(len(fCapabBlob))
	kfxgenOffset := containerInfoOffset + uint32(len(containerInfo))
	headerLen := kfxgenOffset + uint32(len(kfxgenMeta))

	// Build final buffer
	var buf bytes.Buffer

	// Fixed header (18 bytes)
	buf.WriteString(ContainerSignature)
	var header [14]byte
	WriteLittleEndianU16(header[0:2], ContainerVersion)
	WriteLittleEndianU32(header[2:6], headerLen)
	WriteLittleEndianU32(header[6:10], containerInfoOffset)
	WriteLittleEndianU32(header[10:14], uint32(len(containerInfo)))
	buf.Write(header[:])

	// Entity directory
	buf.Write(entityDir.Bytes())

	// Doc symbol table
	buf.Write(docSymBlob)

	// Format capabilities
	buf.Write(fCapabBlob)

	// Container info
	buf.Write(containerInfo)

	// Kfxgen metadata
	buf.Write(kfxgenMeta)

	// Entity payloads
	buf.Write(entityPayloads.Bytes())

	return buf.Bytes(), nil
}

// serializeEntity serializes a fragment to ENTY format.
func (c *Container) serializeEntity(frag *Fragment) ([]byte, error) {
	var buf bytes.Buffer

	// ENTY signature
	buf.WriteString(EntitySignature)

	// Version (u16)
	var version [2]byte
	WriteLittleEndianU16(version[:], EntityVersion)
	buf.Write(version[:])

	// Placeholder for header_len (u32)
	headerLenPos := buf.Len()
	buf.Write([]byte{0, 0, 0, 0})

	// entity_info (Ion struct with $410=0, $411=0)
	entityInfo, err := c.buildEntityInfo()
	if err != nil {
		return nil, err
	}
	buf.Write(entityInfo)

	// Update header_len
	headerLen := uint32(buf.Len())
	WriteLittleEndianU32(buf.Bytes()[headerLenPos:], headerLen)

	// Payload
	if frag.IsRaw() {
		// Raw fragments store bytes directly
		switch v := frag.Value.(type) {
		case []byte:
			buf.Write(v)
		case RawValue:
			buf.Write(v)
		default:
			return nil, fmt.Errorf("raw fragment %s has non-bytes value: %T", frag, frag.Value)
		}
	} else {
		// Regular fragments are Ion-encoded
		payload, err := c.serializeFragmentValue(frag)
		if err != nil {
			return nil, err
		}
		buf.Write(payload)
	}

	return buf.Bytes(), nil
}

// buildEntityInfo builds the entity_info Ion struct.
func (c *Container) buildEntityInfo() ([]byte, error) {
	w := NewIonWriter()
	if err := w.BeginStruct(); err != nil {
		return nil, err
	}
	if err := w.WriteIntField(SymComprType, 0); err != nil {
		return nil, err
	}
	if err := w.WriteIntField(SymDRMScheme, 0); err != nil {
		return nil, err
	}
	if err := w.EndStruct(); err != nil {
		return nil, err
	}
	return w.BytesWithBVM()
}

// serializeFragmentValue serializes a fragment's value to Ion binary.
func (c *Container) serializeFragmentValue(frag *Fragment) ([]byte, error) {
	w := NewIonWriter()

	// For non-root fragments, the value is wrapped with annotation
	if !frag.IsRoot() {
		if err := w.WriteAnnotation(frag.FType); err != nil {
			return nil, err
		}
	}

	if err := c.writeValue(w, frag.Value); err != nil {
		return nil, err
	}

	return w.BytesWithBVM()
}

// writeValue writes any value to the Ion writer.
func (c *Container) writeValue(w *IonWriter, value any) error {
	switch v := value.(type) {
	case nil:
		return w.WriteNull()
	case bool:
		return w.WriteBool(v)
	case int:
		return w.WriteInt(int64(v))
	case int64:
		return w.WriteInt(v)
	case string:
		return w.WriteString(v)
	case []byte:
		return w.WriteBlob(v)
	case SymbolValue:
		return w.WriteSymbolID(int(v))
	case StructValue:
		return c.writeStruct(w, v)
	case map[int]any:
		return c.writeStruct(w, v)
	case map[string]any:
		return c.writeStructString(w, v)
	case ListValue:
		return c.writeList(w, []any(v))
	case []any:
		return c.writeList(w, v)
	default:
		return fmt.Errorf("unsupported value type: %T", value)
	}
}

func (c *Container) writeStruct(w *IonWriter, m map[int]any) error {
	if err := w.BeginStruct(); err != nil {
		return err
	}
	// Sort keys for deterministic output
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if err := w.WriteSymbolField(k); err != nil {
			return err
		}
		if err := c.writeValue(w, m[k]); err != nil {
			return err
		}
	}
	return w.EndStruct()
}

func (c *Container) writeStructString(w *IonWriter, m map[string]any) error {
	if err := w.BeginStruct(); err != nil {
		return err
	}
	// Sort keys for deterministic output
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		tok := ion.NewSymbolTokenFromString(k)
		if err := w.writer.FieldName(tok); err != nil {
			return err
		}
		if err := c.writeValue(w, m[k]); err != nil {
			return err
		}
	}
	return w.EndStruct()
}

func (c *Container) writeList(w *IonWriter, items []any) error {
	if err := w.BeginList(); err != nil {
		return err
	}
	for _, item := range items {
		if err := c.writeValue(w, item); err != nil {
			return err
		}
	}
	return w.EndList()
}

// buildDocSymbolTable builds the $ion_symbol_table blob.
// For now, we don't add local symbols - the YJ_symbols covers everything we need.
func (c *Container) buildDocSymbolTable() ([]byte, error) {
	w := NewIonWriter()

	// The prolog from NewIonWriter already includes symbol table import
	// We just need to create an empty local symbol table extension
	// Actually, for writing, we can just return the prolog as is

	return w.Bytes()
}

// buildFormatCapabilities builds the $593 format capabilities blob.
func (c *Container) buildFormatCapabilities() ([]byte, error) {
	if c.FormatCapabilities == nil {
		return nil, nil
	}

	w := NewIonWriter()

	// Add $593 annotation
	if err := w.WriteAnnotation(SymFormatCapab); err != nil {
		return nil, err
	}

	if err := c.writeValue(w, c.FormatCapabilities); err != nil {
		return nil, err
	}

	return w.BytesWithBVM()
}

// buildContainerInfoWithOffsets builds container_info with specified offsets.
func (c *Container) buildContainerInfoWithOffsets(
	indexTabOffset, indexTabLen uint32,
	docSymOffset, docSymLen uint32,
	fCapabOffset, fCapabLen uint32,
) ([]byte, error) {
	w := NewIonWriter()

	if err := w.BeginStruct(); err != nil {
		return nil, err
	}

	// $409 container id
	if c.ContainerID != "" {
		if err := w.WriteStringField(SymContainerId, c.ContainerID); err != nil {
			return nil, err
		}
	}

	// $410 compression type
	if err := w.WriteIntField(SymComprType, int64(c.CompressionType)); err != nil {
		return nil, err
	}

	// $411 DRM scheme
	if err := w.WriteIntField(SymDRMScheme, int64(c.DRMScheme)); err != nil {
		return nil, err
	}

	// $412 chunk size
	if err := w.WriteIntField(SymChunkSize, int64(c.ChunkSize)); err != nil {
		return nil, err
	}

	// $413/$414 entity directory
	if err := w.WriteIntField(SymIndexTabOffset, int64(indexTabOffset)); err != nil {
		return nil, err
	}
	if err := w.WriteIntField(SymIndexTabLength, int64(indexTabLen)); err != nil {
		return nil, err
	}

	// $415/$416 doc symbol table
	if docSymLen > 0 {
		if err := w.WriteIntField(SymDocSymOffset, int64(docSymOffset)); err != nil {
			return nil, err
		}
		if err := w.WriteIntField(SymDocSymLength, int64(docSymLen)); err != nil {
			return nil, err
		}
	}

	// $594/$595 format capabilities (v2 only)
	if fCapabLen > 0 {
		if err := w.WriteIntField(SymFCapabOffset, int64(fCapabOffset)); err != nil {
			return nil, err
		}
		if err := w.WriteIntField(SymFCapabLength, int64(fCapabLen)); err != nil {
			return nil, err
		}
	}

	if err := w.EndStruct(); err != nil {
		return nil, err
	}

	return w.BytesWithBVM()
}

// buildKfxgenMetadata builds the kfxgen metadata JSON blob.
func (c *Container) buildKfxgenMetadata(payloadSHA1 string) []byte {
	items := []map[string]string{
		{"key": "kfxgen_package_version", "value": c.GeneratorPkg},
		{"key": "kfxgen_application_version", "value": c.GeneratorApp},
		{"key": "kfxgen_payload_sha1", "value": payloadSHA1},
		{"key": "kfxgen_acr", "value": c.ContainerID},
	}

	data, _ := json.Marshal(items)
	text := string(data)

	// Convert to pseudo-JSON format
	text = strings.ReplaceAll(text, `"key":`, "key:")
	text = strings.ReplaceAll(text, `"value":`, "value:")

	return []byte(text)
}
