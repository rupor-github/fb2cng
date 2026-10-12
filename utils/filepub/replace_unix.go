//go:build !windows

package filepub

import "os"

func replaceOutput(source, destination string) error { return os.Rename(source, destination) }
