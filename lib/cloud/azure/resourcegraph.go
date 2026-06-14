/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package azure

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

const (
	// resourceGraphPageSize is the number of results to request per page.
	// According to Azure Docs, the maximum page size is 1000, which is what we use here.
	// https://learn.microsoft.com/en-us/azure/governance/resource-graph/concepts/work-with-data#paging-results
	resourceGraphPageSize = 1_000

	// resourceGraphMaxPages is the maximum number of pages to fetch when paginating through results.
	// Azure Docs don't specify a maximum number of pages, but setting a high limit here is a safeguard against infinite loops.
	// With a page size of 1000, fetching 10000 pages allows us to fetch up to 10 million resources, which should be more than enough.
	resourceGraphMaxPages = 10_000
)

// ResourceGraphClient is a client for Azure Resource Graph (ARG) VM discovery.
type ResourceGraphClient interface {
	// QueryLinuxVMs returns a list of running Linux VMs in the specified subscription.
	// You can filter the results by resource group ("*" or empty for all resource groups) and by location ("*" or empty for all locations).
	QueryLinuxVMs(ctx context.Context, params QueryLinuxVMsParams) ([]*VirtualMachine, error)
}

// argResourcesAPI is the slice of armresourcegraph.Client we depend on, extracted as an interface
// so unit tests can fake the SDK without spinning up a real ARG client.
type argResourcesAPI interface {
	Resources(ctx context.Context, query armresourcegraph.QueryRequest, options *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error)
}

type resourceGraphClient struct {
	logger       *slog.Logger
	resourcesAPI argResourcesAPI
}

// NewResourceGraphClient returns a ResourceGraphClient backed by the official
// Azure SDK's armresourcegraph.Client.
func NewResourceGraphClient(cred azcore.TokenCredential, options *arm.ClientOptions) (ResourceGraphClient, error) {
	client, err := armresourcegraph.NewClient(cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &resourceGraphClient{
		logger:       slog.With(teleport.ComponentKey, "azure_resource_graph_client"),
		resourcesAPI: client,
	}, nil
}

func queryResultGetString(result map[string]any, key string) (string, error) {
	if v, ok := result[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s, nil
		}
	}
	return "", trace.BadParameter("missing or invalid key %q in result", key)
}

func queryResultGetKeyValueString(ctx context.Context, log *slog.Logger, keyVal any) map[string]string {
	if keyVal == nil {
		return map[string]string{}
	}

	result, ok := keyVal.(map[string]any)
	if !ok {
		return map[string]string{}
	}

	out := make(map[string]string, len(result))
	for tagKey, tagValue := range result {
		valueAsString, ok := tagValue.(string)
		if !ok {
			log.WarnContext(ctx, "skipping non-string value in key-value map", "key", tagKey, "value_type", fmt.Sprintf("%T", tagValue))
			continue
		}
		out[tagKey] = valueAsString
	}

	return out
}
