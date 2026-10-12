package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"fbc/convert/kfx"
	"fbc/convert/kfx/envelope"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:], os.Stderr)
	stop()
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, log io.Writer) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(log, "usage: kfxcompress <compress|uncompress> [--overwrite] INPUT OUTPUT")
		fmt.Fprintln(log, "compress --uncompressed-envelope writes the diagnostic plaintext control")
		return nil
	}
	if len(args) == 0 || (args[0] != "compress" && args[0] != "uncompress") {
		return errors.New("usage: kfxcompress <compress|uncompress> [--overwrite] INPUT OUTPUT")
	}
	operation := args[0]
	flags := flag.NewFlagSet(operation, flag.ContinueOnError)
	flags.SetOutput(log)
	overwrite := flags.Bool("overwrite", false, "atomically replace existing output (including input)")
	var plaintext bool
	if operation == "compress" {
		flags.BoolVar(&plaintext, "uncompressed-envelope", false, "write a diagnostic plaintext envelope without LZMA")
	}
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return errors.New("exactly one INPUT and one explicit OUTPUT are required")
	}
	started := time.Now()
	input, output := flags.Arg(0), flags.Arg(1)
	if _, err := os.Stat(output); err == nil && !*overwrite {
		return fmt.Errorf("output already exists: %s (use --overwrite)", output)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("read %s: %w", input, err)
	}
	result, info, err := transform(ctx, data, operation, plaintext)
	if err != nil {
		return err
	}
	if err := writeAtomic(ctx, output, result, *overwrite); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}
	fmt.Fprintln(log, info.Summary())
	fmt.Fprintf(log, "%s: %s -> %s, elapsed=%s\n", operation, input, output, time.Since(started))
	return nil
}

func transform(ctx context.Context, data []byte, operation string, plaintext bool) ([]byte, envelope.Info, error) {
	inner := data
	var info envelope.Info
	wrapped := envelope.IsEnvelope(data)
	if wrapped {
		e, err := envelope.Inspect(ctx, data)
		if err != nil {
			return nil, info, err
		}
		info = e.Info()
		if operation == "compress" {
			if plaintext {
				return nil, info, errors.New("diagnostic plaintext control requires ordinary CONT input")
			}
			if info.Mode == envelope.LZMA {
				return nil, info, errors.New("input is already compressed")
			}
		}
		inner, err = e.Decode(ctx)
		if err != nil {
			return nil, info, err
		}
	} else if operation == "uncompress" {
		return nil, info, errors.New("input is already ordinary CONT; an envelope is required")
	}
	if err := kfx.ValidateContainer(inner); err != nil {
		return nil, info, fmt.Errorf("invalid ordinary CONT input: %w", err)
	}
	if operation == "uncompress" {
		return inner, info, nil
	}
	mode := envelope.LZMA
	if plaintext {
		mode = envelope.Plaintext
	}
	return envelope.Encode(ctx, inner, mode)
}

func writeAtomic(ctx context.Context, path string, data []byte, overwrite bool) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		// Close may already have succeeded; cleanup must not replace the publication result.
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	if err := f.Chmod(0644); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if overwrite {
		return replaceOutput(f.Name(), path)
	}
	// Publish atomically only if the destination is still absent; plain Rename
	// would overwrite a file created while we were encoding.
	return publishOutput(f.Name(), path)
}
