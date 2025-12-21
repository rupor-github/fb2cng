package kfx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"go.uber.org/zap"

	"fbc/content"
	"fbc/convert/kfx/ionutil"
	"fbc/convert/kfx/model"
)

func dumpDebug(c *content.Content, containerID string, prolog *ionutil.Prolog, fragments []model.Fragment, log *zap.Logger) error {
	if c == nil || !c.Debug || c.WorkDir == "" || prolog == nil {
		return nil
	}

	dir := filepath.Join(c.WorkDir, "kfx")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// These files are put into the debug report automatically because WorkDir is stored in the report.
	if err := os.WriteFile(filepath.Join(dir, "container_id.txt"), []byte(containerID+"\n"), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "document_symbols.ion"), prolog.DocSymbols, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "tree.txt"), []byte(buildKFXDebugTree(containerID, len(prolog.DocSymbols), fragments)), 0644); err != nil {
		return err
	}

	var manifest strings.Builder
	manifest.WriteString("fid\tftype\tpayload_bytes\n")

	fragDir := filepath.Join(dir, "fragments")
	if err := os.MkdirAll(fragDir, 0755); err != nil {
		return err
	}

	for _, fr := range fragments {
		payload, err := ionutil.MarshalPayload(fr.Value, prolog)
		if err != nil {
			return fmt.Errorf("marshal fragment %s/%s: %w", fr.FID, fr.FType, err)
		}

		manifest.WriteString(fmt.Sprintf("%s\t%s\t%d\n", fr.FID, fr.FType, len(payload)))

		// Store per-fragment payloads as BVM+IonValue (no LST), matching how fragments are stored in KFX.
		name := fmt.Sprintf("%s_%s.ion", sanitizeFilename(fr.FID), sanitizeFilename(fr.FType))
		if err := os.WriteFile(filepath.Join(fragDir, name), payload, 0644); err != nil {
			return err
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "fragments.tsv"), []byte(manifest.String()), 0644); err != nil {
		return err
	}

	log.Debug("Stored KFX debug dumps", zap.String("dir", dir))
	return nil
}

func sanitizeFilename(s string) string {
	if s == "" {
		return "empty"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, s)
}
