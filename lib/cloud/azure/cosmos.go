// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azure

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cosmos/armcosmos"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// armCosmosDatabaseAccountsClient is an interface defines a subset of functions of
// armcosmos.DatabaseAccountsClient.
type armCosmosDatabaseAccountsClient interface {
	ListKeys(ctx context.Context, resourceGroupName string, accountName string, options *armcosmos.DatabaseAccountsClientListKeysOptions) (armcosmos.DatabaseAccountsClientListKeysResponse, error)
	BeginRegenerateKey(ctx context.Context, resourceGroupName string, accountName string, keyToRegenerate armcosmos.DatabaseAccountRegenerateKeyParameters, options *armcosmos.DatabaseAccountsClientBeginRegenerateKeyOptions) (*runtime.Poller[armcosmos.DatabaseAccountsClientRegenerateKeyResponse], error)
}

// cosmosDatabaseAccountsClient is an Azure CosmosDB database accounts client.
type cosmosDatabaseAccountsClient struct {
	api     armCosmosDatabaseAccountsClient
	fncache *utils.FnCache
}

// NewCosmosDatabaseAccountsClient creates a new Azure SQL Server client by subscription and
// credentials.
func NewCosmosDatabaseAccountsClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (CosmosDatabaseAccountsClient, error) {
	api, err := armcosmos.NewDatabaseAccountsClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fncache, err := newDefaultCosmosFnCache()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &cosmosDatabaseAccountsClient{api, fncache}, nil
}

// NewSQLClientByAPI creates a new Azure SQL Serverclient by ARM API client and
// cache options.
func NewCosmosDatabaseAccountsClientByAPI(api armCosmosDatabaseAccountsClient, fncache *utils.FnCache) CosmosDatabaseAccountsClient {
	return &cosmosDatabaseAccountsClient{api, fncache}
}

// GetKey fetches the CosmosDB account key kind. It caches the Azure API result
// for a short period because this API can take a few seconds to complete and
// also avoids getting rate limit errors.
func (c *cosmosDatabaseAccountsClient) GetKey(ctx context.Context, resourceGroup string, accountName string, key armcosmos.KeyKind) (string, error) {
	return utils.FnCacheGet(ctx, c.fncache, key, func(ctx context.Context) (string, error) {
		return c.getKey(ctx, resourceGroup, accountName, key)
	})
}

// RegenerateKey regenerates a CosmosDB account key.
func (c *cosmosDatabaseAccountsClient) RegenerateKey(ctx context.Context, resourceGroup string, accountName string, key armcosmos.KeyKind) error {
	poller, err := c.api.BeginRegenerateKey(ctx, resourceGroup, accountName, armcosmos.DatabaseAccountRegenerateKeyParameters{
		KeyKind: &key,
	}, nil)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// getKey retrieves the CosmosDB account key using Azure API.
func (c *cosmosDatabaseAccountsClient) getKey(ctx context.Context, resourceGroup string, accountName string, key armcosmos.KeyKind) (string, error) {
	resp, err := c.api.ListKeys(ctx, resourceGroup, accountName, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}

	switch key {
	case armcosmos.KeyKindPrimary:
		return *resp.PrimaryMasterKey, nil
	case armcosmos.KeyKindPrimaryReadonly:
		return *resp.PrimaryReadonlyMasterKey, nil
	case armcosmos.KeyKindSecondary:
		return *resp.SecondaryMasterKey, nil
	case armcosmos.KeyKindSecondaryReadonly:
		return *resp.SecondaryReadonlyMasterKey, nil
	default:
		return "", trace.BadParameter("%q key is not supported", string(key))
	}
}

// newDefaultCosmosFnCache initialze a utils.FnCache with default values.
func newDefaultCosmosFnCache() (*utils.FnCache, error) {
	return utils.NewFnCache(utils.FnCacheConfig{
		TTL:             10 * time.Second,
		CleanupInterval: 20 * time.Second,
		ReloadOnErr:     true,
	})
}
