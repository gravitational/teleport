package plugins

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/jamf/testenv"
	"github.com/gravitational/teleport/e/lib/services"
	storage "github.com/gravitational/teleport/integrations/access/common/auth/storage"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

type (
	fakeAuthorizer  struct{}
	staticRefLookup map[string]map[string]string
)

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

func TestPluginManagerStartStopOAuth(t *testing.T) {
	modifySpec := func(t *testing.T, plugin *types.PluginV1) {
		slackSpec := plugin.Spec.GetSlackAccessPlugin()
		require.NotNil(t, slackSpec)
		slackSpec.FallbackChannel = "#teleport-rules"
	}
	plugin := createSlackPlugin(t, "slack-default").(*types.PluginV1)
	testPluginStartStop(t, plugin, modifySpec)
}

func TestPluginManagerStartStopStaticCreds(t *testing.T) {
	modifySpec := func(t *testing.T, plugin *types.PluginV1) {
		oktaSpec := plugin.Spec.GetOkta()
		require.NotNil(t, oktaSpec)
		oktaSpec.OrgUrl = "https://www.new-okta.com"
	}

	plugin, creds := createOktaPlugin(t, "okta")
	testPluginStartStop(t, plugin.(*types.PluginV1), modifySpec, creds)
}

func testPluginStartStop(t *testing.T, plugin *types.PluginV1, modifySpec func(t *testing.T, plugin *types.PluginV1), staticCreds ...types.PluginStaticCredentials) {
	mem, err := memory.New(memory.Config{
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	testLog := slog.With("test", t.Name())

	authorizers := NewAuthorizerSet()
	authorizers.Add(types.PluginTypeSlack, &Authorizer{
		Authorizer: &fakeAuthorizer{},
		ClientID:   "123456",
	})
	pluginService := local.NewPluginsService(mem)
	pluginStaticCredentialsService, err := local.NewPluginStaticCredentialsService(mem)
	require.NoError(t, err)
	events := &fakeEvents{}

	managerCtx, managerCancel := context.WithCancel(context.Background())
	defer managerCancel()

	// Add in any provided static credentials
	for _, staticCred := range staticCreds {
		require.NoError(t, pluginStaticCredentialsService.CreatePluginStaticCredentials(managerCtx, staticCred))
	}

	var instanceStarted, instanceStopped int64
	makeInstanceDelegate := func(deps instanceDependencies) func() error {
		return func() error {
			atomic.AddInt64(&instanceStarted, 1)
			<-deps.lifetime.Done()
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

	staticRefs := staticRefLookup{}
	cfg := ManagerConfig{
		Authorizers:             authorizers,
		Plugins:                 pluginService,
		PluginStaticCredentials: pluginStaticCredentialsService,
		Events:                  events,
		Factories: map[types.PluginType]instanceFactory{
			plugin.GetType(): func(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
				for _, cred := range deps.staticCredentials {
					staticRefs[cred.GetName()] = cred.GetStaticLabels()
				}
				return makeInstanceDelegate(deps), nil
			},
		},
		Logger: testLog,

		// the following are not used in the test
		TeleportClient: &auth.Server{},
		ParentProcess:  &service.TeleportProcess{},
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)

	go manager.Run(managerCtx)

	// Wait for manager to subscribe to events
	require.Eventually(t, func() bool {
		return events.numWatchers() == 1
	}, time.Second, time.Second/100)

	testLog.InfoContext(context.Background(), "Sending plugin start event")
	// 1) Create plugin: start
	events.send(types.Event{
		Type:     types.OpPut,
		Resource: plugin,
	})
	assertStartStop(1, 0)

	// Verify the static credentials
	for _, cred := range staticCreds {
		require.Equal(t, cred.GetStaticLabels(), staticRefs[cred.GetName()])
	}
	// Clear out the static refs
	staticRefs = staticRefLookup{}

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
	modifySpec(t, plugin)

	events.send(types.Event{
		Type:     types.OpPut,
		Resource: plugin,
	})
	assertStartStop(2, 1)

	// Verify the static credentials again
	for _, cred := range staticCreds {
		require.Equal(t, cred.GetStaticLabels(), staticRefs[cred.GetName()])
	}

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
				Name: plugin.GetName(),
			},
		},
	})
	assertStartStop(3, 3)
}

