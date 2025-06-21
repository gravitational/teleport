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

package common

import (
	"fmt"
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysqlflexibleservers"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redis/armredis/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/lib/cloud/awstesthelpers"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/services"
)

// TestMakeDiscoverySuffix tests makeDiscoverySuffix in isolation.
func TestMakeDiscoverySuffix(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		extraParts   []string
		wantSuffix   string
	}{
		{
			name:         "no suffix made without extra parts",
			resourceName: "foo",
			wantSuffix:   "",
		},
		{
			name:         "simple parts",
			resourceName: "foo",
			extraParts:   []string{"one", "two", "three"},
			wantSuffix:   "one-two-three",
		},
		{
			name:         "skips empty parts",
			resourceName: "foo",
			extraParts:   []string{"one", "", "three"},
			wantSuffix:   "one-three",
		},
		{
			name:         "converts extra whitespace to hyphens",
			resourceName: "foo",
			extraParts:   []string{"one", "t w o", "three"},
			wantSuffix:   "one-t-w-o-three",
		},
		{
			name:         "removes repeated hypens",
			resourceName: "foo",
			extraParts:   []string{"one---", "t w  --  o  ", "---three"},
			wantSuffix:   "one-t-w-o-three",
		},
		{
			name:         "removes leading and trailing hypens",
			resourceName: "foo",
			extraParts:   []string{"one---", "t w  --  o  ", "---three"},
			wantSuffix:   "one-t-w-o-three",
		},
		{
			name:         "skips adding redundant info",
			resourceName: "PostgreSQL-RDS-us-west-1",
			// suffixes are added to make resource names unique.
			// Adding info as a suffix when that info is already contained in
			// the resource name verbatim would pointlessly make a resource name
			// longer and ugly, i.e. we don't want users to see like
			// "PostgreSQL-RDS-us-west-1-rds-us-west-1-123456789012" as a resource
			// name.
			extraParts: []string{"rds", "us-west-1", "123456789012"},
			wantSuffix: "123456789012",
		},
		{
			name:         "skips invalid parts",
			resourceName: "foo",
			// parentheses are illegal in both database and kube cluster names in Teleport.
			extraParts: []string{"mysql", "EastUS", "weird)(group-name", "11111111-2222-3333-4444-555555555555"},
			wantSuffix: "mysql-EastUS-11111111-2222-3333-4444-555555555555",
		},
	}
	for validatorKind, validatorFn := range map[string]suffixValidatorFn{
		"databases":     databaseNamePartValidator,
		"kube clusters": kubeClusterNamePartValidator,
	} {
		for _, test := range tests {
			t.Run(fmt.Sprintf("%s/%s", validatorKind, test.name), func(t *testing.T) {
				got := makeDiscoverySuffix(validatorFn, test.resourceName, test.extraParts...)
				require.Equal(t, test.wantSuffix, got)
			})
		}
	}
}

// renameFunc is a callback to specialize on the renaming func to use for a
// resource under test.
type renameFunc func(types.ResourceWithLabels)

// renameTest is a test helper struct to group common test structure for
// renaming resources.
type renameTest struct {
	// resource is the resource under test. It will be modified during test run
	// if the resource is renamed.
	resource types.ResourceWithLabels
	// renameFn is used to specialize the renaming func to use.
	renameFn renameFunc
	// originalName is the name of the resource as it was before renaming.
	originalName string
	// nameOverrideLabel is the cloud override label used to manually override a
	// resource name. Renaming should be skipped when this label is present.
	nameOverrideLabel string
	// wantNewName is the name the test expects after the resource is renamed
	// according to the discovery renaming format.
	wantNewName string
}

