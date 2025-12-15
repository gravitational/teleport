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

package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
)

// localProxyApp is a generic app that can start local proxies.
type localProxyApp struct {
	tc *client.TeleportClient
	// profile is a cached profile status for the current login session.
	profile *client.ProfileStatus
	// routeToApp is a route to the app without TargetPort being set.
	routeToApp proto.RouteToApp
	// app is available only when starting a localProxyApp through newLocalProxyAppWithPortMapping.
	app         types.Application
	insecure    bool
	portMapping client.PortMapping

	localALPNProxy    *alpnproxy.LocalProxy
	localForwardProxy *alpnproxy.ForwardProxy
}

type requestMatcher func(req *http.Request) bool

// newLocalProxyApp creates a new generic app proxy.
func newLocalProxyApp(tc *client.TeleportClient, profile *client.ProfileStatus, routeToApp proto.RouteToApp, rawLocalPort string, insecure bool) (*localProxyApp, error) {
	var portMapping client.PortMapping
	if rawLocalPort != "" {
		localPort, err := strconv.Atoi(rawLocalPort)
		if err != nil {
			return nil, trace.Wrap(err, "parsing port")
		}
		portMapping.LocalPort = localPort
	}

	return &localProxyApp{
		tc:          tc,
		profile:     profile,
		routeToApp:  routeToApp,
		portMapping: portMapping,
		insecure:    insecure,
	}, nil
}

// newLocalProxyAppWithPortMapping creates a new generic app proxy. Unlike newLocalProxyApp, it
// accepts a specific port mapping as an argument.
func newLocalProxyAppWithPortMapping(ctx context.Context, tc *client.TeleportClient, profile *client.ProfileStatus, routeToApp proto.RouteToApp, app types.Application, portMapping client.PortMapping, insecure bool) (*localProxyApp, error) {
	if err := validateTargetPort(app, portMapping.TargetPort); err != nil {
		return nil, trace.Wrap(err)
	}
	return &localProxyApp{
		tc:          tc,
		profile:     profile,
		routeToApp:  routeToApp,
		app:         app,
		portMapping: portMapping,
		insecure:    insecure,
	}, nil
}

// validateTargetPort is used in both tsh proxy app and tsh app login.
func validateTargetPort(app types.Application, targetPort int) error {
	if targetPort == 0 {
		return nil
	}

	tcpPorts := app.GetTCPPorts()
	if len(tcpPorts) == 0 {
		return trace.BadParameter("cannot specify target port %d because app %q does not provide access to multiple ports",
			targetPort, app.GetName())
	}

	if !tcpPorts.Contains(targetPort) {
		return trace.BadParameter("port %d is not included in target ports of app %q; valid ports: %s",
			targetPort, app.GetName(), tcpPorts)
	}

	return nil
}

