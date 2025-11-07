package state

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"fbc/config"
)

func TestContextWithEnv(t *testing.T) {
	ctx := ContextWithEnv(context.Background())
	if ctx == nil {
		t.Fatal("ContextWithEnv() returned nil")
	}

	env := EnvFromContext(ctx)
	if env == nil {
		t.Fatal("EnvFromContext() returned nil")
	}

	if env.start.IsZero() {
		t.Error("Environment start time not set")
	}
}

func TestEnvFromContext(t *testing.T) {
	t.Run("valid context", func(t *testing.T) {
		ctx := ContextWithEnv(context.Background())
		env := EnvFromContext(ctx)

		if env == nil {
			t.Error("Expected non-nil environment")
		}
	})

	t.Run("panic on missing env", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when env not in context")
			}
		}()

		// Use plain context without env
		EnvFromContext(context.Background())
	})
}

func TestLocalEnv_Uptime(t *testing.T) {
	ctx := ContextWithEnv(context.Background())
	env := EnvFromContext(ctx)

	time.Sleep(10 * time.Millisecond)
	uptime := env.Uptime()

	if uptime < 10*time.Millisecond {
		t.Errorf("Uptime() = %v, expected at least 10ms", uptime)
	}
	if uptime > 1*time.Second {
		t.Errorf("Uptime() = %v, unexpectedly large", uptime)
	}
}

func TestLocalEnv_RedirectStdLog(t *testing.T) {
	t.Run("with logger", func(t *testing.T) {
		env := &LocalEnv{
			Log: zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1))),
		}

		env.RedirectStdLog()
		if env.restoreStdLog == nil {
			t.Error("Expected restoreStdLog to be set")
		}

		env.RestoreStdLog()
	})

	t.Run("without logger", func(t *testing.T) {
		env := &LocalEnv{
			Log: nil,
		}

		// Should not panic
		env.RedirectStdLog()
		if env.restoreStdLog != nil {
			t.Error("Expected restoreStdLog to remain nil")
		}
	})
}

func TestLocalEnv_RestoreStdLog(t *testing.T) {
	t.Run("with redirect", func(t *testing.T) {
		env := &LocalEnv{
			Log: zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1))),
		}

		env.RedirectStdLog()
		// Should not panic
		env.RestoreStdLog()
	})

	t.Run("without redirect", func(t *testing.T) {
		env := &LocalEnv{
			Log: zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1))),
		}

		// Should not panic even without redirect
		env.RestoreStdLog()
	})

	t.Run("nil logger", func(t *testing.T) {
		env := &LocalEnv{
			Log: nil,
		}

		// Should not panic
		env.RestoreStdLog()
	})
}

func TestLocalEnv_Fields(t *testing.T) {
	cfg := &config.Config{
		Version: 1,
	}
	rpt := &config.Report{}
	log := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))

	env := &LocalEnv{
		Cfg:       cfg,
		Rpt:       rpt,
		Log:       log,
		NoDirs:    true,
		Overwrite: false,
		start:     time.Now(),
	}

	if env.Cfg != cfg {
		t.Error("Config not set correctly")
	}
	if env.Rpt != rpt {
		t.Error("Report not set correctly")
	}
	if env.Log != log {
		t.Error("Logger not set correctly")
	}
	if !env.NoDirs {
		t.Error("NoDirs not set correctly")
	}
	if env.Overwrite {
		t.Error("Overwrite not set correctly")
	}
}

func TestLocalEnv_RedirectAndRestore(t *testing.T) {
	env := &LocalEnv{
		Log: zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1))),
	}

	// Test multiple redirect/restore cycles
	for i := 0; i < 3; i++ {
		env.RedirectStdLog()
		if env.restoreStdLog == nil {
			t.Errorf("Iteration %d: restoreStdLog not set", i)
		}
		env.RestoreStdLog()
	}
}

func TestLocalEnv_UptimeAccuracy(t *testing.T) {
	env := &LocalEnv{
		start: time.Now(),
	}

	delays := []time.Duration{
		5 * time.Millisecond,
		10 * time.Millisecond,
		15 * time.Millisecond,
	}

	for _, delay := range delays {
		time.Sleep(delay)
		uptime := env.Uptime()
		if uptime < delay {
			t.Errorf("After %v delay, uptime %v is too small", delay, uptime)
		}
	}
}

func TestEnvKey(t *testing.T) {
	// Verify that envKey is a unique type
	var key envKey
	ctx := context.WithValue(context.Background(), key, &LocalEnv{start: time.Now()})

	val := ctx.Value(key)
	if val == nil {
		t.Error("Failed to retrieve value with envKey")
	}

	if _, ok := val.(*LocalEnv); !ok {
		t.Error("Retrieved value is not *LocalEnv")
	}
}

func TestLocalEnv_NilCodePage(t *testing.T) {
	env := &LocalEnv{
		CodePage: nil,
	}

	if env.CodePage != nil {
		t.Error("Expected CodePage to be nil")
	}
}

func TestLocalEnv_Integration(t *testing.T) {
	// Simulate a typical usage pattern
	ctx := ContextWithEnv(context.Background())
	env := EnvFromContext(ctx)

	// Set up environment
	env.Cfg = &config.Config{Version: 1}
	env.Log = zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	env.Rpt = &config.Report{}
	env.NoDirs = true
	env.Overwrite = false

	// Redirect logs
	env.RedirectStdLog()

	// Simulate some work
	time.Sleep(5 * time.Millisecond)

	// Check uptime
	if env.Uptime() < 5*time.Millisecond {
		t.Error("Uptime too small")
	}

	// Restore logs
	env.RestoreStdLog()

	// Verify all fields are accessible
	if env.Cfg == nil || env.Log == nil || env.Rpt == nil {
		t.Error("Environment not properly initialized")
	}
}
