package convert

import (
	"archive/zip"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	cli "github.com/urfave/cli/v3"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/ianaindex"

	"fbc/archive"
	"fbc/common"
	"fbc/content"
	"fbc/convert/epub"
	"fbc/convert/kfx"
	"fbc/state"
)

//go:embed default.jpeg
var defaultCoverImage []byte

//go:embed default.css
var defaultStylesheet []byte

func Run(ctx context.Context, cmd *cli.Command) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}

	env := state.EnvFromContext(ctx)
	log := env.Log.Named("convert")

	src := cmd.Args().Get(0)
	if len(src) == 0 {
		return errors.New("no input source has been specified")
	}
	src, err = filepath.Abs(src)
	if err != nil {
		return err
	}

	dst := cmd.Args().Get(1)
	if len(dst) == 0 {
		if dst, err = os.Getwd(); err != nil {
			return fmt.Errorf("unable to get working directory: %w", err)
		}
	}
	if dst, err = filepath.Abs(dst); err != nil {
		return err
	}
	if cmd.Args().Len() > 2 {
		log.Warn("Mailformed command line, too many destinations", zap.Strings("ignoring", cmd.Args().Slice()[2:]))
	}

	format, err := common.ParseOutputFmt(cmd.String("to"))
	if err != nil {
		log.Warn("Unknown output format requested, switching to epub2", zap.Error(err))
		format = common.OutputFmtEpub2
	}

	// Amazon formats must always have valid cover page
	if format.ForKindle() {
		env.Cfg.Document.Images.Cover.Generate = true
		if env.Cfg.Document.Images.Cover.Resize == common.ImageResizeModeNone {
			env.Cfg.Document.Images.Cover.Resize = common.ImageResizeModeKeepAR
		}
		env.Cfg.Document.Images.Cover.Generate = true
	}

	if env.Cfg.Document.Images.Cover.Generate {
		env.DefaultCover = defaultCoverImage
		if env.Cfg.Document.Images.Cover.DefaultImagePath != "" {
			data, err := os.ReadFile(env.Cfg.Document.Images.Cover.DefaultImagePath)
			if err != nil {
				return fmt.Errorf("unable to read default cover image from %q: %w", env.Cfg.Document.Images.Cover.DefaultImagePath, err)
			}
			env.DefaultCover = data
		}
	}

	env.DefaultStyle = defaultStylesheet
	if env.Cfg.Document.StylesheetPath != "" {
		data, err := os.ReadFile(env.Cfg.Document.StylesheetPath)
		if err != nil {
			return fmt.Errorf("unable to read style css from %q: %w", env.Cfg.Document.StylesheetPath, err)
		}
		env.DefaultStyle = data
	}

	env.NoDirs, env.Overwrite = cmd.Bool("nodirs"), cmd.Bool("overwrite")

	// Since zip "standard" does not define file name encoding we may need to
	// force archaic code page for old archives
	cp := cmd.String("force-zip-cp")
	if len(cp) > 0 {
		env.CodePage, err = ianaindex.IANA.Encoding(cp)
		if err != nil {
			log.Warn("Unknown character set specification. Ignoring...", zap.String("charset", cp), zap.Error(err))
			env.CodePage = nil
		} else {
			n, _ := ianaindex.IANA.Name(env.CodePage)
			log.Debug("Forcefully converting all non UTF-8 file names in archives", zap.String("charset", n))
		}
	}

	log.Info("Processing starting", zap.String("source", src), zap.String("destination", dst), zap.Stringer("format", format))
	defer func(start time.Time) {
		log.Info("Processing completed", zap.Duration("elapsed", time.Since(start)))
	}(time.Now())

	return process(ctx, src, dst, format, log)
}