func runRenameTest(t *testing.T, test renameTest) {
	t.Helper()
	// all tests should start out with the override name label set, to indicate that the resource shouldn't be renamed.
	requireOverrideLabelIsSet(t, test.resource, test.nameOverrideLabel)
	// try renaming the resource.
	test.renameFn(test.resource)
	// verify it was not renamed.
	requireOverrideLabelSkipsRenaming(t, test.resource, test.originalName, test.nameOverrideLabel)
	// clear the override label.
	labels := test.resource.GetStaticLabels()
	delete(labels, test.nameOverrideLabel)
	test.resource.SetStaticLabels(labels)
	// now try renaming without an override label.
	test.renameFn(test.resource)
	// verify that the resource was renamed as we expected.
	require.Equal(t, test.wantNewName, test.resource.GetName())
	// verify that the original name was saved as a label after renaming.
	requireDiscoveredNameLabel(t, test.resource, test.originalName, test.nameOverrideLabel)
}

func TestApplyAWSDatabaseNameSuffix(t *testing.T) {
	tests := []struct {
		desc,
		dbName,
		region,
		accountID,
		wantRename string
		makeDBFunc func(t *testing.T, name, region, account, overrideLabel string) types.Database
	}{
		{
			desc:       "RDS instance",
			dbName:     "some-db",
			region:     "us-west-1",
			accountID:  "123456789012",
			wantRename: "some-db-rds-us-west-1-123456789012",
			makeDBFunc: makeRDSInstanceDB,
		},
		{
			desc:       "RDS Aurora cluster",
			dbName:     "some-db",
			region:     "us-west-1",
			accountID:  "123456789012",
			wantRename: "some-db-rds-aurora-us-west-1-123456789012",
			makeDBFunc: makeAuroraPrimaryDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			for _, overrideLabel := range types.AWSDatabaseNameOverrideLabels {
				database := tt.makeDBFunc(t, tt.dbName, tt.region, tt.accountID, overrideLabel)
				test := renameTest{
					resource: database,
					renameFn: func(r types.ResourceWithLabels) {
						db := r.(types.Database)
						ApplyAWSDatabaseNameSuffix(db, types.AWSMatcherRDS)
					},
					originalName:      tt.dbName,
					nameOverrideLabel: overrideLabel,
					wantNewName:       tt.wantRename,
				}
				runRenameTest(t, test)
			}
		})
	}
}

func TestApplyAzureDatabaseNameSuffix(t *testing.T) {
	tests := []struct {
		desc,
		dbName,
		region,
		resourceGroup,
		subscriptionID,
		matcherType,
		wantRename string
		makeDBFunc func(t *testing.T, name, region, group, subscription string) types.Database
	}{
		{
			desc:           "Azure MySQL Flex",
			dbName:         "some-db",
			region:         "East US", // we normalize regions, so this should become "eastus".
			resourceGroup:  "Some Group",
			subscriptionID: "11111111-2222-3333-4444-555555555555",
			matcherType:    types.AzureMatcherMySQL,
			wantRename:     "some-db-mysql-eastus-Some-Group-11111111-2222-3333-4444-555555555555",
			makeDBFunc:     makeAzureMySQLFlexDatabase,
		},
		{
			desc:           "skips invalid resource group",
			dbName:         "some-db",
			region:         "eastus", // use the normalized region.
			resourceGroup:  "(parens are invalid)",
			subscriptionID: "11111111-2222-3333-4444-555555555555",
			matcherType:    types.AzureMatcherMySQL,
			wantRename:     "some-db-mysql-eastus-11111111-2222-3333-4444-555555555555",
			makeDBFunc:     makeAzureMySQLFlexDatabase,
		},
		{
			desc:           "Azure Redis",
			dbName:         "some-db",
			region:         "eastus",
			resourceGroup:  "Some Group",
			subscriptionID: "11111111-2222-3333-4444-555555555555",
			matcherType:    types.AzureMatcherRedis,
			wantRename:     "some-db-redis-eastus-Some-Group-11111111-2222-3333-4444-555555555555",
			makeDBFunc:     makeAzureRedisDB,
		},
		{
			desc:           "Azure Redis Enterprise",
			dbName:         "some-db",
			region:         "eastus",
			resourceGroup:  "Some Group",
			subscriptionID: "11111111-2222-3333-4444-555555555555",
			matcherType:    types.AzureMatcherRedis,
			wantRename:     "some-db-redis-enterprise-eastus-Some-Group-11111111-2222-3333-4444-555555555555",
			makeDBFunc:     makeAzureRedisEnterpriseDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			database := tt.makeDBFunc(t, tt.dbName, tt.region, tt.resourceGroup, tt.subscriptionID)
			runRenameTest(t, renameTest{
				resource: database,
				renameFn: func(r types.ResourceWithLabels) {
					db := r.(types.Database)
					ApplyAzureDatabaseNameSuffix(db, tt.matcherType)
				},
				originalName:      tt.dbName,
				nameOverrideLabel: types.AzureDatabaseNameOverrideLabel,
				wantNewName:       tt.wantRename,
			})
		})
	}
}

