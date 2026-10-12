//go:build windows

package main

import "golang.org/x/sys/windows"

func replaceOutput(source, destination string) error {
	return moveOutput(source, destination, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

func publishOutput(source, destination string) error {
	return moveOutput(source, destination, windows.MOVEFILE_WRITE_THROUGH)
}

func moveOutput(source, destination string, flags uint32) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, flags)
}
