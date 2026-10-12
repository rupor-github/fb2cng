//go:build !linux && !darwin && !windows

package filepub

import "os"

// Preserve atomic no-overwrite publication on hosts without a native helper.
func publishOutput(source, destination string) error { return os.Link(source, destination) }
