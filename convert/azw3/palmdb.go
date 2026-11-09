package azw3

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// writePalmDB writes one standalone BOOK/MOBI database. Layout is checked before
// any bytes are written; callers must still stage output to handle I/O failures.
func writePalmDB(ctx context.Context, w io.Writer, name string, records [][]byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(name) == 0 || len(name) > 31 {
		return errors.New("PalmDB name must contain 1 to 31 ASCII bytes")
	}
	for _, ch := range []byte(name) {
		if ch < 0x20 || ch > 0x7e {
			return errors.New("PalmDB name must contain printable ASCII only")
		}
	}
	if len(records) == 0 || len(records) > math.MaxUint16 {
		return errors.New("PalmDB requires 1 to 65535 records")
	}
	const palmHeaderSize = 78
	header := make([]byte, palmHeaderSize+8*len(records)+2)
	copy(header, name)
	copy(header[60:], "BOOKMOBI")
	binary.BigEndian.PutUint32(header[68:], uint32(2*len(records)+1))
	binary.BigEndian.PutUint16(header[76:], uint16(len(records)))
	offset := uint64(len(header))
	for i, record := range records {
		if len(record) == 0 {
			return fmt.Errorf("PalmDB record %d is empty", i)
		}
		end := offset + uint64(len(record))
		if end > math.MaxUint32 {
			return errors.New("PalmDB exceeds 32-bit file offsets")
		}
		entry := header[palmHeaderSize+8*i:]
		binary.BigEndian.PutUint32(entry, uint32(offset))
		binary.BigEndian.PutUint32(entry[4:], uint32(2*i))
		offset = end
	}
	if err := writeBytes(w, header); err != nil {
		return fmt.Errorf("write PalmDB header: %w", err)
	}
	for i, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writeBytes(w, record); err != nil {
			return fmt.Errorf("write PalmDB record %d: %w", i, err)
		}
	}
	return nil
}

func writeBytes(w io.Writer, data []byte) error {
	n, err := w.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}
