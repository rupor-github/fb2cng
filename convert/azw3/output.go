package azw3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"fbc/utils/filepub"
)

type stagedOutput struct {
	path string
	temp string
	info os.FileInfo
}

type outputBundle struct {
	book        stagedOutput
	apnx        *stagedOutput
	apnxData    []byte
	backup      stagedOutput
	directories []string
	overwrite   bool
	used        bool
	publishFile func(source, destination string, overwrite bool) error
}

// preflightOutputBundle checks only the selected outputs and makes no filesystem
// changes. Unselected APNX files and unrelated .sdr contents are never removed.
// Generate uses this boundary instead of the converter's single-file publisher.
func preflightOutputBundle(ctx context.Context, bookPath string, apnx *apnxLayout, overwrite bool) (*outputBundle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if bookPath == "" || os.IsPathSeparator(bookPath[len(bookPath)-1]) {
		return nil, errors.New("AZW3 publication requires a final book filename")
	}
	if name := filepath.Base(bookPath); name == "." || name == ".." {
		return nil, errors.New("AZW3 publication requires a file, not a directory path")
	}
	bookPath, err := filepath.Abs(bookPath)
	if err != nil {
		return nil, fmt.Errorf("resolve AZW3 output path: %w", err)
	}
	b := &outputBundle{book: stagedOutput{path: bookPath}, overwrite: overwrite, publishFile: filepub.Publish}
	if apnx != nil {
		if apnx.Path == "" || len(apnx.Data) == 0 {
			return nil, errors.New("APNX publication requires a prepared path and nonempty data")
		}
		path, err := filepath.Abs(apnx.Path)
		if err != nil {
			return nil, fmt.Errorf("resolve APNX output path: %w", err)
		}
		app, appErr := apnxOutputPath(bookPath, "app")
		eink, einkErr := apnxOutputPath(bookPath, "eink")
		if !(appErr == nil && path == app || einkErr == nil && path == eink) {
			return nil, errors.New("APNX path must match app or eink placement for the final book filename")
		}
		b.apnx, b.apnxData = &stagedOutput{path: path}, apnx.Data
		b.backup.path = path
	}
	if err := b.checkDestinations(); err != nil {
		return nil, err
	}
	return b, nil
}

func checkBundleDestination(path string, overwrite bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("output %q is not a regular file (symlinks are not replaced)", path)
	}
	if !overwrite {
		return fmt.Errorf("output %q already exists: %w", path, os.ErrExist)
	}
	return nil
}

func (b *outputBundle) checkDestinations() error {
	if err := checkBundleDestination(b.book.path, b.overwrite); err != nil {
		return err
	}
	if b.apnx != nil {
		// Unlike a user-selected output parent, the derived .sdr directory must
		// not be a symlink redirecting the sidecar elsewhere.
		if filepath.Dir(b.apnx.path) != filepath.Dir(b.book.path) {
			info, err := os.Lstat(filepath.Dir(b.apnx.path))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err == nil && !info.IsDir() {
				return fmt.Errorf("APNX directory %q is not a directory or is a symlink", filepath.Dir(b.apnx.path))
			}
		}
		if err := checkBundleDestination(b.apnx.path, b.overwrite); err != nil {
			return err
		}
	}
	return nil
}

