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

package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// ServerHandler implements an interface which can handle a connection
// (perform a handshake then process).
type ServerHandler interface {
	// HandleConnection performs a handshake then process the connection.
	HandleConnection(conn net.Conn)
}

// transportConfig is configuration for a rewriting transport.
type transportConfig struct {
	proxyClient  reversetunnelclient.Tunnel
	accessPoint  authclient.ReadProxyAccessPoint
	cipherSuites []uint16
	identity     *tlsca.Identity
	servers      []types.AppServer
	ws           types.WebSession
	clusterName  string
	log          *slog.Logger
	clock        clockwork.Clock

	// integrationAppHandler is used to handle App proxy requests for Apps that are configured to use an Integration.
	// Instead of proxying the connection to an AppService, the app is immediately proxied from the Proxy.
	integrationAppHandler ServerHandler
}

// Check validates configuration.
func (c *transportConfig) Check() error {
	if c.proxyClient == nil {
		return trace.BadParameter("proxy client missing")
	}
	if c.accessPoint == nil {
		return trace.BadParameter("access point missing")
	}
	if len(c.cipherSuites) == 0 {
		return trace.BadParameter("cipe suites missing")
	}
	if c.identity == nil {
		return trace.BadParameter("identity missing")
	}
	if len(c.servers) == 0 {
		return trace.BadParameter("servers missing")
	}
	if c.ws == nil {
		return trace.BadParameter("web session missing")
	}
	if c.clusterName == "" {
		return trace.BadParameter("cluster name missing")
	}
	if c.integrationAppHandler == nil {
		return trace.BadParameter("integration app handler missing")
	}
	if c.log == nil {
		c.log = slog.Default()
	}
	if c.clock == nil {
		c.clock = clockwork.NewRealClock()
	}

	return nil
}

// isRemoteApp returns true if the route to app points at remote cluster.
func (c *transportConfig) isRemoteApp() bool {
	return c.clusterName != c.identity.RouteToApp.ClusterName
}

// transport is a rewriting http.RoundTripper that can forward requests to
// an application service.
type transport struct {
	c *transportConfig

	// tr is used for forwarding http connections.
	tr http.RoundTripper

	// clientTLSConfig is the TLS config used for mutual authentication.
	clientTLSConfig *tls.Config

	// mu protects access to servers in the transportConfig
	mu sync.Mutex
}

