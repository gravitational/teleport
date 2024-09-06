/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package hsm

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	totalReloads = 64
	concurrency  = 8
)

// TestReloads starts up an Auth and Proxy process and repeatedly reloads both
// of them, asserting that the reload is always successful in a reasonable
// amount of time. This is meant to be a simplified test that should be able to
// catch flaky Teleport reload bugs that have been caught by the HSM tests in
// the past.
func TestReloads(t *testing.T) {
	for i := 0; i < concurrency; i++ {
		t.Run(fmt.Sprintf("%d", i), testReloads)
	}
}

func testReloads(t *testing.T) {
	t.Parallel()
	log := utils.NewLoggerForTests()
	testCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	authConfig := newAuthConfig(t, log)
	auth := newTeleportService(t, authConfig, "auth")
	err := auth.start(testCtx)
	require.NoError(t, err, trace.DebugReport(err))
	t.Cleanup(func() { require.NoError(t, auth.close()) })

	proxyConfig := newProxyConfig(t, auth.authAddr(t), log)
	proxy := newTeleportService(t, proxyConfig, "proxy")
	require.NoError(t, proxy.start(testCtx))
	t.Cleanup(func() { require.NoError(t, proxy.close()) })

	for i := 0; i < totalReloads/concurrency; i++ {
		// Each reload event is broadcast in its own goroutine to try to make
		// the reloads as simultaneous as possible, or at least introduce some
		// randomness, to maximize the chance of catching errors.
		go func() {
			auth.process.BroadcastEvent(service.Event{Name: service.TeleportReloadEvent})
		}()
		go func() {
			proxy.process.BroadcastEvent(service.Event{Name: service.TeleportReloadEvent})
		}()

		require.NoError(t, withTimeout(testCtx, 30*time.Second, auth.waitForRestart), "attempt %d: waiting for auth restart", i+1)
		require.NoError(t, withTimeout(testCtx, 30*time.Second, proxy.waitForRestart), "attempt %d: waiting for proxy restart", i+1)
	}
}

func withTimeout(ctx context.Context, d time.Duration, f func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	return f(ctx)
}
