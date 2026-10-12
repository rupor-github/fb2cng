package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"fbc/cmd/debug/internal/dumputil"
	"fbc/convert/kfx"
	"fbc/convert/kfx/envelope"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("kfxdump", flag.ContinueOnError)
	flags.SetOutput(stderr)
	all := flags.Bool("all", false, "enable all text/resource dumps, including envelope details (not inner extraction)")
	dump := flags.Bool("dump", false, "dump all fragments into <file>-dump.txt")
	resources := flags.Bool("resources", false, "dump $417/$418 raw bytes into <file>-resources.zip")
	styles := flags.Bool("styles", false, "dump $157 (style) fragments into <file>-styles.txt")
	storyline := flags.Bool("storyline", false, "dump $259 (storyline) fragments into <file>-storyline.txt with expanded symbols and styles")
	margins := flags.Bool("margins", false, "dump vertical margin tree into <file>-margins.txt for easy comparison")
	envelopeDump := flags.Bool("envelope", false, "dump outer envelope layout/page headers into <file>-envelope.txt")
	extractInner := flags.Bool("extract-inner", false, "write exact decoded CONT bytes into <file>-inner.kfx")
	overwrite := flags.Bool("overwrite", false, "overwrite existing output")
	flags.Usage = func() {
		_, _ = fmt.Fprintf(
			stderr,
			"usage: kfxdump [-all] [-dump] [-resources] [-styles] [-storyline] [-margins] [-envelope] [-extract-inner] [-overwrite] <file> [outdir]\n",
		)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() < 1 || flags.NArg() > 2 {
		flags.Usage()
		return errors.New("expected one input file and optional output directory")
	}

	if *all {
		*dump = true
		*resources = true
		*styles = true
		*storyline = true
		*margins = true
		*envelopeDump = true
	}

	if !*dump && !*resources && !*styles && !*storyline && !*margins && !*envelopeDump && !*extractInner {
		flags.Usage()
		return errors.New("at least one dump/extraction option is required")
	}

	defer func(startedAt time.Time) {
		duration := time.Since(startedAt)
		fmt.Fprintf(stderr, "\nExecution time: %s\n", duration)
	}(time.Now())

	path := flags.Arg(0)
	outDir := ""
	if flags.NArg() == 2 {
		outDir = flags.Arg(1)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	summary := "KFX file: ordinary CONT (no outer envelope)"
	detail := summary + "\n"
	if envelope.IsEnvelope(b) {
		e, err := envelope.Inspect(ctx, b)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
		info := e.Info()
		summary, detail = info.Summary(), info.String()
		// Only inspection is necessary for --envelope alone.
		if !*dump && !*resources && !*styles && !*storyline && !*margins && !*extractInner {
			fmt.Fprintln(stdout, summary)
			return dumputil.WriteOutput(path, outDir, "-envelope.txt", []byte(detail), *overwrite)
		}
		b, err = e.Decode(ctx)
		if err != nil {
			return err
		}
	} else if err := kfx.ValidateContainer(b); err != nil {
		return err
	}
	fmt.Fprintln(stdout, summary)
	if *envelopeDump {
		if err := dumputil.WriteOutput(path, outDir, "-envelope.txt", []byte(detail), *overwrite); err != nil {
			return err
		}
	}
	if *extractInner {
		if err := kfx.ValidateContainer(b); err != nil {
			return err
		}
		if err := dumputil.WriteOutput(path, outDir, "-inner.kfx", b, *overwrite); err != nil {
			return err
		}
	}
	if !*dump && !*resources && !*styles && !*storyline && !*margins {
		return nil
	}
	file, err := kfx.ReadFile(ctx, b)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	container := file.Container

	if *dump {
		if err := dumputil.DumpDumpTxt(container, path, outDir, *overwrite, summary); err != nil {
			return fmt.Errorf("dump: %w", err)
		}
		fmt.Fprintln(stdout, container.StatsString())
	}

	if *resources {
		if err := dumputil.DumpResources(container, path, outDir, *overwrite); err != nil {
			return fmt.Errorf("dump resources: %w", err)
		}
	}

	if *styles {
		if err := dumputil.DumpStylesTxt(container, path, outDir, *overwrite); err != nil {
			return fmt.Errorf("dump styles: %w", err)
		}
	}

	if *storyline {
		if err := dumputil.DumpStorylineTxt(container, path, outDir, *overwrite); err != nil {
			return fmt.Errorf("dump storyline: %w", err)
		}
	}

	if *margins {
		if err := dumputil.DumpMarginsTxt(container, path, outDir, *overwrite); err != nil {
			return fmt.Errorf("dump margins: %w", err)
		}
	}
	return nil
}
