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

package database

import (
	"context"
	"errors"
	"net"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestTunnelService_Run_CancellationDuringRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pinger := &fakePinger{err: errors.New("proxy unreachable")}
		registry := readyz.NewRegistry()
		reporter := registry.AddService("application-tunnel", "test")
		readyCh := make(chan struct{})
		close(readyCh)

		listener, err := net.Listen("tcp", "127.0.0.1:")
		require.NoError(t, err)

		svc := &TunnelService{
			cfg:                &TunnelConfig{Name: "test", Listener: listener},
			proxyPinger:        pinger,
			botIdentityReadyCh: readyCh,
			statusReporter:     reporter,
			log:                logtest.NewLogger(),
		}

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		synctest.Wait()

		// Verify that the reporter wiring is correct and it's reporting Unhealthy
		status, ok := registry.ServiceStatus("test")
		require.True(t, ok)
		require.Equal(t, readyz.Unhealthy, status.Status)
		require.Contains(t, status.Reason, "proxy unreachable")

		// Verify that Run has not returned, therefore it's retrying.
		select {
		case <-done:
			require.Fail(t, "Run returned instead of retrying")
		default:
		}

		cancel()
		synctest.Wait()

		// Verify that context cancellation is not propagating an error
		// from the retry wrapper.
		require.NoError(t, <-done)
	})
}

type fakePinger struct {
	err error
}

func (p fakePinger) Ping(_ context.Context) (*connection.ProxyPong, error) {
	return nil, p.err
}
