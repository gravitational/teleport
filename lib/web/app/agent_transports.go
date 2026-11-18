/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib/identityforwarding"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/utils"
)

// agentTransportCacheTTL is how long a transport is reused. Expiry drops the
// transports of agents that have gone, and retires busy connections so none
// keeps its handshake much longer than this. Kube uses the same TTL.
const agentTransportCacheTTL = 5 * time.Hour

// agentDialSource is the client address of every connection to an agent. A
// connection carries requests from many clients, so each request carries its
// client's address in a header instead.
var agentDialSource = &utils.NetAddr{AddrNetwork: "tcp", Addr: "0.0.0.0:0"}

// agentTransportsConfig configures agentTransports.
type agentTransportsConfig struct {
	// clusterName is the local cluster. Forwarded requests never leave it.
	clusterName   string
	clusterGetter reversetunnelclient.ClusterGetter
	// getCertificate returns the Proxy's RoleProxy certificate. Agents accept
	// forwarded identities only from that, not the Instance certificate.
	getCertificate utils.GetCertificateFunc
	// getRoots returns the local cluster's Host CA, which signs agents'
	// certificates.
	getRoots     utils.GetRootsFunc
	cipherSuites []uint16
	metrics      *handlerMetrics
}

func (c *agentTransportsConfig) checkAndSetDefaults() error {
	if c.clusterName == "" {
		return trace.BadParameter("missing cluster name")
	}
	if c.clusterGetter == nil {
		return trace.BadParameter("missing cluster getter")
	}
	if c.getCertificate == nil {
		return trace.BadParameter("missing certificate getter")
	}
	if c.getRoots == nil {
		return trace.BadParameter("missing roots getter")
	}
	if len(c.cipherSuites) == 0 {
		return trace.BadParameter("missing cipher suites")
	}
	if c.metrics == nil {
		return trace.BadParameter("missing metrics")
	}
	return nil
}

// agentTransports holds one transport per agent for forwarded requests. Each
// dials only its own agent, so a request reaches the agent chosen for it, and
// its connections are shared by every client and app on that agent.
type agentTransports struct {
	cfg        agentTransportsConfig
	tlsConfig  *tls.Config
	transports *utils.FnCache
}

// agentDialServerKey is the request context key of the app server that
// roundTrip sends the request for. Go's transport keeps the request's context
// values when it dials, which is how the dialer gets the server.
type agentDialServerKey struct{}

func newAgentTransports(ctx context.Context, cfg agentTransportsConfig) (*agentTransports, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	transports, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:     agentTransportCacheTTL,
		Context: ctx,
		// Don't keep a failed build for the whole TTL.
		ReloadOnErr: true,
		// A gone agent's key is never read again, so sweep every TTL rather
		// than the default 16.
		CleanupInterval: agentTransportCacheTTL,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig := utils.TLSConfig(cfg.cipherSuites)
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		tlsCert, err := cfg.getCertificate()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return tlsCert, nil
	}
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.VerifyConnection = utils.VerifyConnection(time.Now, cfg.getRoots)
	// Agents choose the CAs that verify the Proxy by the cluster name in SNI,
	// and VerifyConnection checks the agent's certificate for the same name.
	tlsConfig.ServerName = apiutils.EncodeClusterName(cfg.clusterName)

	return &agentTransports{cfg: cfg, tlsConfig: tlsConfig, transports: transports}, nil
}

// roundTrip sends req, with its forwarded identity, to the agent that
// registered server. A connection it dials is shared by every app on that
// agent, so the dial uses only the agent's details from server.
func (t *agentTransports) roundTrip(req *http.Request, server readonly.AppServer) (*http.Response, error) {
	rt, err := t.forAgent(req.Context(), server.GetHostID())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx := context.WithValue(req.Context(), agentDialServerKey{}, server)
	resp, err := rt.RoundTrip(req.WithContext(ctx))
	return resp, trace.Wrap(err)
}

// forAgent returns the transport of the agent with hostID. Its requests must
// go through roundTrip, which tells its dialer how to reach the agent.
func (t *agentTransports) forAgent(ctx context.Context, hostID string) (http.RoundTripper, error) {
	rt, err := t.transports.Get(ctx, hostID, func(context.Context) (http.RoundTripper, error) {
		return t.newTransport(hostID)
	})
	return rt, trace.Wrap(err)
}

func (t *agentTransports) newTransport(hostID string) (http.RoundTripper, error) {
	tr, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return t.dial(ctx, hostID)
	}
	tr.TLSClientConfig = t.tlsConfig.Clone()
	// Set explicitly because the agent's forwardedIdleTimeout (lib/srv/app)
	// must outlast it, or the agent could close a connection this pool reuses.
	tr.IdleConnTimeout = defaults.HTTPIdleTimeout
	// Requests go over the reverse tunnel, never through a proxy from the
	// environment.
	tr.Proxy = nil
	// Agents offer h2, so pin HTTP/1.1 to keep the protocol the Proxy's choice.
	tr.Protocols = new(http.Protocols)
	tr.Protocols.SetHTTP1(true)
	return identityforwarding.NewRoundTripper(tr), nil
}

// dial connects to the agent with hostID through the app server of the
// request that needs the connection. It fails rather than dial another agent,
// because the connection is pooled for this one.
func (t *agentTransports) dial(ctx context.Context, hostID string) (net.Conn, error) {
	server, ok := ctx.Value(agentDialServerKey{}).(readonly.AppServer)
	if !ok || server.GetHostID() != hostID {
		return nil, trace.BadParameter("request carries no app server of agent %q to dial", hostID)
	}
	// The pool is for the agent's Application Service tunnel. An app server on
	// another tunnel, such as Okta's, would reach a different endpoint.
	if server.GetTunnelType() != types.AppTunnel {
		return nil, trace.BadParameter("app server %q uses a %v tunnel", server.GetName(), server.GetTunnelType())
	}
	cluster, err := t.cfg.clusterGetter.Cluster(ctx, t.cfg.clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conn, err := t.cfg.metrics.sharedDial.dial(func() (net.Conn, error) {
		return dialAppServerFrom(cluster, server, agentDialSource, nil)
	})
	return conn, trace.Wrap(err)
}
