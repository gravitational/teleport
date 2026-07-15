package database

import (
	"context"
	"errors"
	"net"
	"testing"
	"testing/synctest"

	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/stretchr/testify/require"
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
