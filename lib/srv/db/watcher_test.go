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
	"maps"
	"sort"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	elasticache "github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/memorydb"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	rss "github.com/aws/aws-sdk-go-v2/service/redshiftserverless"
	rsstypes "github.com/aws/aws-sdk-go-v2/service/redshiftserverless/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/azure/azuretest"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	discovery "github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/srv/discovery/fetchers/db"
)

// TestWatcher verifies that database server properly detects and applies
// changes to database resources.
func TestWatcher(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)

	// Make a static configuration database.
	db0, err := makeStaticDatabase("db0", nil)
	require.NoError(t, err)

	// Create database server that proxies one static database and
	// watches for databases with label group=a.
	testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: []types.Database{db0},
		ResourceMatchers: []services.ResourceMatcher{{
			Labels: types.Labels{
				"group": []string{"a"},
			},
			// these should not be applied to non-AWS databases.
			AWS: services.ResourceMatcherAWS{AssumeRoleARN: "some-role", ExternalID: "some-externalid"},
		}},
	})

	// Observe what the agent publishes to the cluster.
	w := newBackendDatabaseWatcher(ctx, t, testCtx)

	// Only db0 should be registered initially.
	w.awaitDatabases(types.Databases{db0})

	// Create database with label group=a.
	db1, err := makeDynamicDatabase("db1", map[string]string{"group": "a"})
	require.NoError(t, err)
	err = testCtx.authServer.CreateDatabase(ctx, db1)
	require.NoError(t, err)

	// It should be registered.
	w.awaitDatabases(types.Databases{db0, db1})

	// Try to update db0 which is registered statically.
	db0Updated, err := makeDynamicDatabase("db0", map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	require.NoError(t, err)
	err = testCtx.authServer.CreateDatabase(ctx, db0Updated)
	require.NoError(t, err)

	// It should not be registered, old db0 should remain.
	w.awaitDatabases(types.Databases{db0, db1})

	// Create database with label group=b.
	db2, err := makeDynamicDatabase("db2", map[string]string{"group": "b"})
	require.NoError(t, err)
	err = testCtx.authServer.CreateDatabase(ctx, db2)
	require.NoError(t, err)

	// It shouldn't be registered.
	w.awaitDatabases(types.Databases{db0, db1})

	// Update db2 labels so it matches.
	db2.SetStaticLabels(map[string]string{"group": "a", types.OriginLabel: types.OriginDynamic})
	err = testCtx.authServer.UpdateDatabase(ctx, db2)
	require.NoError(t, err)

	// Both should be registered now.
	w.awaitDatabases(types.Databases{db0, db1, db2})

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		servers, err := testCtx.authServer.GetDatabaseServers(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		require.Len(t, servers, 3)
		require.Equal(t, "db0", servers[0].GetName())
		wantDBs := types.Databases(types.DatabaseServers(servers).ToDatabases()).ToMap()
		for _, db := range []types.Database{db0, db1, db2} {
			require.Contains(t, wantDBs, db.GetName())
		}
	}, 10*time.Second, 100*time.Millisecond, "waiting for database heartbeats to be registered")

	// Update db2 URI so it gets re-registered.
	db2.SetURI("localhost:2345")
	err = testCtx.authServer.UpdateDatabase(ctx, db2)
	require.NoError(t, err)

	// db2 should get updated.
	w.awaitDatabases(types.Databases{db0, db1, db2})

	// Update db1 labels so it doesn't match.
	db1.SetStaticLabels(map[string]string{"group": "c", types.OriginLabel: types.OriginDynamic})
	err = testCtx.authServer.UpdateDatabase(ctx, db1)
	require.NoError(t, err)

	// Only db0 and db2 should remain registered.
	w.awaitDatabases(types.Databases{db0, db2})

	// Remove db2.
	err = testCtx.authServer.DeleteDatabase(ctx, db2.GetName())
	require.NoError(t, err)

	// Only static database should remain.
	w.awaitDatabases(types.Databases{db0})

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		servers, err := testCtx.authServer.GetDatabaseServers(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		require.Len(t, servers, 1)
		require.Equal(t, "db0", servers[0].GetName())
	}, 10*time.Second, 100*time.Millisecond, "waiting for database heartbeats to be cleaned up")
}

