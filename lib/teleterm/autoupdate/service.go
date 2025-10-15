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
	"os"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/client/webclient"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/auto_update/v1"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

const pingTimeout = 5 * time.Second

// Service implements gRPC service for autoupdate.
type Service struct {
	api.UnimplementedAutoUpdateServiceServer

	cfg Config
}

// New creates an instance of Service.
func New(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		cfg: cfg,
	}, nil
}

// Config contains configuration of the Service.
type Config struct {
	ClusterProvider ClusterProvider
	// InsecureSkipVerify signifies whether webclient.Find is going to verify the identity of the proxy service.
	InsecureSkipVerify bool
}

// ClusterProvider allows listing clusters.
type ClusterProvider interface {
	// ListRootClusters returns a list of root clusters.
	ListRootClusters(ctx context.Context) ([]*clusters.Cluster, error)
}

// CheckAndSetDefaults checks and sets the defaults.
func (c *Config) CheckAndSetDefaults() error {
	if c.ClusterProvider == nil {
		return trace.BadParameter("missing ClusterProvider")
	}

	return nil
}

// GetClusterVersions returns client tools versions for all clusters.
func (s *Service) GetClusterVersions(ctx context.Context, _ *api.GetClusterVersionsRequest) (*api.GetClusterVersionsResponse, error) {
	rootClusters, err := s.cfg.ClusterProvider.ListRootClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reachableClusters := make([]*api.ClusterVersionInfo, 0, len(rootClusters))
	unreachableClusters := make([]*api.UnreachableCluster, 0, len(rootClusters))
	mu := sync.Mutex{}

	group, groupCtx := errgroup.WithContext(ctx)
	// Arbitrary limit allowing 10 concurrent calls.
	group.SetLimit(10)

	for _, cluster := range rootClusters {
		group.Go(func() error {
			ping, pingErr := s.pingCluster(groupCtx, cluster)
			if pingErr != nil {
				mu.Lock()
				unreachableClusters = append(unreachableClusters, &api.UnreachableCluster{
					ClusterUri:   cluster.URI.String(),
					ErrorMessage: pingErr.Error(),
				})
				mu.Unlock()
				return nil
			}

			mu.Lock()
			reachableClusters = append(reachableClusters, &api.ClusterVersionInfo{
				ClusterUri:      cluster.URI.String(),
				ToolsAutoUpdate: ping.AutoUpdate.ToolsAutoUpdate,
				ToolsVersion:    ping.AutoUpdate.ToolsVersion,
				MinToolsVersion: ping.MinClientVersion,
			})
			mu.Unlock()
			return nil
		})
	}

	err = group.Wait()
	return &api.GetClusterVersionsResponse{
		ReachableClusters:   reachableClusters,
		UnreachableClusters: unreachableClusters,
	}, trace.Wrap(err)
}

func (s *Service) pingCluster(ctx context.Context, cluster *clusters.Cluster) (*webclient.PingResponse, error) {
	find, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: cluster.WebProxyAddr,
		Insecure:  s.cfg.InsecureSkipVerify,
		Timeout:   pingTimeout,
	})
	return find, trace.Wrap(err)
}

// GetDownloadBaseUrl returns base URL for downloading Teleport packages.
func (s *Service) GetDownloadBaseUrl(_ context.Context, _ *api.GetDownloadBaseUrlRequest) (*api.GetDownloadBaseUrlResponse, error) {
	baseURL, err := resolveBaseURL()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.GetDownloadBaseUrlResponse{
		BaseUrl: baseURL,
	}, trace.Wrap(err)
}

// resolveBaseURL generates the base URL using the same logic as the teleport/lib/autoupdate/tools package.
func resolveBaseURL() (string, error) {
	envBaseURL := os.Getenv(autoupdate.BaseURLEnvVar)
	if envBaseURL != "" {
		// TODO(gzdunek): Validate if it's correct URL.
		return envBaseURL, nil
	}

	m := modules.GetModules()
	if m.BuildType() == modules.BuildOSS {
		return "", trace.BadParameter("Client tools updates are disabled as they are licensed under AGPL. To use Community Edition builds or custom binaries, set the 'TELEPORT_CDN_BASE_URL' environment variable.")
	}

	return autoupdate.DefaultBaseURL, nil
}
