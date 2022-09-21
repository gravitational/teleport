/*
Copyright 2022 Gravitational, Inc.

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

package azure

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/redisenterprise/armredisenterprise"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/trace"
)

// armRedisEnterpriseDatabaseClient is an interface defines a subset of
// functions of armredisenterprise.DatabaseClient
type armRedisEnterpriseDatabaseClient interface {
	ListKeys(ctx context.Context, resourceGroupName string, clusterName string, databaseName string, options *armredisenterprise.DatabasesClientListKeysOptions) (armredisenterprise.DatabasesClientListKeysResponse, error)
	NewListByClusterPager(resourceGroupName string, clusterName string, options *armredisenterprise.DatabasesClientListByClusterOptions) *runtime.Pager[armredisenterprise.DatabasesClientListByClusterResponse]
}

// armRedisEnterpriseClusterClient is an interface defines a subset of
// functions of armredisenterprise.Client
type armRedisEnterpriseClusterClient interface {
	NewListPager(options *armredisenterprise.ClientListOptions) *runtime.Pager[armredisenterprise.ClientListResponse]
	NewListByResourceGroupPager(resourceGroupName string, options *armredisenterprise.ClientListByResourceGroupOptions) *runtime.Pager[armredisenterprise.ClientListByResourceGroupResponse]
}

// redisEnterpriseClient is an Azure Redis Enterprise client.
type redisEnterpriseClient struct {
	clusterAPI  armRedisEnterpriseClusterClient
	databaseAPI armRedisEnterpriseDatabaseClient
}

// NewRedisEnterpriseClient creates a new Azure Redis Enterprise client by
// subscription and credentials.
func NewRedisEnterpriseClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (CacheForRedisClient, error) {
	logrus.Debug("Initializing Azure Redis Enterprise client.")
	clusterAPI, err := armredisenterprise.NewClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	databaseAPI, err := armredisenterprise.NewDatabasesClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewRedisEnterpriseClientByAPI(clusterAPI, databaseAPI), nil
}

// NewRedisEnterpriseClientByAPI creates a new Azure Redis Enterprise client by
// ARM API client(s).
func NewRedisEnterpriseClientByAPI(clusterAPI armRedisEnterpriseClusterClient, databaseAPI armRedisEnterpriseDatabaseClient) CacheForRedisClient {
	return &redisEnterpriseClient{
		clusterAPI:  clusterAPI,
		databaseAPI: databaseAPI,
	}
}

// GetToken retrieves the auth token for provided resource group and resource
// name.
func (c *redisEnterpriseClient) GetToken(ctx context.Context, resourceID string) (string, error) {
	id, err := arm.ParseResourceID(resourceID)
	if err != nil {
		return "", trace.Wrap(err)
	}
	clusterName, databaseName, err := c.getClusterAndDatabaseName(id)
	if err != nil {
		return "", trace.Wrap(err)
	}

	resp, err := c.databaseAPI.ListKeys(ctx, id.ResourceGroupName, clusterName, databaseName, &armredisenterprise.DatabasesClientListKeysOptions{})
	if err != nil {
		return "", trace.Wrap(ConvertResponseError(err))
	}

	// There are two keys. Pick first one available.
	if resp.PrimaryKey != nil {
		return *resp.PrimaryKey, nil
	}
	if resp.SecondaryKey != nil {
		return *resp.SecondaryKey, nil
	}
	return "", trace.NotFound("missing keys")
}

// getClusterAndDatabaseName returns the cluster name and the database name
// based on the resource ID. Both armredisenterprise.Cluster.ID and
// armredisenterprise.Database.ID are supported.
func (c *redisEnterpriseClient) getClusterAndDatabaseName(id *arm.ResourceID) (string, string, error) {
	switch id.ResourceType.String() {
	case "Microsoft.Cache/redisEnterprise":
		// It appears an Enterprise cluster always has only one "database", and
		// the database name is always "default".
		return id.Name, RedisEnterpriseClusterDefaultDatabase, nil
	case "Microsoft.Cache/redisEnterprise/databases":
		return id.Parent.Name, id.Name, nil
	default:
		return "", "", trace.BadParameter("unknown Azure Cache for Redis resource type: %v", id.ResourceType)
	}
}

// ListAll returns all Azure Redis servers within an Azure subscription.
func (c *redisEnterpriseClient) ListAll(ctx context.Context) ([]RedisServer, error) {
	var servers []RedisServer
	pager := c.clusterAPI.NewListPager(&armredisenterprise.ClientListOptions{})
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}

		clusterServers, err := c.listByClusters(ctx, page.Value)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		servers = append(servers, clusterServers...)
	}
	return servers, nil
}

// ListWithinGroup returns all Azure Redis servers within an Azure resource group.
func (c *redisEnterpriseClient) ListWithinGroup(ctx context.Context, group string) ([]RedisServer, error) {
	var servers []RedisServer
	pager := c.clusterAPI.NewListByResourceGroupPager(group, &armredisenterprise.ClientListByResourceGroupOptions{})
	for pageNum := 0; pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}

		clusterServers, err := c.listByClusters(ctx, page.Value)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		servers = append(servers, clusterServers...)
	}
	return servers, nil
}

func (c *redisEnterpriseClient) listByClusters(ctx context.Context, clusters []*armredisenterprise.Cluster) ([]RedisServer, error) {
	var servers []RedisServer
	for _, cluster := range clusters {
		if cluster == nil { // should never happen, but checking just in case.
			return nil, trace.BadParameter("cluster is nil")
		}

		resourceID, err := arm.ParseResourceID(stringVal(cluster.ID))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// It appears an Enterprise cluster always has only one "database", and
		// the database name is always "default". However, here doing a
		// ListByCluster (instead of a Get) just in case something changes
		// later.
		pager := c.databaseAPI.NewListByClusterPager(resourceID.ResourceGroupName, stringVal(cluster.Name), &armredisenterprise.DatabasesClientListByClusterOptions{})
		for pageNum := 0; pager.More(); pageNum++ {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, trace.Wrap(ConvertResponseError(err))
			}

			servers = append(servers, redisEnterpriseListResultToRedisServers(page.Value, cluster)...)
		}
	}
	return servers, nil
}

func (c *redisEnterpriseClient) getClusterAndDatabaseName(resourceName string) (string, string) {
	// The resource name can be either:
	//   - cluster resource name: <clusterName>
	//   - database resource name: <clusterName>/databases/<databaseName>
	//
	// Though it appears an Enterprise cluster always has only one "database",
	// and the database name is always "default".
	clusterName, databaseName, ok := strings.Cut(resourceName, "/databases/")
	if ok {
		return clusterName, databaseName
	}
}

// RedisEnterpriseClusterDefaultDatabase is the default database name for a
// Redis Enterprise cluster.
const RedisEnterpriseClusterDefaultDatabase = "default"

func redisEnterpriseListResultToRedisServers(resources []*armredisenterprise.Database, cluster *armredisenterprise.Cluster) []RedisServer {
	if cluster.Properties == nil { // should never happen, but checking just in case.
		logrus.Debugf("Missing properties for armredisenterprise.Cluster %v.", stringVal(cluster.Name))
		return nil
	}

	var servers []RedisServer
	for _, database := range resources {
		if database == nil { // should never happen, but checking just in case.
			logrus.Debugf("Database resource is nil.")
			continue
		}
		if database.Properties == nil { // should never happen, but checking just in case.
			logrus.Debugf("Missing properties for armredisenterprise.Database %v.", stringVal(database.Name))
			continue
		}
		servers = append(servers, &redisEnterpriseServer{
			database: database,
			cluster:  cluster,
		})
	}
	return servers
}

type redisEnterpriseServer struct {
	database *armredisenterprise.Database
	cluster  *armredisenterprise.Cluster
}

func (s *redisEnterpriseServer) GetName() string {
	if stringVal(s.database.Name) == "default" {
		return stringVal(s.cluster.Name)
	}
	return fmt.Sprintf("%s-%s", stringVal(s.cluster.Name), stringVal(s.database.Name))
}
func (s *redisEnterpriseServer) GetLocation() string {
	return stringVal(s.cluster.Location)
}
func (s *redisEnterpriseServer) GetResourceID() string {
	return stringVal(s.database.ID)
}
func (s *redisEnterpriseServer) GetHostname() string {
	return stringVal(s.cluster.Properties.HostName)
}
func (s *redisEnterpriseServer) GetPort() string {
	if s.database.Properties.Port != nil {
		return strconv.Itoa(int(*s.database.Properties.Port))
	}
	return ""
}
func (s *redisEnterpriseServer) GetTags() map[string]string {
	return convertTags(s.cluster.Tags)
}
func (s *redisEnterpriseServer) GetClusteringPolicy() string {
	return stringVal(s.database.Properties.ClusteringPolicy)
}
func (s *redisEnterpriseServer) GetEngineVersion() string {
	return stringVal(s.cluster.Properties.RedisVersion)
}
func (s *redisEnterpriseServer) IsSupported() (bool, error) {
	if stringVal(s.database.Properties.ClientProtocol) == string(armredisenterprise.ProtocolEncrypted) {
		return true, nil
	}
	return false, trace.BadParameter("protocol %v not supported", stringVal(s.database.Properties.ClientProtocol))
}
func (s *redisEnterpriseServer) IsAvailable() (bool, error) {
	switch armredisenterprise.ProvisioningState(stringVal(s.database.Properties.ResourceState)) {
	case armredisenterprise.ProvisioningStateSucceeded,
		armredisenterprise.ProvisioningStateUpdating:
		return true, nil
	case armredisenterprise.ProvisioningStateCanceled,
		armredisenterprise.ProvisioningStateCreating,
		armredisenterprise.ProvisioningStateDeleting,
		armredisenterprise.ProvisioningStateFailed:
		return false, trace.BadParameter("the current provisioning state is %v", stringVal(s.database.Properties.ProvisioningState))
	default:
		logrus.Warnf("Unknown status type: %q. Assuming Azure Redis %q is available.",
			stringVal(s.database.Properties.ProvisioningState),
			s.GetName(),
		)
		return true, nil
	}
}
