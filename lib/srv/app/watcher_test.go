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
	"maps"
	"testing"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

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

	// Create a single Proxy with a PublicAddr to exercise that
	// apps without a PublicAddr automatically get one specified
	// by the watcher.
	require.NoError(t, s.authServer.AuthServer.UpsertProxy(t.Context(), &types.ServerV2{
		Kind:    types.KindProxy,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "FakeProxy",
		},
		Spec: types.ServerSpecV2{
			PublicAddrs: []string{"test.example.com"},
		},
	}))

	// Only app0 should be registered initially.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		apps, err := s.authServer.AuthServer.GetApplicationServers(ctx, defaults.Namespace)
		require.NoError(t, err)
		require.Len(t, apps, 1)
		require.Empty(t, gocmp.Diff(app0, apps[0].GetApp()))
	}, 20*time.Second, 100*time.Millisecond)

	// Create a watcher to observe events from the backend in response
	// to the resource monitor creating application servers.
	watcher, err := s.authServer.AuthServer.NewWatcher(ctx, types.Watch{
		Name:            "lib/srv/app.TestWatcher",
		MetricComponent: "lib/srv/app.TestWatcher",
		Kinds:           []types.WatchKind{{Kind: types.KindAppServer}},
	})
	require.NoError(t, err)

	select {
	case event := <-watcher.Events():
		require.Equal(t, types.OpInit, event.Type)
	case <-time.After(20 * time.Second):
		t.Fatal("Timed out waiting for OpInit event")
	}

	// Create an app with label group=a.
	app1, err := makeDynamicApp("app1", map[string]string{"group": "a"})
	require.NoError(t, err)
	err = s.authServer.AuthServer.CreateApp(ctx, app1)
	require.NoError(t, err)

	// Set the PublicAddr _after_ creating the app. The watched apps will
	// automatically have the address set if empty. In order for the comparisons
	// below to pass this needs to be set on app1.
	app1.SetPublicAddr("app1.test.example.com")

	// The app should match and be registered.
	select {
	case event := <-watcher.Events():
		require.Equal(t, types.OpPut, event.Type)
		require.Empty(t, gocmp.Diff(app1, event.Resource.(types.AppServer).GetApp(), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	case <-time.After(20 * time.Second):
		t.Fatal("Timed out waiting for app1 OpPut event")
	}

	// Updating app0 should not override the statically registered app.
	app0Updated, err := makeDynamicApp("app0", map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	require.NoError(t, err)
	err = s.authServer.AuthServer.CreateApp(ctx, app0Updated)
	require.NoError(t, err)

	// Create a second app with label group=b, which should not match.
	app2, err := makeDynamicApp("app2", map[string]string{"group": "b"})
	require.NoError(t, err)

	// Set the PublicAddr _before_ creating the app. The watcher should
	// not modify apps with an already specified PublicAddr.
	app2.SetPublicAddr("app2.some.other.addr.example.com")

	err = s.authServer.AuthServer.CreateApp(ctx, app2)
	require.NoError(t, err)

	// Update app2 labels so it matches and gets registered.
	app2.SetStaticLabels(map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	err = s.authServer.AuthServer.UpdateApp(ctx, app2)
	require.NoError(t, err)

	select {
	case event := <-watcher.Events():
		require.Equal(t, types.OpPut, event.Type)
		require.Empty(t, gocmp.Diff(app2, event.Resource.(types.AppServer).GetApp(), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	case <-time.After(20 * time.Second):
		t.Fatal("Timed out waiting for app2 OpPut event")
	}

	// Update app2 URI so it gets re-registered.
	app2.SetURI("localhost:2345")
	err = s.authServer.AuthServer.UpdateApp(ctx, app2)
	require.NoError(t, err)

	// app2 should get deleted and then recreated.
	select {
	case event := <-watcher.Events():
		require.Equal(t, types.OpDelete, event.Type)
		require.Equal(t, app2.GetName(), event.Resource.GetName())
	case <-time.After(20 * time.Second):
		t.Fatal("Timed out waiting for app2 OpDelete event")
	}

	select {
	case event := <-watcher.Events():
		require.Equal(t, types.OpPut, event.Type)
		require.Empty(t, gocmp.Diff(app2, event.Resource.(types.AppServer).GetApp(), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	case <-time.After(20 * time.Second):
		t.Fatal("Timed out waiting for app2 OpPut event")
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		apps, err := s.authServer.AuthServer.GetApplicationServers(ctx, defaults.Namespace)
		require.NoError(t, err)
		require.Len(t, apps, 3)
		require.Empty(t, gocmp.Diff(app0, apps[0].GetApp()))
		require.Empty(t, gocmp.Diff(app1, apps[1].GetApp(), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
		require.Empty(t, gocmp.Diff(app2, apps[2].GetApp(), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	}, 10*time.Second, 100*time.Millisecond)

	// Update app1 labels so it doesn't match.
	app1.SetStaticLabels(map[string]string{"group": "c", types.OriginLabel: types.OriginDynamic})
	err = s.authServer.AuthServer.UpdateApp(ctx, app1)
	require.NoError(t, err)

	// Only app0 and app2 should remain registered.
	select {
	case event := <-watcher.Events():
		require.Equal(t, types.OpDelete, event.Type)
		require.Empty(t, gocmp.Diff(app1.GetName(), event.Resource.GetName()))
	case <-time.After(20 * time.Second):
		t.Fatal("Timed out waiting for app1 OpDelete event")
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		apps, err := s.authServer.AuthServer.GetApplicationServers(ctx, defaults.Namespace)
		require.NoError(t, err)
		require.Len(t, apps, 2)
		require.Empty(t, gocmp.Diff(app0, apps[0].GetApp()))
		require.Empty(t, gocmp.Diff(app2, apps[1].GetApp(), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	}, 20*time.Second, 100*time.Millisecond)

	// Remove app2.
	err = s.authServer.AuthServer.DeleteApp(ctx, app2.GetName())
	require.NoError(t, err)

	select {
	case event := <-watcher.Events():
		require.Equal(t, types.OpDelete, event.Type)
		require.Empty(t, gocmp.Diff(app2.GetName(), event.Resource.GetName(), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	case <-time.After(20 * time.Second):
		t.Fatal("Timed out waiting for app2 OpDelete event")
	}

	// Only the static app should remain.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		apps, err := s.authServer.AuthServer.GetApplicationServers(ctx, defaults.Namespace)
		require.NoError(t, err)
		require.Len(t, apps, 1)
		require.Empty(t, gocmp.Diff(app0, apps[0].GetApp()))
	}, 20*time.Second, 100*time.Millisecond)
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