// process handles the core conversion logic independently of CLI framework. It
// determines the input type (directory, archive, or single file) and processes
// accordingly.
func process(ctx context.Context, src, dst string, format common.OutputFmt, log *zap.Logger) error {
	var head, tail string
	for head = src; len(head) != 0; head, tail = filepath.Split(head) {
		if err := ctx.Err(); err != nil {
			return err
		}

		head = strings.TrimSuffix(head, string(filepath.Separator))

		fi, err := os.Stat(head)
		if err != nil {
			// does not exists - probably path in archive
			continue
		}

		if fi.Mode().IsDir() {
			if len(tail) != 0 {
				// directory cannot have tail - it would be simple file
				return fmt.Errorf("input source was not found (%s) => (%s)", head, strings.TrimPrefix(src, head))
			}
			if err := processDir(ctx, head, dst, format, log); err != nil {
				return errors.New("unable to process directory")
			}
			break
		}

		if !fi.Mode().IsRegular() {
			return fmt.Errorf("unexpected path mode for (%s) => (%s)", head, strings.TrimPrefix(src, head))
		}

		archive, err := isArchiveFile(head)
		if err != nil {
			// checking format - but cannot open target file
			return fmt.Errorf("unable to check archive type: %w", err)
		}
		if archive {
			// we need to look inside to see if path makes sense
			tail = strings.TrimPrefix(strings.TrimPrefix(src, head), string(filepath.Separator))
			if err := processArchive(ctx, head, tail, "", dst, format, log); err != nil {
				return fmt.Errorf("unable to process archive: %w", err)
			}
			break
		}

		book, enc, err := isBookFile(head)
		if err != nil {
			// checking format - but cannot open target file
			return fmt.Errorf("unable to check file type: %w", err)

		}
		if book && len(tail) == 0 {
			// we have book, it cannot have tail
			// encoding will be handled properly by processBook
			if file, err := os.Open(head); err != nil {
				log.Error("Unable to process file", zap.String("file", head), zap.Error(err))
			} else {
				defer file.Close()
				if err := processBook(ctx, selectReader(file, enc), filepath.Base(head), dst, format, log); err != nil {
					log.Error("Unable to process file", zap.String("file", head), zap.Error(err))
				}
			}
			break
		}
		return fmt.Errorf("input was not recognized as FB2 book (%s)", head)

	}
	if len(head) == 0 {
		return fmt.Errorf("input source was not found (%s)", src)
	}
	return nil
}

// processDir walks directory tree finding fb2 files and processes them.
func processDir(ctx context.Context, dir, dst string, format common.OutputFmt, log *zap.Logger) (err error) {
	count := 0
	defer func() {
		if err == nil && count == 0 {
			log.Debug("Nothing to process", zap.String("dir", dir))
		}
	}()

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err != nil {
			log.Warn("Skipping path", zap.String("path", path), zap.Error(err))
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		archive, err := isArchiveFile(path)
		if err != nil {
			// checking format - but cannot open target file
			log.Warn("Skipping file", zap.String("file", path), zap.Error(err))
			return nil
		}
		if archive {
			if err := processArchive(ctx, path, "", filepath.Dir(strings.TrimPrefix(path, dir)), dst, format, log); err != nil {
				log.Error("Unable to process archive", zap.String("file", path), zap.Error(err))
			}
			return nil
		}

		book, enc, err := isBookFile(path)
		if err != nil {
			log.Warn("Skipping file", zap.String("file", path), zap.Error(err))
			return nil
		}
		if !book {
			log.Debug("Skipping file, not recognized as book or archive", zap.String("file", path))
			return nil
		}

		count++

		file, err := os.Open(path)
		if err != nil {
			log.Error("Unable to process file", zap.String("file", path), zap.Error(err))
			return nil
		}
		defer file.Close()

		src := strings.TrimPrefix(strings.TrimPrefix(path, dir), string(filepath.Separator))
		if err := processBook(ctx, selectReader(file, enc), src, dst, format, log); err != nil {
			log.Error("Unable to process file", zap.String("file", path), zap.Error(err))
		}
		return nil
	})
	return err
}