func TestApplyEKSNameSuffix(t *testing.T) {
	clusterName := "some-cluster"
	region := "us-west-1"
	accountID := "123456789012"
	for _, overrideLabel := range types.AWSKubeClusterNameOverrideLabels {
		cluster := makeEKSKubeCluster(t, clusterName, region, accountID, overrideLabel)
		test := renameTest{
			resource: cluster,
			renameFn: func(r types.ResourceWithLabels) {
				c := r.(types.KubeCluster)
				ApplyEKSNameSuffix(c)
			},
			originalName:      clusterName,
			nameOverrideLabel: overrideLabel,
			wantNewName:       "some-cluster-eks-us-west-1-123456789012",
		}
		runRenameTest(t, test)
	}
}

func TestApplyAKSNameSuffix(t *testing.T) {
	clusterName := "some-cluster"
	region := "westus"
	resourceGroup := "Some Group"
	subscriptionID := "11111111-2222-3333-4444-555555555555"
	cluster := makeAKSKubeCluster(t, clusterName, region, resourceGroup, subscriptionID)
	test := renameTest{
		resource: cluster,
		renameFn: func(r types.ResourceWithLabels) {
			c := r.(types.KubeCluster)
			ApplyAKSNameSuffix(c)
		},
		originalName:      clusterName,
		nameOverrideLabel: types.AzureKubeClusterNameOverrideLabel,
		wantNewName:       "some-cluster-aks-westus-Some-Group-11111111-2222-3333-4444-555555555555",
	}
	runRenameTest(t, test)
}

func TestApplyGKENameSuffix(t *testing.T) {
	clusterName := "some-cluster"
	region := "central-1"
	projectID := "dev-123456"
	cluster := makeGKEKubeCluster(t, clusterName, region, projectID)
	test := renameTest{
		resource: cluster,
		renameFn: func(r types.ResourceWithLabels) {
			c := r.(types.KubeCluster)
			ApplyGKENameSuffix(c)
		},
		originalName:      clusterName,
		nameOverrideLabel: types.GCPKubeClusterNameOverrideLabel,
		wantNewName:       "some-cluster-gke-central-1-dev-123456",
	}
	runRenameTest(t, test)
}

// requireDiscoveredNameLabel is a test helper that requires a resource have
// the originally "discovered" name as a label.
func requireDiscoveredNameLabel(t *testing.T, r types.ResourceWithLabels, want, overrideLabel string) {
	t.Helper()
	override, ok := r.GetLabel(overrideLabel)
	require.False(t, ok, "override label should not be present")
	require.Empty(t, override, "override label should not be present")
	got, gotOk := r.GetLabel(types.DiscoveredNameLabel)
	require.True(t, gotOk, "should have the original discovered name saved in a label")
	require.Equal(t, want, got, "should have the original discovered name saved in a label")
}

func requireOverrideLabelIsSet(t *testing.T, r types.ResourceWithLabels, overrideLabel string) {
	t.Helper()
	override, ok := r.GetLabel(overrideLabel)
	require.True(t, ok, "override label %v should be present", overrideLabel)
	require.NotEmpty(t, override, "override label %v should be present", overrideLabel)
	require.Equal(t, override, r.GetName(), "name should equal the %v override label", overrideLabel)
}