// TestWatcherDynamicResource tests dynamic resource registration where the
// ResourceMatchers should be always evaluated for the dynamic registered
// resources.
func TestWatcherDynamicResource(t *testing.T) {
	var db1, db2, db3, db4, db5, db6 *types.DatabaseV3
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)

	db0, err := makeStaticDatabase("db0", nil)
	require.NoError(t, err)

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
		DiscoveryResourceChecker: &fakeDiscoveryResourceChecker{
			byName: map[string]func(context.Context, types.Database) error{
				"db-fail-check": func(context.Context, types.Database) error {
					return trace.BadParameter("bad db")
				},
				"db5": func(_ context.Context, db types.Database) error {
					if db.GetURI() == "evil-llama.com" {
						return trace.BadParameter("bad database URI")
					}
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
	w := newBackendDatabaseWatcher(ctx, t, testCtx)
	w.awaitDatabases(types.Databases{db0})

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
		w.awaitDatabases(types.Databases{db0})
	})

	t.Run("dynamic resource - match", func(t *testing.T) {
		// Create an RDS dynamic resource with labels that matches ResourceMatchers.
		db2, err = makeDynamicDatabase("db2", map[string]string{"group": "a"}, withRDSURL)
		require.NoError(t, err)
		require.True(t, db2.IsRDS())

		err = testCtx.authServer.CreateDatabase(ctx, db2)
		require.NoError(t, err)
		// The db2 service should be properly registered by the agent.
		w.awaitDatabases(types.Databases{db0, db2})
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
		w.awaitDatabases(types.Databases{db0, db2})
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
		w.awaitDatabases(types.Databases{db0, db2, db4})
	})

	t.Run("discovery resource - AssumeRoleARN", func(t *testing.T) {
		// Created a discovery service created database resource that matches
		// ResourceMatchers and has AssumeRoleARN set by the discovery service.
		discoveredDB5, err := makeDiscoveryDatabase("db5", map[string]string{"group": "b"}, withRDSURL, withDiscoveryAssumeRoleARN)
		require.NoError(t, err)
		require.True(t, discoveredDB5.IsAWSHosted())
		require.True(t, discoveredDB5.IsRDS())

		err = testCtx.authServer.CreateDatabase(ctx, discoveredDB5)
		require.NoError(t, err)

		// Validate that AssumeRoleARN is overwritten by the one configured in
		// the resource matcher.
		db5 = discoveredDB5.Copy()
		setStatusAWSAssumeRole(db5, "arn:aws:iam::123456789012:role/DBAccess", "external-id")

		w.awaitDatabases(types.Databases{db0, db2, db4, db5})
	})

	t.Run("non-AWS discovery resource - AssumeRoleARN not applied", func(t *testing.T) {
		// Created a discovery service created database resource that matches
		// ResourceMatchers but is not an AWS database
		_, azureDB := makeAzureSQLServer(t, "discovery-azure", "group")
		setDiscoveryTypeLabel(azureDB, types.AzureMatcherSQLServer)
		setLabels(azureDB, map[string]string{"group": "b"})
		azureDB.SetOrigin(types.OriginCloud)
		require.False(t, azureDB.IsAWSHosted())
		require.True(t, azureDB.GetAWS().IsEmpty())
		require.True(t, azureDB.IsAzure())
		err = testCtx.authServer.CreateDatabase(ctx, azureDB)
		require.NoError(t, err)

		db6 = azureDB.Copy()
		w.awaitDatabases(types.Databases{db0, db2, db4, db5, db6})
	})

	t.Run("discovery resource - fail check on create", func(t *testing.T) {
		// Created a discovery service created database resource that fails the
		// fakeDiscoveryResourceChecker.
		dbFailCheck, err := makeDiscoveryDatabase("db-fail-check", map[string]string{"group": "a"}, withRDSURL)
		require.NoError(t, err)
		require.NoError(t, testCtx.authServer.CreateDatabase(ctx, dbFailCheck))

		// dbFailCheck should not be proxied.
		w.awaitDatabases(types.Databases{db0, db2, db4, db5, db6})
	})

	t.Run("discovery resource - fail check on update", func(t *testing.T) {
		badDB := db5.Copy()
		badDB.SetURI("evil-llama.com")
		err = testCtx.authServer.UpdateDatabase(ctx, badDB)
		require.NoError(t, err)
		// update is rejected for failed URI check.
		w.awaitDatabases(types.Databases{db0, db2, db4, db5, db6})
	})
}