// TestInstanceFactory runs registered plugins instance factory to test start and stop events
func TestInstanceFactory(t *testing.T) {
	jamfEnv := testenv.NewUsingT(t, &testenv.Opts{
		DeviceTrustEnv: true,
	})
	defer jamfEnv.Close()

	testCases := []struct {
		name                     string
		pluginType               string
		plugin                   *types.PluginV1
		readyEvent, stoppedEvent string
	}{
		{
			name:       "oktaInstanceFactoryNoSync",
			pluginType: types.PluginTypeOkta,
			plugin: types.NewPluginV1(
				types.Metadata{
					Name: "okta",
				},
				types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Okta{
						Okta: &types.PluginOktaSettings{
							OrgUrl:       "https://test.url",
							SyncSettings: &types.PluginOktaSyncSettings{},
						},
					},
				},
				&types.PluginCredentialsV1{
					Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
						StaticCredentialsRef: &types.PluginStaticCredentialsRef{
							Labels: map[string]string{
								"label1": "value1",
							},
						},
					},
				},
			),
			readyEvent:   services.EventWithComponents(services.OktaReady, "okta", fmt.Sprintf("%d", clockwork.NewFakeClock().Now().Unix())),
			stoppedEvent: services.EventWithComponents(services.OktaStopped, "okta", fmt.Sprintf("%d", clockwork.NewFakeClock().Now().Unix())),
		},
		{
			name:       "oktaInstanceFactoryWithSync",
			pluginType: types.PluginTypeOkta,
			plugin: types.NewPluginV1(
				types.Metadata{
					Name: "okta",
				},
				types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Okta{
						Okta: &types.PluginOktaSettings{
							OrgUrl: "https://test.url",
							SyncSettings: &types.PluginOktaSyncSettings{
								SyncUsers: true,
							},
						},
					},
				},
				&types.PluginCredentialsV1{
					Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
						StaticCredentialsRef: &types.PluginStaticCredentialsRef{
							Labels: map[string]string{
								"label1": "value1",
							},
						},
					},
				},
			),
			readyEvent:   services.EventWithComponents(services.OktaReady, "okta", fmt.Sprintf("%d", clockwork.NewFakeClock().Now().Unix())),
			stoppedEvent: services.EventWithComponents(services.OktaStopped, "okta", fmt.Sprintf("%d", clockwork.NewFakeClock().Now().Unix())),
		},
		{
			name:       "jamfInstanceFactory",
			pluginType: types.PluginTypeJamf,
			plugin: types.NewPluginV1(
				types.Metadata{
					Name: "jamf",
				},
				types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Jamf{
						Jamf: &types.PluginJamfSettings{
							JamfSpec: &types.JamfSpecV1{
								ApiEndpoint: jamfEnv.APIEndpoint,
							},
						},
					},
				},
				&types.PluginCredentialsV1{
					Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
						StaticCredentialsRef: &types.PluginStaticCredentialsRef{
							Labels: map[string]string{
								"jamf/api-endpoint": jamfEnv.APIEndpoint,
							},
						},
					},
				},
			),
			readyEvent:   services.JamfReadyEvent,
			stoppedEvent: services.JamfStoppedEvent,
		},
	}

	// GIVEN a running Teleport Cluster...
	process := testAuthProcess(t)
	require.NoError(t, process.Start())

	for _, tc := range testCases {
		factoryCtx, factoryCancel := context.WithCancel(context.Background())
		pluginLifetime, pluginCancel := context.WithCancel(context.Background())
		t.Run(tc.name, func(t *testing.T) {
			var factoryFunc func() error

			switch tc.pluginType {
			case types.PluginTypeOkta:
				var err error
				// Run plugin
				factoryFunc, err = oktaInstanceFactory(factoryCtx, tc.plugin, instanceDependencies{
					lifetime:      pluginLifetime,
					logger:        slog.Default(),
					parentProcess: process,
					staticCredentials: []types.PluginStaticCredentials{
						&types.PluginStaticCredentialsV1{
							ResourceHeader: types.ResourceHeader{
								Metadata: types.Metadata{
									Name: "cred",
								},
							},
							Spec: &types.PluginStaticCredentialsSpecV1{
								Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
									APIToken: "test",
								},
							},
						},
					},
				})
				require.NoError(t, err)
			case types.PluginTypeJamf:
				var err error
				// Run plugin
				factoryFunc, err = jamfInstanceFactory(factoryCtx, tc.plugin, instanceDependencies{
					lifetime:      pluginLifetime,
					logger:        slog.Default(),
					HTTPClient:    jamfEnv.HTTPClient,
					parentProcess: process,
					staticCredentials: []types.PluginStaticCredentials{
						&types.PluginStaticCredentialsV1{
							ResourceHeader: types.ResourceHeader{
								Metadata: types.Metadata{
									Name: "cred",
								},
							},
							Spec: &types.PluginStaticCredentialsSpecV1{
								Credentials: &types.PluginStaticCredentialsSpecV1_BasicAuth{
									BasicAuth: &types.PluginStaticCredentialsBasicAuth{
										Username: testenv.DefaultUsers[0].Username,
										Password: testenv.DefaultUsers[0].Password,
									},
								},
							},
						},
					},
				})
				require.NoError(t, err)
			default:
				t.Fatalf("Unknown plugin type: %q", tc.pluginType)
			}

			// make sure that anything holding a reference to the wrong context is
			// terminated with extreme prejudice
			factoryCancel()

			factoryErr := make(chan error, 1)
			go func() {
				factoryErr <- factoryFunc()
			}()

			// EXPECT that the plugin process emits a `ready` event
			_, err := process.WaitForEventTimeout(5*time.Second, tc.readyEvent)
			require.NoError(t, err)

			// terminate plugin
			pluginCancel()

			// EXPECT that the plugin process emits a `close` event and eventually
			// terminates
			_, err = process.WaitForEventTimeout(5*time.Second, tc.stoppedEvent)
			require.NoError(t, err)

			select {
			case err := <-factoryErr:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for start error")
			}
		})
	}
}

func testAuthProcess(t *testing.T) *service.TeleportProcess {
	cfg := servicecfg.MakeDefaultConfig()
	cfg.Clock = clockwork.NewFakeClock()
	cfg.DataDir = t.TempDir()
	cfg.DiagnosticAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"}
	cfg.SetAuthServerAddress(utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"})
	cfg.Auth.Enabled = true
	cfg.Auth.StorageConfig.Params["path"] = t.TempDir()
	cfg.Auth.ListenAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"}
	cfg.Proxy.DisableWebInterface = true
	cfg.Proxy.WebAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"}
	cfg.SSH.Enabled = false
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	process, err := service.NewTeleport(cfg)
	require.NoError(t, err)
	return process
}
