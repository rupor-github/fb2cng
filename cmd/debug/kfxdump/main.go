package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/h2non/filetype"

	"fbc/convert/kfx"
)

func main() {
	bcRawMedia := flag.Bool("bcRawMedia", false, "dump $417 (bcRawMedia) raw bytes into <file>-bcRawMedia directory")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "usage: kfxdump [-bcRawMedia] <file.kfx>\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	path := flag.Arg(0)
	b, err := os.ReadFile(path)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		os.Exit(1)
	}

	container, err := kfx.ReadContainer(b)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
		os.Exit(1)
	}

	if *bcRawMedia {
		if err := dumpBcRawMedia(container, path); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "dump bcRawMedia: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println(container.String())
	fmt.Println()
	fmt.Println(container.DumpFragments())
}

func dumpBcRawMedia(container *kfx.Container, inPath string) error {
	base := filepath.Base(inPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	outDir := filepath.Join(filepath.Dir(inPath), stem+"-bcRawMedia")
	if _, err := os.Stat(outDir); err == nil {
		return fmt.Errorf("output directory already exists: %s", outDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Mkdir(outDir, 0o755); err != nil {
		return err
	}

	written := 0
	for _, f := range container.Fragments.GetByType(kfx.SymRawMedia) {
		blob, ok := asBlob(f.Value)
		if !ok || len(blob) == 0 {
			continue
		}

		idName := ""
		if f.FIDName != "" {
			idName = f.FIDName
		} else if container.DocSymbolTable != nil {
			if n, ok := container.DocSymbolTable.FindByID(uint64(f.FID)); ok {
				idName = n
			}
		}

		idPrefix := fmt.Sprintf("%d", f.FID)
		if idName != "" {
			idPrefix += "_" + sanitizeFileComponent(idName)
		}

		ext := extFromFiletype(blob)
		outPath := filepath.Join(outDir, idPrefix+ext)
		for i := 2; ; i++ {
			if _, err := os.Stat(outPath); err != nil {
				break
			}
			outPath = filepath.Join(outDir, idPrefix+fmt.Sprintf("_%d", i)+ext)
		}

		if err := os.WriteFile(outPath, blob, 0o644); err != nil {
			return err
		}
		written++
	}

	_, _ = fmt.Fprintf(os.Stderr, "bcRawMedia: wrote %d file(s) into %s/\n", written, outDir)
	return nil
}

func asBlob(v any) ([]byte, bool) {
	switch vv := v.(type) {
	case []byte:
		return vv, true
	case kfx.RawValue:
		return []byte(vv), true
	default:
		return nil, false
	}
}

func extFromFiletype(b []byte) string {
	kind, err := filetype.Match(b)
	if err == nil && kind != filetype.Unknown && kind.Extension != "" {
		return "." + kind.Extension
	}
	return ".bin"
}

func sanitizeFileComponent(s string) string {
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
