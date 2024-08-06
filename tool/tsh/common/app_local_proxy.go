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
	"cmp"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
)

// localProxyApp is a generic app that can start local proxies.
type localProxyApp struct {
	tc       *client.TeleportClient
	appInfo  *appInfo
	insecure bool
	port     string

	localALPNProxy    *alpnproxy.LocalProxy
	localForwardProxy *alpnproxy.ForwardProxy
}

type requestMatcher func(req *http.Request) bool

// newLocalProxyApp creates a new generic app.
func newLocalProxyApp(tc *client.TeleportClient, appInfo *appInfo, port string, insecure bool) *localProxyApp {
	return &localProxyApp{
		tc:       tc,
		appInfo:  appInfo,
		port:     port,
		insecure: insecure,
	}
}

// StartLocalProxy sets up local proxies for serving app clients.
func (a *localProxyApp) StartLocalProxy(ctx context.Context, opts ...alpnproxy.LocalProxyConfigOpt) error {
	if err := a.startLocalALPNProxy(ctx, a.port, false /*withTLS*/, opts...); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// StartLocalProxy sets up local proxies for serving app clients.
func (a *localProxyApp) StartLocalProxyWithTLS(ctx context.Context, opts ...alpnproxy.LocalProxyConfigOpt) error {
	if err := a.startLocalALPNProxy(ctx, a.port, true /*withTLS*/, opts...); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// StartLocalProxy sets up local proxies for serving app clients.
func (a *localProxyApp) StartLocalProxyWithForwarder(ctx context.Context, forwardMatcher requestMatcher, opts ...alpnproxy.LocalProxyConfigOpt) error {
	if err := a.startLocalALPNProxy(ctx, "", true /*withTLS*/, opts...); err != nil {
		return trace.Wrap(err)
	}

	if err := a.startLocalForwardProxy(ctx, a.port, forwardMatcher); err != nil {
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
func (a *localProxyApp) startLocalALPNProxy(ctx context.Context, port string, withTLS bool, opts ...alpnproxy.LocalProxyConfigOpt) error {
	// Create an app cert checker to check and reissue app certs for the local app proxy.
	appCertChecker := client.NewAppCertChecker(a.tc, a.appInfo.RouteToApp, nil)

	// If a stored cert is found for the app, try using it.
	// Otherwise, let the checker reissue one as needed.
	cert, err := loadAppCertificate(a.tc, a.appInfo.RouteToApp.Name)
	if err == nil {
		appCertChecker.SetCert(cert)
	}

	listenAddr := fmt.Sprintf("localhost:%s", cmp.Or(port, "0"))

	var listener net.Listener
	if withTLS {
		appLocalCAPath := a.appInfo.appLocalCAPath(a.tc.SiteName)
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
			log.WithError(err).Errorf("Failed to start local ALPN proxy.")
		}
	}()
	return nil
}

// startLocalForwardProxy starts a local forward proxy that forwards matching requests
// to the local ALPN proxy and unmatched requests to their original hosts.
func (a *localProxyApp) startLocalForwardProxy(ctx context.Context, port string, forwardMatcher requestMatcher) error {
	listenAddr := fmt.Sprintf("localhost:%s", cmp.Or(port, "0"))
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
			log.WithError(err).Errorf("Failed to start local forward proxy.")
		}
	}()
	return nil
}

func (a *localProxyApp) GetAddr() string {
	return a.localALPNProxy.GetAddr()
}
