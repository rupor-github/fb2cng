// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
	"os"
)

// ParseFont parses a font from raw bytes, auto-detecting the format.
// Supports TTF, OTF, and WOFF1 fonts.
func ParseFont(data []byte) (Face, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("font data too short")
	}
	sig := binary.BigEndian.Uint32(data[0:4])
	if sig == woffMagic {
		ttfData, err := decodeWOFF(data)
		if err != nil {
			return nil, fmt.Errorf("decode WOFF: %w", err)
		}
		return ParseTTF(ttfData)
	}
	return ParseTTF(data)
}

// LoadFont reads and parses a font file from disk, auto-detecting the format.
// Supports TTF, OTF, and WOFF1 fonts.
func LoadFont(path string) (Face, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read font file: %w", err)
	}
	return ParseFont(data)
}
