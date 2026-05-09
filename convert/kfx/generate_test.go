package kfx

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"fbc/config"
	"fbc/content"
	"fbc/fb2"
)

type cancelAfterNErrContext struct {
	context.Context
	remainingNilErrs int
}

func (c *cancelAfterNErrContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *cancelAfterNErrContext) Done() <-chan struct{}       { return nil }
func (c *cancelAfterNErrContext) Value(key any) any           { return c.Context.Value(key) }

func (c *cancelAfterNErrContext) Err() error {
	if c.remainingNilErrs > 0 {
		c.remainingNilErrs--
		return nil
	}
	return context.Canceled
}

func TestBuildFragmentsObservesCancellationAfterStart(t *testing.T) {
	ctx := &cancelAfterNErrContext{
		Context:          context.Background(),
		remainingNilErrs: 1,
	}
	c := &content.Content{Book: &fb2.FictionBook{}}

	err := buildFragments(ctx, NewContainer(), c, &config.DocumentConfig{}, zaptest.NewLogger(t))
	if err != context.Canceled {
		t.Fatalf("buildFragments() error = %v, want %v", err, context.Canceled)
	}
}
