// Teleport
// Copyright (C) 2026  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
)

// localHTTPProxyConfig configures the local HTTP forward proxy and its ALPN
// proxy upstream.
type localHTTPProxyConfig struct {
	LocalPort           int
	ALPNTLSConfig       *tls.Config
	ALPNProxyConfigOpts []alpnproxy.LocalProxyConfigOpt
	ForwardMatcher      requestMatcher

	InsecureSystemProxy bool
}

// localHTTPProxy combines a local HTTP forward proxy with the ALPN proxy that
// handles matched requests.
type localHTTPProxy struct {
	tc  *client.TeleportClient
	cfg localHTTPProxyConfig

	localALPNProxy    *alpnproxy.LocalProxy
	localForwardProxy *alpnproxy.ForwardProxy
}

// newLocalHTTPProxy initializes a local HTTP proxy from cfg.
func newLocalHTTPProxy(tc *client.TeleportClient, cfg localHTTPProxyConfig) *localHTTPProxy {
	return &localHTTPProxy{
		tc:  tc,
		cfg: cfg,
	}
}

// Start starts the ALPN proxy and the HTTP forward proxy.
func (p *localHTTPProxy) Start(ctx context.Context) error {
	listener, err := p.newALPNListener()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := p.startALPNProxy(ctx, listener); err != nil {
		return trace.Wrap(err)
	}

	if err := p.startForwardProxy(ctx); err != nil {
		cerr := p.localALPNProxy.Close()
		return trace.NewAggregate(err, cerr)
	}

	return nil
}

// Close closes the HTTP forward proxy and the ALPN proxy.
func (p *localHTTPProxy) Close() error {
	errs := make([]error, 0)

	if p.localForwardProxy != nil {
		errs = append(errs, p.localForwardProxy.Close())
	}

	if p.localALPNProxy != nil {
		errs = append(errs, p.localALPNProxy.Close())
	}

	return trace.NewAggregate(errs...)
}

// newALPNListener creates the internal listener used by the ALPN proxy.
// When localHTTPProxy.ALPNTLSConfig is non-nil, TLS is configured for incoming
// connections.
func (p *localHTTPProxy) newALPNListener() (net.Listener, error) {
	listenAddr := "localhost:0"

	var listener net.Listener
	var err error
	if p.cfg.ALPNTLSConfig != nil {
		listener, err = tls.Listen("tcp", listenAddr, p.cfg.ALPNTLSConfig)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if listener, err = net.Listen("tcp", listenAddr); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return listener, nil
}

// startALPNProxy initializes and starts the ALPN proxy using listener.
func (p *localHTTPProxy) startALPNProxy(ctx context.Context, listener net.Listener) error {
	localALPNProxy, err := alpnproxy.NewLocalProxy(
		// By default, set insecure to false. This can be overridden via opts.
		makeBasicLocalProxyConfig(ctx, p.tc, listener, false),
		p.cfg.ALPNProxyConfigOpts...,
	)
	if err != nil {
		if cerr := listener.Close(); cerr != nil {
			return trace.NewAggregate(err, cerr)
		}
		return trace.Wrap(err)
	}

	p.localALPNProxy = localALPNProxy

	go func() {
		if err := localALPNProxy.Start(ctx); err != nil {
			logger.ErrorContext(ctx, "Failed to start local ALPN proxy", "error", err)
		}
	}()

	return nil
}

// startForwardProxy initializes and starts the HTTP forward proxy, forwarding
// matching requests to the ALPN proxy.
func (p *localHTTPProxy) startForwardProxy(ctx context.Context) error {
	listenAddr := fmt.Sprintf("localhost:%d", p.cfg.LocalPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	localForwardProxy, err := newForwardProxy(ctx, listener, p.cfg.InsecureSystemProxy, p.localALPNProxy.GetAddr(), p.cfg.ForwardMatcher)
	if err != nil {
		if cerr := listener.Close(); cerr != nil {
			return trace.NewAggregate(err, cerr)
		}
		return trace.Wrap(err)
	}
	p.localForwardProxy = localForwardProxy

	go func() {
		if err := localForwardProxy.Start(); err != nil {
			logger.ErrorContext(ctx, "Failed to start local forward proxy", "error", err)
		}
	}()

	return nil
}

// newForwardProxy constructs a forward proxy using listener. The caller owns
// starting and closing the returned proxy and closing listener on error.
func newForwardProxy(ctx context.Context, listener net.Listener, insecure bool, alpnProxyAddr string, forwardMatcher requestMatcher) (*alpnproxy.ForwardProxy, error) {
	localForwardProxy, err := alpnproxy.NewForwardProxy(alpnproxy.ForwardProxyConfig{
		Listener:     listener,
		CloseContext: ctx,
		Handlers: []alpnproxy.ConnectRequestHandler{
			// Forward matched requests to ALPN proxy.
			alpnproxy.NewForwardToHostHandler(alpnproxy.ForwardToHostHandlerConfig{
				MatchFunc: forwardMatcher,
				Host:      alpnProxyAddr,
			}),

			// Forward unmatched requests to user's system proxy, if configured.
			alpnproxy.NewForwardToSystemProxyHandler(alpnproxy.ForwardToSystemProxyHandlerConfig{
				InsecureSystemProxy: insecure,
			}),

			// Forward unmatched requests to their original hosts.
			alpnproxy.NewForwardToOriginalHostHandler(),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return localForwardProxy, nil
}
