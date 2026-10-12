package kfx

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"fbc/convert/kfx/envelope"
)

// File holds a decoded book and separate outer-file information.
type File struct {
	Container    *Container
	InnerBytes   []byte
	EnvelopeInfo *envelope.Info
}

// ReadFile reads ordinary CONT or the supported plaintext KFX envelope.
func ReadFile(ctx context.Context, data []byte) (*File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f := &File{InnerBytes: data}
	if envelope.IsEnvelope(data) {
		e, err := envelope.Inspect(ctx, data)
		if err != nil {
			return nil, err
		}
		f.InnerBytes, err = e.Decode(ctx)
		if err != nil {
			return nil, err
		}
		info := e.Info()
		f.EnvelopeInfo = &info
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var err error
	f.Container, err = ReadContainer(f.InnerBytes)
	if err != nil {
		return nil, fmt.Errorf("read decoded CONT: %w", err)
	}
	return f, nil
}

// ValidateContainer validates structural byte ranges without rebuilding the book or
// interpreting entity payloads. Transforms preserve even unknown/raw entity bytes.
func ValidateContainer(data []byte) error {
	if len(data) < MinContainerLen {
		return errors.New("truncated CONT header")
	}
	var h containerHeader
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &h); err != nil {
		return err
	}
	if err := h.Validate(); err != nil {
		return err
	}
	if uint64(h.Size) > uint64(len(data)) || h.InfoOffset < MinContainerLen || h.InfoSize == 0 ||
		uint64(h.InfoOffset)+uint64(h.InfoSize) > uint64(h.Size) {
		return errors.New("CONT header/info range out of bounds")
	}
	var info containerInfo
	if err := DecodeIon(GetIonProlog(), data[h.InfoOffset:uint64(h.InfoOffset)+uint64(h.InfoSize)], &info); err != nil {
		return fmt.Errorf("decode container_info: %w", err)
	}
	if info.ComprType != 0 || info.DRMScheme != 0 {
		return errors.New("unsupported inner CONT compression/DRM")
	}
	for _, region := range []struct {
		name         string
		offset, size int
	}{
		{"symbols", info.DocSymOff, info.DocSymLen}, {"directory", info.IndexTabOff, info.IndexTabLen},
		{"capabilities", info.FCapabOff, info.FCapabLen},
	} {
		if region.offset < 0 || region.size < 0 || uint64(region.offset)+uint64(region.size) > uint64(h.Size) {
			return fmt.Errorf("CONT %s range out of bounds", region.name)
		}
	}
	if info.DocSymLen == 0 || info.IndexTabLen%EntityDirEntrySize != 0 {
		return errors.New("invalid CONT symbols/entity directory")
	}
	directory := bytes.NewReader(data[info.IndexTabOff : info.IndexTabOff+info.IndexTabLen])
	for directory.Len() != 0 {
		var entry indexTableEntry
		if err := entry.readFrom(directory); err != nil {
			return err
		}
		available := uint64(len(data)) - uint64(h.Size)
		if entry.Offset > available || entry.Size > available-entry.Offset || entry.Size < MinEntityLen {
			return errors.New("CONT entity range out of bounds")
		}
		start := uint64(h.Size) + entry.Offset
		var eh entityHeader
		if err := binary.Read(bytes.NewReader(data[start:start+entry.Size]), binary.LittleEndian, &eh); err != nil {
			return err
		}
		if err := eh.Validate(); err != nil {
			return err
		}
		if uint64(eh.Size) > entry.Size {
			return errors.New("ENTY header range out of bounds")
		}
		var entInfo entityInfo
		if err := DecodeIon(GetIonProlog(), data[start+MinEntityLen:start+uint64(eh.Size)], &entInfo); err != nil {
			return fmt.Errorf("decode entity_info: %w", err)
		}
		if entInfo.ComprType != 0 || entInfo.DRMScheme != 0 {
			return errors.New("unsupported inner ENTY compression/DRM")
		}
	}
	return nil
}
