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
	"errors"
	"maps"
	"slices"
	"testing"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/services"
)

func TestCloudHostedAppServiceRejectsDynamicLabels(t *testing.T) {
	t.Parallel()

	reconcileCh := make(chan types.Apps)

	s := SetUpSuiteWithConfig(t, suiteConfig{
		ResourceMatchers: []services.ResourceMatcher{
			{Labels: types.Labels{"group": []string{"a"}}},
		},
		OnReconcile: func(a types.Apps) {
			reconcileCh <- a
		},
	})

	s.appServer.c.IgnoreAppsWithCommandLabels = true

	// Drain the initial event that fires when the watcher starts up
	// (before we create any dynamic apps).
	select {
	case <-reconcileCh:
	case <-time.After(time.Second):
		t.Fatal("Didn't receive initial reconcile event after 1s.")
	}

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
	select {
	case apps := <-reconcileCh:
		require.Nil(t, apps.Find(app.GetName()), "app with dynamic labels should not have been registered")
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Remove the dynamic labels
	app.SetDynamicLabels(make(map[string]types.CommandLabel))
	err = s.authServer.AuthServer.UpdateApp(t.Context(), app)
	require.NoError(t, err)

	// It should now be registered.
	select {
	case apps := <-reconcileCh:
		require.NotNil(t, apps.Find(app.GetName()))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}
}

func TestWatcherDeleteLastDynamicAppAfterHeartbeatFailure(t *testing.T) {
	ctx := t.Context()

	reconcileCh := make(chan types.Apps, 8)
	heartbeatCh := make(chan error, 8)
	s := SetUpSuiteWithConfig(t, suiteConfig{
		DisableDefaultApps: true,
		InventoryHandle:    newFailingInventoryHandle(),
		ResourceMatchers: []services.ResourceMatcher{
			{Labels: types.Labels{
				"group": []string{"a"},
			}},
		},
		OnReconcile: func(a types.Apps) {
			reconcileCh <- a
		},
		OnHeartbeat: func(err error) {
			heartbeatCh <- err
		},
	})

	requireReconciledAppNames(t, reconcileCh)
	requireHeartbeatResult(t, heartbeatCh, false)

	app, err := makeDynamicApp("app1", map[string]string{"group": "a"})
	require.NoError(t, err)
	require.NoError(t, s.authServer.AuthServer.CreateApp(ctx, app))
	requireReconciledAppNames(t, reconcileCh, app.GetName())
	requireHeartbeatResult(t, heartbeatCh, true)
	drainHeartbeatResults(heartbeatCh)

	require.NoError(t, s.authServer.AuthServer.DeleteApp(ctx, app.GetName()))
	requireReconciledAppNames(t, reconcileCh)
	require.Eventually(t, func() bool {
		appServers, err := s.authServer.AuthServer.GetApplicationServers(ctx, defaults.Namespace)
		return err == nil && len(appServers) == 0
	}, 10*time.Second, 100*time.Millisecond, "waiting for dynamic app heartbeat record to be absent")
	requireHeartbeatResult(t, heartbeatCh, false)
}

// TestWatcher verifies that app agent properly detects and applies
// changes to application resources.
func TestWatcher(t *testing.T) {
	ctx := t.Context()

	// Make a static configuration app.
	app0, err := makeStaticApp("app0", nil)
	require.NoError(t, err)

	// This channel will receive new set of apps the server proxies
	// after each reconciliation.
	reconcileCh := make(chan types.Apps)

	// Setup app server that proxies one static app and
	// watches for apps with label group=a.
	s := SetUpSuiteWithConfig(t, suiteConfig{
		Apps: types.Apps{app0},
		ResourceMatchers: []services.ResourceMatcher{
			{Labels: types.Labels{
				"group": []string{"a"},
			}},
		},
		OnReconcile: func(a types.Apps) {
			reconcileCh <- a
		},
	})

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
	select {
	case a := <-reconcileCh:
		slices.SortFunc(a, func(a, b types.Application) int {
			return cmp.Compare(a.GetName(), b.GetName())
		})
		require.Empty(t, gocmp.Diff(types.Apps{app0}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

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
	select {
	case a := <-reconcileCh:
		slices.SortFunc(a, func(a, b types.Application) int {
			return cmp.Compare(a.GetName(), b.GetName())
		})
		require.Empty(t, gocmp.Diff(types.Apps{app0, app1}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Try to update app0 which is registered statically.
	app0Updated, err := makeDynamicApp("app0", map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	require.NoError(t, err)
	err = s.authServer.AuthServer.CreateApp(ctx, app0Updated)
	require.NoError(t, err)

	// It should not be registered, old app0 should remain.
	select {
	case a := <-reconcileCh:
		slices.SortFunc(a, func(a, b types.Application) int {
			return cmp.Compare(a.GetName(), b.GetName())
		})
		require.Empty(t, gocmp.Diff(types.Apps{app0, app1}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Create app with label group=b.
	app2, err := makeDynamicApp("app2", map[string]string{"group": "b"})
	require.NoError(t, err)

	// Set the PublicAddr _before_ creating the app. The watcher should
	// not modify apps with an already specified PublicAddr.
	app2.SetPublicAddr("app2.some.other.addr.example.com")

	err = s.authServer.AuthServer.CreateApp(ctx, app2)
	require.NoError(t, err)

	// It shouldn't be registered.
	select {
	case a := <-reconcileCh:
		slices.SortFunc(a, func(a, b types.Application) int {
			return cmp.Compare(a.GetName(), b.GetName())
		})
		require.Empty(t, gocmp.Diff(types.Apps{app0, app1}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Update app2 labels so it matches.
	app2.SetStaticLabels(map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	err = s.authServer.AuthServer.UpdateApp(ctx, app2)
	require.NoError(t, err)

	// Both should be registered now.
	select {
	case a := <-reconcileCh:
		slices.SortFunc(a, func(a, b types.Application) int {
			return cmp.Compare(a.GetName(), b.GetName())
		})
		require.Empty(t, gocmp.Diff(types.Apps{app0, app1, app2}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Update app2 URI so it gets re-registered.
	app2.SetURI("localhost:2345")
	err = s.authServer.AuthServer.UpdateApp(ctx, app2)
	require.NoError(t, err)

	// app2 should get updated.
	select {
	case a := <-reconcileCh:
		slices.SortFunc(a, func(a, b types.Application) int {
			return cmp.Compare(a.GetName(), b.GetName())
		})
		require.Empty(t, gocmp.Diff(types.Apps{app0, app1, app2}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Update app1 labels so it doesn't match.
	app1.SetStaticLabels(map[string]string{"group": "c", types.OriginLabel: types.OriginDynamic})
	err = s.authServer.AuthServer.UpdateApp(ctx, app1)
	require.NoError(t, err)

	// Only app0 and app2 should remain registered.
	select {
	case a := <-reconcileCh:
		slices.SortFunc(a, func(a, b types.Application) int {
			return cmp.Compare(a.GetName(), b.GetName())
		})
		require.Empty(t, gocmp.Diff(types.Apps{app0, app2}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Remove app2.
	err = s.authServer.AuthServer.DeleteApp(ctx, app2.GetName())
	require.NoError(t, err)

	// Only static app should remain.
	select {
	case a := <-reconcileCh:
		require.Empty(t, gocmp.Diff(types.Apps{app0}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
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

func requireReconciledAppNames(t *testing.T, reconcileCh <-chan types.Apps, wantNames ...string) {
	t.Helper()

	select {
	case apps := <-reconcileCh:
		gotNames := make([]string, 0, len(apps))
		for _, app := range apps {
			gotNames = append(gotNames, app.GetName())
		}
		require.ElementsMatch(t, wantNames, gotNames)
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for reconciled apps %v", wantNames)
	}
}

func requireHeartbeatResult(t *testing.T, heartbeatCh <-chan error, wantErr bool) {
	t.Helper()

	select {
	case err := <-heartbeatCh:
		if wantErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for app heartbeat")
	}
}

func drainHeartbeatResults(heartbeatCh <-chan error) {
	for {
		select {
		case <-heartbeatCh:
		default:
			return
		}
	}
}

func newFailingInventoryHandle() inventory.DownstreamHandle {
	ctx, cancel := context.WithCancel(context.Background())
	senderC := make(chan inventory.DownstreamSender, 1)
	senderC <- failingInventorySender{done: ctx.Done()}
	return &failingInventoryHandle{
		senderC:      senderC,
		closeContext: ctx,
		cancel:       cancel,
	}
}

type failingInventoryHandle struct {
	senderC      chan inventory.DownstreamSender
	closeContext context.Context
	cancel       context.CancelFunc
}

func (h *failingInventoryHandle) Sender() <-chan inventory.DownstreamSender {
	return h.senderC
}

func (h *failingInventoryHandle) GetSender() (inventory.DownstreamSender, bool) {
	return failingInventorySender{done: h.closeContext.Done()}, true
}

func (h *failingInventoryHandle) RegisterPingHandler(inventory.DownstreamPingHandler) func() {
	return func() {}
}

func (h *failingInventoryHandle) CloseContext() context.Context {
	return h.closeContext
}

func (h *failingInventoryHandle) Close() error {
	h.cancel()
	return nil
}

func (h *failingInventoryHandle) SetAndSendGoodbye(context.Context, bool, bool) error {
	return nil
}

func (h *failingInventoryHandle) GetUpstreamLabels(proto.LabelUpdateKind) map[string]string {
	return nil
}

type failingInventorySender struct {
	done <-chan struct{}
}

func (s failingInventorySender) Send(context.Context, proto.UpstreamInventoryMessage) error {
	return errors.New("inventory heartbeat failed")
}

func (s failingInventorySender) Hello() *proto.DownstreamInventoryHello {
	return proto.DownstreamInventoryHello_builder{}.Build()
}

func (s failingInventorySender) Done() <-chan struct{} {
	return s.done
}
