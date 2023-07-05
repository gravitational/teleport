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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
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
	assertReconciledResource(t, reconcileCh, types.Databases{db0})

	// Create database with label group=a.
	db1, err := makeDynamicDatabase("db1", map[string]string{"group": "a"})
	require.NoError(t, err)
	err = testCtx.authServer.CreateDatabase(ctx, db1)
	require.NoError(t, err)

	// It should be registered.
	assertReconciledResource(t, reconcileCh, types.Databases{db0, db1})

	// Try to update db0 which is registered statically.
	db0Updated, err := makeDynamicDatabase("db0", map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	require.NoError(t, err)
	err = testCtx.authServer.CreateDatabase(ctx, db0Updated)
	require.NoError(t, err)

	// It should not be registered, old db0 should remain.
	assertReconciledResource(t, reconcileCh, types.Databases{db0, db1})

	// Create database with label group=b.
	db2, err := makeDynamicDatabase("db2", map[string]string{"group": "b"})
	require.NoError(t, err)
	err = testCtx.authServer.CreateDatabase(ctx, db2)
	require.NoError(t, err)

	// It shouldn't be registered.
	assertReconciledResource(t, reconcileCh, types.Databases{db0, db1})

	// Update db2 labels so it matches.
	db2.SetStaticLabels(map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	err = testCtx.authServer.UpdateDatabase(ctx, db2)
	require.NoError(t, err)

	// Both should be registered now.
	assertReconciledResource(t, reconcileCh, types.Databases{db0, db1, db2})

	// Update db2 URI so it gets re-registered.
	db2.SetURI("localhost:2345")
	err = testCtx.authServer.UpdateDatabase(ctx, db2)
	require.NoError(t, err)

	// db2 should get updated.
	assertReconciledResource(t, reconcileCh, types.Databases{db0, db1, db2})

	// Update db1 labels so it doesn't match.
	db1.SetStaticLabels(map[string]string{"group": "c", types.OriginLabel: types.OriginDynamic})
	err = testCtx.authServer.UpdateDatabase(ctx, db1)
	require.NoError(t, err)

	// Only db0 and db2 should remain registered.

	assertReconciledResource(t, reconcileCh, types.Databases{db0, db2})

	// Remove db2.
	err = testCtx.authServer.DeleteDatabase(ctx, db2.GetName())
	require.NoError(t, err)

	// Only static database should remain.
	assertReconciledResource(t, reconcileCh, types.Databases{db0})
}

// TestWatcherRDSDynamicResource RDS dynamic resource registration where the ResourceMatchers should be always
// evaluated for the dynamic registered resources.
func TestWatcherCloudDynamicResource(t *testing.T) {
	var db1, db2, db3 *types.DatabaseV3
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)

	db0, err := makeStaticDatabase("db0", nil)
	require.NoError(t, err)

	reconcileCh := make(chan types.Databases)
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
	assertReconciledResource(t, reconcileCh, types.Databases{db0})

	withRDSURL := func(v3 *types.DatabaseSpecV3) {
		v3.URI = "mypostgresql.c6c8mwvfdgv0.us-west-2.rds.amazonaws.com:5432"
	}

	t.Run("dynamic resource - no match", func(t *testing.T) {
		// Created an RDS db dynamic resource that doesn't match any db service ResourceMatchers.
		db1, err = makeDynamicDatabase("db1", map[string]string{"group": "z"}, withRDSURL)
		require.NoError(t, err)
		require.True(t, db1.IsRDS())
		err = testCtx.authServer.CreateDatabase(ctx, db1)
		require.NoError(t, err)
		// The db1 should not be registered by the agent due to ResourceMatchers mismatch:
		assertReconciledResource(t, reconcileCh, types.Databases{db0})
	})

	t.Run("dynamic resource - match", func(t *testing.T) {
		// Create an RDS dynamic resource with labels that matches ResourceMatchers.
		db2, err = makeDynamicDatabase("db2", map[string]string{"group": "a"}, withRDSURL)
		require.NoError(t, err)
		require.True(t, db2.IsRDS())

		err = testCtx.authServer.CreateDatabase(ctx, db2)
		require.NoError(t, err)
		// The db2 service should be properly registered by the agent.
		assertReconciledResource(t, reconcileCh, types.Databases{db0, db2})
	})

	t.Run("cloud resource - no match", func(t *testing.T) {
		// Create an RDS Cloud resource with a label that doesn't match resource matcher.
		db3, err = makeCloudDatabase("db3", map[string]string{"group": "z"})
		require.NoError(t, err)
		require.True(t, db3.IsRDS())

		// The db3 DB RDS Cloud origin resource should properly register by the agent even if  DB labels don't match
		// any ResourceMatchers. The RDS Cloud origin databases relays could fetchers that return only matching databases.
		err = testCtx.authServer.CreateDatabase(ctx, db3)
		require.NoError(t, err)
		assertReconciledResource(t, reconcileCh, types.Databases{db0, db2, db3})
	})
}

func assertReconciledResource(t *testing.T, ch chan types.Databases, databases types.Databases) {
	t.Helper()
	select {
	case d := <-ch:
		sort.Sort(d)
		require.Equal(t, len(d), len(databases))
		require.Empty(t, cmp.Diff(databases, d,
			cmpopts.IgnoreFields(types.Metadata{}, "ID"),
			cmpopts.IgnoreFields(types.DatabaseStatusV3{}, "CACert"),
		))
	case <-time.After(time.Second):
		t.Fatal("Didn't receive reconcile event after 1s.")
	}
}

func makeStaticDatabase(name string, labels map[string]string, opts ...makeDatabaseOpt) (*types.DatabaseV3, error) {
	return makeDatabase(name, labels, map[string]string{
		types.OriginLabel: types.OriginConfigFile,
	}, opts...)
}

func makeDynamicDatabase(name string, labels map[string]string, opts ...makeDatabaseOpt) (*types.DatabaseV3, error) {
	return makeDatabase(name, labels, map[string]string{
		types.OriginLabel: types.OriginDynamic,
	}, opts...)
}

func makeCloudDatabase(name string, labels map[string]string) (*types.DatabaseV3, error) {
	return makeDatabase(name, labels, map[string]string{
		types.OriginLabel: types.OriginCloud,
	}, func(v3 *types.DatabaseSpecV3) {
		v3.URI = "mypostgresql.c6c8mwvfdgv0.us-west-2.rds.amazonaws.com:5432"
	})
}

type makeDatabaseOpt func(*types.DatabaseSpecV3)

func makeDatabase(name string, labels map[string]string, additionalLabels map[string]string, opts ...makeDatabaseOpt) (*types.DatabaseV3, error) {
	if labels == nil {
		labels = make(map[string]string)
	}

	for k, v := range additionalLabels {
		labels[k] = v
	}

	ds := types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	}

	for _, o := range opts {
		o(&ds)
	}

	return types.NewDatabaseV3(types.Metadata{
		Name:   name,
		Labels: labels,
	}, ds)
}
