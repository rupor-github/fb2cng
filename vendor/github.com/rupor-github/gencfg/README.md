
gencfg
========

Combination of importable module and a CLI tool on top of it to simplify dealing
with YAML configuration files in Go code.

Rather than using 3rd party packages to handle configuration lately it seems
preferable to use Go built in support for marshaling and struct tags. Taking
into account requirements of local development and testing it's difficult to
prevent configuration logic from spreading across your code. Package
provides some tooling to solve this problem to a degree allowing most of the
logic (including setting defaults) to be in the configuration template
(possibly embedded into resulting binary). 

It introduces an ability to use [Go Templating engine](https://golang.org/pkg/text/template/)
with added support for [slim-sprig lib](https://go-task.github.io/slim-sprig/)
and some small extensions to derive "values" in the configuration. This
makes it possible to both generate configuration files and to create
configurations for production usage, development and testing in a coherent way
hopefully from a single configuration template.

## Template variables defined by project

	.Name (string) - name of the YAML node value is being assigned to
	.ProjectDir (string) - used to expand relative paths in configuration, could be passed in
	.Hostname (string) - Go's os.Hostname()
    .IPv4 (string) - IPv4 address of local host, not loopback address
	.Containerized (bool) - true if code is executed in container
    .Testing (bool) - true if expansion happens when code is being run with "go test"
    .CPUs (int) - Go's runtime.NumCPU()
    .OS (string) - Go's runtime.GOOS
    .ARCH (string) - Go's runtime.GOARCH
    .Arguments (map[strting]string) - could be passed to Process() using .WithArgument(name,value) calls

## Template functions defined by project in addition to sprig

    joinPath - Joins any number of arguments into a path. The same as Go's filepath.Join.
    freeLocalPort - takes no arguments, returns free unique local port to be used for testing. For running tests in parallel implementation keeps global port map.


## Example of using in your code, just to give you an idea

    import (
        _ "embed"
        "github.com/rupor-github/gencfg"
    )

    // Embedded configuration template - provides smart defaults
    //go:embed config.yaml.tmpl
    var ConfigTmpl []byte

    // Configuration structures
    type (
        Config struct {
            SourcePath   string `yaml:"source" sanitize:"path_abs,path_toslash" validate:"required,dir"`
            TargetPath   string `yaml:"target" sanitize:"path_clean,path_toslash" validate:"required,filepath|email"`
        }
    )

    func unmarshalConfig(data []byte, cfg *Config, process bool) (*Config, error) {
        // We want to use only fields we defined so we cannot use yaml.Unmarshal directly here
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
            if err := gencfg.Validate(cfg, gencfg.WithAdditionalChecks(checks)); err != nil {
                return nil, err
            }
        }
        return cfg, nil
    }

    // LoadConfiguration reads the configuration from the file at the given path, superimposes its values on
    // top of expanded configuration tamplate to provide sane defaults and performs validation.
    func LoadConfiguration(path string, options ...func(*gencfg.ProcessingOptions)) (*Config, error) {
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

ProcessingOptions allows specifying root directory for expanding relative paths
in configuration uniformly (.WithRootDir), passing additional arguments to
templates (.WithArgument) and marking some fields not to be expanded as
templates (.WithDoNotExpandField). You could also add some custom validation
code if necessary (see below).

## Command line tool

Sometimes you may want to get actual configuration file for your project. Use
CLI tool from this project (could be part of your build). It's also good
example of how to use imported gencfg in your code.

    ❯ gencfg -h

    NAME:
       gencnf - generate configuration file from template

    USAGE:
       gencnf [options] TEMPLATE [DESTINATION]

    OPTIONS:
       --project-dir value, -d value  Project directory to use for expansion (default is current directory)
       --help, -h                     show help (default: false)
       --version, -v                  print the version (default: false)

## Some examples of template expansion in configuration

Reading database user/password from environment and using defaults otherwise:

    db:
        username: '{{ default "user" (env "DB_USERNAME") }}'
        password: '{{ default "pass" (env "DB_PASSWORD") }}'

Setting parameters for logging from environment:

    logging:
            level: info
            # do not use "log timestamps" when running inside docker, rely on journald and docker logs to maintain timestamps
            use_timestamp: "{{ not .Containerized }}"

## Sanitizing configuration values

`gencfg` module has additional capability of sanitizing configuration values.
You can set "sanitize" tag on a configuration field and call Sanitize() function
after your configuration unmarshaled into the struct in your code.

    type SomeConfig struct {
        WorkerPoolSize uint32 `yaml:"worker_pool_size"`
        TempDir        string `yaml:"temp_dir" sanitize:"path_clean"`
    }

    cfg := &SomeConfig{}
    // Code to initialize cfg structure goes here (read from file, unmarshal, set...)
    .........

    // sanitize will call filepath.Clean() on TempDir value and assign the resulting cleaned path back to the TempDir field.
	if err := gencfg.Sanitize(&cfg); err != nil {
		// Prossing sanitization errors here
        .........
	}

Recognized actions called on fields with tags so path will be reset if
necessary. Sanitize supports comma-separated list of actions and calls them in
definition order.

Presently defined actions:

    path_clean - same as calling filepath.Clean(value) on the configuration field
    path_toslash - same as calling filepath.ToSlash(value) on the configuration field
    path_abs - same as calling filepath.Abs(value) on the configuration field
    assure_dir_exists - will call os.MkdirAll(value), not changing field itself
    assure_dir_exists_for_file - will call os.MkdirAll(filepath.Dir(value)), not changing field itself
    assure_file_access - will do filepath.Abs and os.stat on the result
    oneof_or_tag - checks if value is in whitespace-separated list of allowed values; if not, applies the last token as a sanitize tag
                   Example: sanitize:"oneof_or_tag=opt1 opt2 opt3 path_clean" - if value is opt1, opt2, or opt3, leave as-is; otherwise apply path_clean

## Validating configuration values

`gencfg` module has additional capability of validating configuration values
using code from [validator](https://pkg.go.dev/github.com/go-playground/validator/v10) project.

Note, that when supported tags aren't sufficient you could pass in custom
function to perform additional checks.

    type SomeConfig struct {
        WorkerPoolSize uint32 `yaml:"worker_pool_size" validate:"required"`
        TempDir        string `yaml:"temp_dir" sanitize:"path_clean,assure_dir_exists" validate:"required,gt=1"`
    }

    // To perform "unusual" specific checking, most likely involves cross-field, cross embedded stuctures validation
    func additionalChecks(sl validator.StructLevel) {

        c := sl.Current().Interface().(SomeConfig)

        if WorkerPoolSize == 999 {
            sl.ReportError(c.WorkerPoolSize, "WorkerPoolSize", "", "do not like size must be 666", "")
        }
        ..........
    }
    ..........

    cfg := &SomeConfig{}
    // Code to initialize cfg structure goes here (read from file, unmarshal, set...)
    .........

	if err := gencfg.Sanitize(&cfg); err != nil {
		// Processing sanitization errors here
        .........
	}
	if err := gencfg.Validate(&cfg, gencfg.WithAdditionalChecks(additionalChecks)); err != nil {
		// Processing validation errors here
        .........
	}

Validate() returns all found problems at once, it would not fail on each
consecutive violation encountered. Please, read
[documentation](https://pkg.go.dev/github.com/go-playground/validator/v10#readme-baked-in-validations)
for more details on available checks.
