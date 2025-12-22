package main

import (
	"fmt"
	"os"

	"github.com/amazon-ion/ion-go/ion"

	"fbc/convert/kfx"
	"fbc/convert/kfx/container"
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

	u, err := container.Unpack(b)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unpack %s: %v\n", path, err)
		os.Exit(1)
	}

	fmt.Print(kfx.BuildDebugTree(u.ContainerID, len(u.DocumentSymbols), u.Fragments))

	if u.ContainerInfo != nil {
		if t, err := ion.MarshalText(u.ContainerInfo); err == nil {
			fmt.Printf("\ncontainer_info=%s\n", string(t))
		}
	}
	if u.FormatCapabilities != nil {
		if t, err := ion.MarshalText(u.FormatCapabilities); err == nil {
			fmt.Printf("\nformat_capabilities=%s\n", string(t))
		}
	}
}
