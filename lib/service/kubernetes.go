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
	"errors"
	"log/slog"
	"net"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	kubeproxy "github.com/gravitational/teleport/lib/kube/proxy"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
)

func (process *TeleportProcess) initKubernetes() {
	logger := process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentKube, process.id))

	process.RegisterWithAuthServer(types.RoleKube, KubeIdentityEvent)
	process.RegisterCriticalFunc("kube.init", func() error {
		conn, err := process.WaitForConnector(KubeIdentityEvent, logger)
		if conn == nil {
			return trace.Wrap(err)
		}
		features := process.GetClusterFeatures()
		k8s := modules.GetProtoEntitlement(&features, entitlements.K8s)
		if !k8s.Enabled {
			logger.WarnContext(process.ExitContext(), "Warning: Kubernetes service not initialized because Teleport Auth Server is not licensed for Kubernetes Access. Please contact the cluster administrator to enable it.")
			return nil
		}
		if err := process.initKubernetesService(logger, conn); err != nil {
			warnOnErr(process.ExitContext(), conn.Close(), logger)
			return trace.Wrap(err)
		}
		return nil
	})
}

func (process *TeleportProcess) initKubernetesService(logger *slog.Logger, conn *Connector) (retErr error) {
	// clean up unused descriptors passed for proxy, but not used by it
	defer func() {
		if err := process.closeImportedDescriptors(teleport.ComponentKube); err != nil {
			logger.WarnContext(process.ExitContext(), "Failed closing imported file descriptors.", "error", err)
		}
	}()
	cfg := process.Config

	// Create a caching auth client.
	accessPoint, err := process.newLocalCacheForKubernetes(conn.Client, []string{teleport.ComponentKube})
	if err != nil {
		return trace.Wrap(err)
	}

	teleportClusterName := conn.ClusterName()
	proxyGetter := reversetunnel.NewConnectedProxyGetter()

	// This service can run in 2 modes:
	// 1. Reachable (by the proxy) - registers with auth server directly and
	//    creates a local listener to accept proxy conns.
	// 2. Not reachable ("IoT mode") - creates a reverse tunnel to a proxy and
	//    handles registration and incoming connections through that.
	//
	// The listener exposes incoming connections over either mode.
	var listener net.Listener
	var agentPool *reversetunnel.AgentPool
	switch {
	// Filter out cases where both listen_addr and tunnel are set or both are
	// not set.
	case conn.UseTunnel() && !cfg.Kube.ListenAddr.IsEmpty():
		return trace.BadParameter("either set kubernetes_service.listen_addr if this process can be reached from a teleport proxy or point teleport.proxy_server to a proxy to dial out, but don't set both")
	case !conn.UseTunnel() && cfg.Kube.ListenAddr.IsEmpty():
		// TODO(awly): if this process runs auth, proxy and kubernetes
		// services, the proxy should be able to route requests to this
		// kubernetes service. This means either always connecting over a
		// reverse tunnel (with a performance penalty), or somehow passing the
		// connections in-memory between proxy and kubernetes services.
		//
		// For now, as a lazy shortcut, kuberentes_service.listen_addr is
		// always required when running in the same process with a proxy.
		return trace.BadParameter("set kubernetes_service.listen_addr if this process can be reached from a teleport proxy or point teleport.proxy_server to a proxy to dial out")

	// Start a local listener and let proxies dial in.
	case !conn.UseTunnel() && !cfg.Kube.ListenAddr.IsEmpty():
		logger.DebugContext(process.ExitContext(), "Turning on Kubernetes service listening address.")
		listener, err = process.importOrCreateListener(ListenerKube, cfg.Kube.ListenAddr.Addr)
		if err != nil {
			return trace.Wrap(err)
		}
		defer func() {
			if retErr != nil {
				warnOnErr(process.ExitContext(), listener.Close(), logger)
			}
		}()

	// Dialed out to a proxy, start servicing the reverse tunnel as a listener.
	case conn.UseTunnel() && cfg.Kube.ListenAddr.IsEmpty():
		// create an adapter, from reversetunnel.ServerHandler to net.Listener.
		shtl := reversetunnel.NewServerHandlerToListener(reversetunnelclient.LocalKubernetes)
		listener = shtl
		agentPool, err = reversetunnel.NewAgentPool(
			process.ExitContext(),
			reversetunnel.AgentPoolConfig{
				Component:            teleport.ComponentKube,
				HostUUID:             conn.HostID(),
				Resolver:             conn.TunnelProxyResolver(),
				Client:               conn.Client,
				AccessPoint:          accessPoint,
				AuthMethods:          conn.ClientAuthMethods(),
				Cluster:              teleportClusterName,
				Server:               shtl,
				FIPS:                 process.Config.FIPS,
				ConnectedProxyGetter: proxyGetter,
			})
		if err != nil {
			return trace.Wrap(err)
		}
		if err = agentPool.Start(); err != nil {
			return trace.Wrap(err)
		}
		defer func() {
			if retErr != nil {
				agentPool.Stop()
			}
		}()
		logger.InfoContext(process.ExitContext(), "Started reverse tunnel client.")
	}

	var dynLabels *labels.Dynamic
	if len(cfg.Kube.DynamicLabels) != 0 {
		dynLabels, err = labels.NewDynamic(process.ExitContext(), &labels.DynamicConfig{
			Labels: cfg.Kube.DynamicLabels,
			Log:    process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentKube, process.id)),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		dynLabels.Sync()
		go dynLabels.Start()
		defer func() {
			if retErr != nil {
				dynLabels.Close()
			}
		}()
	}

	lockWatcher, err := services.NewLockWatcher(process.ExitContext(), services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentKube,
			Logger:    process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentKube, process.id)),
			Client:    conn.Client,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Create the kube server to service listener.
	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: teleportClusterName,
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
		Logger:      process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentKube, process.id)),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	tlsConfig, err := conn.ServerTLSConfig(cfg.CipherSuites)
	if err != nil {
		return trace.Wrap(err)
	}

	// asyncEmitter makes sure that sessions do not block
	// in case if connections are slow
	asyncEmitter, err := process.NewAsyncEmitter(conn.Client)
	if err != nil {
		return trace.Wrap(err)
	}

	var publicAddr string
	if len(cfg.Kube.PublicAddrs) > 0 {
		publicAddr = cfg.Kube.PublicAddrs[0].String()
	}

	kubeServer, err := kubeproxy.NewTLSServer(kubeproxy.TLSServerConfig{
		ForwarderConfig: kubeproxy.ForwarderConfig{
			Namespace:         apidefaults.Namespace,
			Keygen:            cfg.Keygen,
			ClusterName:       teleportClusterName,
			Authz:             authorizer,
			AuthClient:        conn.Client,
			Emitter:           asyncEmitter,
			DataDir:           cfg.DataDir,
			CachingAuthClient: accessPoint,
			HostID:            cfg.HostUUID,
			Context:           process.ExitContext(),
			KubeconfigPath:    cfg.Kube.KubeconfigPath,
			KubeClusterName:   cfg.Kube.KubeClusterName,
			KubeServiceType:   kubeproxy.KubeService,
			Component:         teleport.ComponentKube,

			LockWatcher:                   lockWatcher,
			CheckImpersonationPermissions: cfg.Kube.CheckImpersonationPermissions,
			PublicAddr:                    publicAddr,
			ClusterFeatures:               process.GetClusterFeatures,
		},
		TLS:                  tlsConfig,
		AccessPoint:          accessPoint,
		LimiterConfig:        cfg.Kube.Limiter,
		OnHeartbeat:          process.OnHeartbeat(teleport.ComponentKube),
		GetRotation:          process.GetRotation,
		ConnectedProxyGetter: proxyGetter,
		ResourceMatchers:     cfg.Kube.ResourceMatchers,
		StaticLabels:         cfg.Kube.StaticLabels,
		DynamicLabels:        dynLabels,
		CloudLabels:          process.cloudLabels,
		Log:                  process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentKube, process.id)),
		PROXYProtocolMode:    multiplexer.PROXYProtocolOff, // Kube service doesn't need to process unsigned PROXY headers.
		InventoryHandle:      process.inventoryHandle,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if retErr != nil {
			warnOnErr(process.ExitContext(), kubeServer.Close(), logger)
		}
	}()
	process.RegisterCriticalFunc("kube.serve", func() error {
		if conn.UseTunnel() {
			logger.InfoContext(process.ExitContext(), "Starting Kube service via proxy reverse tunnel.")
		} else {
			logger.InfoContext(process.ExitContext(), "Starting Kube service.", "listen_address", listener.Addr())
		}
		process.BroadcastEvent(Event{Name: KubernetesReady, Payload: nil})
		err := kubeServer.Serve(listener)
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return trace.Wrap(err)
		}
		return nil
	})

	// Cleanup, when process is exiting.
	process.OnExit("kube.shutdown", func(payload interface{}) {
		// Clean up items in reverse order from their initialization.
		if payload != nil {
			// Graceful shutdown.
			warnOnErr(process.ExitContext(), kubeServer.Shutdown(payloadContext(payload)), logger)
			agentPool.Stop()
			agentPool.Wait()
		} else {
			// Fast shutdown.
			warnOnErr(process.ExitContext(), kubeServer.Close(), logger)
			agentPool.Stop()
		}
		if asyncEmitter != nil {
			warnOnErr(process.ExitContext(), asyncEmitter.Close(), logger)
		}
		warnOnErr(process.ExitContext(), listener.Close(), logger)
		warnOnErr(process.ExitContext(), conn.Close(), logger)

		if dynLabels != nil {
			dynLabels.Close()
		}

		logger.InfoContext(process.ExitContext(), "Exited.")
	})
	return nil
}
