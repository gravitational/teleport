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
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/client/webclient"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/auto_update/v1"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

const (
	pingTimeout = 5 * time.Second

	// When tsh runs as a daemon, auto-updates must be disabled. Connect enforces this by
	// launching tsh with TELEPORT_TOOLS_VERSION=off, and forwards the real value via
	// FORWARDED_TELEPORT_TOOLS_VERSION.
	forwardedTeleportToolsEnvVar = "FORWARDED_TELEPORT_TOOLS_VERSION"
	teleportToolsVersionOff      = "off"
)

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

// GetConfig retrieves the local auto updates configuration.
func (s *Service) GetConfig(_ context.Context, _ *api.GetConfigRequest) (*api.GetConfigResponse, error) {
	config, err := platformGetConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	toolsVersionValue := strings.TrimSpace(config.GetToolsVersion().Value)
	toolsVersionSource := config.GetToolsVersion().Source
	switch toolsVersionValue {
	case "":
		toolsVersionSource = api.ConfigSource_CONFIG_SOURCE_UNSPECIFIED
	case teleportToolsVersionOff:
		break
	default:
		if _, err = semver.NewVersion(toolsVersionValue); err != nil {
			return nil, trace.BadParameter("invalid version %v for tools version", toolsVersionValue)
		}
	}

	cdnBaseUrlValue := strings.TrimSpace(config.GetCdnBaseUrl().Value)
	cdnBaseUrlSource := config.GetCdnBaseUrl().Source
	switch cdnBaseUrlValue {
	case "":
		cdnBaseUrlSource = api.ConfigSource_CONFIG_SOURCE_UNSPECIFIED
	default:
		if err = validateURL(cdnBaseUrlValue); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	m := modules.GetModules()
	// Uses the same logic as the teleport/lib/autoupdate/tools package.
	if cdnBaseUrlValue == "" && m.BuildType() != modules.BuildOSS {
		cdnBaseUrlValue = autoupdate.DefaultBaseURL
		cdnBaseUrlSource = api.ConfigSource_CONFIG_SOURCE_DEFAULT
	}

	return &api.GetConfigResponse{
		ToolsVersion: &api.ConfigValue{Value: toolsVersionValue, Source: toolsVersionSource},
		CdnBaseUrl:   &api.ConfigValue{Value: cdnBaseUrlValue, Source: cdnBaseUrlSource},
	}, nil
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return trace.BadParameter("invalid CDN base URL: %v", err)
	}
	if u.Scheme != "https" {
		return trace.BadParameter("CDN base URL must be https")
	}
	if u.Host == "" {
		return trace.BadParameter("CDN base URL must include host")
	}
	return nil
}

func readConfigFromEnvVars() (*api.GetConfigResponse, error) {
	envBaseURL := os.Getenv(autoupdate.BaseURLEnvVar)
	envTeleportToolsVersion := os.Getenv(forwardedTeleportToolsEnvVar)

	return &api.GetConfigResponse{
		CdnBaseUrl: &api.ConfigValue{
			Value:  envBaseURL,
			Source: api.ConfigSource_CONFIG_SOURCE_ENV_VAR,
		},
		ToolsVersion: &api.ConfigValue{
			Value:  envTeleportToolsVersion,
			Source: api.ConfigSource_CONFIG_SOURCE_ENV_VAR,
		},
	}, nil
}
