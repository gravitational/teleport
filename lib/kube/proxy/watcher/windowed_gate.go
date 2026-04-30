package watcher

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

type WindowedGateConfig struct {
	Window             time.Duration
	StartupGracePeriod time.Duration
}

func (cfg *WindowedGateConfig) CheckAndSetDefaults() error {
	if cfg.Window <= 0 {
		return trace.BadParameter("Window cannot be <= 0")
	}
	return nil
}

// WindowedGate provides a time gated execution controller
type WindowedGate struct {
	WindowedGateConfig
	ctx context.Context

	mu      sync.Mutex
	current *call
	next    time.Time
}

type call struct {
	done chan struct{}
	// err stores the error of this result to propagate to callers.
	err error
}

func NewWindowedGate(ctx context.Context, cfg WindowedGateConfig) (*WindowedGate, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &WindowedGate{
		WindowedGateConfig: cfg,
		ctx:                ctx,
		next:               time.Now().Add(cfg.StartupGracePeriod),
	}, nil
}

func (g *WindowedGate) Do(ctx context.Context, fn func(context.Context) error) error {
	g.mu.Lock()

	if c := g.current; c != nil {
		done := c.done
		g.mu.Unlock()

		select {
		case <-g.ctx.Done(): // parent context
			return trace.Wrap(g.ctx.Err(), "parent context closed")
		case <-ctx.Done(): // caller context closure
			return trace.Wrap(ctx.Err(), "caller context closed")
		case <-done:
			return c.err
		}
	}

	if time.Now().Before(g.next) {
		g.mu.Unlock()
		return nil
	}

	c := &call{done: make(chan struct{})}
	g.current = c

	g.mu.Unlock()

	err := trace.Wrap(fn(ctx))

	g.mu.Lock()
	c.err = err
	g.current = nil
	// Advance window with jitter, this happens regardless of result. This is sufficent for current
	// use case in the watcher.
	g.next = time.Now().Add(retryutils.SeventhJitter(g.Window))
	g.mu.Unlock()

	close(c.done)
	return err
}
