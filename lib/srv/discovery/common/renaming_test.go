/*
Copyright 2023 Gravitational, Inc.

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

package common

import (
	"fmt"
	"testing"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysqlflexibleservers"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
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
	requireOverrideLabelSkipsRenaming(t, test.resource, test.nameOverrideLabel)
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
	dbName := "some-db"
	region := "us-west-1"
	accountID := "123456789012"
	for _, overrideLabel := range types.AWSDatabaseNameOverrideLabels {
		database := makeRDSDatabase(t, dbName, region, accountID, overrideLabel)
		test := renameTest{
			resource: database,
			renameFn: func(r types.ResourceWithLabels) {
				db := r.(types.Database)
				ApplyAWSDatabaseNameSuffix(db, services.AWSMatcherRDS)
			},
			originalName:      dbName,
			nameOverrideLabel: overrideLabel,
			wantNewName:       "some-db-rds-us-west-1-123456789012",
		}
		runRenameTest(t, test)
	}
}

func TestApplyAzureDatabaseNameSuffix(t *testing.T) {
	tests := []struct {
		desc,
		dbName,
		region,
		resourceGroup,
		subscriptionID,
		wantRename string
	}{
		{
			desc:           "all parts valid",
			dbName:         "some-db",
			region:         "East US", // we normalize regions, so this should become "eastus".
			resourceGroup:  "Some Group",
			subscriptionID: "11111111-2222-3333-4444-555555555555",
			wantRename:     "some-db-mysql-eastus-Some-Group-11111111-2222-3333-4444-555555555555",
		},
		{
			desc:           "skips invalid resource group",
			dbName:         "some-db",
			region:         "eastus", // use the normalized region.
			resourceGroup:  "(parens are invalid)",
			subscriptionID: "11111111-2222-3333-4444-555555555555",
			wantRename:     "some-db-mysql-eastus-11111111-2222-3333-4444-555555555555",
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			database := makeAzureMySQLFlexDatabase(t, tt.dbName, tt.region, tt.resourceGroup, tt.subscriptionID)
			runRenameTest(t, renameTest{
				resource: database,
				renameFn: func(r types.ResourceWithLabels) {
					db := r.(types.Database)
					ApplyAzureDatabaseNameSuffix(db, services.AzureMatcherMySQL)
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
	require.True(t, ok, "override label should be present")
	require.NotEmpty(t, override, "override label should be present")
	require.Equal(t, override, r.GetName(), "name should equal the override label")
}

// requireDiscoveredNameLabel is a test helper that requires a resource have
// not have the originally "discovered" name as a label, and did not change its name.
func requireOverrideLabelSkipsRenaming(t *testing.T, r types.ResourceWithLabels, overrideLabel string) {
	t.Helper()
	requireOverrideLabelIsSet(t, r, overrideLabel)
	got, gotOk := r.GetLabel(types.DiscoveredNameLabel)
	require.False(t, gotOk, "should not have the original discovered name saved in a label")
	require.Empty(t, got, "should not have the original discovered name saved in a label")
}

func makeRDSDatabase(t *testing.T, name, region, accountID, overrideLabel string) types.Database {
	t.Helper()
	instance := &rds.DBInstance{
		DBInstanceArn:        aws.String(fmt.Sprintf("arn:aws:rds:%s:%s:db:%v", region, accountID, name)),
		DBInstanceIdentifier: aws.String(name),
		DbiResourceId:        aws.String(uuid.New().String()),
		Engine:               aws.String(services.RDSEnginePostgres),
		DBInstanceStatus:     aws.String("available"),
		Endpoint: &rds.Endpoint{
			Address: aws.String("localhost"),
			Port:    aws.Int64(5432),
		},
		TagList: libcloudaws.LabelsToTags[rds.Tag](map[string]string{
			overrideLabel: name,
		}),
	}
	database, err := services.NewDatabaseFromRDSInstance(instance)
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
		},
		),
		ID:   &id,
		Name: &name,
		Type: &resourceType,
	}
	database, err := services.NewDatabaseFromAzureMySQLFlexServer(server)
	require.NoError(t, err)
	return database
}

func labelsToAzureTags(labels map[string]string) map[string]*string {
	tags := make(map[string]*string, len(labels))
	for k, v := range labels {
		v := v
		tags[k] = &v
	}
	return tags
}

func makeEKSKubeCluster(t *testing.T, name, region, accountID, overrideLabel string) types.KubeCluster {
	t.Helper()
	eksCluster := &eks.Cluster{
		Name:   aws.String(name),
		Arn:    aws.String(fmt.Sprintf("arn:aws:eks:%s:%s:cluster/%s", region, accountID, name)),
		Status: aws.String(eks.ClusterStatusActive),
		Tags: map[string]*string{
			overrideLabel: aws.String(name),
		},
	}
	kubeCluster, err := services.NewKubeClusterFromAWSEKS(eksCluster)
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
	kubeCluster, err := services.NewKubeClusterFromAzureAKS(aksCluster)
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

	kubeCluster, err := services.NewKubeClusterFromGCPGKE(gkeCluster)
	require.NoError(t, err)
	require.True(t, kubeCluster.IsGCP())
	return kubeCluster
}
