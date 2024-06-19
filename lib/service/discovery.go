/*
Copyright 2022 Gravitational, Inc.

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

package service

import (
	"context"
	"os"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

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
	log := process.log.WithField(trace.Component, teleport.Component(
		teleport.ComponentDiscovery, process.id))

	conn, err := process.WaitForConnector(DiscoveryIdentityEvent, log)
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
	// tlsConfig is the DiscoveryService's TLS certificate signed by the cluster's
	// Host certificate authority.
	// It is used to authenticate the DiscoveryService to the Access Graph service.
	tlsConfig, err := conn.ServerIdentity.TLSConfig(process.Config.CipherSuites)
	if err != nil {
		return trace.Wrap(err)
	}

	if tlsConfig != nil {
		tlsConfig.ServerName = "" /* empty the server name to avoid SNI collisions with access graph addr */
	}

	accessGraphCfg, err := buildAccessGraphFromTAGOrFallbackToAuth(
		process.ExitContext(),
		process.Config,
		conn.Client,
		log,
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
		Log:               process.log,
		ClusterName:       conn.ClientIdentity.ClusterName,
		ClusterFeatures:   process.GetClusterFeatures,
		PollInterval:      process.Config.Discovery.PollInterval,
		ServerCredentials: tlsConfig,
		AccessGraphConfig: accessGraphCfg,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	process.OnExit("discovery.stop", func(payload interface{}) {
		log.Info("Shutting down.")
		if discoveryService != nil {
			discoveryService.Stop()
		}
		if asyncEmitter != nil {
			warnOnErr(asyncEmitter.Close(), process.log)
		}
		warnOnErr(conn.Close(), log)
		log.Info("Exited.")
	})

	process.BroadcastEvent(Event{Name: DiscoveryReady, Payload: nil})

	if err := discoveryService.Start(); err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Discovery service has successfully started")

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
func buildAccessGraphFromTAGOrFallbackToAuth(ctx context.Context, config *servicecfg.Config, client authclient.ClientI, logger logrus.FieldLogger) (discovery.AccessGraphConfig, error) {
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
		logger.Debug("Access graph is disabled or not configured. Falling back to the Auth server's access graph configuration.")
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		rsp, err := client.GetClusterAccessGraphConfig(ctx)
		cancel()
		switch {
		case trace.IsNotImplemented(err):
			logger.Debug("Auth server does not support access graph's GetClusterAccessGraphConfig RPC")
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
