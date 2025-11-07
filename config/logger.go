package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"

	"fbc/misc"
)

type LoggerConfig struct {
	Level       string `yaml:"level" validate:"required,oneof=none debug normal"`
	Destination string `yaml:"destination,omitempty" sanitize:"path_clean,assure_dir_exists_for_file" validate:"omitempty,filepath"`
	Mode        string `yaml:"mode,omitempty" validate:"omitempty,oneof=append overwrite"`
}

type LoggingConfig struct {
	FileLogger    LoggerConfig `yaml:"file"`
	ConsoleLogger LoggerConfig `yaml:"console"`
}

// Prepare returns our standard logger - configured zap logger for use by the program.
func (conf *LoggingConfig) Prepare(rpt *Report) (*zap.Logger, error) {

	// Console - split stdout and stderr, handle colors and redirection

	ec := zap.NewDevelopmentEncoderConfig()
	ec.EncodeCaller = nil
	if EnableColorOutput(os.Stdout) {
		ec.EncodeLevel = zapcore.CapitalColorLevelEncoder
		ec.TimeKey = zapcore.OmitKey
	} else {
		ec.EncodeLevel = zapcore.CapitalLevelEncoder
	}
	consoleEncoderLP := zapcore.NewConsoleEncoder(ec)

	ec = zap.NewDevelopmentEncoderConfig()
	ec.EncodeCaller = nil
	if EnableColorOutput(os.Stderr) {
		ec.EncodeLevel = zapcore.CapitalColorLevelEncoder
		ec.TimeKey = zapcore.OmitKey
	} else {
		ec.EncodeLevel = zapcore.CapitalLevelEncoder
	}
	consoleEncoderHP := newEncoder(ec) // filter errorVerbose

	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})

	var consoleCoreHP, consoleCoreLP zapcore.Core
	switch conf.ConsoleLogger.Level {
	case "normal":
		consoleCoreLP = zapcore.NewCore(consoleEncoderLP, zapcore.Lock(os.Stdout),
			zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return zapcore.InfoLevel <= lvl && lvl < zapcore.ErrorLevel
			}))
		consoleCoreHP = zapcore.NewCore(consoleEncoderHP, zapcore.Lock(os.Stderr), highPriority)
	case "debug":
		consoleCoreLP = zapcore.NewCore(consoleEncoderLP, zapcore.Lock(os.Stdout),
			zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return zapcore.DebugLevel <= lvl && lvl < zapcore.ErrorLevel
			}))
		consoleCoreHP = zapcore.NewCore(consoleEncoderHP, zapcore.Lock(os.Stderr), highPriority)
	default:
		consoleCoreLP = zapcore.NewNopCore()
		consoleCoreHP = zapcore.NewNopCore()
	}

	// File

	opener := func(fname, mode string) (f *os.File, err error) {
		flags := os.O_CREATE | os.O_WRONLY
		if mode == "append" {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		if f, err = os.OpenFile(fname, flags, 0644); err != nil {
			return nil, err
		}
		return f, nil
	}

	var (
		fileEncoder    zapcore.Encoder
		fileCore       zapcore.Core
		logLevel       zap.AtomicLevel
		logRequested   bool
		levelRequested = conf.FileLogger.Level
		modeRequested  = conf.FileLogger.Mode
	)

	if rpt != nil {
		// if report is requested always set maximum available logging level for file logger
		levelRequested = "debug"
		modeRequested = "overwrite"
	}

	switch levelRequested {
	case "debug":
		fileEncoder = zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
		logLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
		logRequested = true
	case "normal":
		fileEncoder = zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
		logLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
		logRequested = true
	}

	var newName string
	if logRequested {

		// capture panic log if possible
		var (
			ef  *os.File
			err error
		)
		if ef, err = opener(filepath.Join(filepath.Dir(conf.FileLogger.Destination), misc.GetAppName()+"-panic.log"), modeRequested); err == nil {
		} else if ef, err = os.CreateTemp("", misc.GetAppName()+"-panic.*.log"); err == nil {
		} else {
			// just quietly ignore
			ef = nil
		}
		if ef != nil {
			debug.SetCrashOutput(ef, debug.CrashOptions{})
			rpt.Store("panic.log", ef.Name())
			ef.Close()
		}

		if f, err := opener(conf.FileLogger.Destination, modeRequested); err == nil {
			fileCore = zapcore.NewCore(fileEncoder, zapcore.Lock(f), logLevel)
			rpt.Store("final.log", f.Name())
		} else if f, err = os.CreateTemp("", misc.GetAppName()+".*.log"); err == nil {
			newName = f.Name()
			fileCore = zapcore.NewCore(fileEncoder, zapcore.Lock(f), logLevel)
			rpt.Store("final.log", newName)
		} else {
			return nil, fmt.Errorf("unable to access file log destination (%s): %w", conf.FileLogger.Destination, err)
		}
	} else {
		fileCore = zapcore.NewNopCore()
	}

	core := zap.New(zapcore.NewTee(consoleCoreHP, consoleCoreLP, fileCore), zap.AddCaller())
	if len(newName) != 0 {
		// log was redirected - we need to report this
		core.Warn("Log file was redirected to new location", zap.String("location", newName))
	}
	return core.Named(misc.GetAppName()), nil
}

// When logging error to console - do not output verbose message.

type consoleEnc struct {
	zapcore.Encoder
}

func newEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return consoleEnc{zapcore.NewConsoleEncoder(cfg)}
}

func (c consoleEnc) Clone() zapcore.Encoder {
	return consoleEnc{c.Encoder.Clone()}
}

func (c consoleEnc) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	var newFields []zapcore.Field
	for _, f := range fields {
		if f.Type == zapcore.ErrorType {
			// presently superficial - but we may need to shorten what is printed to console in the future
			e := f.Interface.(error)
			f.Interface = errors.New(e.Error())
		}
		newFields = append(newFields, f)
	}
	return c.Encoder.EncodeEntry(ent, newFields)
}
