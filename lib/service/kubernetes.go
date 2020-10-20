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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/defaults"
	kubeproxy "github.com/gravitational/teleport/lib/kube/proxy"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func (process *TeleportProcess) initKubernetes() {
	log := logrus.WithFields(logrus.Fields{
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
			log.Debugf("Process is exiting.")
			return nil
		}

		conn, ok := (event.Payload).(*Connector)
		if !ok {
			return trace.BadParameter("unsupported connector type: %T", event.Payload)
		}

		err := process.initKubernetesService(log, conn)
		if err != nil {
			warnOnErr(conn.Close())
			return trace.Wrap(err)
		}
		return nil
	})
}

func (process *TeleportProcess) initKubernetesService(log *logrus.Entry, conn *Connector) error {
	// clean up unused descriptors passed for proxy, but not used by it
	defer func() {
		if err := process.closeImportedDescriptors(teleport.ComponentKube); err != nil {
			log.Warnf("Failed closing imported file descriptors: %v", err)
		}
	}()
	cfg := process.Config

	// Create a caching auth client.
	accessPoint, err := process.newLocalCache(conn.Client, cache.ForKubernetes, []string{teleport.ComponentKube})
	if err != nil {
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
		return trace.BadParameter("set kubernetes_service.listen_addr if this process can be reached from a teleport proxy or point teleport.auth_servers to a proxy to dial out")

	// Start a local listener and let proxies dial in.
	case !conn.UseTunnel() && !cfg.Kube.ListenAddr.IsEmpty():
		log.Debugf("Turning on Kubernetes service listening address.")
		listener, err = process.importOrCreateListener(listenerKube, cfg.Kube.ListenAddr.Addr)
		if err != nil {
			return trace.Wrap(err)
		}

	// Dialed out to a proxy, start servicing the reverse tunnel as a listener.
	case conn.UseTunnel() && cfg.Kube.ListenAddr.IsEmpty():
		// create an adapter, from reversetunnel.ServerHandler to net.Listener.
		shtl := reversetunnel.NewServerHandlerToListener()
		listener = shtl
		agentPool, err = reversetunnel.NewAgentPool(
			process.ExitContext(),
			reversetunnel.AgentPoolConfig{
				Component:   teleport.ComponentKube,
				HostUUID:    conn.ServerIdentity.ID.HostUUID,
				ProxyAddr:   conn.TunnelProxy(),
				Client:      conn.Client,
				AccessPoint: conn.Client,
				HostSigner:  conn.ServerIdentity.KeySigner,
				Cluster:     conn.ServerIdentity.Cert.Extensions[utils.CertExtensionAuthority],
				Server:      shtl,
			})
		if err != nil {
			return trace.Wrap(err)
		}
		err = agentPool.Start()
		if err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Started reverse tunnel client.")
	}

	// Create the kube server to service listener.
	authorizer, err := auth.NewAuthorizer(conn.Client, conn.Client, conn.Client)
	if err != nil {
		return trace.Wrap(err)
	}
	tlsConfig, err := conn.ServerIdentity.TLSConfig(cfg.CipherSuites)
	if err != nil {
		return trace.Wrap(err)
	}
	kubeServer, err := kubeproxy.NewTLSServer(kubeproxy.TLSServerConfig{
		ForwarderConfig: kubeproxy.ForwarderConfig{
			Namespace:      defaults.Namespace,
			Keygen:         cfg.Keygen,
			ClusterName:    conn.ServerIdentity.Cert.Extensions[utils.CertExtensionAuthority],
			Auth:           authorizer,
			Client:         conn.Client,
			DataDir:        cfg.DataDir,
			AccessPoint:    accessPoint,
			ServerID:       cfg.HostUUID,
			KubeconfigPath: cfg.Kube.KubeconfigPath,
		},
		TLS:           tlsConfig,
		AccessPoint:   accessPoint,
		Component:     teleport.ComponentKube,
		LimiterConfig: cfg.Kube.Limiter,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	process.RegisterCriticalFunc("kube.serve", func() error {
		if conn.UseTunnel() {
			log.Info("Starting Kube service via proxy reverse tunnel.")
			utils.Consolef(cfg.Console, teleport.ComponentKube, "Kubernetes service %s:%s is starting via proxy reverse tunnel.", teleport.Version, teleport.Gitref)
		} else {
			log.Infof("Starting Kube service on %v.", listener.Addr())
			utils.Consolef(cfg.Console, teleport.ComponentKube, "Kubernetes service %s:%s is starting on %v.", teleport.Version, teleport.Gitref, listener.Addr())
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

	// Start the heartbeat to announce kubernetes_service presence.
	heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
		Mode:            srv.HeartbeatModeKube,
		Context:         process.ExitContext(),
		Component:       teleport.ComponentKube,
		Announcer:       conn.Client,
		GetServerInfo:   kubeServer.GetServerInfo,
		KeepAlivePeriod: defaults.ServerKeepAliveTTL,
		AnnouncePeriod:  defaults.ServerAnnounceTTL/2 + utils.RandomDuration(defaults.ServerAnnounceTTL/10),
		ServerTTL:       defaults.ServerAnnounceTTL,
		CheckPeriod:     defaults.HeartbeatCheckPeriod,
		Clock:           cfg.Clock,
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
	process.RegisterCriticalFunc("kube.heartbeat", heartbeat.Run)

	// Cleanup, when process is exiting.
	process.onExit("kube.shutdown", func(payload interface{}) {
		// Clean up items in reverse order from their initialization.
		warnOnErr(heartbeat.Close())
		if payload != nil {
			// Graceful shutdown.
			warnOnErr(kubeServer.Shutdown(payloadContext(payload)))
			if agentPool != nil {
				agentPool.Stop()
				agentPool.Wait()
			}
		} else {
			// Fast shutdown.
			warnOnErr(kubeServer.Close())
			if agentPool != nil {
				agentPool.Stop()
			}
		}
		warnOnErr(listener.Close())
		warnOnErr(conn.Close())

		log.Infof("Exited.")
	})
	return nil
}
