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
)

func TestWithLeaderLock(t *testing.T) {
	clock := clockwork.NewFakeClock()
	var pluginStartCount int32
	var pluginExitCount int32

	pluginHandler := func(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
		return func() error {
			atomic.AddInt32(&pluginStartCount, 1)
			defer func() {
				atomic.AddInt32(&pluginExitCount, 1)
			}()
			select {
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			case <-deps.lifetime.Done():
				return trace.Wrap(deps.lifetime.Err())
			}
		}, nil
	}

	process := testAuthProcess(t, withClock(clock))
	require.NoError(t, process.Start())
	t.Cleanup(func() {
		require.NoError(t, process.Close())
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps := instanceDependencies{
		lifetime:      ctx,
		parentProcess: process,
		logger:        slog.Default(),
	}
	call := withLeaderLock(pluginHandler)

	plugin := &types.PluginV1{Metadata: types.Metadata{Name: "test-plugin"}}
	runFunc, err := call(deps.lifetime, plugin, deps)
	require.NoError(t, err)
	go func() {
		assert.NoError(t, runFunc())
	}()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&pluginStartCount) == 1
	}, time.Second, time.Millisecond*50)
	require.Equal(t, int32(0), atomic.LoadInt32(&pluginExitCount))

	clock.Advance(time.Hour)

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&pluginStartCount) == 1
	}, time.Second, time.Millisecond*50)
	require.Equal(t, int32(0), atomic.LoadInt32(&pluginExitCount))

	cancel()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&pluginExitCount) == 1
	}, time.Second, time.Millisecond*50)
}
