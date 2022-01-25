/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package db

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

// TestWatcher verifies that database server properly detects and applies
// changes to database resources.
func TestWatcher(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)

	// Make a static configuration database.
	db0, err := makeStaticDatabase("db0", nil)
	require.NoError(t, err)

	// This channel will receive new set of databases the server proxies
	// after each reconciliation.
	reconcileCh := make(chan types.Databases)

	// Create database server that proxies one static database and
	// watches for databases with label group=a.
	testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: []types.Database{db0},
		ResourceMatchers: []services.ResourceMatcher{
			{Labels: types.Labels{
				"group": []string{"a"},
			}},
		},
		OnReconcile: func(d types.Databases) {
			reconcileCh <- d
		},
	})

	// Only db0 should be registered initially.
	select {
	case d := <-reconcileCh:
		sort.Sort(d)
		require.Empty(t, cmp.Diff(types.Databases{db0}, d,
			cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Create database with label group=a.
	db1, err := makeDynamicDatabase("db1", map[string]string{"group": "a"})
	require.NoError(t, err)
	err = testCtx.authServer.CreateDatabase(ctx, db1)
	require.NoError(t, err)

	// It should be registered.
	select {
	case d := <-reconcileCh:
		sort.Sort(d)
		require.Empty(t, cmp.Diff(types.Databases{db0, db1}, d,
			cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Try to update db0 which is registered statically.
	db0Updated, err := makeDynamicDatabase("db0", map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	require.NoError(t, err)
	err = testCtx.authServer.CreateDatabase(ctx, db0Updated)
	require.NoError(t, err)

	// It should not be registered, old db0 should remain.
	select {
	case d := <-reconcileCh:
		sort.Sort(d)
		require.Empty(t, cmp.Diff(types.Databases{db0, db1}, d,
			cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Create database with label group=b.
	db2, err := makeDynamicDatabase("db2", map[string]string{"group": "b"})
	require.NoError(t, err)
	err = testCtx.authServer.CreateDatabase(ctx, db2)
	require.NoError(t, err)

	// It shouldn't be registered.
	select {
	case d := <-reconcileCh:
		sort.Sort(d)
		require.Empty(t, cmp.Diff(types.Databases{db0, db1}, d,
			cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Update db2 labels so it matches.
	db2.SetStaticLabels(map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	err = testCtx.authServer.UpdateDatabase(ctx, db2)
	require.NoError(t, err)

	// Both should be registered now.
	select {
	case d := <-reconcileCh:
		sort.Sort(d)
		require.Empty(t, cmp.Diff(types.Databases{db0, db1, db2}, d,
			cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Update db2 URI so it gets re-registered.
	db2.SetURI("localhost:2345")
	err = testCtx.authServer.UpdateDatabase(ctx, db2)
	require.NoError(t, err)

	// db2 should get updated.
	select {
	case d := <-reconcileCh:
		sort.Sort(d)
		require.Empty(t, cmp.Diff(types.Databases{db0, db1, db2}, d,
			cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Update db1 labels so it doesn't match.
	db1.SetStaticLabels(map[string]string{"group": "c", types.OriginLabel: types.OriginDynamic})
	err = testCtx.authServer.UpdateDatabase(ctx, db1)
	require.NoError(t, err)

	// Only db0 and db2 should remain registered.
	select {
	case d := <-reconcileCh:
		sort.Sort(d)
		require.Empty(t, cmp.Diff(types.Databases{db0, db2}, d,
			cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}

	// Remove db2.
	err = testCtx.authServer.DeleteDatabase(ctx, db2.GetName())
	require.NoError(t, err)

	// Only static database should remain.
	select {
	case d := <-reconcileCh:
		require.Empty(t, cmp.Diff(types.Databases{db0}, d,
			cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}
}

func makeStaticDatabase(name string, labels map[string]string) (*types.DatabaseV3, error) {
	return makeDatabase(name, labels, map[string]string{
		types.OriginLabel: types.OriginConfigFile,
	})
}

func makeDynamicDatabase(name string, labels map[string]string) (*types.DatabaseV3, error) {
	return makeDatabase(name, labels, map[string]string{
		types.OriginLabel: types.OriginDynamic,
	})
}

func makeDatabase(name string, labels map[string]string, additionalLabels map[string]string) (*types.DatabaseV3, error) {
	if labels == nil {
		labels = make(map[string]string)
	}
	for k, v := range additionalLabels {
		labels[k] = v
	}
	return types.NewDatabaseV3(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
}