// StartLocalProxy sets up local proxies for serving app clients.
func (a *localProxyApp) StartLocalProxy(ctx context.Context, opts ...alpnproxy.LocalProxyConfigOpt) error {
	if err := a.startLocalALPNProxy(ctx, a.portMapping, false /*withTLS*/, opts...); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// StartLocalProxy sets up local proxies for serving app clients.
func (a *localProxyApp) StartLocalProxyWithTLS(ctx context.Context, opts ...alpnproxy.LocalProxyConfigOpt) error {
	if err := a.startLocalALPNProxy(ctx, a.portMapping, true /*withTLS*/, opts...); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// StartLocalProxy sets up local proxies for serving app clients.
func (a *localProxyApp) StartLocalProxyWithForwarder(ctx context.Context, forwardMatcher requestMatcher, opts ...alpnproxy.LocalProxyConfigOpt) error {
	if err := a.startLocalALPNProxy(ctx, client.PortMapping{}, true /*withTLS*/, opts...); err != nil {
		return trace.Wrap(err)
	}

	if err := a.startLocalForwardProxy(ctx, a.portMapping.LocalPort, forwardMatcher); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Close makes all necessary close calls.
func (a *localProxyApp) Close() error {
	var errs []error
	if a.localALPNProxy != nil {
		errs = append(errs, a.localALPNProxy.Close())
	}
	if a.localForwardProxy != nil {
		errs = append(errs, a.localForwardProxy.Close())
	}
	return trace.NewAggregate(errs...)
}

// startLocalALPNProxy starts the local ALPN proxy.
func (a *localProxyApp) startLocalALPNProxy(ctx context.Context, portMapping client.PortMapping, withTLS bool, opts ...alpnproxy.LocalProxyConfigOpt) error {
	routeToAppWithTargetPort := a.routeToApp
	if portMapping.TargetPort != 0 {
		routeToAppWithTargetPort.TargetPort = uint32(portMapping.TargetPort)
	}
	// Create an app cert checker to check and reissue app certs for the local app proxy.
	appCertChecker := client.NewAppCertChecker(a.tc, routeToAppWithTargetPort, nil, client.WithTTL(a.tc.KeyTTL))

	// If a stored cert is found for the app, try using it.
	// Otherwise, let the checker reissue one as needed.
	cert, err := loadAppCertificate(a.tc, routeToAppWithTargetPort.Name)
	if err == nil {
		if a.app != nil && len(a.app.GetTCPPorts()) > 0 {
			// There are too many cases to cover when dealing with a multi-port app and an existing cert.
			// We'd need to consider portMapping passed from argv and TargetPort in the cert.
			// As tsh proxy app is not the recommended way to access multi-port apps, let's bail out of
			// using the existing cert and instead always generate a new one.
			fmt.Println("Warning: Ignoring existing app cert and generating a new one. Connections made through this proxy will be routed according to command arguments.")
		} else {
			appCertChecker.SetCert(cert)
		}
	}

	listenAddr := fmt.Sprintf("localhost:%d", portMapping.LocalPort)

	var listener net.Listener
	if withTLS {
		appLocalCAPath := a.profile.AppLocalCAPath(a.tc.SiteName, routeToAppWithTargetPort.Name)
		localCertGenerator, err := client.NewLocalCertGenerator(ctx, appCertChecker, appLocalCAPath)
		if err != nil {
			return trace.Wrap(err)
		}

		if listener, err = tls.Listen("tcp", listenAddr, &tls.Config{
			GetCertificate: localCertGenerator.GetCertificate,
		}); err != nil {
			return trace.Wrap(err)
		}
	} else {
		if listener, err = net.Listen("tcp", listenAddr); err != nil {
			return trace.Wrap(err)
		}
	}

	a.localALPNProxy, err = alpnproxy.NewLocalProxy(
		makeBasicLocalProxyConfig(ctx, a.tc, listener, a.insecure),
		append(opts,
			alpnproxy.WithClusterCAsIfConnUpgrade(ctx, a.tc.RootClusterCACertPool),
			alpnproxy.WithMiddleware(appCertChecker),
		)...,
	)
	if err != nil {
		if cerr := listener.Close(); cerr != nil {
			return trace.NewAggregate(err, cerr)
		}
		return trace.Wrap(err)
	}

	go func() {
		if err = a.localALPNProxy.Start(ctx); err != nil {
			logger.ErrorContext(ctx, "Failed to start local ALPN proxy", "error", err)
		}
	}()
	return nil
}

// startLocalForwardProxy starts a local forward proxy that forwards matching requests
// to the local ALPN proxy and unmatched requests to their original hosts.
func (a *localProxyApp) startLocalForwardProxy(ctx context.Context, port int, forwardMatcher requestMatcher) error {
	listenAddr := fmt.Sprintf("localhost:%d", port)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	a.localForwardProxy, err = alpnproxy.NewForwardProxy(alpnproxy.ForwardProxyConfig{
		Listener:     listener,
		CloseContext: ctx,
		Handlers: []alpnproxy.ConnectRequestHandler{
			// Forward matched requests to ALPN proxy.
			alpnproxy.NewForwardToHostHandler(alpnproxy.ForwardToHostHandlerConfig{
				MatchFunc: forwardMatcher,
				Host:      a.localALPNProxy.GetAddr(),
			}),

			// Forward unmatched requests to user's system proxy, if configured.
			alpnproxy.NewForwardToSystemProxyHandler(alpnproxy.ForwardToSystemProxyHandlerConfig{
				InsecureSystemProxy: a.insecure,
			}),

			// Forward unmatched requests to their original hosts.
			alpnproxy.NewForwardToOriginalHostHandler(),
		},
	})
	if err != nil {
		if cerr := listener.Close(); cerr != nil {
			return trace.NewAggregate(err, cerr)
		}
		return trace.Wrap(err)
	}

	go func() {
		if err := a.localForwardProxy.Start(); err != nil {
			logger.ErrorContext(ctx, "Failed to start local forward proxy", "error", err)
		}
	}()
	return nil
}

func (a *localProxyApp) GetAddr() string {
	return a.localALPNProxy.GetAddr()
}
