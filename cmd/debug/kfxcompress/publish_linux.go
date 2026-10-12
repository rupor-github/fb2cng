package main

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func publishOutput(source, destination string) error {
	err := unix.Renameat2(unix.AT_FDCWD, source, unix.AT_FDCWD, destination, unix.RENAME_NOREPLACE)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EOPNOTSUPP) {
		// Older kernels/filesystems may lack no-replace rename. Hard links remain
		// a safe atomic fallback where supported.
		return os.Link(source, destination)
	}
	if err != nil {
		return &os.LinkError{Op: "rename", Old: source, New: destination, Err: err}
	}
	return nil
}
