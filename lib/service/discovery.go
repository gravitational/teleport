/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package service

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/srv/discovery"
)

func (process *TeleportProcess) shouldInitDiscovery() bool {
	return process.Config.Discovery.Enabled && !process.Config.Discovery.IsEmpty()
}

func (process *TeleportProcess) initDiscovery() {
	process.RegisterWithAuthServer(types.RoleDiscovery, DiscoveryIdentityEvent)
	process.RegisterCriticalFunc("discovery.init", process.initDiscoveryService)
}

func (process *TeleportProcess) initDiscoveryService() error {
	logger := process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentDiscovery, process.id))

	conn, err := process.WaitForConnector(DiscoveryIdentityEvent, logger)
	if conn == nil {
		return trace.Wrap(err)
	}

	accessPoint, err := process.newLocalCacheForDiscovery(conn.Client,
		[]string{teleport.ComponentDiscovery})
	if err != nil {
		return trace.Wrap(err)
	}

	// asyncEmitter makes sure that sessions do not block
	// in case if connections are slow
	asyncEmitter, err := process.NewAsyncEmitter(conn.Client)
	if err != nil {
		return trace.Wrap(err)
	}

	accessGraphCfg, err := buildAccessGraphFromTAGOrFallbackToAuth(
		process.ExitContext(),
		process.Config,
		conn.Client,
		logger,
	)
	if err != nil {
		return trace.Wrap(err, "failed to build access graph configuration")
	}

	discoveryService, err := discovery.New(process.ExitContext(), &discovery.Config{
		IntegrationOnlyCredentials: process.integrationOnlyCredentials(),
		Matchers: discovery.Matchers{
			AWS:         process.Config.Discovery.AWSMatchers,
			Azure:       process.Config.Discovery.AzureMatchers,
			GCP:         process.Config.Discovery.GCPMatchers,
			Kubernetes:  process.Config.Discovery.KubernetesMatchers,
			AccessGraph: process.Config.Discovery.AccessGraph,
		},
		DiscoveryGroup:    process.Config.Discovery.DiscoveryGroup,
		Emitter:           asyncEmitter,
		AccessPoint:       accessPoint,
		ServerID:          process.Config.HostUUID,
		Log:               process.logger,
		ClusterName:       conn.ClusterName(),
		ClusterFeatures:   process.GetClusterFeatures,
		PollInterval:      process.Config.Discovery.PollInterval,
		GetClientCert:     conn.ClientGetCertificate,
		AccessGraphConfig: accessGraphCfg,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	process.OnExit("discovery.stop", func(payload interface{}) {
		logger.InfoContext(process.ExitContext(), "Shutting down.")
		if discoveryService != nil {
			discoveryService.Stop()
		}
		if asyncEmitter != nil {
			warnOnErr(process.ExitContext(), asyncEmitter.Close(), logger)
		}
		warnOnErr(process.ExitContext(), conn.Close(), logger)
		logger.InfoContext(process.ExitContext(), "Exited.")
	})

	process.BroadcastEvent(Event{Name: DiscoveryReady, Payload: nil})

	if err := discoveryService.Start(); err != nil {
		return trace.Wrap(err)
	}
	logger.InfoContext(process.ExitContext(), "Discovery service has successfully started")

	// The Discovery service doesn't have heartbeats so we cannot use them to check health.
	// For now, we just mark ourselves ready all the time on startup.
	// If we don't, a process only running the Discovery service will never report ready.
	process.OnHeartbeat(teleport.ComponentDiscovery)(nil)

	if err := discoveryService.Wait(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// integrationOnlyCredentials indicates whether the DiscoveryService must only use Cloud APIs credentials using an integration.
//
// If Auth is running alongside this DiscoveryService and License is Cloud, then this process is running in Teleport's Cloud Infra.
// In those situations, ambient credentials (used by the AWS SDK) will provide access to the tenant's infra, which is not desired.
// Setting IntegrationOnlyCredentials to true, will prevent usage of the ambient credentials.
func (process *TeleportProcess) integrationOnlyCredentials() bool {
	return process.Config.Auth.Enabled && modules.GetModules().Features().Cloud
}

// buildAccessGraphFromTAGOrFallbackToAuth builds the AccessGraphConfig from the Teleport Agent configuration or falls back to the Auth server's configuration.
// If the AccessGraph configuration is not enabled locally, it will fall back to the Auth server's configuration.
func buildAccessGraphFromTAGOrFallbackToAuth(ctx context.Context, config *servicecfg.Config, client authclient.ClientI, logger *slog.Logger) (discovery.AccessGraphConfig, error) {
	var (
		accessGraphCAData []byte
		err               error
	)
	if config == nil {
		return discovery.AccessGraphConfig{}, trace.BadParameter("config is nil")
	}
	if config.AccessGraph.CA != "" {
		accessGraphCAData, err = os.ReadFile(config.AccessGraph.CA)
		if err != nil {
			return discovery.AccessGraphConfig{}, trace.Wrap(err, "failed to read access graph CA file")
		}
	}
	accessGraphCfg := discovery.AccessGraphConfig{
		Enabled:  config.AccessGraph.Enabled,
		Addr:     config.AccessGraph.Addr,
		Insecure: config.AccessGraph.Insecure,
		CA:       accessGraphCAData,
	}
	if !accessGraphCfg.Enabled {
		logger.DebugContext(ctx, "Access graph is disabled or not configured. Falling back to the Auth server's access graph configuration.")
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		rsp, err := client.GetClusterAccessGraphConfig(ctx)
		cancel()
		switch {
		case trace.IsNotImplemented(err):
			logger.DebugContext(ctx, "Auth server does not support access graph's GetClusterAccessGraphConfig RPC")
		case err != nil:
			return discovery.AccessGraphConfig{}, trace.Wrap(err)
		default:
			accessGraphCfg.Enabled = rsp.Enabled
			accessGraphCfg.Addr = rsp.Address
			accessGraphCfg.CA = rsp.Ca
			accessGraphCfg.Insecure = rsp.Insecure
		}
	}
	return accessGraphCfg, nil
}
