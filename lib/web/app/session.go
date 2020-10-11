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

package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/http"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type session struct {
	fwd *forward.Forwarder
}

func (h *Handler) getSession(ctx context.Context, ws services.WebSession) (*session, error) {
	// If a cached session exists, return it right away.
	session, err := h.cacheGet(ws.GetName())
	if err == nil {
		return session, nil
	}

	// Create a new session with a forwarder in it.
	session, err = h.newSession(ctx, ws)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Put the session in the cache so the next request can use it.
	err = h.cacheSet(ws.GetName(), session, ws.Expiry().Sub(h.c.Clock.Now()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

func (h *Handler) newSession(ctx context.Context, ws services.WebSession) (*session, error) {
	// Extract the identity of the user.
	certificate, err := tlsca.ParseCertificatePEM(ws.GetTLSCert())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := tlsca.FromSubject(certificate.Subject, certificate.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Find the address of the Teleport application proxy server for this public
	// address and cluster name pair.
	clusterClient, err := h.c.ProxyClient.GetSite(identity.RouteToApp.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO(russjones): Close this access point.
	accessPoint, err := clusterClient.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	application, server, err := getApp(ctx, accessPoint, identity.RouteToApp.PublicAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a http.RoundTripper connected to the Teleport application proxy
	// with the identity of the user. This allows all requests to use the
	// auth.AuthMiddleware for authorization and authentication.
	transport, err := h.newTransport(identity, application, server, ws)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the forwarder.
	fwder, err := newForwarder(forwarderConfig{
		tr:  transport,
		log: h.log,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fwd, err := forward.New(
		forward.RoundTripper(fwder),
		forward.Rewriter(fwder),
		forward.Logger(h.log))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &session{
		fwd: fwd,
	}, nil
}

// forwarderConfig is the configuration for a forwarder.
type forwarderConfig struct {
	tr  http.RoundTripper
	log *logrus.Entry
}

// Check will valid the configuration of a forwarder.
func (c forwarderConfig) Check() error {
	if c.tr == nil {
		return trace.BadParameter("round tripper missing")
	}
	if c.log == nil {
		return trace.BadParameter("logger missing")
	}

	return nil
}

// forwarder will rewrite and forward the request to the target address.
type forwarder struct {
	c forwarderConfig
}

// newForwarder returns a new forwarder.
func newForwarder(c forwarderConfig) (*forwarder, error) {
	if err := c.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &forwarder{
		c: c,
	}, nil
}

func (f *forwarder) RoundTrip(r *http.Request) (*http.Response, error) {
	// Set dummy values for the request forwarder. Dialing through the tunnel is
	// actually performed using the transport created for this session.
	r.URL.Scheme = "https"
	r.URL.Host = teleport.APIDomain

	resp, err := f.c.tr.RoundTrip(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

func (f *forwarder) Rewrite(r *http.Request) {
	// Remove the application specific session cookie from the header. This is
	// done by first wiping out the "Cookie" header then adding back all cookies
	// except the Teleport application specific session cookie. This appears to
	// be the best way to serialize cookies.
	cookies := r.Cookies()
	r.Header.Del("Cookie")
	for _, cookie := range cookies {
		if cookie.Name == cookieName {
			continue
		}
		r.AddCookie(cookie)
	}
}

// newTransport creates a http.RoundTripper that connects to the Teleport
// application proxy with the identity of the caller using the reverse
// tunnel subsystem to build the connection.
func (h *Handler) newTransport(identity *tlsca.Identity, application *services.App, server services.Server, ws services.WebSession) (*http.Transport, error) {
	var err error

	// Clone the default transport to pick up sensible defaults.
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, trace.BadParameter("invalid transport type %T", http.DefaultTransport)
	}
	tr := defaultTransport.Clone()

	// Configure TLS client with the identity of the caller.
	tr.TLSClientConfig, err = h.configureTLS(identity, server, ws)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Increase the size of the transports connection pool. This substantially
	// improves the performance of Teleport under load as it reduces the number
	// of TLS handshakes performed.
	tr.MaxIdleConns = defaults.HTTPMaxIdleConns
	tr.MaxIdleConnsPerHost = defaults.HTTPMaxIdleConnsPerHost

	// Set IdleConnTimeout on the transport, this defines the maximum amount of
	// time before idle connections are closed. Leaving this unset will lead to
	// connections open forever and will cause memory leaks in a long running
	// process.
	tr.IdleConnTimeout = defaults.HTTPIdleTimeout

	// Build a dialer that will connect to the Teleport application proxy server
	// that will forward the request to the target application over the reverse
	// tunnel subsystem. This will get the request to the Teleport application
	// proxy, but does not mean the request will be proxied. The identity and
	// target application will then be resolved by the Teleport application proxy
	// to determine if the caller has access.
	tr.DialContext = func(ctx context.Context, network string, addr string) (net.Conn, error) {
		clusterClient, err := h.c.ProxyClient.GetSite(identity.RouteToApp.ClusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// TODO(russjones): Do these connections need to be tracked and closed or
		// does the transport take care of that?
		conn, err := clusterClient.Dial(reversetunnel.DialParams{
			// The "From" and "To" addresses are not actually used for tunnel dialing,
			// they're filled out with "dummy values" that make logs easier to read and
			// debug within the reverse tunnel subsystem.
			From:     &utils.NetAddr{AddrNetwork: "tcp", Addr: "@web-proxy"},
			To:       &utils.NetAddr{AddrNetwork: "tcp", Addr: fmt.Sprintf("@app-%v", application.Name)},
			ServerID: fmt.Sprintf("%v.%v", server.GetName(), identity.RouteToApp.ClusterName),
			ConnType: services.AppTunnel,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}

	return tr, nil
}

// getApp looks for an application registered for the requested public address
// in the cluster and returns it. In the situation multiple applications match,
// a random selection is returned. This is done on purpose to support HA to
// allow multiple application proxy nodes to be run and if one is down, at
// least the application can be accessible on the other.
//
// In the future this function should be updated to keep state on application
// servers that are down and to not route requests to that server.
func getApp(ctx context.Context, accessPoint auth.AccessPoint, publicAddr string) (*services.App, services.Server, error) {
	var am []*services.App
	var sm []services.Server

	servers, err := accessPoint.GetAppServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	for _, server := range servers {
		for _, app := range server.GetApps() {
			if app.PublicAddr == publicAddr {
				am = append(am, app)
				sm = append(sm, server)
			}
		}
	}

	if len(am) == 0 {
		return nil, nil, trace.NotFound("%q not found", publicAddr)
	}
	index := rand.Intn(len(am))
	return am[index], sm[index], nil
}

// configureTLS creates and configures a *tls.Config will be used for
// mutual authentication.
func (h *Handler) configureTLS(identity *tlsca.Identity, server services.Server, ws services.WebSession) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(h.c.CipherSuites)

	// Configure the pool of certificates that will be used to verify the
	// identity of the server. This allows the client to verify the identity of
	// the server it is connecting to.
	ca, err := h.c.AuthClient.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: identity.RouteToApp.ClusterName,
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
	certificate, err := tls.X509KeyPair(ws.GetTLSCert(), ws.GetPriv())
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse certificate or key")
	}
	tlsConfig.Certificates = []tls.Certificate{certificate}

	// TODO(russjones): This should not be hostname, but should be uuid.clusterName
	// encoded using auth.EncodeClusterName. This will allow Teleport application
	// proxy to decode the name of the server and then pull back just the CAs needed in GetConfigForClient.
	//
	// To get to that point, the first thing that needs to be done is
	// uuid.clusterName needs to be encoded into the servers certificate.
	tlsConfig.ServerName = server.GetHostname()

	// TODO(russjones): Is this hack still needed? It occurs in some places within
	// our codebase and not others.
	// This hack is needed to always send the client certificate in when
	// establishing a connection.
	cert := tlsConfig.Certificates[0]
	tlsConfig.Certificates = nil
	tlsConfig.GetClientCertificate = func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return &cert, nil
	}

	return tlsConfig, nil
}
