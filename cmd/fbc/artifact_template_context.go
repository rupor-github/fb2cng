package main

import (
	"strings"
	"time"

	cli "github.com/urfave/cli/v3"

	"fbc/common"
	"fbc/config"
)

// artifactTemplateValues builds the context used only for log/report destination
// templates. urfave/cli runs root Before hooks before passing the subcommand
// object to its own Before hook, so root initialization cannot read parsed
// convert arguments from cmd directly. We keep the real command execution under
// urfave/cli and pre-read just enough raw invocation data to populate early
// artifact names without moving logger/report setup later in the lifecycle.
func artifactTemplateValues(started time.Time, cmd *cli.Command, args []string) config.ArtifactTemplateValues {
	values := config.ArtifactTemplateValues{
		Started: started,
		Command: cmd.Name,
	}

	path := cmd.Path()
	if len(path) > 0 {
		values.Command = path[len(path)-1]
	}
	if values.Command == cmd.Root().Name {
		if command, index := preparseInvocationCommand(args); command != "" {
			values.Command = command
			if command == "convert" {
				values.Source, values.Format = preparseConvertInvocation(args[index+1:])
			}
		}
	}

	if values.Command == "convert" {
		if values.Source == "" {
			values.Source = cmd.Args().Get(0)
		}
		if !values.Format.IsValid() {
			format, err := common.ParseOutputFmt(cmd.String("to"))
			if err != nil {
				format = common.OutputFmtEpub2
			}
			values.Format = format
		}
	}

	return values
}

func preparseInvocationCommand(args []string) (string, int) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--config" || arg == "-c":
			i++
		case strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "-c="):
		case arg == "--debug" || arg == "-d":
		case strings.HasPrefix(arg, "-"):
		default:
			return arg, i
		}
	}
	return "", -1
}

func preparseConvertInvocation(args []string) (string, common.OutputFmt) {
	format := common.OutputFmtEpub2
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			if i+1 < len(args) {
				return args[i+1], format
			}
			return "", format
		case arg == "--to":
			if i+1 < len(args) {
				if parsed, err := common.ParseOutputFmt(args[i+1]); err == nil {
					format = parsed
				}
				i++
			}
		case strings.HasPrefix(arg, "--to="):
			if parsed, err := common.ParseOutputFmt(strings.TrimPrefix(arg, "--to=")); err == nil {
				format = parsed
			}
		case convertFlagTakesValue(arg):
			i++
		case strings.HasPrefix(arg, "-"):
		default:
			return arg, format
		}
	}
	return "", format
}

func convertFlagTakesValue(arg string) bool {
	return arg == "--asin" || arg == "--output-file" || arg == "-o" || arg == "--force-zip-cp"
}
