package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"github.com/rupor-github/gencfg"

	"fbc/common"
	"fbc/misc"
)

// In MHL
// params := Format('"%s" "%s"', [InpFile, ChangeFileExt(OutFile, '.epub')]);
// Result := ExecAndWait(FAppPath + 'converters\fb2epub\fb2epub.exe', params, SW_HIDE);

const usageMsg = `
	MyHomeLib wrapper for fb2 (ng) converter
	Version %s (%s) : %s

	Expected usage (MyHomeLib invocation): [fb2epub|fb2mobi] <from fb2> <to target file>

	MyHomeLib expect converters to be located in the installation directory with following structure:

	MHL installation directory
	|	MyHomeLib.exe
	|
	\---converters
		+---fb2converter
		|		fbc.exe
		|		mhl-connector.exe
		|
		+---fb2epub
		|		fb2epub.exe  (copy of mhl-connector.exe or symlink to it)
		|		fb2epub.yaml (fbc.exe configuration file if needed)
		|
		\---fb2mobi
				fb2mobi.exe  (copy of mhl-connector.exe or symlink to it)
				fb2mobi.yaml (fbc.exe configuration file if needed)

	If you are copying mhl-connector.exe you could either follow above structure or have fbc.exe in a OS PATH.

	If you are using symlinks, mhl-connector.exe should be located next to fbc.exe and they could be anywhere,
	no fb2converter directory or OS PATH modification is necessary.

	Since passing additional arguments via MyHomeLib is inconvinient -
	additional configuration file "connector.yaml" is supported. If required it
	should be located next to either connector copy or symlink (the same place
	where fb2epub.exe or fb2mobi.exe is). In most cases it is unnecessary.
	Today it supports mhl and fbc integration debugging and additional format
	specifications if necessary.
`

//go:embed connector.yaml.tmpl
var ConfigTmpl []byte

type (
	Config struct {
		Version        int               `yaml:"version" validate:"eq=1"`
		LogDestination string            `yaml:"log_destination,omitempty" sanitize:"path_clean,assure_dir_exists_for_file" validate:"omitempty,filepath"`
		Debug          bool              `yaml:"debug"`
		OutputFormat   *common.OutputFmt `yaml:"output_format,omitempty" validate:"omitempty,oneof=0 1 2 3"`
	}
)

func unmarshalConfig(data []byte, cfg *Config, process bool) (*Config, error) {
	// We want to use only fields we defined so we cannot use yaml.Unmarshal
	// directly here
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("failed to decode configuration data: %w", err)
	}
	if process {
		// sanitize and validate what has been loaded
		if err := gencfg.Sanitize(cfg); err != nil {
			return nil, err
		}
		if err := gencfg.Validate(cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// loadConfiguration reads the configuration from the file at the given path,
// superimposes its values on top of expanded configuration tamplate to provide
// sane defaults and performs validation.
func loadConfiguration(path string, options ...func(*gencfg.ProcessingOptions)) (*Config, error) {
	haveFile := len(path) > 0

	data, err := gencfg.Process(ConfigTmpl, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to process configuration template: %w", err)
	}
	cfg, err := unmarshalConfig(data, &Config{}, !haveFile)
	if err != nil {
		return nil, fmt.Errorf("failed to process configuration template: %w", err)
	}
	if !haveFile {
		return cfg, nil
	}

	// overwrite cfg values with values from the file
	data, err = os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	cfg, err = unmarshalConfig(data, cfg, haveFile)
	if err != nil {
		return nil, fmt.Errorf("failed to process configuration file: %w", err)
	}
	return cfg, nil
}

func main() {

	log.SetPrefix("\n*** ")

	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, usageMsg, misc.GetVersion(), runtime.Version(), misc.GetGitHash())
		os.Exit(0)
	}

	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}

	configPath := filepath.Join(filepath.Dir(exePath), "connector.yaml")
	if _, err := os.Stat(configPath); err != nil {
		configPath = ""
	}

	cfg, err := loadConfiguration(configPath)
	if err != nil {
		log.Fatalf("Unable to load configuration: %v", err)
	}

	if cfg.LogDestination != "" {
		f, err := os.OpenFile(cfg.LogDestination, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			log.Fatalf("Unable to open log file '%s': %v", cfg.LogDestination, err)
		}
		defer f.Close()

		// Set the standard logger's output to the file.
		log.SetOutput(f)
	}

	resolvedPath, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Started as: %s", exePath)
	log.Printf("Actual path: %s", resolvedPath)

	// let's locate actual conversion engine
	converterName := misc.GetAppName() + ".exe"

	paths := []string{
		filepath.Join(filepath.Dir(resolvedPath), converterName),            // where symlink points
		filepath.Join(filepath.Dir(exePath), converterName),                 // where I was started from
		filepath.Join(filepath.Dir(exePath), "fb2converter", converterName), // `pwd`/../fb2converter
		filepath.Join(converterName),                                        // in the system PATH
	}

	var converterPath string
	for _, p := range paths {
		if converterPath, err = exec.LookPath(p); err == nil {
			break
		}
		converterPath = ""
	}
	if len(converterPath) == 0 {
		log.Fatalf("Unable to locate conversion engine: %s", converterName)
	}

	// let's get the target name from the executable name
	target := strings.TrimSuffix(filepath.Base(exePath), filepath.Ext(exePath))
	if !strings.EqualFold(target, "fb2mobi") && !strings.EqualFold(target, "fb2epub") {
		log.Fatalf("MHL connector could be named either fb2mobi or fb2epub (or started via appropriate symlinks), current name is: %s. It should be invoked by MyHomeLib, never directly", target)
	}

	from, err := filepath.Abs(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stat(from); err != nil {
		log.Printf("Source file does not exist: %s", from)
		log.Fatal(err)
	}

	to, err := filepath.Abs(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	to = filepath.Dir(to)
	if _, err := os.Stat(to); err != nil {
		log.Printf("Destination directory does not exist: %s", to)
		log.Fatal(err)
	}

	args := make([]string, 0, 10)

	config := filepath.Join(filepath.Dir(exePath), target+".yaml")
	if _, err := os.Stat(config); err == nil {
		args = append(args, "-config", config)
	}

	if cfg.Debug {
		args = append(args, "-debug")
	}

	args = append(args, "convert")
	args = append(args, "--ow")

	if cfg.OutputFormat != nil {
		switch target {
		case "fb2mobi":
			if cfg.OutputFormat.ForKindle() {
				args = append(args, "--to", cfg.OutputFormat.String())
			} else {
				log.Printf("Output format %s is not supported for target %s, using kfx instead", cfg.OutputFormat.String(), target)
				args = append(args, "--to", "kfx")
			}
		case "fb2epub":
			if !cfg.OutputFormat.ForKindle() {
				args = append(args, "--to", cfg.OutputFormat.String())
			} else {
				log.Printf("Output format %s is not supported for target %s, using epub2 instead", cfg.OutputFormat.String(), target)
				args = append(args, "--to", "epub2")
			}
		}
	} else {
		switch target {
		case "fb2mobi":
			args = append(args, "--to", "kfx")
		case "fb2epub":
			args = append(args, "--to", "epub2")
		}
	}

	args = append(args, from)
	args = append(args, to)

	cmd := exec.Command(converterPath, args...)

	log.Printf("Starting %s with %q\n", converterPath, args)

	out, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Unable to redirect conversion engine output: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("Unable to start conversion engine: %v", err)
	}

	// read and print converter stdout
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		log.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Conversion engine stdout pipe broken: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			log.Println(string(ee.Stderr))
		}
		log.Fatalf("Conversion engine returned error: %v", err)
	}
}
