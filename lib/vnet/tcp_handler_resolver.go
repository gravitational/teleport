// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package vnet

import (
	"context"
	"net"
	"strconv"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// tcpHandlerResolver resolves fully-qualified domain names to a tcpHandlerSpec
// that defines the CIDR range to assign an IP to that handler from, and a
// handler for future TCP connections to that IP address.
type tcpHandlerResolver struct {
	cfg *tcpHandlerResolverConfig
}

type tcpHandlerResolverConfig struct {
	clt                      *clientApplicationServiceClient
	appProvider              *appProvider
	sshProvider              *sshProvider
	clock                    clockwork.Clock
	alwaysTrustRootClusterCA bool
}

func newTCPHandlerResolver(cfg *tcpHandlerResolverConfig) *tcpHandlerResolver {
	return &tcpHandlerResolver{
		cfg: cfg,
	}
}

// resolveTCPHandler decides if fqdn should match a TCP handler.
//
// If fqdn matches a valid VNet target it returns a tcpHandlerSpec defining the
// CIDR range to assign an IP from, and a TCP handler for future connections to
// the assigned IP.
//
// If fqdn does not match anything it must return errNoTCPHandler.
func (r *tcpHandlerResolver) resolveTCPHandler(ctx context.Context, fqdn string) (*tcpHandlerSpec, error) {
	resp, err := r.cfg.clt.ResolveFQDN(ctx, fqdn)
	if err != nil {
		return nil, err
	}
	if matchedTCPApp := resp.GetMatchedTcpApp(); matchedTCPApp != nil {
		// The query matched a valid TCP app, return a handler that will proxy
		// all connections to that app.
		appInfo := matchedTCPApp.GetAppInfo()
		return &tcpHandlerSpec{
			ipv4CIDRRange: appInfo.GetIpv4CidrRange(),
			tcpHandler: newTCPAppHandler(&tcpAppHandlerConfig{
				appInfo:                  appInfo,
				appProvider:              r.cfg.appProvider,
				clock:                    r.cfg.clock,
				alwaysTrustRootClusterCA: r.cfg.alwaysTrustRootClusterCA,
			}),
		}, nil
	}
	if resp.GetMatchedWebApp() != nil {
		// We prefer not to assign a handler for web apps so that the DNS query
		// gets forwarded to upstream resolvers, it should eventually resolve to
		// the proxy address so that regular web app access continues to work
		// without sending any traffic through VNet.
		return nil, errNoTCPHandler
	}
	if matchedCluster := resp.GetMatchedCluster(); matchedCluster != nil {
		// We know the query matched a cluster, but we won't know until we get a
		// TCP connection if this may match an SSH node or an app that may be
		// added later so we return an undecidedHandler.
		handler, err := newUndecidedHandler(&undecidedHandlerConfig{
			tcpHandlerResolverConfig: r.cfg,
			fqdn:                     fqdn,
			webProxyAddr:             matchedCluster.GetWebProxyAddr(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &tcpHandlerSpec{
			ipv4CIDRRange: matchedCluster.GetIpv4CidrRange(),
			tcpHandler:    handler,
		}, nil
	}
	return nil, errNoTCPHandler
}

// undecidedHandler handles TCP connections at addresses that matched a
// subdomain of the cluster name but haven't been confirmed to match an SSH node
// yet. When receiving a TCP connection the cluster will be queried again for a
// match and the handler may become "decided".
type undecidedHandler struct {
	cfg          *undecidedHandlerConfig
	webProxyPort uint16

	// mu guards access to the below fields.
	mu sync.Mutex
	// decidedHandler will be set when it's decided which target this handler
	// should actually forward connections to.
	decidedHandler tcpHandler
}

type undecidedHandlerConfig struct {
	*tcpHandlerResolverConfig
	fqdn         string
	webProxyAddr string
}

func newUndecidedHandler(cfg *undecidedHandlerConfig) (*undecidedHandler, error) {
	_, proxyPort, err := net.SplitHostPort(cfg.webProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err, "parsing web proxy address")
	}
	webProxyPort, err := strconv.ParseUint(proxyPort, 10, 16)
	if err != nil {
		return nil, trace.Wrap(err, "parsing web proxy port")
	}
	return &undecidedHandler{
		cfg:          cfg,
		webProxyPort: uint16(webProxyPort),
	}, nil
}

func (h *undecidedHandler) getDecidedHandler() tcpHandler {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.decidedHandler
}

func (h *undecidedHandler) setDecidedHandler(handler tcpHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.decidedHandler = handler
}

func (h *undecidedHandler) handleTCPConnector(ctx context.Context, localPort uint16, connector func() (net.Conn, error)) error {
	if decidedHandler := h.getDecidedHandler(); decidedHandler != nil {
		return decidedHandler.handleTCPConnector(ctx, localPort, connector)
	}

	// Handling an incoming TCP connection but we're not sure what this
	// address should point to yet, query again in case an app was added.
	resp, err := h.cfg.clt.ResolveFQDN(ctx, h.cfg.fqdn)
	if err != nil {
		return trace.Wrap(err, "resolving target in undecidedHandler")
	}
	log := log.With("fqdn", h.cfg.fqdn)
	if matchedTCPApp := resp.GetMatchedTcpApp(); matchedTCPApp != nil {
		// If matched a TCP app, build a tcpAppHandler that will be used for this
		// and all subsequent connections to this address.
		log.DebugContext(ctx, "Resolved FQDN to a matched TCP app")
		tcpAppHandler := newTCPAppHandler(&tcpAppHandlerConfig{
			appInfo:                  matchedTCPApp.GetAppInfo(),
			appProvider:              h.cfg.appProvider,
			clock:                    h.cfg.clock,
			alwaysTrustRootClusterCA: h.cfg.alwaysTrustRootClusterCA,
		})
		h.setDecidedHandler(tcpAppHandler)
		return tcpAppHandler.handleTCPConnector(ctx, localPort, connector)
	}
	if matchedWebApp := resp.GetMatchedWebApp(); matchedWebApp != nil && localPort == h.webProxyPort {
		// If matched a web app, build a webAppHandler that will be used for this
		// and all subsequent connections to this address.
		log.DebugContext(ctx, "Resolved FQDN to a matched web app")
		webAppHandler := newWebAppHandler(h.cfg.webProxyAddr, h.webProxyPort)
		h.setDecidedHandler(webAppHandler)
		return webAppHandler.handleTCPConnector(ctx, localPort, connector)
	}
	if matchedCluster := resp.GetMatchedCluster(); matchedCluster != nil && localPort == 22 {
		// Matched a cluster, this FQDN could potentially match an SSH node.
		log.DebugContext(ctx, "Resolved FQDN to a matched cluster")
		// Attempt a dial to the target SSH node to see if it exists.
		target := computeDialTarget(matchedCluster, h.cfg.fqdn)
		targetConn, err := h.cfg.sshProvider.dial(ctx, target)
		if err != nil {
			if trace.IsConnectionProblem(err) {
				log.DebugContext(ctx, "Failed TCP dial to target, node might be offline")
				return nil
			}
			return trace.Wrap(err, "unexpected error TCP dialing to target node at %s", h.cfg.fqdn)
		}
		defer targetConn.Close()
		log.DebugContext(ctx, "TCP dial to target SSH node succeeded", "fqdn", h.cfg.fqdn)
		// Now that we know there is a matching SSH node, this handler will
		// permanently handle SSH connections at this address and avoid app
		// queries on subsequent connections.
		sshHandler := newSSHHandler(sshHandlerConfig{
			sshProvider: h.cfg.sshProvider,
			target:      target,
		})
		h.setDecidedHandler(sshHandler)
		// Handle the incoming connection with the TCP connection to the target
		// SSH node that has already been established.
		return sshHandler.handleTCPConnectorWithTargetConn(ctx, localPort, connector, targetConn)
	}
	return trace.Errorf("rejecting connection to %s:%d", h.cfg.fqdn, localPort)
}

// webAppHandler proxies incoming TCP connections on webProxyPort to the
// webProxyAddr. This is a plain TCP proxy, VNet does not add any user
// authentication for web app access, auth happens in the browser as it usually
// does for web apps as if VNet was not even in the network path.
type webAppHandler struct {
	webProxyAddr string
	webProxyPort uint16
}

func newWebAppHandler(webProxyAddr string, webProxyPort uint16) *webAppHandler {
	return &webAppHandler{
		webProxyAddr: webProxyAddr,
		webProxyPort: webProxyPort,
	}
}

func (h *webAppHandler) handleTCPConnector(ctx context.Context, localPort uint16, connector func() (net.Conn, error)) error {
	if localPort != h.webProxyPort {
		return trace.Errorf("rejecting web app connection on port %d not matching web proxy port %d", localPort, h.webProxyPort)
	}
	dialer := &net.Dialer{Timeout: defaults.DefaultIOTimeout}
	proxyConn, err := dialer.DialContext(ctx, "tcp", h.webProxyAddr)
	if err != nil {
		return trace.Wrap(err, "dialing proxy addr %s", h.webProxyAddr)
	}
	localConn, err := connector()
	if err != nil {
		return trace.Wrap(err, "accepting incoming TCP conn")
	}
	return trace.Wrap(utils.ProxyConn(ctx, localConn, proxyConn))
}
