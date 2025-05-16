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
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"

	"github.com/gravitational/trace"

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
	logger := process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentWindowsDesktop, process.id))
	process.RegisterWithAuthServer(types.RoleWindowsDesktop, WindowsDesktopIdentityEvent)
	process.RegisterCriticalFunc("windows_desktop.init", func() error {
		conn, err := process.WaitForConnector(WindowsDesktopIdentityEvent, logger)
		if conn == nil {
			return trace.Wrap(err)
		}

		if err := process.initWindowsDesktopServiceRegistered(logger, conn); err != nil {
			warnOnErr(process.ExitContext(), conn.Close(), logger)
			return trace.Wrap(err)
		}
		return nil
	})
}

func (process *TeleportProcess) initWindowsDesktopServiceRegistered(logger *slog.Logger, conn *Connector) (retErr error) {
	defer func() {
		if err := process.closeImportedDescriptors(teleport.ComponentWindowsDesktop); err != nil {
			logger.WarnContext(process.ExitContext(), "Failed closing imported file descriptors.")
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
		logger.InfoContext(process.ExitContext(), "Using local listener and registering directly with auth server")
		listener, err = process.importOrCreateListener(ListenerWindowsDesktop, cfg.WindowsDesktop.ListenAddr.Addr)
		if err != nil {
			return trace.Wrap(err)
		}
		defer func() {
			if retErr != nil {
				warnOnErr(process.ExitContext(), listener.Close(), logger)
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
				HostUUID:             conn.HostID(),
				Resolver:             conn.TunnelProxyResolver(),
				Client:               conn.Client,
				AccessPoint:          accessPoint,
				AuthMethods:          conn.ClientAuthMethods(),
				Cluster:              conn.ClusterName(),
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
		logger.InfoContext(process.ExitContext(), "Using a reverse tunnel to register and handle proxy connections")
	}

	lockWatcher, err := services.NewLockWatcher(process.ExitContext(), services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentWindowsDesktop,
			Logger:    process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentWindowsDesktop, process.id)),
			Clock:     cfg.Clock,
			Client:    conn.Client,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	clusterName := conn.ClusterName()

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: clusterName,
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
		Logger:      process.log.WithField(teleport.ComponentKey, teleport.Component(teleport.ComponentWindowsDesktop, process.id)),
		DeviceAuthorization: authz.DeviceAuthorizationOpts{
			// Ignore the global device_trust.mode toggle, but allow role-based
			// settings to be applied.
			DisableGlobalMode: true,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	tlsConfig, err := conn.ServerTLSConfig(cfg.CipherSuites)
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
				logger.DebugContext(process.ExitContext(), "Ignoring unsupported cluster name.", "cluster_name", info.ServerName)
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

	connLimiter := limiter.NewConnectionsLimiter(cfg.WindowsDesktop.ConnLimiter.MaxConnections)

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
		LicenseStore: process.storage,
		Logger:       process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentWindowsDesktop, process.id)),
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
		KDCAddr:                      cfg.WindowsDesktop.KDCAddr,
		PKIDomain:                    cfg.WindowsDesktop.PKIDomain,
		DiscoveryBaseDN:              cfg.WindowsDesktop.Discovery.BaseDN,
		DiscoveryLDAPFilters:         cfg.WindowsDesktop.Discovery.Filters,
		DiscoveryLDAPAttributeLabels: cfg.WindowsDesktop.Discovery.LabelAttributes,
		Hostname:                     cfg.Hostname,
		ConnectedProxyGetter:         proxyGetter,
		ResourceMatchers:             cfg.WindowsDesktop.ResourceMatchers,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if retErr != nil {
			warnOnErr(process.ExitContext(), srv.Close(), logger)
		}
	}()
	process.RegisterCriticalFunc("windows_desktop.serve", func() error {
		if useTunnel {
			logger.InfoContext(process.ExitContext(), "Starting Windows desktop service via proxy reverse tunnel.")
		} else {
			logger.InfoContext(process.ExitContext(), "Starting Windows desktop service.", "listen_address", listener.Addr())
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
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return trace.Wrap(err)
		}
		return nil
	})

	// Cleanup, when process is exiting.
	process.OnExit("windows_desktop.shutdown", func(payload interface{}) {
		// Fast shutdown.
		warnOnErr(process.ExitContext(), srv.Close(), logger)
		agentPool.Stop()
		if payload != nil {
			// Graceful shutdown.
			agentPool.Wait()
		}
		warnOnErr(process.ExitContext(), listener.Close(), logger)
		warnOnErr(process.ExitContext(), conn.Close(), logger)

		logger.InfoContext(process.ExitContext(), "Exited.")
	})
	return nil
}
