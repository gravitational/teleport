//go:build !windows
// +build !windows

package srv

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/moby/term"

	rsession "github.com/gravitational/teleport/lib/session"
)

// SetWinSize sets the window size of the terminal.
func (t *terminal) SetWinSize(ctx context.Context, params rsession.TerminalParams) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pty == nil {
		return trace.NotFound("no pty")
	}

	if err := term.SetWinsize(t.pty.Fd(), params.Winsize()); err != nil {
		return trace.Wrap(err)
	}
	t.params = params
	return nil
}
