package dumputil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"fbc/utils/filepub"
)

func writeOutputFile(path string, overwrite bool, write func(io.Writer) error) error {
	if !overwrite {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("output file already exists: %s (use -overwrite): %w", path, os.ErrExist)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".fbc-dump-*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		// Close may already have succeeded; cleanup must not replace the publication result.
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	if err := f.Chmod(0644); err != nil {
		return err
	}
	if err := write(f); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := filepub.Publish(f.Name(), path, overwrite); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("output file already exists: %s (use -overwrite): %w", path, err)
		}
		return err
	}
	return nil
}
