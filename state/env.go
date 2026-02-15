// Package state defines shared program state.
package state

import (
	"context"
	"time"

	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"fbc/common"
	"fbc/config"
)

type envKey struct{}

// LocalEnv keeps everything program needs in a single place.
type LocalEnv struct {
	Cfg *config.Config
	Rpt *config.Report
	Log *zap.Logger

	// used by convert subcommand
	NoDirs           bool
	Overwrite        bool
	KindleEbook      bool
	KindleASIN       string
	CodePage         encoding.Encoding
	DefaultCover     []byte
	DefaultStyle     []byte
	DefaultVignettes map[common.VignettePos][]byte

	start         time.Time
	restoreStdLog func()
}

func EnvFromContext(ctx context.Context) *LocalEnv {
	if env, ok := ctx.Value(envKey{}).(*LocalEnv); ok {
		return env
	}
	// this should never happen
	panic("localenv not found in context")
}

func ContextWithEnv(ctx context.Context) context.Context {
	return context.WithValue(ctx, envKey{}, newLocalEnv())
}

func (e *LocalEnv) Uptime() time.Duration {
	return time.Since(e.start)
}

func (e *LocalEnv) RedirectStdLog() {
	if e.Log == nil {
		return
	}
	e.restoreStdLog = zap.RedirectStdLog(e.Log)
}

func (e *LocalEnv) RestoreStdLog() {
	if e.Log != nil {
		_ = e.Log.Sync()
	}
	if e.restoreStdLog != nil {
		e.restoreStdLog()
	}
}
