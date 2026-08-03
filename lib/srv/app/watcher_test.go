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

package app

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"testing"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestCloudHostedAppServiceRejectsDynamicLabels(t *testing.T) {
	t.Parallel()

	s := SetUpSuiteWithConfig(t, suiteConfig{
		ResourceMatchers: []services.ResourceMatcher{
			{Labels: types.Labels{"group": []string{"a"}}},
		},
		IgnoreAppsWithCommandLabels: true,
	})

	w := newBackendAppWatcher(t.Context(), t, s)

	// Create app with label group=a and dynamic labels.
	app, err := makeDynamicApp("app-with-dynamic-labels", map[string]string{"group": "a"})
	require.NoError(t, err)
	app.SetDynamicLabels(map[string]types.CommandLabel{
		"foo": &types.CommandLabelV2{
			Period:  types.Duration(5 * time.Second),
			Command: []string{"echo", "bar"},
		},
	})
	err = s.authServer.AuthServer.CreateApp(t.Context(), app)
	require.NoError(t, err)

	// It should not have been registered.
	w.awaitAppPresence(app.GetName(), false)

	// Remove the dynamic labels
	app.SetDynamicLabels(make(map[string]types.CommandLabel))
	err = s.authServer.AuthServer.UpdateApp(t.Context(), app)
	require.NoError(t, err)

	// It should now be registered.
	w.awaitAppPresence(app.GetName(), true)
}

