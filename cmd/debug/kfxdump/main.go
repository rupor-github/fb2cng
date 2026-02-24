package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"fbc/cmd/debug/internal/dumputil"
	"fbc/convert/kfx"
)

func main() {
	all := flag.Bool("all", false, "enable all dump flags (-dump, -resources, -styles, -storyline)")
	dump := flag.Bool("dump", false, "dump all fragments into <file>-dump.txt")
	resources := flag.Bool("resources", false, "dump $417/$418 raw bytes into <file>-resources.zip")
	styles := flag.Bool("styles", false, "dump $157 (style) fragments into <file>-styles.txt")
	storyline := flag.Bool("storyline", false, "dump $259 (storyline) fragments into <file>-storyline.txt with expanded symbols and styles")
	margins := flag.Bool("margins", false, "dump vertical margin tree into <file>-margins.txt for easy comparison")
	overwrite := flag.Bool("overwrite", false, "overwrite existing output")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "usage: kfxdump [-all] [-dump] [-resources] [-styles] [-storyline] [-margins] [-overwrite] <file.kfx> [outdir]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 || flag.NArg() > 2 {
		flag.Usage()
		os.Exit(2)
	}

	if *all {
		*dump = true
		*resources = true
		*styles = true
		*storyline = true
		*margins = true
	}

	if !*dump && !*resources && !*styles && !*storyline && !*margins {
		flag.Usage()
		os.Exit(2)
	}

	defer func(startedAt time.Time) {
		duration := time.Since(startedAt)
		fmt.Fprintf(os.Stderr, "\nExecution time: %s\n", duration)
	}(time.Now())

	path := flag.Arg(0)
	outDir := ""
	if flag.NArg() == 2 {
		outDir = flag.Arg(1)
	}

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

	if *dump {
		if err := dumputil.DumpDumpTxt(container, path, outDir, *overwrite); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "dump: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(container.StatsString())
	}

	if *resources {
		if err := dumputil.DumpResources(container, path, outDir, *overwrite); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "dump resources: %v\n", err)
			os.Exit(1)
		}
	}

	if *styles {
		if err := dumputil.DumpStylesTxt(container, path, outDir, *overwrite); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "dump styles: %v\n", err)
			os.Exit(1)
		}
	}

	if *storyline {
		if err := dumputil.DumpStorylineTxt(container, path, outDir, *overwrite); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "dump storyline: %v\n", err)
			os.Exit(1)
		}
	}

	if *margins {
		if err := dumputil.DumpMarginsTxt(container, path, outDir, *overwrite); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "dump margins: %v\n", err)
			os.Exit(1)
		}
	}
}
