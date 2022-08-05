/*
Copyright 2021 Gravitational, Inc.

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

package clusters

import (
	"context"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"

	"github.com/gravitational/trace"
)

// Database describes database
type Server struct {
	// URI is the database URI
	URI uri.ResourceURI

	types.Server
}

// GetServers returns cluster servers
func (c *Cluster) GetServers(ctx context.Context) ([]Server, error) {
	proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	clusterServers, err := proxyClient.FindServersByLabels(ctx, defaults.Namespace, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results := []Server{}
	for _, server := range clusterServers {
		results = append(results, Server{
			URI:    c.URI.AppendServer(server.GetName()),
			Server: server,
		})
	}

	return results, nil
}
