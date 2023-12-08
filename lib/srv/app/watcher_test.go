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
	"context"
	"maps"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// TestWatcher verifies that app agent properly detects and applies
// changes to application resources.
func TestWatcher(t *testing.T) {
	ctx := context.Background()

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

	// Only app0 should be registered initially.
	select {
	case a := <-reconcileCh:
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.Apps{app0}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Create app with label group=a.
	app1, err := makeDynamicApp("app1", map[string]string{"group": "a"})
	require.NoError(t, err)
	err = s.authServer.AuthServer.CreateApp(ctx, app1)
	require.NoError(t, err)

	// It should be registered.
	select {
	case a := <-reconcileCh:
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.Apps{app0, app1}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
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
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.Apps{app0, app1}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Create app with label group=b.
	app2, err := makeDynamicApp("app2", map[string]string{"group": "b"})
	require.NoError(t, err)
	err = s.authServer.AuthServer.CreateApp(ctx, app2)
	require.NoError(t, err)

	// It shouldn't be registered.
	select {
	case a := <-reconcileCh:
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.Apps{app0, app1}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
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
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.Apps{app0, app1, app2}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
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
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.Apps{app0, app1, app2}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
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
		sort.Sort(a)
		require.Empty(t, cmp.Diff(types.Apps{app0, app2}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
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
		require.Empty(t, cmp.Diff(types.Apps{app0}, a,
			cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
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
