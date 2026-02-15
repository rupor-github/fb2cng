package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"

	cli "github.com/urfave/cli/v3"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/convert"
	"fbc/misc"
	"fbc/state"
)

// initializeAppContext prepares application context before command execution but
// after command line has been parsed
func initializeAppContext(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	var err error

	if cmd.NArg() == 0 {
		// nothing to do, just return
		return ctx, nil
	}

	env := state.EnvFromContext(ctx)

	configFile := cmd.String("config")
	if env.Cfg, err = config.LoadConfiguration(configFile); err != nil {
		return ctx, fmt.Errorf("unable to prepare configuration: %w", err)
	}
	if cmd.Bool("debug") {
		if env.Rpt, err = env.Cfg.Reporting.Prepare(); err != nil {
			return ctx, fmt.Errorf("unable to prepare debug reporter: %w", err)
		}
		// save complete processed configuration if external configuration was provided
		if len(configFile) > 0 {
			// we do not want any of your secrets!
			if data, err := config.Dump(env.Cfg); err == nil {
				env.Rpt.StoreData(fmt.Sprintf("config/%s", filepath.Base(configFile)), data)
			}
		}
	}
	if env.Log, err = env.Cfg.Logging.Prepare(env.Rpt); err != nil {
		return ctx, fmt.Errorf("unable to prepare logs: %w", err)
	}
	env.RedirectStdLog()

	env.Log.Debug("Program started", zap.Strings("args", os.Args), zap.String("ver", misc.GetVersion()), zap.String("runtime", runtime.Version()), zap.String("hash", misc.GetGitHash()))

	if env.Rpt != nil {
		env.Log.Info("Creating debug report", zap.String("location", env.Rpt.Name()))
	}
	if len(configFile) == 0 && env.Log != nil {
		env.Log.Info("Using defaults (no configuration file)")
	}
	return ctx, nil
}

func destroyAppContext(ctx context.Context, cmd *cli.Command) (err error) {
	env := state.EnvFromContext(ctx)

	if env.Log != nil {
		env.Log.Debug("Program ended", zap.Duration("elapsed", env.Uptime()), zap.Strings("parsed args", cmd.Args().Slice()))
	}

	// close logging
	env.RestoreStdLog()

	// log is synced now and result can be used in report if necessary, errors
	// must be reported directly to stderr from now on
	if env.Rpt != nil {
		if er := env.Rpt.Close(); er != nil {
			err = multierr.Append(err, fmt.Errorf("unable to close debug report: %w", er))
		}
	}
	// reporting is closed now - remove empty panic file if any
	if env.Cfg != nil && len(env.Cfg.Logging.FileLogger.Destination) > 0 {
		debug.SetCrashOutput(nil, debug.CrashOptions{})
		fname := filepath.Join(filepath.Dir(env.Cfg.Logging.FileLogger.Destination), misc.GetAppName()+"-panic.log")
		if fi, er := os.Stat(fname); er == nil && fi.Size() == 0 {
			if er := os.Remove(fname); er != nil {
				err = multierr.Append(err, fmt.Errorf("unable to remove empty panic log file '%s': %w", fname, er))
			}
		}
	}
	return
}

// Ignore urfave/cli default error handling - for me cli.Exit() looks
// non-transparent and unnesessary. I will return regular errors from
// subcommands.
var errWasHandled bool

// this is called before appContext is destroyed, so we have a chance to
// properly log any error from subcommand
func exitErrHandler(ctx context.Context, _ *cli.Command, err error) {

	env := state.EnvFromContext(ctx)

	if env.Log != nil {
		env.Log.Error("Program ended with error", zap.Error(err))
		errWasHandled = true
	}
}

func usageErrorHandler(_ context.Context, _ *cli.Command, err error, _ bool) error {
	// do nothing special, error is reported either by exitErrHandler or on
	// exit directly to stderr.
	return err
}

func subcommandNotFoundHandler(ctx context.Context, _ *cli.Command, name string) {
	state.EnvFromContext(ctx).Log.Warn("Unknown command, nothing to do", zap.String("command", name))
}

