/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package installer

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client/debug"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// defaultReadyzCheckTimeout is the timeout for a single readyz query.
	defaultReadyzCheckTimeout = 5 * time.Second
)

// readyzChecker queries the Teleport debug socket's readyz endpoint and classifies errors
// for poll-based callers.
type readyzChecker struct {
	// logger is used for diagnostic logging during readyz checks.
	logger *slog.Logger

	// dataDir is the Teleport data directory that contains the debug socket.
	dataDir string

	// timeout is the per-attempt timeout for a single readyz query.
	// Defaults to defaultReadyzCheckTimeout (5s) when zero.
	timeout time.Duration

	// checkOverride, when set, replaces the default debug-socket readyz
	// call. Used for testing.
	checkOverride func(ctx context.Context) (debug.Readiness, error)
}

// check queries readyz once with a per-attempt timeout. Returns (true, nil) when
// the agent is ready, (false, nil) when not ready or on transient errors
// the caller should retry, and (false, err) only for context cancellation.
func (r *readyzChecker) check(ctx context.Context) (ready bool, err error) {
	timeout := r.timeout
	if timeout == 0 {
		timeout = defaultReadyzCheckTimeout
	}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var readiness debug.Readiness
	if r.checkOverride != nil {
		readiness, err = r.checkOverride(checkCtx)
	} else {
		clt := debug.NewClient(r.dataDir)
		readiness, err = clt.GetReadiness(checkCtx)
	}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return false, trace.Wrap(ctxErr)
		}

		// Socket genuinely absent (connection error) or endpoint doesn't exist, so keep polling.
		if utils.IsConnectionError(err) || trace.IsNotFound(err) {
			r.logger.DebugContext(checkCtx, "Debug socket unavailable", "error", trace.UserMessage(err))
			return false, nil
		}

		// Per-attempt timeout is expected when Teleport is overloaded; keep polling.
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.DebugContext(checkCtx, "Readyz check timed out", "timeout", timeout)
			return false, nil
		}

		r.logger.WarnContext(checkCtx, "Readyz check returned unexpected error", "error", trace.UserMessage(err))
		return false, nil
	}

	if !readiness.Ready {
		r.logger.InfoContext(checkCtx, "Teleport agent is not ready yet", "status", readiness.Status)
		return false, nil
	}

	r.logger.InfoContext(checkCtx, "Teleport agent is ready and has joined the cluster")
	return true, nil
}
