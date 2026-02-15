package images

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/jpeg"
)

type DpiType uint8

const (
	DpiNoUnits DpiType = iota
	DpiPxPerInch
	DpiPxPerSm
)

// EnsureJFIFAPP0 inserts JFIF APP0 marker segment if it is missing.
// This is required for some Kindle devices.
func EnsureJFIFAPP0(jpegData []byte, dpit DpiType, xdensity, ydensity int16) ([]byte, bool, error) {
	if len(jpegData) < 4 {
		return nil, false, errors.New("jpeg too small")
	}

	// Must start with SOI marker.
	if jpegData[0] != 0xFF || jpegData[1] != 0xD8 {
		return nil, false, errors.New("not a jpeg")
	}

	marker := []byte{0xFF, 0xE0}                             // APP0 segment marker
	jfif := []byte{0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0x02} // jfif + version

	// If JFIF APP0 segment is already there - do not do anything.
	if jpegData[2] == marker[0] && jpegData[3] == marker[1] {
		return jpegData, false, nil
	}

	buf := new(bytes.Buffer)
	buf.Write(jpegData[:2])
	buf.Write(marker)

	// Build JFIF APP0 segment body (16 bytes, big-endian).
	var seg [16]byte
	binary.BigEndian.PutUint16(seg[0:2], 0x10)               // segment length (16 including these 2 bytes)
	copy(seg[2:9], jfif)                                     // "JFIF\0" + version
	seg[9] = uint8(dpit)                                     // density units
	binary.BigEndian.PutUint16(seg[10:12], uint16(xdensity)) // X density
	binary.BigEndian.PutUint16(seg[12:14], uint16(ydensity)) // Y density
	// seg[14:16]: thumbnail dimensions (0x00, 0x00) — already zero from array init.
	buf.Write(seg[:])
	buf.Write(jpegData[2:])
	return buf.Bytes(), true, nil
}

func EncodeJPEGWithDPI(img image.Image, quality int, dpit DpiType, xdensity, ydensity int16) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	out, _, err := EnsureJFIFAPP0(buf.Bytes(), dpit, xdensity, ydensity)
	if err != nil {
		return nil, err
	}
	return out, nil
}
