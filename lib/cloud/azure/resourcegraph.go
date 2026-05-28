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
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
)

// ResourceGraphClient is a client for Azure Resource Graph (ARG) VM discovery.
type ResourceGraphClient interface {
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
		logger:       slog.Default().With(teleport.ComponentKey, "azure_resource_graph_client"),
		resourcesAPI: client,
	}, nil
}