// publish stages, syncs and closes every file before changing either destination.
// APNX is published first; the single-file atomic book replacement is the commit
// point. An ordinary failure before that point rolls APNX back without using the
// canceled context. This is NOT a multi-file atomic or crash-durable transaction:
// readers can briefly see new APNX with the old book, and a crash can leave that
// pair. Callers must serialize competing writers to the same output bundle;
// overwrite mode cannot provide cross-platform compare-and-swap isolation.
func (b *outputBundle) publish(ctx context.Context, book *bookLayout, created time.Time) (rerr error) {
	if b.used {
		return errors.New("AZW3 output bundle has already been used")
	}
	b.used = true
	if err := ctx.Err(); err != nil {
		return err
	}
	if book == nil {
		return errors.New("AZW3 publication requires a compiled book")
	}
	committed, apnxPublished := false, false
	defer func() {
		keepBackup := false
		if apnxPublished && !committed {
			if err := b.rollbackAPNX(); err != nil {
				keepBackup = true
				rerr = errors.Join(rerr, fmt.Errorf("APNX rollback failed; AZW3 book was not published and sidecar %q may be inconsistent: %w",
					b.apnx.path, err))
				if b.backup.temp != "" {
					rerr = errors.Join(rerr, fmt.Errorf("previous APNX retained for recovery at %q", b.backup.temp))
				}
			}
		}
		if err := b.cleanup(keepBackup, committed); err != nil {
			if committed {
				err = fmt.Errorf("AZW3 bundle was published, but cleanup failed: %w", err)
			}
			rerr = errors.Join(rerr, err)
		}
	}()
	if err := b.checkDestinations(); err != nil {
		return err
	}
	if err := b.makeDirectory(filepath.Dir(b.book.path), 0755); err != nil {
		return fmt.Errorf("create AZW3 output directory: %w", err)
	}
	if err := stageBundleFile(ctx, &b.book, 0644, func(w io.Writer) error {
		return writePalmDB(ctx, w, book.Database, created, book.Records)
	}); err != nil {
		return fmt.Errorf("stage AZW3 book: %w", err)
	}
	if b.apnx != nil {
		if err := b.makeDirectory(filepath.Dir(b.apnx.path), 0700); err != nil {
			return fmt.Errorf("create APNX output directory: %w", err)
		}
		if err := stageBundleFile(ctx, b.apnx, 0644, func(w io.Writer) error { return writeBytes(w, b.apnxData) }); err != nil {
			return fmt.Errorf("stage APNX sidecar: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := b.checkDestinations(); err != nil {
		return err
	}
	if b.apnx != nil {
		if err := b.backupAPNX(ctx); err != nil {
			return fmt.Errorf("back up previous APNX: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := b.publishFile(b.apnx.temp, b.apnx.path, b.overwrite); err != nil {
			return fmt.Errorf("publish APNX sidecar: %w", err)
		}
		apnxPublished = true
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkBundleDestination(b.book.path, b.overwrite); err != nil {
		return err
	}
	if err := b.publishFile(b.book.temp, b.book.path, b.overwrite); err != nil {
		return fmt.Errorf("publish AZW3 book: %w", err)
	}
	committed = true
	// Cancellation after the commit point must not report an unpublished book.
	return nil
}

func stageBundleFile(ctx context.Context, output *stagedOutput, mode os.FileMode, write func(io.Writer) error) (rerr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(output.path), ".fbc-azw3-*.tmp")
	if err != nil {
		return err
	}
	output.temp = f.Name()
	defer func() { rerr = errors.Join(rerr, f.Close()) }()
	if err := write(f); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	output.info, err = f.Stat()
	return err
}

func (b *outputBundle) backupAPNX(ctx context.Context) (rerr error) {
	info, err := os.Lstat(b.apnx.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := checkBundleDestination(b.apnx.path, b.overwrite); err != nil {
		return err
	}
	f, err := os.Open(b.apnx.path)
	if err != nil {
		return err
	}
	defer func() { rerr = errors.Join(rerr, f.Close()) }()
	opened, err := f.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(info, opened) {
		return errors.New("APNX changed while opening its backup")
	}
	// Copy rather than hard-link: in-place writes to the original must not also
	// modify the recovery copy. Preserve bytes and permission bits, not timestamps.
	return stageBundleFile(ctx, &b.backup, info.Mode().Perm(), func(w io.Writer) error {
		buffer := make([]byte, 32*1024)
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			n, err := f.Read(buffer)
			if writeErr := writeBytes(w, buffer[:n]); writeErr != nil {
				return writeErr
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
		}
	})
}

func (b *outputBundle) rollbackAPNX() error {
	info, err := os.Lstat(b.apnx.path)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return err
	}
	if !missing && !os.SameFile(info, b.apnx.info) {
		return errors.New("APNX destination changed after publication; refusing to replace another writer's file")
	}
	if b.backup.temp != "" {
		return b.publishFile(b.backup.temp, b.apnx.path, !missing)
	}
	if missing {
		return nil
	}
	return os.Remove(b.apnx.path)
}

func (b *outputBundle) makeDirectory(path string, mode os.FileMode) error {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("output parent %q is not a directory", path)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return err
	}
	if err := b.makeDirectory(parent, 0755); err != nil {
		return err
	}
	if err := os.Mkdir(path, mode); err != nil {
		// Another writer may have created the directory; it is not ours to clean.
		if errors.Is(err, os.ErrExist) {
			info, statErr := os.Stat(path)
			if statErr == nil && info.IsDir() {
				return nil
			}
		}
		return err
	}
	b.directories = append(b.directories, path)
	return nil
}

func (b *outputBundle) cleanup(keepBackup, committed bool) error {
	var failures []error
	files := []*stagedOutput{&b.book, b.apnx}
	if !keepBackup {
		files = append(files, &b.backup)
	}
	for _, file := range files {
		if file == nil || file.temp == "" {
			continue
		}
		if err := os.Remove(file.temp); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, fmt.Errorf("remove AZW3 temporary file %q: %w", file.temp, err))
		}
	}
	if !committed {
		for i := len(b.directories) - 1; i >= 0; i-- {
			path := b.directories[i]
			entries, err := os.ReadDir(path)
			if errors.Is(err, os.ErrNotExist) || err == nil && len(entries) > 0 {
				continue // Never remove unrelated or retained recovery files.
			}
			if err == nil {
				err = os.Remove(path) // Empty directories only; never RemoveAll.
			}
			if err != nil {
				failures = append(failures, fmt.Errorf("remove empty AZW3 output directory %q: %w", path, err))
			}
		}
	}
	return errors.Join(failures...)
}
