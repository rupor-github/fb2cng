package azw3

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
)

const (
	compressionNone    = uint16(1)
	compressionPalmDOC = uint16(2)
	compressionHuffdic = uint16(17480)
)

type compressedText struct {
	Mode       string
	Type       uint16
	Records    [][]byte
	Dictionary [][]byte
}

func compileTextRecords(ctx context.Context, chunks []textRecord, mode string) (*compressedText, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := &compressedText{Mode: mode, Type: compressionPalmDOC}
	var dictionary *huffDictionary
	switch mode {
	case "", "standard":
		result.Mode = "standard"
	case "none":
		result.Type = compressionNone
	case "huffdic":
		result.Type = compressionHuffdic
		var err error
		dictionary, err = buildHuffDictionary(ctx, chunks)
		if err != nil {
			return nil, err
		}
		result.Dictionary = dictionary.Records
	default:
		return nil, fmt.Errorf("unsupported AZW3 compression %q: expected none, standard or huffdic", mode)
	}
	for _, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var encoded []byte
		switch result.Type {
		case compressionNone:
			encoded = bytes.Clone(chunk.Data)
		case compressionPalmDOC:
			encoded = compressPalmDOC(chunk.Data)
		case compressionHuffdic:
			encoded = dictionary.compress(chunk.Data)
		}
		encoded = append(encoded, chunk.Overlap...)
		result.Records = append(result.Records, append(encoded, byte(len(chunk.Overlap))))
	}
	return result, nil
}

// compressPalmDOC encodes one independent text record. Matches are 3..10 bytes
// long at distances 1..2047, including overlapping copies. A bounded hash-chain
// search avoids quadratic work on repetitive or adversarial text. Callers split
// the input into at most textRecordSize bytes before calling this function.
func compressPalmDOC(data []byte) []byte {
	const hashSize = 4096
	var heads [hashSize]int
	previous := make([]int, len(data))
	hash := func(pos int) int {
		return int((uint32(data[pos])*251+uint32(data[pos+1]))*251+uint32(data[pos+2])) & (hashSize - 1)
	}
	insert := func(pos int) {
		if pos+2 < len(data) {
			key := hash(pos)
			previous[pos], heads[key] = heads[key], pos+1 // Zero means no candidate.
		}
	}
	result := make([]byte, 0, len(data))
	for pos := 0; pos < len(data); {
		length, distance := 0, 0
		if pos+2 < len(data) {
			limit := min(10, len(data)-pos)
			candidate := heads[hash(pos)] - 1
			for attempts := 0; candidate >= 0 && pos-candidate <= 2047 && attempts < 64; attempts++ {
				matched := 0
				for matched < limit && data[candidate+matched] == data[pos+matched] {
					matched++
				}
				if matched > length {
					length, distance = matched, pos-candidate
					if length == limit {
						break
					}
				}
				candidate = previous[candidate] - 1
			}
		}
		consumed := 1
		switch {
		case length >= 3:
			result = binary.BigEndian.AppendUint16(result, 0x8000|uint16(distance<<3)|uint16(length-3))
			consumed = length
		case data[pos] == ' ' && pos+1 < len(data) && data[pos+1] >= 0x40 && data[pos+1] <= 0x7f:
			result = append(result, data[pos+1]^0x80)
			consumed = 2
		case data[pos] == 0 || data[pos] >= 9 && data[pos] <= 0x7f:
			result = append(result, data[pos])
		default:
			for consumed < 8 && pos+consumed < len(data) {
				value := data[pos+consumed]
				if value == 0 || value >= 9 && value <= 0x7f {
					break
				}
				consumed++
			}
			result = append(result, byte(consumed))
			result = append(result, data[pos:pos+consumed]...)
		}
		for offset := range consumed {
			insert(pos + offset)
		}
		pos += consumed
	}
	return result
}
