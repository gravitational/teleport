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
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/windows"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/desktop"
	"github.com/gravitational/teleport/lib/utils"
)

func (process *TeleportProcess) initWindowsDesktopService() {
	log := process.log.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentWindowsDesktop, process.id),
	})
	process.RegisterWithAuthServer(types.RoleWindowsDesktop, WindowsDesktopIdentityEvent)
	process.RegisterCriticalFunc("windows_desktop.init", func() error {
		conn, err := process.WaitForConnector(WindowsDesktopIdentityEvent, log)
		if conn == nil {
			return trace.Wrap(err)
		}

		if err := process.initWindowsDesktopServiceRegistered(log, conn); err != nil {
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

	proxyGetter := reversetunnel.NewConnectedProxyGetter()

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
		return trace.BadParameter("either set windows_desktop_service.listen_addr if this process can be reached from a teleport proxy or point teleport.proxy_server to a proxy to dial out, but don't set both")
	case !useTunnel && cfg.WindowsDesktop.ListenAddr.IsEmpty():
		return trace.BadParameter("set windows_desktop_service.listen_addr if this process can be reached from a teleport proxy or point teleport.proxy_server to a proxy to dial out")

	// Start a local listener and let proxies dial in.
	case !useTunnel && !cfg.WindowsDesktop.ListenAddr.IsEmpty():
		log.Info("Using local listener and registering directly with auth server")
		listener, err = process.importOrCreateListener(ListenerWindowsDesktop, cfg.WindowsDesktop.ListenAddr.Addr)
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
		shtl := reversetunnel.NewServerHandlerToListener(reversetunnelclient.LocalWindowsDesktop)
		listener = shtl
		agentPool, err = reversetunnel.NewAgentPool(
			process.ExitContext(),
			reversetunnel.AgentPoolConfig{
				Component:            teleport.ComponentWindowsDesktop,
				HostUUID:             conn.ServerIdentity.ID.HostUUID,
				Resolver:             conn.TunnelProxyResolver(),
				Client:               conn.Client,
				AccessPoint:          accessPoint,
				HostSigner:           conn.ServerIdentity.KeySigner,
				Cluster:              conn.ServerIdentity.Cert.Extensions[utils.CertExtensionAuthority],
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

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: clusterName,
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
		Logger:      log,
		// Device authorization breaks browser-based access.
		DisableDeviceAuthorization: true,
	})
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
		pool, _, _, err := authclient.DefaultClientCertPool(info.Context(), accessPoint, clusterName)
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
		DataDir:      process.Config.DataDir,
		Log:          log,
		Clock:        process.Clock,
		Authorizer:   authorizer,
		Emitter:      conn.Client,
		TLS:          tlsConfig,
		AccessPoint:  accessPoint,
		ConnLimiter:  connLimiter,
		LockWatcher:  lockWatcher,
		AuthClient:   conn.Client,
		Labels:       cfg.WindowsDesktop.Labels,
		HostLabelsFn: cfg.WindowsDesktop.HostLabels.LabelsForHost,
		Heartbeat: desktop.HeartbeatConfig{
			HostUUID:    cfg.HostUUID,
			PublicAddr:  publicAddr,
			StaticHosts: cfg.WindowsDesktop.StaticHosts,
			OnHeartbeat: process.OnHeartbeat(teleport.ComponentWindowsDesktop),
		},
		ShowDesktopWallpaper:         cfg.WindowsDesktop.ShowDesktopWallpaper,
		LDAPConfig:                   windows.LDAPConfig(cfg.WindowsDesktop.LDAP),
		PKIDomain:                    cfg.WindowsDesktop.PKIDomain,
		DiscoveryBaseDN:              cfg.WindowsDesktop.Discovery.BaseDN,
		DiscoveryLDAPFilters:         cfg.WindowsDesktop.Discovery.Filters,
		DiscoveryLDAPAttributeLabels: cfg.WindowsDesktop.Discovery.LabelAttributes,
		Hostname:                     cfg.Hostname,
		ConnectedProxyGetter:         proxyGetter,
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
		} else {
			log.Infof("Starting Windows desktop service on %v.", listener.Addr())
		}
		process.BroadcastEvent(Event{Name: WindowsDesktopReady, Payload: nil})

		mux, err := multiplexer.New(multiplexer.Config{
			Context:             process.ExitContext(),
			Listener:            listener,
			PROXYProtocolMode:   multiplexer.PROXYProtocolOff, // Desktop service never should process unsigned PROXY headers.
			ID:                  teleport.Component(teleport.ComponentWindowsDesktop),
			CertAuthorityGetter: accessPoint.GetCertAuthority,
			LocalClusterName:    clusterName,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		go func() {
			if err := mux.Serve(); err != nil && !utils.IsOKNetworkError(err) {
				mux.Entry.WithError(err).Error("mux encountered err serving")
			}
		}()

		err = srv.Serve(mux.TLS())
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