// requireDiscoveredNameLabel is a test helper that requires a resource
// not have the originally "discovered" name as a label, and did not change its name.
func requireOverrideLabelSkipsRenaming(t *testing.T, r types.ResourceWithLabels, originalName, overrideLabel string) {
	t.Helper()
	requireOverrideLabelIsSet(t, r, overrideLabel)
	got, gotOk := r.GetLabel(types.DiscoveredNameLabel)
	require.False(t, gotOk, "should not have the original discovered name saved in a label")
	require.Empty(t, got, "should not have the original discovered name saved in a label")
	require.Equal(t, originalName, r.GetName(),
		"should not have renamed the resource when override label %v is present", overrideLabel)
}

func makeAuroraPrimaryDB(t *testing.T, name, region, accountID, overrideLabel string) types.Database {
	t.Helper()
	cluster := &rdstypes.DBCluster{
		DBClusterArn:                     aws.String(fmt.Sprintf("arn:aws:rds:%s:%s:cluster:%v", region, accountID, name)),
		DBClusterIdentifier:              aws.String("cluster-1"),
		DbClusterResourceId:              aws.String("resource-1"),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		Engine:                           aws.String("aurora-mysql"),
		EngineVersion:                    aws.String("8.0.0"),
		Endpoint:                         aws.String("localhost"),
		Port:                             aws.Int32(3306),
		TagList: awstesthelpers.LabelsToRDSTags(map[string]string{
			overrideLabel: name,
		}),
	}
	database, err := NewDatabaseFromRDSCluster(cluster, []rdstypes.DBInstance{})
	require.NoError(t, err)
	return database
}

func makeRDSInstanceDB(t *testing.T, name, region, accountID, overrideLabel string) types.Database {
	t.Helper()
	instance := &rdstypes.DBInstance{
		DBInstanceArn:        aws.String(fmt.Sprintf("arn:aws:rds:%s:%s:db:%v", region, accountID, name)),
		DBInstanceIdentifier: aws.String(name),
		DbiResourceId:        aws.String(uuid.New().String()),
		Engine:               aws.String(services.RDSEnginePostgres),
		DBInstanceStatus:     aws.String("available"),
		Endpoint: &rdstypes.Endpoint{
			Address: aws.String("localhost"),
			Port:    aws.Int32(5432),
		},
		TagList: awstesthelpers.LabelsToRDSTags(map[string]string{
			overrideLabel: name,
		}),
	}
	database, err := NewDatabaseFromRDSInstance(instance)
	require.NoError(t, err)
	return database
}

func makeAzureMySQLFlexDatabase(t *testing.T, name, region, group, subscription string) types.Database {
	t.Helper()
	resourceType := "Microsoft.DBforMySQL/flexibleServers"
	id := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/%v/%v",
		subscription,
		group,
		resourceType,
		name,
	)

	fqdn := name + ".mysql" + azureutils.DatabaseEndpointSuffix
	state := armmysqlflexibleservers.ServerStateReady
	version := armmysqlflexibleservers.ServerVersionEight021
	server := &armmysqlflexibleservers.Server{
		Location: &region,
		Properties: &armmysqlflexibleservers.ServerProperties{
			FullyQualifiedDomainName: &fqdn,
			State:                    &state,
			Version:                  &version,
		},
		Tags: labelsToAzureTags(map[string]string{
			types.AzureDatabaseNameOverrideLabel: name,
		}),
		ID:   &id,
		Name: &name,
		Type: &resourceType,
	}
	database, err := NewDatabaseFromAzureMySQLFlexServer(server)
	require.NoError(t, err)
	return database
}

func makeAzureRedisDB(t *testing.T, name, region, group, subscription string) types.Database {
	id := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Cache/Redis/%v", subscription, group, name)
	resourceInfo := &armredis.ResourceInfo{
		Name:     to.Ptr(name),
		ID:       to.Ptr(id),
		Location: to.Ptr(region),
		Tags: labelsToAzureTags(map[string]string{
			types.AzureDatabaseNameOverrideLabel: name,
		}),
		Properties: &armredis.Properties{
			HostName:          to.Ptr(fmt.Sprintf("%v.redis.cache.windows.net", name)),
			SSLPort:           to.Ptr(int32(6380)),
			ProvisioningState: to.Ptr(armredis.ProvisioningStateSucceeded),
			RedisVersion:      to.Ptr("6.0"),
		},
	}
	database, err := NewDatabaseFromAzureRedis(resourceInfo)
	require.NoError(t, err)
	return database
}

