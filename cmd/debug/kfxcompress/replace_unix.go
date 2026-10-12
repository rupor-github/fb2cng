//go:build !windows

package main

import "os"

func replaceOutput(source, destination string) error { return os.Rename(source, destination) }
