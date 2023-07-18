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

package clusters

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
)

type GatewayCreator struct {
	resolver Resolver
}

func NewGatewayCreator(resolver Resolver) GatewayCreator {
	return GatewayCreator{
		resolver: resolver,
	}
}

func (g GatewayCreator) CreateGateway(ctx context.Context, params CreateGatewayParams) (*gateway.Gateway, error) {
	cluster, _, err := g.resolver.ResolveCluster(params.TargetURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gateway, err := cluster.CreateGateway(ctx, params)
	return gateway, trace.Wrap(err)
}

// Resolver is a subset of [Storage], mostly so that it's possible to provide a mock implementation
// in tests.
type Resolver interface {
	// ResolveCluster returns a cluster from storage given the URI. See [Storage.ResolveCluster].
	ResolveCluster(string) (*Cluster, *client.TeleportClient, error)
}
