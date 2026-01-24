/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package accessgraph

import (
	"context"
	"os"

	"github.com/gravitational/trace"

	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
)

// ClusterClientGetter is a function that returns a ClusterConfigServiceClient
type ClusterClientGetter = func(ctx context.Context) (clusterconfigv1.ClusterConfigServiceClient, error)

// AccessGraphConfig represents the Access Graph configuration.
type AccessGraphConfig struct {
	// Enabled indicates whether Access Graph is enabled locally or in the cluster.
	Enabled bool
	// Addr is the address of the Access Graph service.
	Addr string
	// Insecure is true if the connection to the Access Graph service should be insecure.
	Insecure bool
	// CA is the PEM-encoded CA certificate used to verify the Access Graph GRPC connection.
	CA []byte
	// CipherSuites is the list of TLS cipher suites to use for the connection.
	CipherSuites []uint16
}

// GetAccessGraphSettingsConfig represents the Access Graph configuration
// that can be read from the local config file.
type GetAccessGraphSettingsConfig struct {
	// LocallyEnabled indicates whether Access Graph reporting is enabled locally
	// via the config file.
	LocallyEnabled bool

	// Addr of the Access Graph service addr
	Addr string

	// CA is the path to the CA certificate file
	CA string

	// Insecure is true if the connection to the Access Graph service should be insecure
	Insecure bool

	// CipherSuites is the list of TLS cipher suites to use for the connection
	CipherSuites []uint16

	// IsAuthServer indicates whether this config is for an auth server
	IsAuthServer bool

	// ClusterClientGetter is a function that returns a ClusterConfigServiceClient
	ClusterClientGetter ClusterClientGetter
}

// GetAccessGraphSettings retrieves Access Graph configuration either from local
// config or from the cluster via the auth server.
func GetAccessGraphSettings(ctx context.Context, config GetAccessGraphSettingsConfig) (AccessGraphConfig, error) {
	if config.ClusterClientGetter == nil {
		return AccessGraphConfig{}, trace.BadParameter("missing ClusterClientGetter")
	}
	if config.LocallyEnabled {
		var ca []byte
		if config.CA != "" {
			caBytes, err := os.ReadFile(config.CA)
			if err != nil {
				return AccessGraphConfig{}, trace.Wrap(err, "failed to read access graph CA from path %q", config.CA)
			}
			ca = caBytes
		}
		return AccessGraphConfig{
			Enabled:      true,
			Addr:         config.Addr,
			Insecure:     config.Insecure,
			CipherSuites: config.CipherSuites,
			CA:           ca,
		}, nil
	}

	// Fetch settings from the cluster if not enabled locally.
	return getAccessGraphSettingsFromCluster(ctx, config)
}

// getAccessGraphSettingsFromCluster fetches Access Graph configuration from the cluster
// via the auth server.
func getAccessGraphSettingsFromCluster(ctx context.Context, config GetAccessGraphSettingsConfig) (AccessGraphConfig, error) {
	if config.IsAuthServer {
		return AccessGraphConfig{}, nil
	}

	client, err := config.ClusterClientGetter(ctx)
	if err != nil {
		return AccessGraphConfig{}, trace.Wrap(err, "failed to create cluster config client")
	}
	resp, err := client.GetClusterAccessGraphConfig(
		ctx,
		&clusterconfigv1.GetClusterAccessGraphConfigRequest{},
	)
	if err != nil {
		return AccessGraphConfig{}, trace.Wrap(err, "failed to get access graph settings from auth server")
	}

	agConfig := resp.GetAccessGraph()
	if agConfig == nil || !agConfig.GetEnabled() {
		return AccessGraphConfig{}, trace.BadParameter("access graph is not enabled in the cluster")
	}

	return AccessGraphConfig{
		Enabled:      agConfig.GetEnabled(),
		Addr:         agConfig.GetAddress(),
		Insecure:     agConfig.GetInsecure(),
		CA:           agConfig.GetCa(),
		CipherSuites: config.CipherSuites,
	}, nil
}
