//go:build windows

package config

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/term"
)

// CleanFileName removes not allowed characters form file name.
func CleanFileName(in string) string {
	out := strings.Map(func(sym rune) rune {
		if sym == 0 || strings.ContainsRune(`<>":/\|?*`+string(os.PathSeparator)+string(os.PathListSeparator), sym) {
			return -1
		}
		return sym
	}, in)
	if len(out) == 0 {
		out = "_bad_file_name_"
	}
	return out
}

// EnableColorOutput checks if colorized output is possible and
// enables proper VT100 sequence processing in Windows console.
func EnableColorOutput(stream *os.File) bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	v, _, err := k.GetIntegerValue("CurrentMajorVersionNumber")
	if err != nil {
		return false
	}

	if v < 10 {
		return false
	}

	if !term.IsTerminal(int(stream.Fd())) {
		return false
	}

	var mode uint32
	err = windows.GetConsoleMode(windows.Handle(stream.Fd()), &mode)
	if err != nil {
		return false
	}

	const EnableVirtualTerminalProcessing uint32 = 0x4
	mode |= EnableVirtualTerminalProcessing

	err = windows.SetConsoleMode(windows.Handle(stream.Fd()), mode)
	if err != nil {
		return false
	}
	return true
}
