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

package tools

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// proxyLookupClusterMaxSize is max the number of nodes in the cluster to attempt an opportunistic node lookup
	// in the proxy cache. We always do embedding lookups if the cluster is larger than this number.
	proxyLookupClusterMaxSize = 100
	maxEmbeddingsPerLookup    = 10

	// TODO(joel): remove/change when migrating to embeddings
	maxShownRequestableItems = 50
)

// *ToolContext contains various "data" which is commonly needed by various tools.
type ToolContext struct {
	assist.AssistEmbeddingServiceClient
	AccessRequestClient
	AccessPoint
	services.AccessChecker
	NodeWatcher NodeWatcher
	User        string
	ClusterName string
}

// NodeWatcher abstracts away services.NodeWatcher for testing purposes.
type NodeWatcher interface {
	// GetNodes returns a list of nodes that match the given filter.
	GetNodes(ctx context.Context, fn func(n services.Node) bool) []types.Server

	// NodeCount returns the number of nodes in the cluster.
	NodeCount() int
}

// AccessPoint allows reading resources from proxy cache.
type AccessPoint interface {
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

// AccessRequestClient abstracts away the access request client for testing purposes.
type AccessRequestClient interface {
	CreateAccessRequest(ctx context.Context, req types.AccessRequest) error
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
}

// Tool is an interface that allows the agent to interact with the outside world.
// It is used to implement things such as vector document retrieval and command execution.
type Tool interface {
	Name() string
	Description() string
	Run(ctx context.Context, toolCtx *ToolContext, input string) (string, error)
}