func main() {

	// allow graceful shutdown on interrupt.
	// NOTE: normally in cli tool this is not necessary, but just in case we
	// may decide to do some heavy async processing later let's follow the
	// rules
	ctx, stop := signal.NotifyContext(state.ContextWithEnv(context.Background()), os.Interrupt, syscall.SIGTERM)

	app := &cli.Command{
		Name:            misc.GetAppName(),
		Usage:           "conversion engine for fiction book (FB2) files",
		Version:         misc.GetVersion() + " (" + runtime.Version() + ") : " + misc.GetGitHash(),
		HideHelpCommand: true,
		Before:          initializeAppContext,
		After:           destroyAppContext,
		OnUsageError:    usageErrorHandler,
		ExitErrHandler:  exitErrHandler,
		CommandNotFound: subcommandNotFoundHandler,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "config", Aliases: []string{"c"}, DefaultText: "", Usage: "load configuration from `FILE` (YAML)"},
			&cli.BoolFlag{Name: "debug", Aliases: []string{"d"}, Usage: "changes program behavior to help troubleshooting, produces report archive"},
		},
		Commands: []*cli.Command{
			{
				Name:         "convert",
				Usage:        "Converts FB2 file(s) to specified format",
				OnUsageError: usageErrorHandler,
				Action:       convert.Run,
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "to", Value: common.OutputFmtEpub2.String(),
						Usage: "conversion output `TYPE` (supported types: " + strings.Join(common.OutputFmtNames(), ", ") + ")"},
					&cli.BoolFlag{Name: "ebook", Aliases: []string{"eb"}, Usage: "for Kindle formats generate as ebook (EBOK) instead of personal document (PDOC)"},
					&cli.StringFlag{Name: "asin", Usage: "set ASIN (10 chars, A-Z0-9); used only for Kindle formats"},
					&cli.BoolFlag{Name: "nodirs", Aliases: []string{"nd"}, Usage: "when producing output do not keep input directory structure"},
					&cli.BoolFlag{Name: "overwrite", Aliases: []string{"ow"}, Usage: "continue even if destination exits, overwrite files"},
					&cli.StringFlag{Name: "force-zip-cp",
						Usage: "Force `ENCODING` for ALL non UTF-8 file names in processed archives (see IANA.org for character set names)"},
				},
				ArgsUsage: "SOURCE [DESTINATION]",
				CustomHelpTemplate: fmt.Sprintf(`%s
SOURCE:
    path to fb2 file(s) to process, following formats are supported:
        path to a file: "[path_to_file]file.fb2"
        path to a directory: "[path_to_directory]directory" - recursively process all files under directory (symbolic links are not followed)
        path to archive with path inside archive to a particular fb2 file: "[path_to_archive]archive.zip[path_in_archive]/file.fb2"
        path to archive with path inside archive: "[path_to_archive]archive.zip[path_in_archive]" - recursively process all fb2 files under archive path

	When working on archive recursively only fb2 files will be considered,
	processing of archives inside archives is not supported.

DESTINATION:
    always a path, output file name(s) and extension will be derived from other parameters
    if absent - current working directory
`, cli.CommandHelpTemplate),
			},
			{
				Name:  "dumpconfig",
				Usage: "Dumps either default or actual configuration (YAML)",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "default", Usage: "output default embedded configuration"},
				},
				OnUsageError: usageErrorHandler,
				Action:       outputConfiguration,
				ArgsUsage:    "DESTINATION",
				CustomHelpTemplate: fmt.Sprintf(`%s

DESTINATION:
    file name to write configuration to, if absent - STDOUT

Produces file with actual "active" configuration values wich is composition of
default values and values specified in configuration file. To see default
configuration embedded into the program use --default flag.
`, cli.CommandHelpTemplate),
			},
		},
	}

	var err error
	// NOTE: os.Exit is called at the end of main to set exit code, make sure
	// there are no other deffered functions after that
	defer func() {
		stop()
		if err != nil {
			// It may happen that log is either not set yet (argument parsing) or already closed,
			// report errors to stderr directly
			if !errWasHandled {
				fmt.Fprintf(os.Stderr, "Program ended with error: %v\n", err)
			}
			os.Exit(1)
		}
	}()
	err = app.Run(ctx, os.Args)
}

func outputConfiguration(ctx context.Context, cmd *cli.Command) error {

	env := state.EnvFromContext(ctx)
	if cmd.Args().Len() > 1 {
		env.Log.Warn("Malformed command line, too many destinations", zap.Strings("ignoring", cmd.Args().Slice()[1:]))
	}

	fname := cmd.Args().Get(0)

	var (
		err   error
		data  []byte
		state string
	)

	out := os.Stdout
	if len(fname) > 0 {
		out, err = os.Create(fname)
		if err != nil {
			return fmt.Errorf("unable to create destination file '%s': %w", fname, err)
		}
		defer out.Close()

	}

	if cmd.Bool("default") {
		state = "default"
		data, err = config.Prepare()
	} else {
		state = "actual"
		data, err = config.Dump(env.Cfg)
	}
	if err != nil {
		return fmt.Errorf("unable to get configuration: %w", err)
	}

	if len(fname) == 0 {
		fname = "STDOUT"
	}
	env.Log.Info("Outputing configuration", zap.String("state", state), zap.String("file", fname))

	_, err = out.Write(data)
	if err != nil {
		return fmt.Errorf("unable to write configuration: %w", err)
	}
	return nil
}
