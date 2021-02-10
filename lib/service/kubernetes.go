/*
Copyright 2020 Gravitational, Inc.

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
	"net"
	"net/http"

	"github.com/gravitational/teleport"
	auth "github.com/gravitational/teleport/lib/auth/server"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	kubeproxy "github.com/gravitational/teleport/lib/kube/proxy"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func (process *TeleportProcess) initKubernetes() {
	log := process.log.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentKube, process.id),
	})

	process.registerWithAuthServer(teleport.RoleKube, KubeIdentityEvent)
	process.RegisterCriticalFunc("kube.init", func() error {
		eventsC := make(chan Event)
		process.WaitForEvent(process.ExitContext(), KubeIdentityEvent, eventsC)

		var event Event
		select {
		case event = <-eventsC:
			log.Debugf("Received event %q.", event.Name)
		case <-process.ExitContext().Done():
			log.Debug("Process is exiting.")
			return nil
		}

		conn, ok := (event.Payload).(*Connector)
		if !ok {
			return trace.BadParameter("unsupported connector type: %T", event.Payload)
		}

		err := process.initKubernetesService(log, conn)
		if err != nil {
			warnOnErr(conn.Close(), log)
			return trace.Wrap(err)
		}
		return nil
	})
}

func (process *TeleportProcess) initKubernetesService(log *logrus.Entry, conn *Connector) (retErr error) {
	// clean up unused descriptors passed for proxy, but not used by it
	defer func() {
		if err := process.closeImportedDescriptors(teleport.ComponentKube); err != nil {
			log.WithError(err).Warn("Failed closing imported file descriptors.")
		}
	}()
	cfg := process.Config

	// Create a caching auth client.
	accessPoint, err := process.newLocalCache(conn.Client, cache.ForKubernetes, []string{teleport.ComponentKube})
	if err != nil {
		return trace.Wrap(err)
	}

	// Start uploader that will scan a path on disk and upload completed
	// sessions to the Auth Server.
	if err := process.initUploaderService(accessPoint, conn.Client); err != nil {
		return trace.Wrap(err)
	}

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
		return trace.BadParameter("either set kubernetes_service.listen_addr if this process can be reached from a teleport proxy or point teleport.auth_servers to a proxy to dial out, but don't set both")
	case !conn.UseTunnel() && cfg.Kube.ListenAddr.IsEmpty():
		// TODO(awly): if this process runs auth, proxy and kubernetes
		// services, the proxy should be able to route requests to this
		// kubernetes service. This means either always connecting over a
		// reverse tunnel (with a performance penalty), or somehow passing the
		// connections in-memory between proxy and kubernetes services.
		//
		// For now, as a lazy shortcut, kuberentes_service.listen_addr is
		// always required when running in the same process with a proxy.
		return trace.BadParameter("set kubernetes_service.listen_addr if this process can be reached from a teleport proxy or point teleport.auth_servers to a proxy to dial out")

	// Start a local listener and let proxies dial in.
	case !conn.UseTunnel() && !cfg.Kube.ListenAddr.IsEmpty():
		log.Debug("Turning on Kubernetes service listening address.")
		listener, err = process.importOrCreateListener(listenerKube, cfg.Kube.ListenAddr.Addr)
		if err != nil {
			return trace.Wrap(err)
		}
		defer func() {
			if retErr != nil {
				warnOnErr(listener.Close(), log)
			}
		}()

	// Dialed out to a proxy, start servicing the reverse tunnel as a listener.
	case conn.UseTunnel() && cfg.Kube.ListenAddr.IsEmpty():
		// create an adapter, from reversetunnel.ServerHandler to net.Listener.
		shtl := reversetunnel.NewServerHandlerToListener(reversetunnel.LocalKubernetes)
		listener = shtl
		agentPool, err = reversetunnel.NewAgentPool(
			process.ExitContext(),
			reversetunnel.AgentPoolConfig{
				Component:   teleport.ComponentKube,
				HostUUID:    conn.ServerIdentity.ID.HostUUID,
				ProxyAddr:   conn.TunnelProxy(),
				Client:      conn.Client,
				AccessPoint: accessPoint,
				HostSigner:  conn.ServerIdentity.KeySigner,
				Cluster:     conn.ServerIdentity.Cert.Extensions[utils.CertExtensionAuthority],
				Server:      shtl,
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
		log.Info("Started reverse tunnel client.")
	}

	var dynLabels *labels.Dynamic
	if len(cfg.Kube.DynamicLabels) != 0 {
		dynLabels, err = labels.NewDynamic(process.ExitContext(), &labels.DynamicConfig{
			Labels: cfg.Kube.DynamicLabels,
			Log:    log,
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

	teleportClusterName := conn.ServerIdentity.Cert.Extensions[utils.CertExtensionAuthority]

	// Create the kube server to service listener.
	authorizer, err := auth.NewAuthorizer(teleportClusterName, conn.Client, conn.Client, conn.Client)
	if err != nil {
		return trace.Wrap(err)
	}
	tlsConfig, err := conn.ServerIdentity.TLSConfig(cfg.CipherSuites)
	if err != nil {
		return trace.Wrap(err)
	}

	// asyncEmitter makes sure that sessions do not block
	// in case if connections are slow
	asyncEmitter, err := process.newAsyncEmitter(conn.Client)
	if err != nil {
		return trace.Wrap(err)
	}
	streamer, err := events.NewCheckingStreamer(events.CheckingStreamerConfig{
		Inner:       conn.Client,
		Clock:       process.Clock,
		ClusterName: teleportClusterName,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	streamEmitter := &events.StreamerAndEmitter{
		Emitter:  asyncEmitter,
		Streamer: streamer,
	}

	kubeServer, err := kubeproxy.NewTLSServer(kubeproxy.TLSServerConfig{
		ForwarderConfig: kubeproxy.ForwarderConfig{
			Namespace:         defaults.Namespace,
			Keygen:            cfg.Keygen,
			ClusterName:       teleportClusterName,
			Authz:             authorizer,
			AuthClient:        conn.Client,
			StreamEmitter:     streamEmitter,
			DataDir:           cfg.DataDir,
			CachingAuthClient: accessPoint,
			ServerID:          cfg.HostUUID,
			Context:           process.ExitContext(),
			KubeconfigPath:    cfg.Kube.KubeconfigPath,
			KubeClusterName:   cfg.Kube.KubeClusterName,
			NewKubeService:    true,
			Component:         teleport.ComponentKube,
			StaticLabels:      cfg.Kube.StaticLabels,
			DynamicLabels:     dynLabels,
		},
		TLS:           tlsConfig,
		AccessPoint:   accessPoint,
		LimiterConfig: cfg.Kube.Limiter,
		OnHeartbeat: func(err error) {
			if err != nil {
				process.BroadcastEvent(Event{Name: TeleportDegradedEvent, Payload: teleport.ComponentKube})
			} else {
				process.BroadcastEvent(Event{Name: TeleportOKEvent, Payload: teleport.ComponentKube})
			}
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if retErr != nil {
			warnOnErr(kubeServer.Close(), log)
		}
	}()
	process.RegisterCriticalFunc("kube.serve", func() error {
		if conn.UseTunnel() {
			log.Info("Starting Kube service via proxy reverse tunnel.")
			utils.Consolef(cfg.Console, log, teleport.ComponentKube,
				"Kubernetes service %s:%s is starting via proxy reverse tunnel.",
				teleport.Version, teleport.Gitref)
		} else {
			log.Infof("Starting Kube service on %v.", listener.Addr())
			utils.Consolef(cfg.Console, log, teleport.ComponentKube,
				"Kubernetes service %s:%s is starting on %v.",
				teleport.Version, teleport.Gitref, listener.Addr())
		}
		err := kubeServer.Serve(listener)
		if err != nil {
			if err == http.ErrServerClosed {
				return nil
			}
			return trace.Wrap(err)
		}
		return nil
	})

	// Cleanup, when process is exiting.
	process.OnExit("kube.shutdown", func(payload interface{}) {
		if asyncEmitter != nil {
			warnOnErr(asyncEmitter.Close(), log)
		}
		// Clean up items in reverse order from their initialization.
		if payload != nil {
			// Graceful shutdown.
			warnOnErr(kubeServer.Shutdown(payloadContext(payload, log)), log)
			agentPool.Stop()
			agentPool.Wait()
		} else {
			// Fast shutdown.
			warnOnErr(kubeServer.Close(), log)
			agentPool.Stop()
		}
		warnOnErr(listener.Close(), log)
		warnOnErr(conn.Close(), log)

		if dynLabels != nil {
			dynLabels.Close()
		}

		log.Info("Exited.")
	})
	return nil
}
