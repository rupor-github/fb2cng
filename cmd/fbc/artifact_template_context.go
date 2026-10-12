package main

import (
	"path/filepath"
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
				values.Source, values.Format, values.OutputDir = preparseConvertInvocation(args[index+1:])
			}
		}
	}

	if values.Command == "convert" {
		if values.Source == "" {
			values.Source = cmd.Args().Get(0)
		}
		if cmd.Name == "convert" || !values.Format.IsValid() {
			format, err := common.ParseOutputFmt(cmd.String("to"))
			if err != nil {
				format = common.OutputFmtEpub2
			}
			values.Format = format
		}
		if cmd.Name == "convert" {
			values.OutputDir = cmd.Args().Get(1)
			if outputFile := cmd.String("output-file"); outputFile != "" {
				values.OutputDir = filepath.Dir(outputFile)
			}
		}
	}

	return values
}

func preparseInvocationCommand(args []string) (string, int) {
	for i := 0; i < len(args); i++ {
		arg, _, inline := strings.Cut(args[i], "=")
		switch {
		case arg == "--config" || arg == "-config" || arg == "--c" || arg == "-c":
			if !inline {
				i++
			}
		case strings.HasPrefix(arg, "-"):
		default:
			return args[i], i
		}
	}
	return "", -1
}

func preparseConvertInvocation(args []string) (string, common.OutputFmt, string) {
	format := common.OutputFmtEpub2
	outputFile := ""
	// urfave/cli keeps parsing flags after positional arguments, so collect
	// positionals instead of treating the raw tail as SOURCE and DESTINATION.
	positionals := make([]string, 0, 2)
	for i := 0; i < len(args); i++ {
		arg, value, inline := strings.Cut(args[i], "=")
		switch {
		case arg == "--":
			positionals = append(positionals, args[i+1:]...)
			return convertPositionalValues(positionals, format, outputFile)
		case arg == "-":
			positionals = append(positionals, args[i])
		case arg == "--to" || arg == "-to":
			if !inline && i+1 < len(args) {
				i++
				value = args[i]
			}
			if parsed, err := common.ParseOutputFmt(value); err == nil {
				format = parsed
			} else {
				format = common.OutputFmtEpub2
			}
		case arg == "--output-file" || arg == "-output-file" || arg == "-o" || arg == "--o":
			if !inline && i+1 < len(args) {
				i++
				value = args[i]
			}
			outputFile = value
		case convertFlagTakesValue(arg):
			if !inline {
				i++
			}
		case strings.HasPrefix(arg, "-"):
		default:
			positionals = append(positionals, args[i])
		}
	}
	return convertPositionalValues(positionals, format, outputFile)
}

func convertPositionalValues(args []string, format common.OutputFmt, outputFile string) (string, common.OutputFmt, string) {
	source, outputDir := "", "."
	if len(args) > 0 {
		source = args[0]
	}
	if outputFile != "" {
		outputDir = filepath.Dir(outputFile)
	} else if len(args) > 1 && args[1] != "" {
		outputDir = args[1]
	}
	return source, format, outputDir
}

func convertFlagTakesValue(arg string) bool {
	if !strings.HasPrefix(arg, "-") {
		return false
	}
	switch strings.TrimLeft(arg, "-") {
	case "asin", "output-file", "o", "force-zip-cp", "kfx-compression", "config", "c":
		return true
	default:
		return false
	}
}
