package srv

import (
	"context"

	"github.com/gravitational/trace"

	rsession "github.com/gravitational/teleport/lib/session"
)

// SetWinSize sets the window size of the terminal.
func (t *terminal) SetWinSize(ctx context.Context, params rsession.TerminalParams) error {
	return trace.NotImplemented("set win size not implemented for windows")
}