func setDiscoveryTypeLabel(r types.ResourceWithLabels, matcherType string) {
	setLabels(r, map[string]string{types.DiscoveryTypeLabel: matcherType})
}

func setLabels(r types.ResourceWithLabels, newLabels map[string]string) {
	staticLabels := r.GetStaticLabels()
	if staticLabels == nil {
		staticLabels = make(map[string]string)
	}
	maps.Copy(staticLabels, newLabels)
	r.SetStaticLabels(staticLabels)
}

// TestWatcherCloudFetchers tests usage of discovery database fetchers by the
// database service.
func TestWatcherCloudFetchers(t *testing.T) {
	// Test an AWS fetcher.
	redshiftServerlessWorkgroup := mocks.RedshiftServerlessWorkgroup("discovery-aws", "us-east-1")
	redshiftServerlessDatabase, err := discovery.NewDatabaseFromRedshiftServerlessWorkgroup(redshiftServerlessWorkgroup, nil)
	require.NoError(t, err)
	redshiftServerlessDatabase.SetStatusAWS(redshiftServerlessDatabase.GetAWS())
	setDiscoveryTypeLabel(redshiftServerlessDatabase, types.AWSMatcherRedshiftServerless)
	redshiftServerlessDatabase.SetOrigin(types.OriginCloud)
	discovery.ApplyAWSDatabaseNameSuffix(redshiftServerlessDatabase, types.AWSMatcherRedshiftServerless)
	require.Empty(t, redshiftServerlessDatabase.GetAWS().AssumeRoleARN)
	require.Empty(t, redshiftServerlessDatabase.GetAWS().ExternalID)
	// Test an Azure fetcher.
	azSQLServer, azSQLServerDatabase := makeAzureSQLServer(t, "discovery-azure", "group")
	setDiscoveryTypeLabel(azSQLServerDatabase, types.AzureMatcherSQLServer)
	azSQLServerDatabase.SetOrigin(types.OriginCloud)
	require.False(t, azSQLServerDatabase.IsAWSHosted())
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)

	dbFetcherFactory, err := db.NewAWSFetcherFactory(db.AWSFetcherFactoryConfig{
		AWSConfigProvider: &mocks.AWSConfigProvider{},
		AWSClients: fakeAWSClients{
			rdsClient: &mocks.RDSClient{Unauth: true}, // Access denied error should not affect other fetchers.
			rssClient: &mocks.RedshiftServerlessClient{
				Workgroups: []rsstypes.Workgroup{*redshiftServerlessWorkgroup},
			},
		},
	})
	require.NoError(t, err)
	testCtx.setupDatabaseServer(ctx, t, agentParams{
		// Dynamic matchers are included in config to test that the dynamic
		// matcher AWS settings are not applied to cloud watcher databases.
		ResourceMatchers: []services.ResourceMatcher{{
			Labels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			AWS: services.ResourceMatcherAWS{
				AssumeRoleARN: "role-arn",
				ExternalID:    "external-id",
			},
		}},
		AzureClients: &azuretest.Clients{
			AzureSQLServer: azure.NewSQLClientByAPI(&azure.ARMSQLServerMock{
				AllServers: []*armsql.Server{azSQLServer},
			}),
			AzureManagedSQLServer: azure.NewManagedSQLClientByAPI(&azure.ARMSQLManagedServerMock{}),
		},
		AWSDatabaseFetcherFactory: dbFetcherFactory,
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

	w := newBackendDatabaseWatcher(ctx, t, testCtx)

	wantDatabases := types.Databases{azSQLServerDatabase, redshiftServerlessDatabase}
	sort.Sort(wantDatabases)

	// cloud metadata updater is disabled, so don't check the AWS metadata status.
	w.awaitDatabases(wantDatabases, cmpopts.IgnoreFields(types.DatabaseStatusV3{}, "AWS"))
}

// backendDatabaseWatcher observes the databases the agent publishes to the
// cluster: registrations arrive as DatabaseServer heartbeats (OpPut) and
// unregistrations as OpDelete. Tests assert against this externally visible
// contract instead of internal reconciler state.
type backendDatabaseWatcher struct {
	t       *testing.T
	watcher types.Watcher
	state   map[string]types.Database
}

func newBackendDatabaseWatcher(ctx context.Context, t *testing.T, testCtx *testContext) *backendDatabaseWatcher {
	t.Helper()

	watcher, err := testCtx.authServer.NewWatcher(ctx, types.Watch{
		Name:  "lib/srv/db.watcher_test",
		Kinds: []types.WatchKind{{Kind: types.KindDatabaseServer}},
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
	w := &backendDatabaseWatcher{t: t, watcher: watcher, state: make(map[string]types.Database)}
	servers, err := testCtx.authServer.GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	for _, server := range servers {
		w.state[server.GetDatabase().GetName()] = server.GetDatabase()
	}
	return w
}

func (w *backendDatabaseWatcher) apply(event types.Event) {
	switch event.Type {
	case types.OpPut:
		server, ok := event.Resource.(types.DatabaseServer)
		require.True(w.t, ok, "unexpected resource type %T", event.Resource)
		w.state[server.GetDatabase().GetName()] = server.GetDatabase()
	case types.OpDelete:
		delete(w.state, event.Resource.GetName())
	}
}

// awaitDatabases consumes DatabaseServer events until the set of published
// databases matches want, then briefly continues draining to catch
// registrations that should not have happened (the "no change" assertions).
func (w *backendDatabaseWatcher) awaitDatabases(want types.Databases, opts ...cmp.Option) {
	w.t.Helper()

	// The published heartbeat carries registration/announce-time enrichment;
	// mirror the parts that are deterministic: CloudIAM stamps Status.AWS
	// with the merged AWS metadata for AWS-hosted databases.
	expected := make(types.Databases, 0, len(want))
	for _, database := range want {
		database = database.Copy()
		if database.IsAWSHosted() {
			database.SetStatusAWS(database.GetAWS())
		}
		expected = append(expected, database)
	}

	compare := func() string {
		published := make(types.Databases, 0, len(w.state))
		for _, database := range w.state {
			published = append(published, database)
		}
		sort.Sort(published)
		return cmp.Diff(expected, published,
			append(cmp.Options{
				cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
				// CACert and VNetDNSName are assigned by the server during
				// registration; they are not part of the desired spec.
				cmpopts.IgnoreFields(types.DatabaseStatusV3{}, "CACert", "VNetDNSName"),
				// IAMPolicyStatus is the result of the announce-time cloud
				// IAM check, not part of the desired spec.
				cmpopts.IgnoreFields(types.AWS{}, "IAMPolicyStatus"),
			}, opts...)...,
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
			w.t.Fatalf("timed out waiting for published databases to converge: %v", compare())
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
				w.t.Fatalf("published databases diverged after converging: %v", diff)
			}
		case <-w.watcher.Done():
			w.t.Fatalf("watcher closed: %v", w.watcher.Error())
		case <-settle:
			return
		}
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

	maps.Copy(labels, additionalLabels)

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

type fakeAWSClients struct {
	ec2Client        db.EC2Client
	ecClient         db.ElastiCacheClient
	mdbClient        db.MemoryDBClient
	openSearchClient db.OpenSearchClient
	rdsClient        db.RDSClient
	redshiftClient   db.RedshiftClient
	rssClient        db.RSSClient
}

func (f fakeAWSClients) GetEC2Client(cfg aws.Config, optFns ...func(*ec2.Options)) db.EC2Client {
	return f.ec2Client
}

func (f fakeAWSClients) GetElastiCacheClient(cfg aws.Config, optFns ...func(*elasticache.Options)) db.ElastiCacheClient {
	return f.ecClient
}

func (f fakeAWSClients) GetMemoryDBClient(cfg aws.Config, optFns ...func(*memorydb.Options)) db.MemoryDBClient {
	return f.mdbClient
}

func (f fakeAWSClients) GetOpenSearchClient(cfg aws.Config, optFns ...func(*opensearch.Options)) db.OpenSearchClient {
	return f.openSearchClient
}

func (f fakeAWSClients) GetRDSClient(cfg aws.Config, optFns ...func(*rds.Options)) db.RDSClient {
	return f.rdsClient
}

func (f fakeAWSClients) GetRedshiftClient(cfg aws.Config, optFns ...func(*redshift.Options)) db.RedshiftClient {
	return f.redshiftClient
}

func (f fakeAWSClients) GetRedshiftServerlessClient(cfg aws.Config, optFns ...func(*rss.Options)) db.RSSClient {
	return f.rssClient
}
