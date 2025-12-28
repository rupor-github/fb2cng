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
	_ = binary.Write(buf, binary.BigEndian, uint16(0x10)) // length
	buf.Write(jfif)
	_ = binary.Write(buf, binary.BigEndian, uint8(dpit))
	_ = binary.Write(buf, binary.BigEndian, uint16(xdensity))
	_ = binary.Write(buf, binary.BigEndian, uint16(ydensity))
	_ = binary.Write(buf, binary.BigEndian, uint16(0)) // no thumbnail segment
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
