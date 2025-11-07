package config

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"fbc/misc"
)

type ReporterConfig struct {
	Destination string `yaml:"destination" sanitize:"path_clean,assure_dir_exists_for_file" validate:"required,filepath"`
}

// Prepare creates initialized empty reporter.
func (conf *ReporterConfig) Prepare() (*Report, error) {

	r := &Report{entries: make(map[string]entry)}

	if f, err := os.Create(conf.Destination); err == nil {
		r.file = f
	} else if f, err = os.CreateTemp("", misc.GetAppName()+"-report.*.zip"); err == nil {
		r.file = f
	} else {
		return nil, fmt.Errorf("unable to create report: %w", err)
	}
	return r, nil
}

type entry struct {
	original string
	actual   string
	stamp    time.Time
	data     []byte
}

// Reporter accumulates information necessary to prepare full debug report.
// NOTE: presently not to be used concurrently!
type Report struct {
	// entries is a map of names to entries of files or directories to be put in the final archive later.
	entries map[string]entry
	file    *os.File
}

// Close finalizes debug report.
func (r *Report) Close() error {
	if r == nil {
		// Ignore uninitialized cases to avoid checking in many places. This means no report has been requested.
		return nil
	}
	if r.file == nil {
		return nil
	}
	defer r.file.Close()
	return r.finalize()
}

// Name returns name of underlying file.
func (r *Report) Name() string {
	if r == nil || r.file == nil {
		return ""
	}
	if n, err := filepath.Abs(r.file.Name()); err == nil {
		return n
	}
	return r.file.Name()
}

// Store saves path to file or directory to be put in the final archive later.
func (r *Report) Store(name, path string) {
	if r == nil {
		// Ignore uninitialized cases to avoid checking in many places. This means no report has been requested.
		return
	}

	if old, exists := r.entries[name]; exists && old.original != path {
		// Somewhere I do not know what I am doing.
		panic(fmt.Sprintf("Attempt to overwrite file in the report for [%s]: was %s, now %s", name, old.original, path))
	}

	e := entry{
		original: path,
		actual:   path,
	}
	if p, err := filepath.Abs(path); err == nil {
		e.actual = p
	}
	r.entries[name] = e
}

// StoreData saves binary data to be put in the final archive later as a file under requested name.
func (r *Report) StoreData(name string, data []byte) {
	if r == nil {
		// Ignore uninitialized cases to avoid checking in many places. This means no report has been requested.
		return
	}

	if _, exists := r.entries[name]; exists {
		// Somewhere I do not know what I am doing.
		panic(fmt.Sprintf("Attempt to overwrite data in the report for [%s]", name))
	}

	e := entry{
		data:  data,
		stamp: time.Now(),
	}
	r.entries[name] = e
}

// StoreCopy makes a copy (at the time of a call) of the file or directory into temporary location to be put in the final archive later.
// names are versioned with timestamps to avoid collisions, so it is safe to put the same content into report multiple times.
func (r *Report) StoreCopy(name, path string) error {
	if r == nil {
		// Ignore uninitialized cases to avoid checking in many places. This means no report has been requested.
		return nil
	}

	var err error

	e := entry{
		stamp:    time.Now(),
		original: path,
	}

	// cleanup path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	e.actual = absPath

	if _, exists := r.entries[name]; exists {
		// version the name to avoid collisions
		name = fmt.Sprintf("%s-%d", name, e.stamp.UnixNano())
	}

	dir, err := os.MkdirTemp("", "s2k-r-")
	if err != nil {
		return err
	}

	if info, err := os.Stat(e.actual); err == nil {
		switch {
		case info.Mode().IsRegular():
			where, err := copyFile(dir, e.actual, info.ModTime())
			if err != nil {
				return err
			}
			e.actual = where
		case info.Mode().IsDir():
			if err := copyDir(dir, e.actual); err != nil {
				return err
			}
			e.actual = dir
		}
	} else {
		return err
	}

	r.entries[name] = e
	return nil
}

func copyFile(dir, src string, modTime time.Time) (string, error) {
	// always make sure destination directory exists
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}

	dst := filepath.Join(dir, filepath.Base(src))

	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	if err = out.Sync(); err != nil {
		return "", err
	}
	out.Close()

	if err := os.Chtimes(dst, modTime, modTime); err != nil {
		return "", err
	}
	return dst, nil
}

func copyDir(dir, src string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			// ignore links, sockets, etc.
			return nil
		}

		// get the path of the file relative to the source folder
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		// root file under new path
		newpath := filepath.Join(dir, rel)

		// copy file to the new place
		if _, err := copyFile(filepath.Dir(newpath), path, info.ModTime()); err != nil {
			return err
		}
		return nil
	})
}

// finalize creates the final archive (report) with all previously stored items.
func (r *Report) finalize() error {

	arc := zip.NewWriter(r.file)
	defer arc.Close()

	t := time.Now()

	names, manifest := prepareManifest(r.entries)
	if err := saveFile(arc, "MANIFEST", t, manifest); err != nil {
		return err
	}

	// in the same order as in manifest
	for _, name := range names {
		if len(r.entries[name].data) > 0 {
			if err := saveFile(arc, name, r.entries[name].stamp, bytes.NewReader(r.entries[name].data)); err != nil {
				return err
			}
			continue
		}

		path := r.entries[name].actual
		// ignoring absent files
		if info, err := os.Stat(path); err == nil {
			switch {
			case info.Mode().IsRegular():
				var r io.ReadCloser
				if r, err = os.Open(path); err != nil {
					return err
				}
				if err := saveFile(arc, name, info.ModTime(), r); err != nil {
					r.Close()
					return err
				}
				r.Close()
			case info.Mode().IsDir():
				if err := saveDir(arc, name, path); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func prepareManifest(entries map[string]entry) ([]string, *bytes.Buffer) {

	now := time.Now()

	buf := new(bytes.Buffer)
	if len(entries) == 0 {
		return nil, buf
	}

	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		e := entries[k]
		if e.stamp.IsZero() {
			e.stamp = now
		}
		buf.WriteString(fmt.Sprintf("%s\t%s\t%s : %s\n", e.stamp.UTC().Format(time.UnixDate), k, e.original, e.actual))
	}
	return keys, buf
}

func saveFile(dst *zip.Writer, name string, t time.Time, src io.Reader) error {
	w, err := dst.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: t})
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, src); err != nil {
		return err
	}
	return nil
}

func saveDir(dst *zip.Writer, name, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			// ignore links, sockets, etc.
			return nil
		}

		// Get the path of the file relative to the source folder
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		// root entry under new name
		rel = filepath.ToSlash(filepath.Join(name, rel))

		var r io.ReadCloser
		if r, err = os.Open(path); err != nil {
			return err
		}
		defer r.Close()

		if err = saveFile(dst, rel, info.ModTime(), r); err != nil {
			return err
		}
		return nil
	})
}
