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

package db

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	"github.com/aws/aws-sdk-go/service/redshiftserverless"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	clients "github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	discovery "github.com/gravitational/teleport/lib/srv/discovery/common"
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

// TestWatcherDynamicResource tests dynamic resource registration where the
// ResourceMatchers should be always evaluated for the dynamic registered
// resources.
func TestWatcherDynamicResource(t *testing.T) {
	var db1, db2, db3, db4, db5 *types.DatabaseV3
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)

	db0, err := makeStaticDatabase("db0", nil)
	require.NoError(t, err)

	reconcileCh := make(chan types.Databases)
	testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: []types.Database{db0},
		ResourceMatchers: []services.ResourceMatcher{
			{
				Labels: types.Labels{
					"group": []string{"a"},
				},
			},
			{
				Labels: types.Labels{
					"group": []string{"b"},
				},
				AWS: services.ResourceMatcherAWS{
					AssumeRoleARN: "arn:aws:iam::123456789012:role/DBAccess",
					ExternalID:    "external-id",
				},
			},
		},
		OnReconcile: func(d types.Databases) {
			reconcileCh <- d
		},
		DiscoveryResourceChecker: &fakeDiscoveryResourceChecker{
			byName: map[string]func(context.Context, types.Database) error{
				"db-fail-check": func(context.Context, types.Database) error {
					return trace.BadParameter("bad db")
				},
				"db5": func(_ context.Context, db types.Database) error {
					// Validate AssumeRoleARN and ExternalID matches above
					// services.ResourceMatcherAWS,
					meta := db.GetAWS()
					if meta.AssumeRoleARN != "arn:aws:iam::123456789012:role/DBAccess" ||
						meta.ExternalID != "external-id" {
						return trace.CompareFailed("AssumeRoleARN/ExternalID does not match")
					}
					return nil
				},
			},
		},
	})
	assertReconciledResource(t, reconcileCh, types.Databases{db0})

	withRDSURL := func(v3 *types.DatabaseSpecV3) {
		v3.URI = "mypostgresql.c6c8mwvfdgv0.us-west-2.rds.amazonaws.com:5432"
		v3.AWS.AccountID = "123456789012"
	}
	withDiscoveryAssumeRoleARN := func(v3 *types.DatabaseSpecV3) {
		v3.AWS.AssumeRoleARN = "arn:aws:iam::123456789012:role/DBDiscovery"
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

	t.Run("discovery resource - no match", func(t *testing.T) {
		// Created a discovery service created database resource that doesn't
		// match any db service ResourceMatchers.
		db3, err = makeDiscoveryDatabase("db3", map[string]string{"group": "z"}, withRDSURL)
		require.NoError(t, err)
		require.True(t, db3.IsRDS())
		err = testCtx.authServer.CreateDatabase(ctx, db3)
		require.NoError(t, err)
		// The db3 should not be registered by the agent due to ResourceMatchers mismatch:
		assertReconciledResource(t, reconcileCh, types.Databases{db0, db2})
	})

	t.Run("discovery resource - match", func(t *testing.T) {
		// Created a discovery service created database resource that matches
		// ResourceMatchers.
		db4, err = makeDiscoveryDatabase("db4", map[string]string{"group": "a"}, withRDSURL)
		require.NoError(t, err)
		require.True(t, db4.IsRDS())

		err = testCtx.authServer.CreateDatabase(ctx, db4)
		require.NoError(t, err)
		// The db4 service should be properly registered by the agent.
		assertReconciledResource(t, reconcileCh, types.Databases{db0, db2, db4})
	})

	t.Run("discovery resource - AssumeRoleARN", func(t *testing.T) {
		// Created a discovery service created database resource that matches
		// ResourceMatchers and has AssumeRoleARN set by the discovery service.
		discoveredDB5, err := makeDiscoveryDatabase("db5", map[string]string{"group": "b"}, withRDSURL, withDiscoveryAssumeRoleARN)
		require.NoError(t, err)
		require.True(t, discoveredDB5.IsRDS())

		err = testCtx.authServer.CreateDatabase(ctx, discoveredDB5)
		require.NoError(t, err)

		// Validate that AssumeRoleARN is overwritten by the one configured in
		// the resource matcher.
		db5 = discoveredDB5.Copy()
		setStatusAWSAssumeRole(db5, "arn:aws:iam::123456789012:role/DBAccess", "external-id")

		assertReconciledResource(t, reconcileCh, types.Databases{db0, db2, db4, db5})
	})

	t.Run("discovery resource - fail check", func(t *testing.T) {
		// Created a discovery service created database resource that fails the
		// fakeDiscoveryResourceChecker.
		dbFailCheck, err := makeDiscoveryDatabase("db-fail-check", map[string]string{"group": "a"}, withRDSURL)
		require.NoError(t, err)
		require.NoError(t, testCtx.authServer.CreateDatabase(ctx, dbFailCheck))

		// dbFailCheck should not be proxied.
		assertReconciledResource(t, reconcileCh, types.Databases{db0, db2, db4, db5})
	})
}

