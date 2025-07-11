// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package autoupdate

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/client/webclient"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

const timeout = 5 * time.Second

// GetVersions returns auto update version for clusters that are reachable.
func GetVersions(ctx context.Context, logger *slog.Logger, resolver ClusterResolver, clusters []*clusters.Cluster) ([]*api.Version, error) {
	versions := make([]*api.Version, 0, len(clusters))
	mu := sync.Mutex{}

	group, groupCtx := errgroup.WithContext(ctx)
	// Arbitrary limit allowing 10 concurrent calls.
	group.SetLimit(10)

	for _, cluster := range clusters {
		group.Go(func() error {
			_, tc, err := resolver(cluster.URI)
			if err != nil {
				logger.ErrorContext(groupCtx, "Could not resolve cluster, skipping", "cluster_uri", cluster.URI, "error", err)
				return nil
			}

			find, err := webclient.Find(&webclient.Config{
				Context:   groupCtx,
				ProxyAddr: tc.WebProxyAddr,
				Insecure:  tc.InsecureSkipVerify,
				Timeout:   timeout,
			})
			if err != nil {
				logger.ErrorContext(groupCtx, "Could not read client version for cluster, skipping", "cluster_uri", cluster.URI, "error", err)
				return nil
			}

			mu.Lock()
			versions = append(versions, &api.Version{
				ClusterUri:      cluster.URI.String(),
				ToolsAutoUpdate: find.AutoUpdate.ToolsAutoUpdate,
				ToolsVersion:    find.AutoUpdate.ToolsVersion,
				MinToolsVersion: find.MinClientVersion,
			})
			mu.Unlock()
			return nil
		})
	}

	err := group.Wait()
	return versions, trace.Wrap(err)
}

// ClusterResolver resolves a cluster and its client from a URI.
type ClusterResolver = func(uri uri.ResourceURI) (*clusters.Cluster, *client.TeleportClient, error)
