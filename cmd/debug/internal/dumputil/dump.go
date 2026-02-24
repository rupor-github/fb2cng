// Package dumputil provides shared output helpers for kfxdump and kdfdump debug tools.
// It operates on *kfx.Container and produces dump text, resource ZIPs, style reports,
// storyline expansions, and margin trees.
package dumputil

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/h2non/filetype"

	"fbc/convert/kfx"
)

// DumpDumpTxt writes a full fragment dump to <stem>-dump.txt.
func DumpDumpTxt(container *kfx.Container, inPath, outDir string, overwrite bool) error {
	dump := container.String() + "\n\n" + container.DumpFragments() + "\n"
	return WriteOutput(inPath, outDir, "-dump.txt", []byte(dump), overwrite)
}

// DumpResources writes $417/$418 raw media/font blobs into <stem>-resources.zip.
func DumpResources(container *kfx.Container, inPath, outDir string, overwrite bool) (retErr error) {
	base := filepath.Base(inPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	dir := filepath.Dir(inPath)
	if outDir != "" {
		dir = outDir
	}
	outPath := filepath.Join(dir, stem+"-resources.zip")
	if _, err := os.Stat(outPath); err == nil {
		if !overwrite {
			return fmt.Errorf("output file already exists: %s (use -overwrite)", outPath)
		}
		if err := os.Remove(outPath); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, f.Close()) }()

	zw := zip.NewWriter(f)
	defer func() { retErr = errors.Join(retErr, zw.Close()) }()

	usedNames := make(map[string]int)
	written := 0
	for _, frag := range append(container.Fragments.GetByType(kfx.SymRawMedia), container.Fragments.GetByType(kfx.SymRawFont)...) {
		blob, ok := AsBlob(frag.Value)
		if !ok || len(blob) == 0 {
			continue
		}

		idName := ""
		if frag.FIDName != "" {
			idName = frag.FIDName
		} else if container.DocSymbolTable != nil {
			if n, ok := container.DocSymbolTable.FindByID(uint64(frag.FID)); ok {
				idName = n
			}
		}

		idPrefix := fmt.Sprintf("%d", frag.FID)
		if idName != "" {
			idPrefix += "_" + SanitizeFileComponent(idName)
		}
		if frag.FType == kfx.SymRawFont {
			idPrefix += "_font"
		}

		ext := ExtFromFiletype(blob)
		entryName := idPrefix + ext
		if count := usedNames[entryName]; count > 0 {
			entryName = idPrefix + fmt.Sprintf("_%d", count+1) + ext
		}
		usedNames[idPrefix+ext]++

		w, err := zw.Create(entryName)
		if err != nil {
			return err
		}
		if _, err := w.Write(blob); err != nil {
			return err
		}
		written++
	}

	_, _ = fmt.Fprintf(os.Stderr, "resources: wrote %d file(s) into %s\n", written, outPath)
	return nil
}

// WriteOutput writes data to <stem><suffix> in either the input file's directory or outDir.
func WriteOutput(inPath, outDir, suffix string, data []byte, overwrite bool) error {
	base := filepath.Base(inPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	dir := filepath.Dir(inPath)
	if outDir != "" {
		dir = outDir
	}
	outPath := filepath.Join(dir, stem+suffix)

	if _, err := os.Stat(outPath); err == nil {
		if !overwrite {
			return fmt.Errorf("output file already exists: %s (use -overwrite)", outPath)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", outPath)
	return nil
}

// AsBlob extracts a byte slice from a fragment value.
func AsBlob(v any) ([]byte, bool) {
	switch vv := v.(type) {
	case []byte:
		return vv, true
	case kfx.RawValue:
		return []byte(vv), true
	default:
		return nil, false
	}
}

// ExtFromFiletype detects the file extension from magic bytes.
func ExtFromFiletype(b []byte) string {
	kind, err := filetype.Match(b)
	if err == nil && kind != filetype.Unknown && kind.Extension != "" {
		return "." + kind.Extension
	}
	return ".bin"
}

// SanitizeFileComponent cleans a string for use in a filename.
func SanitizeFileComponent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_' || r == '.':
			return r
		default:
			return '_'
		}
	}, s)
}