func setDiscoveryGroupLabel(r types.ResourceWithLabels, discoveryGroup string) {
	staticLabels := r.GetStaticLabels()
	if staticLabels == nil {
		staticLabels = make(map[string]string)
	}
	if discoveryGroup != "" {
		staticLabels[types.TeleportInternalDiscoveryGroupName] = discoveryGroup
	}
	r.SetStaticLabels(staticLabels)
}

// TestWatcherCloudFetchers tests usage of discovery database fetchers by the
// database service.
func TestWatcherCloudFetchers(t *testing.T) {
	// Test an AWS fetcher. Note that status AWS can be set by Metadata
	// service.
	redshiftServerlessWorkgroup := mocks.RedshiftServerlessWorkgroup("discovery-aws", "us-east-1")
	redshiftServerlessDatabase, err := discovery.NewDatabaseFromRedshiftServerlessWorkgroup(redshiftServerlessWorkgroup, nil)
	require.NoError(t, err)
	redshiftServerlessDatabase.SetStatusAWS(redshiftServerlessDatabase.GetAWS())
	setDiscoveryGroupLabel(redshiftServerlessDatabase, "")
	redshiftServerlessDatabase.SetOrigin(types.OriginCloud)
	discovery.ApplyAWSDatabaseNameSuffix(redshiftServerlessDatabase, types.AWSMatcherRedshiftServerless)
	// Test an Azure fetcher.
	azSQLServer, azSQLServerDatabase := makeAzureSQLServer(t, "discovery-azure", "group")
	setDiscoveryGroupLabel(azSQLServerDatabase, "")
	azSQLServerDatabase.SetOrigin(types.OriginCloud)
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)

	reconcileCh := make(chan types.Databases)
	testCtx.setupDatabaseServer(ctx, t, agentParams{
		// Keep ResourceMatchers as nil to disable resource matchers.
		OnReconcile: func(d types.Databases) {
			reconcileCh <- d
		},
		CloudClients: &clients.TestCloudClients{
			RDS: &mocks.RDSMockUnauth{}, // Access denied error should not affect other fetchers.
			RedshiftServerless: &mocks.RedshiftServerlessMock{
				Workgroups: []*redshiftserverless.Workgroup{redshiftServerlessWorkgroup},
			},
			AzureSQLServer: azure.NewSQLClientByAPI(&azure.ARMSQLServerMock{
				AllServers: []*armsql.Server{azSQLServer},
			}),
			AzureManagedSQLServer: azure.NewManagedSQLClientByAPI(&azure.ARMSQLManagedServerMock{}),
		},
		AzureMatchers: []types.AzureMatcher{{
			Subscriptions: []string{"sub"},
			Types:         []string{types.AzureMatcherSQLServer},
			ResourceTags:  types.Labels{types.Wildcard: []string{types.Wildcard}},
		}},
		AWSMatchers: []types.AWSMatcher{{
			Types:   []string{types.AWSMatcherRDS, types.AWSMatcherRedshiftServerless},
			Regions: []string{"us-east-1"},
			Tags:    types.Labels{types.Wildcard: []string{types.Wildcard}},
		}},
	})

	wantDatabases := types.Databases{azSQLServerDatabase, redshiftServerlessDatabase}
	sort.Sort(wantDatabases)

	assertReconciledResource(t, reconcileCh, wantDatabases)
}

func assertReconciledResource(t *testing.T, ch chan types.Databases, databases types.Databases) {
	t.Helper()
	select {
	case d := <-ch:
		sort.Sort(d)
		require.Equal(t, len(d), len(databases))
		require.Empty(t, cmp.Diff(databases, d,
			cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
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

func makeDiscoveryDatabase(name string, labels map[string]string, opts ...makeDatabaseOpt) (*types.DatabaseV3, error) {
	return makeDatabase(name, labels, map[string]string{
		types.OriginLabel: types.OriginCloud,
	}, opts...)
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

func makeAzureSQLServer(t *testing.T, name, group string) (*armsql.Server, types.Database) {
	t.Helper()

	server := &armsql.Server{
		ID:   to.Ptr(fmt.Sprintf("/subscriptions/sub-id/resourceGroups/%v/providers/Microsoft.Sql/servers/%v", group, name)),
		Name: to.Ptr(fmt.Sprintf("%s-database-windows-net", name)),
		Properties: &armsql.ServerProperties{
			FullyQualifiedDomainName: to.Ptr("localhost"),
		},
	}
	database, err := discovery.NewDatabaseFromAzureSQLServer(server)
	require.NoError(t, err)
	discovery.ApplyAzureDatabaseNameSuffix(database, types.AzureMatcherSQLServer)
	return server, database
}