// newTransport creates a new transport.
func newTransport(c *transportConfig) (*transport, error) {
	var err error
	if err := c.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	t := &transport{c: c}

	t.clientTLSConfig, err = configureTLS(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Clone and configure the transport.
	tr, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tr.DialContext = t.DialContext
	tr.TLSClientConfig = t.clientTLSConfig

	t.tr = tr
	return t, nil
}

// RoundTrip will rewrite the request, forward the request to the target
// application, emit an event to the audit log, then rewrite the response.
func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Clone the request so we can modify it without affecting the original.
	// This is necessary because the request cookies are deleted when the web
	// handler forward the request to the app proxy. When this happens, the
	// cookies are lost and the error handler will not be able to find the
	// session based on cookies.
	r = r.Clone(r.Context())

	// Perform any request rewriting needed before forwarding the request.
	if err := t.rewriteRequest(r); err != nil {
		return nil, trace.Wrap(err)
	}

	// Forward the request to the target application.
	resp, err := t.tr.RoundTrip(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// When proxying app in leaf cluster, the app will have PublicAddr
	// possibly unreachable to the clients connecting to the root cluster.
	// We want to rewrite any redirects to that PublicAddr to equivalent redirect for root cluster.
	if err = t.rewriteRedirect(resp); err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// rewriteRedirect rewrites redirect to a public address of downstream app.
func (t *transport) rewriteRedirect(resp *http.Response) error {
	if !t.c.isRemoteApp() {
		// Not a downstream app, short circuit.
		return nil
	}

	if !utils.IsRedirect(resp.StatusCode) {
		return nil
	}

	location := resp.Header.Get("Location")
	// nothing to do without Location.
	if location == "" {
		return nil
	}

	// Parse the "Location" header.
	u, err := url.Parse(location)
	if err != nil {
		return trace.Wrap(err)
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
	}

	// If this is a redirect to a public addr of a leaf cluster, rewrite it.
	// We want the rewrite to happen using our own public address.
	if host == t.c.identity.RouteToApp.PublicAddr {
		// drop scheme and host, leaving only the relative path.
		u.Host = ""
		u.Scheme = ""

		// since the path can be an empty string, canonicalize it to "/".
		if u.Path == "" {
			u.Path = "/"
		}

		resp.Header.Set("Location", u.String())
	}
	return nil
}

// rewriteRequest applies any rewriting rules to the request before it's forwarded.
func (t *transport) rewriteRequest(r *http.Request) error {
	// Set dummy values for the request forwarder. Dialing through the tunnel is
	// actually performed using the transport created for this session but these
	// are needed for the forwarder.
	r.URL.Scheme = "https"
	r.URL.Host = constants.APIDomain

	// Remove the application session cookie from the header. This is done by
	// first wiping out the "Cookie" header then adding back all cookies
	// except the application session cookies. This appears to be the safest way
	// to serialize cookies.
	cookies := r.Cookies()
	r.Header.Del("Cookie")
	for _, cookie := range cookies {
		switch cookie.Name {
		case CookieName, SubjectCookieName:
			continue
		default:
			r.AddCookie(cookie)
		}
	}

	// If this looks like a Azure CLI request and at least once app server is
	// an Azure app, parse the JWT cookie using the client's public key and
	// resign it with the web session private key.
	if HasClientCert(r) && t.c.identity.RouteToApp.AzureIdentity != "" {
		for _, server := range t.c.servers {
			if !server.GetApp().IsAzureCloud() {
				continue
			}

			if err := t.resignAzureJWTCookie(r); err != nil {
				// If we failed to resign the JWT, treat it as a noop. The App
				// Service should fail to parse the JWT and reject the request,
				// but rejecting here could cause forward compatibility issues,
				// if for example we add new types of JWT tokens.
				t.c.log.DebugContext(r.Context(), "failed to re-sign azure JWT", "error", err)
			}

			break
		}
	}

	return nil
}

// resignAzureJWTCookie checks the auth header bearer token for a JWT
// token containing Azure claims signed by the client's private key. If
// found, the token is resigned using the app session's private key so
// that the App Service can validate it using the app session's public key.
func (t *transport) resignAzureJWTCookie(r *http.Request) error {
	token, err := parseBearerToken(r)
	if err != nil {
		return trace.Wrap(err)
	}

	// Create a new jwt key using the client public key to verify and parse the token.
	clientJWTKey, err := jwt.New(&jwt.Config{
		Clock:       t.c.clock,
		PublicKey:   r.TLS.PeerCertificates[0].PublicKey,
		ClusterName: types.TeleportAzureMSIEndpoint,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Create a new jwt key using the web session private key to sign a new token.
	wsPrivateKey, err := keys.ParsePrivateKey(t.c.ws.GetTLSPriv())
	if err != nil {
		return trace.Wrap(err)
	}
	wsJWTKey, err := jwt.New(&jwt.Config{
		Clock:       t.c.clock,
		PrivateKey:  wsPrivateKey,
		ClusterName: types.TeleportAzureMSIEndpoint,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	claims, err := clientJWTKey.VerifyAzureToken(token)
	if err != nil {
		// jwt signed by unknown key.
		return trace.Wrap(err, "azure jwt signed by unknown key")
	}

	newToken, err := wsJWTKey.SignAzureToken(*claims)
	if err != nil {
		return trace.Wrap(err)
	}

	r.Header.Set("Authorization", "Bearer "+newToken)
	return nil
}

func parseBearerToken(r *http.Request) (string, error) {
	bearerToken := r.Header.Get("Authorization")
	if bearerToken == "" {
		return "", trace.NotFound("auth header not set")
	}

	bearer, token, found := strings.Cut(bearerToken, " ")
	if !found || bearer != "Bearer" {
		return "", trace.BadParameter("unable to parse auth header")
	}

	return token, nil
}

// DialContext dials and connect to the application service over the reverse
// tunnel subsystem.
func (t *transport) DialContext(ctx context.Context, _, _ string) (conn net.Conn, err error) {
	t.mu.Lock()
	if len(t.c.servers) == 0 {
		defer t.mu.Unlock()
		return nil, trace.ConnectionProblem(nil, "no application servers remaining to connect")
	}
	servers := make([]types.AppServer, len(t.c.servers))
	copy(servers, t.c.servers)
	t.mu.Unlock()

	var i int
	for ; i < len(servers); i++ {
		appServer := servers[i]

		appIntegration := appServer.GetApp().GetIntegration()
		if appIntegration != "" {
			src, dst := net.Pipe()
			go t.c.integrationAppHandler.HandleConnection(src)
			return dst, nil
		}

		conn, err = dialAppServer(ctx, t.c.proxyClient, t.c.identity.RouteToApp.ClusterName, appServer)
		if err != nil && isReverseTunnelDownError(err) {
			t.c.log.WarnContext(ctx, "Failed to connect to application server", "app_server", appServer.GetName(), "error", err)
			// Continue to the next server if there is an issue
			// establishing a connection because the tunnel is not
			// healthy. Reset the error to avoid returning it if
			// this is the last server.
			err = nil
			continue
		}

		break
	}

	t.mu.Lock()
	// Only attempt to tidy up the list of servers if they weren't altered
	// while the dialing happened. Since the lock is only held initially when
	// making the servers copy and released during the dials, another dial attempt
	// may have already happened and modified the list of servers.
	if len(servers) == len(t.c.servers) {
		// eliminate any servers from the head of the list that were unreachable
		if i < len(t.c.servers) {
			t.c.servers = t.c.servers[i:]
		} else {
			t.c.servers = nil
		}
	}
	t.mu.Unlock()

	if conn != nil || err != nil {
		return conn, trace.Wrap(err)
	}

	return nil, trace.ConnectionProblem(nil, "no application servers remaining to connect")
}

// DialWebsocket dials a websocket connection over the transport's reverse
// tunnel.
func (t *transport) DialWebsocket(network, address string) (net.Conn, error) {
	conn, err := t.DialContext(context.Background(), network, address)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// App access connections over reverse tunnel use mutual TLS.
	return tls.Client(conn, t.clientTLSConfig), nil
}

// dialAppServer dial and connect to the application service over the reverse
// tunnel subsystem.
func dialAppServer(ctx context.Context, proxyClient reversetunnelclient.Tunnel, clusterName string, server types.AppServer) (net.Conn, error) {
	clusterClient, err := proxyClient.GetSite(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var from net.Addr
	from = &utils.NetAddr{AddrNetwork: "tcp", Addr: "@web-proxy"}
	clientSrcAddr, originalDst := authz.ClientAddrsFromContext(ctx)
	if clientSrcAddr != nil {
		from = clientSrcAddr
	}

	conn, err := clusterClient.Dial(reversetunnelclient.DialParams{
		From:                  from,
		To:                    &utils.NetAddr{AddrNetwork: "tcp", Addr: reversetunnelclient.LocalNode},
		OriginalClientDstAddr: originalDst,
		ServerID:              fmt.Sprintf("%v.%v", server.GetHostID(), clusterName),
		ConnType:              server.GetTunnelType(),
		ProxyIDs:              server.GetProxyIDs(),
	})
	return conn, trace.Wrap(err)
}

// configureTLS creates and configures a *tls.Config that will be used for
// mutual authentication.
func configureTLS(c *transportConfig) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(c.cipherSuites)

	// Configure the pool of certificates that will be used to verify the
	// identity of the server. This allows the client to verify the identity of
	// the server it is connecting to.
	ca, err := c.accessPoint.GetCertAuthority(context.TODO(), types.CertAuthID{
		Type:       types.HostCA,
		DomainName: c.identity.RouteToApp.ClusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certPool, err := services.CertPool(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.RootCAs = certPool

	// Configure the identity that will be used to connect to the server. This
	// allows the server to verify the identity of the caller.
	certificate, err := tls.X509KeyPair(c.ws.GetTLSCert(), c.ws.GetTLSPriv())
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse certificate or key")
	}
	tlsConfig.Certificates = []tls.Certificate{certificate}

	// Use SNI to tell the other side which cluster signed the CA so it doesn't
	// have to fetch all CAs when verifying the cert.
	tlsConfig.ServerName = apiutils.EncodeClusterName(c.clusterName)

	return tlsConfig, nil
}

// isReverseTunnelDownError returns true if the provided error indicates that
// the reverse tunnel connection is down e.g. because the agent is down.
func isReverseTunnelDownError(err error) bool {
	return trace.IsConnectionProblem(err) ||
		strings.Contains(err.Error(), reversetunnelclient.NoApplicationTunnel)
}
