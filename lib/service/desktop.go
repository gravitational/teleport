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

package service

import (
	"crypto/tls"
	"net"
	"net/http"
	"strconv"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/desktop"
	"github.com/gravitational/teleport/lib/utils"
)

func (process *TeleportProcess) initWindowsDesktopService() {
	log := process.log.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentWindowsDesktop, process.id),
	})
	process.registerWithAuthServer(types.RoleWindowsDesktop, WindowsDesktopIdentityEvent)
	process.RegisterCriticalFunc("windows_desktop.init", func() error {
		eventsC := make(chan Event)
		process.WaitForEvent(process.ExitContext(), WindowsDesktopIdentityEvent, eventsC)

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

		err := process.initWindowsDesktopServiceRegistered(log, conn)
		if err != nil {
			warnOnErr(conn.Close(), log)
			return trace.Wrap(err)
		}
		return nil
	})
}

func (process *TeleportProcess) initWindowsDesktopServiceRegistered(log *logrus.Entry, conn *Connector) (retErr error) {
	defer func() {
		if err := process.closeImportedDescriptors(teleport.ComponentWindowsDesktop); err != nil {
			log.WithError(err).Warn("Failed closing imported file descriptors.")
		}
	}()
	cfg := process.Config

	// Create a caching auth client.
	accessPoint, err := process.newLocalCacheForWindowsDesktop(conn.Client, []string{teleport.ComponentWindowsDesktop})
	if err != nil {
		return trace.Wrap(err)
	}

	useTunnel := conn.UseTunnel()
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
	case useTunnel && !cfg.WindowsDesktop.ListenAddr.IsEmpty():
		return trace.BadParameter("either set windows_desktop_service.listen_addr if this process can be reached from a teleport proxy or point teleport.auth_servers to a proxy to dial out, but don't set both")
	case !useTunnel && cfg.WindowsDesktop.ListenAddr.IsEmpty():
		return trace.BadParameter("set windows_desktop_service.listen_addr if this process can be reached from a teleport proxy or point teleport.auth_servers to a proxy to dial out")

	// Start a local listener and let proxies dial in.
	case !useTunnel && !cfg.WindowsDesktop.ListenAddr.IsEmpty():
		log.Info("Using local listener and registering directly with auth server")
		listener, err = process.importOrCreateListener(listenerWindowsDesktop, cfg.WindowsDesktop.ListenAddr.Addr)
		if err != nil {
			return trace.Wrap(err)
		}
		defer func() {
			if retErr != nil {
				warnOnErr(listener.Close(), log)
			}
		}()

	// Dialed out to a proxy, start servicing the reverse tunnel as a listener.
	case useTunnel && cfg.WindowsDesktop.ListenAddr.IsEmpty():
		// create an adapter, from reversetunnel.ServerHandler to net.Listener.
		shtl := reversetunnel.NewServerHandlerToListener(reversetunnel.LocalWindowsDesktop)
		listener = shtl
		agentPool, err = reversetunnel.NewAgentPool(
			process.ExitContext(),
			reversetunnel.AgentPoolConfig{
				Component:   teleport.ComponentWindowsDesktop,
				HostUUID:    conn.ServerIdentity.ID.HostUUID,
				Resolver:    conn.TunnelProxyResolver(),
				Client:      conn.Client,
				AccessPoint: accessPoint,
				HostSigner:  conn.ServerIdentity.KeySigner,
				Cluster:     conn.ServerIdentity.Cert.Extensions[utils.CertExtensionAuthority],
				Server:      shtl,
				FIPS:        process.Config.FIPS,
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
		log.Info("Using a reverse tunnel to register and handle proxy connections")
	}

	lockWatcher, err := services.NewLockWatcher(process.ExitContext(), services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentWindowsDesktop,
			Log:       log,
			Clock:     cfg.Clock,
			Client:    conn.Client,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	clusterName := conn.ServerIdentity.Cert.Extensions[utils.CertExtensionAuthority]

	authorizer, err := auth.NewAuthorizer(clusterName, accessPoint, lockWatcher)
	if err != nil {
		return trace.Wrap(err)
	}

	tlsConfig, err := conn.ServerIdentity.TLSConfig(cfg.CipherSuites)
	if err != nil {
		return trace.Wrap(err)
	}
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	// Populate the correct CAs for the incoming client connection.
	tlsConfig.GetConfigForClient = func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		var clusterName string
		var err error
		if info.ServerName != "" {
			clusterName, err = apiutils.DecodeClusterName(info.ServerName)
			if err != nil && !trace.IsNotFound(err) {
				log.Debugf("Ignoring unsupported cluster name %q.", info.ServerName)
			}
		}
		pool, err := auth.ClientCertPool(accessPoint, clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tlsCopy := tlsConfig.Clone()
		tlsCopy.ClientCAs = pool
		return tlsCopy, nil
	}

	connLimiter, err := limiter.NewConnectionsLimiter(cfg.WindowsDesktop.ConnLimiter)
	if err != nil {
		return trace.Wrap(err)
	}
	var publicAddr string
	switch {
	case useTunnel:
		publicAddr = listener.Addr().String()
	case len(cfg.WindowsDesktop.PublicAddrs) > 0:
		publicAddr = cfg.WindowsDesktop.PublicAddrs[0].String()
	case cfg.Hostname != "":
		publicAddr = net.JoinHostPort(cfg.Hostname, strconv.Itoa(cfg.WindowsDesktop.ListenAddr.Port(defaults.WindowsDesktopListenPort)))
	default:
		publicAddr = listener.Addr().String()
	}

	srv, err := desktop.NewWindowsService(desktop.WindowsServiceConfig{
		Log:          log,
		Clock:        process.Clock,
		Authorizer:   authorizer,
		Emitter:      conn.Client,
		TLS:          tlsConfig,
		AccessPoint:  accessPoint,
		ConnLimiter:  connLimiter,
		LockWatcher:  lockWatcher,
		AuthClient:   conn.Client,
		HostLabelsFn: cfg.WindowsDesktop.HostLabels.LabelsForHost,
		Heartbeat: desktop.HeartbeatConfig{
			HostUUID:    cfg.HostUUID,
			PublicAddr:  publicAddr,
			StaticHosts: cfg.WindowsDesktop.Hosts,
			OnHeartbeat: process.onHeartbeat(teleport.ComponentWindowsDesktop),
		},
		LDAPConfig:           desktop.LDAPConfig(cfg.WindowsDesktop.LDAP),
		DiscoveryBaseDN:      cfg.WindowsDesktop.Discovery.BaseDN,
		DiscoveryLDAPFilters: cfg.WindowsDesktop.Discovery.Filters,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if retErr != nil {
			warnOnErr(srv.Close(), log)
		}
	}()
	process.RegisterCriticalFunc("windows_desktop.serve", func() error {
		if useTunnel {
			log.Info("Starting Windows desktop service via proxy reverse tunnel.")
			utils.Consolef(cfg.Console, log, teleport.ComponentWindowsDesktop,
				"Windows desktop service %s:%s is starting via proxy reverse tunnel.",
				teleport.Version, teleport.Gitref)
		} else {
			log.Infof("Starting Windows desktop service on %v.", listener.Addr())
			utils.Consolef(cfg.Console, log, teleport.ComponentWindowsDesktop,
				"Windows desktop service %s:%s is starting on %v.",
				teleport.Version, teleport.Gitref, listener.Addr())
		}
		err := srv.Serve(listener)
		if err != nil {
			if err == http.ErrServerClosed {
				return nil
			}
			return trace.Wrap(err)
		}
		return nil
	})

	// Cleanup, when process is exiting.
	process.OnExit("windows_desktop.shutdown", func(payload interface{}) {
		// Fast shutdown.
		warnOnErr(srv.Close(), log)
		agentPool.Stop()
		if payload != nil {
			// Graceful shutdown.
			agentPool.Wait()
		}
		warnOnErr(listener.Close(), log)
		warnOnErr(conn.Close(), log)

		log.Info("Exited.")
	})
	return nil
}
