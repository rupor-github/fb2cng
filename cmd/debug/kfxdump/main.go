package main

import (
	"fmt"
	"os"

	"fbc/convert/kfx"
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

	container, err := kfx.ReadContainer(b)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
		os.Exit(1)
	}

	fmt.Println(container.String())
	fmt.Println()
	fmt.Println(container.DumpFragments())
}