// TestWatcher verifies that app agent properly detects and applies
// changes to application resources.
func TestWatcher(t *testing.T) {
	ctx := t.Context()

	// Make a static configuration app.
	app0, err := makeStaticApp("app0", nil)
	require.NoError(t, err)

	// Setup app server that proxies one static app and
	// watches for apps with label group=a.
	s := SetUpSuiteWithConfig(t, suiteConfig{
		Apps: types.Apps{app0},
		ResourceMatchers: []services.ResourceMatcher{
			{Labels: types.Labels{
				"group": []string{"a"},
			}},
		},
	})

	// Observe what the agent publishes to the cluster.
	w := newBackendAppWatcher(ctx, t, s)

	// Create a single Proxy with a PublicAddr to exercise that
	// apps without a PublicAddr automatically get one specified
	// by the watcher.
	_, err = s.authServer.AuthServer.UpsertProxyServer(t.Context(), &types.ServerV2{
		Kind:    types.KindProxy,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "FakeProxy",
		},
		Spec: types.ServerSpecV2{
			PublicAddrs: []string{"test.example.com"},
		},
	})
	require.NoError(t, err)

	// Only app0 should be registered initially.
	w.awaitApps(types.Apps{app0})

	// Create app with label group=a.
	app1, err := makeDynamicApp("app1", map[string]string{"group": "a"})
	require.NoError(t, err)
	err = s.authServer.AuthServer.CreateApp(ctx, app1)
	require.NoError(t, err)

	// Set the PublicAddr _after_ creating the app. The watched apps will
	// automatically have the address set if empty. In order for the comparisons
	// below to pass this needs to be set on app1.
	app1.SetPublicAddr("app1.test.example.com")

	// It should be registered.
	w.awaitApps(types.Apps{app0, app1})

	// Try to update app0 which is registered statically.
	app0Updated, err := makeDynamicApp("app0", map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	require.NoError(t, err)
	err = s.authServer.AuthServer.CreateApp(ctx, app0Updated)
	require.NoError(t, err)

	// It should not be registered, old app0 should remain.
	w.awaitApps(types.Apps{app0, app1})

	// Create app with label group=b.
	app2, err := makeDynamicApp("app2", map[string]string{"group": "b"})
	require.NoError(t, err)

	// Set the PublicAddr _before_ creating the app. The watcher should
	// not modify apps with an already specified PublicAddr.
	app2.SetPublicAddr("app2.some.other.addr.example.com")

	err = s.authServer.AuthServer.CreateApp(ctx, app2)
	require.NoError(t, err)

	// It shouldn't be registered.
	w.awaitApps(types.Apps{app0, app1})

	// Update app2 labels so it matches.
	app2.SetStaticLabels(map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	err = s.authServer.AuthServer.UpdateApp(ctx, app2)
	require.NoError(t, err)

	// Both should be registered now.
	w.awaitApps(types.Apps{app0, app1, app2})

	// Update app2 URI so it gets re-registered.
	app2.SetURI("localhost:2345")
	err = s.authServer.AuthServer.UpdateApp(ctx, app2)
	require.NoError(t, err)

	// app2 should get updated.
	w.awaitApps(types.Apps{app0, app1, app2})

	// Update app1 labels so it doesn't match.
	app1.SetStaticLabels(map[string]string{"group": "c", types.OriginLabel: types.OriginDynamic})
	err = s.authServer.AuthServer.UpdateApp(ctx, app1)
	require.NoError(t, err)

	// Only app0 and app2 should remain registered.
	w.awaitApps(types.Apps{app0, app2})

	// Remove app2.
	err = s.authServer.AuthServer.DeleteApp(ctx, app2.GetName())
	require.NoError(t, err)

	// Only static app should remain.
	w.awaitApps(types.Apps{app0})
}

// awaitAppPresence consumes AppServer events until the named app's presence
// in the published set matches want, then briefly drains to catch a change
// that should not have happened.
func (w *backendAppWatcher) awaitAppPresence(name string, want bool) {
	w.t.Helper()

	present := func() bool {
		_, ok := w.state[name]
		return ok
	}

	deadline := time.After(10 * time.Second)
	for present() != want {
		select {
		case event := <-w.watcher.Events():
			w.apply(event)
		case <-w.watcher.Done():
			w.t.Fatalf("watcher closed: %v", w.watcher.Error())
		case <-deadline:
			w.t.Fatalf("timed out waiting for app %q presence to become %v", name, want)
		}
	}

	settle := time.After(500 * time.Millisecond)
	for {
		select {
		case event := <-w.watcher.Events():
			w.apply(event)
			if present() != want {
				w.t.Fatalf("app %q presence diverged from %v after converging", name, want)
			}
		case <-w.watcher.Done():
			w.t.Fatalf("watcher closed: %v", w.watcher.Error())
		case <-settle:
			return
		}
	}
}

// backendAppWatcher observes the applications the agent publishes to the
// cluster: registrations arrive as AppServer heartbeats (OpPut) and
// unregistrations as OpDelete. Tests assert against this externally visible
// contract instead of internal reconciler state.
type backendAppWatcher struct {
	t       *testing.T
	watcher types.Watcher
	state   map[string]types.Application
}

func newBackendAppWatcher(ctx context.Context, t *testing.T, s *Suite) *backendAppWatcher {
	t.Helper()

	watcher, err := s.authServer.AuthServer.NewWatcher(ctx, types.Watch{
		Name:  "lib/srv/app.watcher_test",
		Kinds: []types.WatchKind{{Kind: types.KindAppServer}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, watcher.Close()) })

	select {
	case event := <-watcher.Events():
		require.Equal(t, types.OpInit, event.Type)
	case <-watcher.Done():
		t.Fatalf("watcher closed: %v", watcher.Error())
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for OpInit")
	}

	// Seed from a list so that heartbeats which landed before the watcher
	// was established are observed too.
	w := &backendAppWatcher{t: t, watcher: watcher, state: make(map[string]types.Application)}
	servers, err := s.authServer.AuthServer.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	for _, server := range servers {
		w.state[server.GetApp().GetName()] = server.GetApp()
	}
	return w
}

func (w *backendAppWatcher) apply(event types.Event) {
	switch event.Type {
	case types.OpPut:
		server, ok := event.Resource.(types.AppServer)
		require.True(w.t, ok, "unexpected resource type %T", event.Resource)
		w.state[server.GetApp().GetName()] = server.GetApp()
	case types.OpDelete:
		delete(w.state, event.Resource.GetName())
	}
}

// awaitApps consumes AppServer events until the set of published apps matches
// want, then briefly continues draining to catch registrations that should
// not have happened (the "no change" assertions).
func (w *backendAppWatcher) awaitApps(want types.Apps) {
	w.t.Helper()

	compare := func() string {
		published := make(types.Apps, 0, len(w.state))
		for _, app := range w.state {
			published = append(published, app)
		}
		slices.SortFunc(published, func(a, b types.Application) int {
			return cmp.Compare(a.GetName(), b.GetName())
		})
		return gocmp.Diff(want, published,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		)
	}

	deadline := time.After(10 * time.Second)
	for compare() != "" {
		select {
		case event := <-w.watcher.Events():
			w.apply(event)
		case <-w.watcher.Done():
			w.t.Fatalf("watcher closed: %v", w.watcher.Error())
		case <-deadline:
			w.t.Fatalf("timed out waiting for published apps to converge: %v", compare())
		}
	}

	// Settle: surface any event that contradicts the expected state, e.g. a
	// registration that must not have happened.
	settle := time.After(500 * time.Millisecond)
	for {
		select {
		case event := <-w.watcher.Events():
			w.apply(event)
			if diff := compare(); diff != "" {
				w.t.Fatalf("published apps diverged after converging: %v", diff)
			}
		case <-w.watcher.Done():
			w.t.Fatalf("watcher closed: %v", w.watcher.Error())
		case <-settle:
			return
		}
	}
}

func makeStaticApp(name string, labels map[string]string) (*types.AppV3, error) {
	return makeApp(name, labels, map[string]string{
		types.OriginLabel: types.OriginConfigFile,
	})
}

func makeDynamicApp(name string, labels map[string]string) (*types.AppV3, error) {
	return makeApp(name, labels, map[string]string{
		types.OriginLabel: types.OriginDynamic,
	})
}

func makeApp(name string, labels map[string]string, additionalLabels map[string]string) (*types.AppV3, error) {
	if labels == nil {
		labels = make(map[string]string)
	}
	maps.Copy(labels, additionalLabels)
	return types.NewAppV3(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.AppSpecV3{
		URI: "localhost",
	})
}
