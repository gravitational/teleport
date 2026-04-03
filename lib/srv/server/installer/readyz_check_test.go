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
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/client/debug"
)

func TestCheckReturnsContextCancellation(t *testing.T) {
	t.Parallel()

	checker := &readyzChecker{
		logger:  slog.Default(),
		dataDir: t.TempDir(),
		checkOverride: func(ctx context.Context) (debug.Readiness, error) {
			<-ctx.Done()
			return debug.Readiness{}, &net.OpError{Op: "dial", Net: "unix", Err: ctx.Err()}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ready, err := checker.check(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, ready)
}

func TestCheckTimeoutReturnsNotReady(t *testing.T) {
	t.Parallel()

	checker := &readyzChecker{
		logger:  slog.Default(),
		dataDir: t.TempDir(),
		timeout: 10 * time.Millisecond,
		checkOverride: func(ctx context.Context) (debug.Readiness, error) {
			<-ctx.Done()
			return debug.Readiness{}, ctx.Err()
		},
	}

	ready, err := checker.check(context.Background())
	require.NoError(t, err)
	require.False(t, ready)
}

func TestCheckRetriesOnConnectionError(t *testing.T) {
	t.Parallel()

	checker := &readyzChecker{
		logger:  slog.Default(),
		dataDir: t.TempDir(),
		checkOverride: func(_ context.Context) (debug.Readiness, error) {
			return debug.Readiness{}, &net.OpError{Op: "dial", Net: "unix", Err: os.ErrNotExist}
		},
	}

	ready, err := checker.check(context.Background())
	require.NoError(t, err)
	require.False(t, ready)
}

func TestCheckReturnsReady(t *testing.T) {
	t.Parallel()

	checker := &readyzChecker{
		logger:  slog.Default(),
		dataDir: t.TempDir(),
		checkOverride: func(_ context.Context) (debug.Readiness, error) {
			return debug.Readiness{Ready: true, Status: "ok"}, nil
		},
	}

	ready, err := checker.check(context.Background())
	require.NoError(t, err)
	require.True(t, ready)
}
