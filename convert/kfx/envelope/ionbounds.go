package envelope

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
)

// Check lengths before ion-go parses input: its scalar readers allocate declared lengths.
// This also bounds nesting, including symbol-table values, before recursive Ion parsing.
func checkIon(ctx context.Context, data []byte) error {
	if !bytes.HasPrefix(data, []byte{0xe0, 1, 0, 0xea}) {
		return errors.New("missing independent binary Ion prolog")
	}
	return checkSequence(ctx, data[4:], 0, false)
}

func checkSequence(ctx context.Context, data []byte, depth int, fields bool) error {
	if depth > 16 {
		return errors.New("ion nesting exceeds 16 levels")
	}
	for len(data) != 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		if fields {
			_, n, err := varUint(data)
			if err != nil {
				return err
			}
			data = data[n:]
		}
		n, err := checkValue(ctx, data, depth)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func checkValue(ctx context.Context, data []byte, depth int) (int, error) {
	if len(data) == 0 {
		return 0, errors.New("truncated Ion value")
	}
	typ, length := data[0]>>4, data[0]&15
	if typ == 15 {
		return 0, errors.New("invalid Ion type")
	}
	if typ == 14 && length == 0 {
		return 0, errors.New("unexpected Ion version marker inside datagram")
	}
	if length == 15 || typ == 1 {
		return 1, nil
	}
	header, size := 1, uint64(length)
	if length == 14 || (typ == 13 && length == 1) {
		var n int
		var err error
		size, n, err = varUint(data[1:])
		if err != nil {
			return 0, err
		}
		header += n
	}
	if size > uint64(len(data)-header) {
		return 0, errors.New("ion value length exceeds input")
	}
	payload := data[header : header+int(size)]
	switch typ {
	case 11, 12, 13:
		if err := checkSequence(ctx, payload, depth+1, typ == 13); err != nil {
			return 0, err
		}
	case 14:
		annotations, n, err := varUint(payload)
		if err != nil {
			return 0, err
		}
		if annotations == 0 || annotations > uint64(len(payload)-n) {
			return 0, errors.New("invalid Ion annotation length")
		}
		value := payload[n+int(annotations):]
		if depth >= 16 {
			return 0, errors.New("ion annotation nesting exceeds 16 levels")
		}
		used, err := checkValue(ctx, value, depth+1)
		if err != nil {
			return 0, err
		}
		if used != len(value) {
			return 0, errors.New("extra bytes in Ion annotation")
		}
	}
	return header + int(size), nil
}

func varUint(data []byte) (uint64, int, error) {
	var value uint64
	for j, b := range data {
		if value > math.MaxUint64>>7 {
			return 0, 0, errors.New("ion VarUInt overflow")
		}
		value = value<<7 | uint64(b&127)
		if b&128 != 0 {
			return value, j + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("truncated Ion VarUInt (%d bytes)", len(data))
}
