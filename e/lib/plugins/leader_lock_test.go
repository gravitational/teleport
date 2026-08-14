package plugins

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/plugins/factory"
)

func TestWithLeaderLock(t *testing.T) {
	clock := clockwork.NewFakeClock()
	var pluginStartCount int32
	var pluginExitCount int32
	started := make(chan struct{})
	exited := make(chan struct{})

	pluginFactory := func(ctx context.Context, plugin *types.PluginV1, deps factory.Dependencies) (factory.Delegate, error) {
		return func(instanceCtx context.Context) error {
			atomic.AddInt32(&pluginStartCount, 1)
			close(started)
			defer func() {
				atomic.AddInt32(&pluginExitCount, 1)
				close(exited)
			}()
			select {
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			case <-instanceCtx.Done():
				return trace.Wrap(instanceCtx.Err())
			}
		}, nil
	}

	process := testAuthProcess(t, withClock(clock))
	require.NoError(t, process.Start())

	pluginCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps := factory.Dependencies{
		ParentProcess: process,
		Logger:        slog.Default(),
	}
	call := withLeaderLock(pluginFactory)

	plugin := &types.PluginV1{Metadata: types.Metadata{Name: "test-plugin"}}
	runFunc, err := call(pluginCtx, plugin, deps)
	require.NoError(t, err)
	go func() {
		assert.NoError(t, runFunc(pluginCtx))
	}()

	select {
	case <-started:
	case <-time.After(30 * time.Second):
		t.Fatal("plugin never started")
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&pluginStartCount))
	require.Equal(t, int32(0), atomic.LoadInt32(&pluginExitCount))

	clock.Advance(time.Hour)

	// The plugin should still be running on the same instance — no restart.
	// pluginExitCount stays at zero implies the original instance is still
	// active, since any restart would have run the deferred increment.
	require.Equal(t, int32(1), atomic.LoadInt32(&pluginStartCount))
	require.Equal(t, int32(0), atomic.LoadInt32(&pluginExitCount))

	cancel()

	select {
	case <-exited:
	case <-time.After(30 * time.Second):
		t.Fatal("plugin never exited")
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&pluginExitCount))
}
