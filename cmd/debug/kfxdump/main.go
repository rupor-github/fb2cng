package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		_, _ = fmt.Fprintf(os.Stderr, "usage: kfxdump <file.kfx>\n")
		os.Exit(2)
	}

	path := os.Args[1]
	b, err := os.ReadFile(path)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		os.Exit(1)
	}

	_ = b
}