func makeAzureRedisEnterpriseDB(t *testing.T, name, region, group, subscription string) types.Database {
	clusterID := fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Cache/redisEnterprise/%v", subscription, group, name)
	databaseID := fmt.Sprintf("%v/databases/default", clusterID)
	armCluster := &armredisenterprise.Cluster{
		Name:     to.Ptr(name),
		ID:       to.Ptr(clusterID),
		Location: to.Ptr(region),
		Tags: labelsToAzureTags(map[string]string{
			types.AzureDatabaseNameOverrideLabel: name,
		}),
		Properties: &armredisenterprise.ClusterProperties{
			HostName:     to.Ptr(fmt.Sprintf("%v.%v.redisenterprise.cache.azure.net", name, region)),
			RedisVersion: to.Ptr("6.0"),
		},
	}
	armDatabase := &armredisenterprise.Database{
		Name: to.Ptr("default"),
		ID:   to.Ptr(databaseID),
		Properties: &armredisenterprise.DatabaseProperties{
			ProvisioningState: to.Ptr(armredisenterprise.ProvisioningStateSucceeded),
			Port:              to.Ptr(int32(10000)),
			ClusteringPolicy:  to.Ptr(armredisenterprise.ClusteringPolicyOSSCluster),
			ClientProtocol:    to.Ptr(armredisenterprise.ProtocolEncrypted),
		},
	}
	database, err := NewDatabaseFromAzureRedisEnterprise(armCluster, armDatabase)
	require.NoError(t, err)
	return database
}

func labelsToAzureTags(labels map[string]string) map[string]*string {
	tags := make(map[string]*string, len(labels))
	for k, v := range labels {
		tags[k] = &v
	}
	return tags
}

func makeEKSKubeCluster(t *testing.T, name, region, accountID, overrideLabel string) types.KubeCluster {
	t.Helper()
	eksCluster := &ekstypes.Cluster{
		Name: aws.String(name),
		Arn:  aws.String(fmt.Sprintf("arn:aws:eks:%s:%s:cluster/%s", region, accountID, name)),
		Tags: map[string]string{
			overrideLabel: name,
		},
	}
	kubeCluster, err := NewKubeClusterFromAWSEKS(aws.ToString(eksCluster.Name), aws.ToString(eksCluster.Arn), eksCluster.Tags)
	require.NoError(t, err)
	require.True(t, kubeCluster.IsAWS())
	return kubeCluster
}

func makeAKSKubeCluster(t *testing.T, name, location, group, subID string) types.KubeCluster {
	t.Helper()
	aksCluster := &azure.AKSCluster{
		Name:           name,
		GroupName:      group,
		TenantID:       "tenantID",
		Location:       location,
		SubscriptionID: subID,
		Tags: map[string]string{
			types.AzureKubeClusterNameOverrideLabel: name,
		},
		Properties: azure.AKSClusterProperties{},
	}
	kubeCluster, err := NewKubeClusterFromAzureAKS(aksCluster)
	require.NoError(t, err)
	require.True(t, kubeCluster.IsAzure())
	return kubeCluster
}

func makeGKEKubeCluster(t *testing.T, name, location, projectID string) types.KubeCluster {
	gkeCluster := gcp.GKECluster{
		Name:   name,
		Status: containerpb.Cluster_RUNNING,
		Labels: map[string]string{
			types.GCPKubeClusterNameOverrideLabel: name,
		},
		ProjectID:   projectID,
		Location:    location,
		Description: "desc1",
	}

	kubeCluster, err := NewKubeClusterFromGCPGKE(gkeCluster)
	require.NoError(t, err)
	require.True(t, kubeCluster.IsGCP())
	return kubeCluster
}
