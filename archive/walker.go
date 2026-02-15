// Package archive builds Walk abstraction on top of "archive/zip".
package archive

import (
	"archive/zip"
	"fmt"
	"path"
	"strings"
)

// WalkFunc is the type of the function called for each file in archive
// visited by Walk. The archive argument contains path to archive passed to Walk
// The file argument is the zip.File structure for file in archive which satisfies
// match condition. If an error is returned, processing stops.
type WalkFunc func(archive string, file *zip.File) error

// Walk walks the all files in the archive which satisfy match condition,
// calling walkFn for each item. Entries with path traversal components
// ("..") or absolute paths are silently skipped to prevent Zip Slip attacks.
func Walk(archive, pattern string, walkFn WalkFunc) error {

	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		name := f.FileHeader.Name
		if !isSafePath(name) {
			return fmt.Errorf("zip entry %q: unsafe path (absolute or contains path traversal)", name)
		}
		if !f.FileInfo().IsDir() && strings.HasPrefix(name, pattern) {
			if err := walkFn(archive, f); err != nil {
				return err
			}
		}
	}
	return nil
}

// isSafePath returns false for paths that could escape the extraction
// directory: absolute paths and those containing ".." components.
func isSafePath(name string) bool {
	if path.IsAbs(name) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}
