package plugins

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	storage "github.com/gravitational/teleport/integrations/access/common/auth/storage"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
)

type fakeAuthorizer struct{}

func (*fakeAuthorizer) Exchange(ctx context.Context, authorizationCode string, redirectURI string) (*storage.Credentials, error) {
	panic("unimplemented")
}

func (*fakeAuthorizer) Refresh(ctx context.Context, refreshToken string) (*storage.Credentials, error) {
	panic("unimplemented")
}

type fakeEvents struct {
	mu       sync.RWMutex
	watchers []*fakeWatcher
}

func (e *fakeEvents) NewWatcher(ctx context.Context, _ types.Watch) (types.Watcher, error) {
	watcher := &fakeWatcher{
		ch:     make(chan types.Event),
		doneCh: make(chan struct{}, 1),
		ctx:    ctx,
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.watchers = append(e.watchers, watcher)
	return watcher, nil
}

// close all existing watchers
func (e *fakeEvents) close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, w := range e.watchers {
		w.Close()
	}
	e.watchers = nil
}

func (e *fakeEvents) numWatchers() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.watchers)
}

func (e *fakeEvents) send(event types.Event) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, watcher := range e.watchers {
		watcher.send(event)
	}
}

type fakeWatcher struct {
	ch     chan types.Event
	doneCh chan struct{}
	ctx    context.Context
}

func (w *fakeWatcher) send(event types.Event) {
	w.ch <- event
}

func (w *fakeWatcher) Close() error {
	close(w.ch)
	close(w.doneCh)
	return nil
}

func (w *fakeWatcher) Done() <-chan struct{} {
	return w.doneCh
}

func (w *fakeWatcher) Error() error {
	select {
	case <-w.doneCh:
		return errors.New("watcher closed")
	default:
		return nil
	}
}

func (w *fakeWatcher) Events() <-chan types.Event {
	return w.ch
}

func TestPluginManagerStartStop(t *testing.T) {
	const pluginName = "slack-default"

	mem, err := memory.New(memory.Config{
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	authorizers := NewAuthorizerSet()
	authorizers.Add(types.PluginTypeSlack, &Authorizer{
		Authorizer: &fakeAuthorizer{},
		ClientID:   "123456",
	})
	backendService := local.NewPluginsService(mem)
	events := &fakeEvents{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var instanceStarted, instanceStopped int64
	makeInstanceDelegate := func(ctx context.Context) func() error {
		return func() error {
			atomic.AddInt64(&instanceStarted, 1)
			<-ctx.Done()
			atomic.AddInt64(&instanceStopped, 1)
			return nil
		}
	}
	assertStartStop := func(started, stopped int64) {
		require.Eventually(t, func() bool {
			return atomic.LoadInt64(&instanceStarted) == started &&
				atomic.LoadInt64(&instanceStopped) == stopped
		}, time.Second, time.Second/100)
	}

	cfg := ManagerConfig{
		Authorizers: authorizers,
		Backend:     backendService,
		Events:      events,
		Factories: map[types.PluginType]instanceFactory{
			types.PluginTypeSlack: func(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
				return makeInstanceDelegate(ctx), nil
			},
		},
		TeleportClient: &client.Client{}, // not actually used in test
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)

	go manager.Run(ctx)

	plugin := createSlackPlugin(t, pluginName).(*types.PluginV1)

	// Wait for manager to subscribe to events
	require.Eventually(t, func() bool {
		return events.numWatchers() == 1
	}, time.Second, time.Second/100)

	// 1) Create plugin: start
	events.send(types.Event{
		Type:     types.OpPut,
		Resource: plugin,
	})
	assertStartStop(1, 0)

	// 2) Modify metadata, but not spec: do not restart
	plugin = plugin.Clone().(*types.PluginV1)
	plugin.Metadata.Labels["foo"] = "bar"
	// No way to reliably assert this: will assert total start-stop count later
	events.send(types.Event{
		Type:     types.OpPut,
		Resource: plugin,
	})

	// 3) Modify spec: restart
	plugin = plugin.Clone().(*types.PluginV1)
	slackSpec := plugin.Spec.GetSlackAccessPlugin()
	require.NotNil(t, slackSpec)
	slackSpec.FallbackChannel = "#teleport-rules"

	events.send(types.Event{
		Type:     types.OpPut,
		Resource: plugin,
	})
	assertStartStop(2, 1)

	// 4) Close existing watcher: loop should stop all instances,
	// and then re-subscribe
	events.close()
	assertStartStop(2, 2)

	// Wait for manager to re-subscribe to events
	require.Eventually(t, func() bool {
		return events.numWatchers() == 1
	}, time.Second, time.Second/100)

	// Re-create plugin via an event.
	// We must do this because we do not mock the backend service itself
	events.send(types.Event{
		Type:     types.OpPut,
		Resource: plugin,
	})

	assertStartStop(3, 2)

	// 5) Delete: stop
	events.send(types.Event{
		Type: types.OpDelete,
		Resource: &types.ResourceHeader{
			Kind: types.KindPlugin,
			Metadata: types.Metadata{
				Name: "slack-default",
			},
		},
	})
	assertStartStop(3, 3)
}