// processArchive walks all files inside archive, finds fb2 files under
// "pathIn" and processes them.
func processArchive(ctx context.Context, path, pathIn, pathOut, dst string, format common.OutputFmt, log *zap.Logger) (err error) {
	count := 0
	defer func() {
		if err == nil && count == 0 {
			log.Debug("Nothing to process", zap.String("archive", path))
		}
	}()

	err = archive.Walk(path, pathIn, func(archive string, f *zip.File) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		book, enc, err := isBookInArchive(f)
		if err != nil {
			log.Warn("Skipping file in archive",
				zap.String("archive", archive), zap.String("path", f.FileHeader.Name), zap.Error(err))
			return nil
		}
		if !book {
			log.Debug("Skipping file, not recognized as book", zap.String("archive", archive), zap.String("file", f.FileHeader.Name))
			return nil
		}

		count++

		r, err := f.Open()
		if err != nil {
			log.Error("Unable to process file in archive",
				zap.String("archive", archive), zap.String("file", f.FileHeader.Name), zap.Error(err))
			return nil
		}
		defer r.Close()

		cp := state.EnvFromContext(ctx).CodePage

		pathInArchive := f.FileHeader.Name
		if cp != nil && f.FileHeader.NonUTF8 {
			// forcing zip file name encoding
			if n, err := cp.NewDecoder().String(pathInArchive); err == nil {
				pathInArchive = n
			} else {
				n, _ = ianaindex.IANA.Name(cp)
				log.Warn("Unable to convert archive name from specified encoding",
					zap.String("charset", n), zap.String("path", pathInArchive), zap.Error(err))
			}
		}
		if err := processBook(ctx, selectReader(r, enc), filepath.Join(pathOut, pathInArchive), dst, format, log); err != nil {
			log.Error("Unable to process file in archive",
				zap.String("archive", archive), zap.String("file", f.FileHeader.Name), zap.Error(err))
		}
		return nil
	})
	return err
}

// processBook processes single FB2 file. "src" is part of the source path
// (always including file name) relative to the original path. When actual file
// was specified it will be just base file name without a path. When looking
// inside archive or directory it will be relative path inside archive or
// directory (including base file name). "dst" is the destination directory
// where the converted file should be written.
func processBook(ctx context.Context, r io.Reader, src string, dst string, format common.OutputFmt, log *zap.Logger) (rerr error) {
	env := state.EnvFromContext(ctx)

	var refID, outputName string

	log.Info("Conversion starting", zap.String("from", src))
	defer func(start time.Time) {
		// NOTE: some of golang graphic processing libraries are not mature
		// enough if multiple books are being processed we do not want to stop.
		if r := recover(); r != nil {
			log.Error("Conversion ended with panic",
				zap.Any("panic", r), zap.Duration("elapsed", time.Since(start)), zap.String("to", outputName), zap.ByteString("stack", debug.Stack()))
			rerr = fmt.Errorf("conversion panic: %v", r)
		} else {
			log.Info("Conversion completed", zap.Duration("elapsed", time.Since(start)), zap.String("to", outputName), zap.String("ref_id", refID))
		}
	}(time.Now())

	c, err := content.Prepare(ctx, r, src, format, log)
	if err != nil {
		return fmt.Errorf("unable to parse fb2 source (%s): %w", src, err)
	}

	refID = c.Book.Description.DocumentInfo.ID

	// Determine output file name and path based on input and configuration.
	outputName = buildOutputPath(c, src, dst, env)

	// Check if output file already exists
	if _, err := os.Stat(outputName); err == nil {
		if !env.Overwrite {
			return fmt.Errorf("output file already exists: %s", outputName)
		}
		log.Warn("Overwriting existing file", zap.String("file", outputName))
		if err = os.Remove(outputName); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	} else if err := os.MkdirAll(filepath.Dir(outputName), 0755); err != nil {
		return fmt.Errorf("unable to create output directory: %w", err)
	}

	// Generate output in the requested format
	switch c.OutputFormat {
	case common.OutputFmtEpub2, common.OutputFmtEpub3, common.OutputFmtKepub:
		if err := epub.Generate(ctx, c, outputName, &env.Cfg.Document, log); err != nil {
			return fmt.Errorf("unable to generate output: %w", err)
		}
	case common.OutputFmtKfx:
		if err := kfx.Generate(ctx, c, outputName, &env.Cfg.Document, log); err != nil {
			return fmt.Errorf("unable to generate output: %w", err)
		}
	}

	// Store conversion result for debugging
	if env.Rpt != nil {
		env.Rpt.Store(fmt.Sprintf("result-%s%s", refID, filepath.Ext(outputName)), outputName)
	}

	return nil
}
