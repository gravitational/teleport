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
	"net"
	"net/http"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// transportConfig is configuration for a rewriting transport.
type transportConfig struct {
	proxyClient  reversetunnel.Server
	accessPoint  auth.AccessPoint
	cipherSuites []uint16
	identity     *tlsca.Identity
	server       services.Server
	app          *services.App
	ws           services.WebSession
}

// Check validates configuration.
func (c transportConfig) Check() error {
	if c.proxyClient == nil {
		return trace.BadParameter("proxy client missing")
	}
	if c.accessPoint == nil {
		return trace.BadParameter("access point missing")
	}
	if len(c.cipherSuites) == 0 {
		return trace.BadParameter("cipe suites misings")
	}
	if c.identity == nil {
		return trace.BadParameter("identity missing")
	}
	if c.server == nil {
		return trace.BadParameter("server missing")
	}
	if c.app == nil {
		return trace.BadParameter("app missing")
	}
	if c.ws == nil {
		return trace.BadParameter("web session missing")
	}

	return nil
}

// transport is a rewriting http.RoundTripper that can forward requests a
// application service.
type transport struct {
	c *transportConfig

	tr http.RoundTripper
}

// newTransport creates a new transport.
func newTransport(c *transportConfig) (*transport, error) {
	if err := c.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Clone and configure the transport.
	tr, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tr.DialContext = dialFunc(c)
	tr.TLSClientConfig, err = configureTLS(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &transport{
		c:  c,
		tr: tr,
	}, nil
}

// RoundTrip will rewrite the request, forward the request to the target
// application, emit an event to the audit log, then rewrite the response.
func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Perform any request rewriting needed before forwarding the request.
	if err := t.rewriteRequest(r); err != nil {
		return nil, trace.Wrap(err)
	}

	// Forward the request to the target application.
	resp, err := t.tr.RoundTrip(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// rewriteRequest applies any rewriting rules to request before it's forwarded.
func (t *transport) rewriteRequest(r *http.Request) error {
	// Set dummy values for the request forwarder. Dialing through the tunnel is
	// actually performed using the transport created for this session but these
	// are needed for the forwarder.
	r.URL.Scheme = "https"
	r.URL.Host = teleport.APIDomain

	// Remove the application session cookie from the header. This is done by
	// first wiping out the "Cookie" header then adding back all cookies
	// except the application session cookie. This appears to be the safest way
	// to serialize cookies.
	cookies := r.Cookies()
	r.Header.Del("Cookie")
	for _, cookie := range cookies {
		if cookie.Name == cookieName {
			continue
		}
		r.AddCookie(cookie)
	}

	return nil
}

// dialFunc returns a function that can Dial and connect to the application
// service over the reverse tunnel subsystem.
func dialFunc(c *transportConfig) func(ctx context.Context, network string, addr string) (net.Conn, error) {
	return func(ctx context.Context, network string, addr string) (net.Conn, error) {
		clusterClient, err := c.proxyClient.GetSite(c.identity.RouteToApp.ClusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		conn, err := clusterClient.Dial(reversetunnel.DialParams{
			// The "From" and "To" addresses are not actually used for tunnel dialing,
			// they're filled out with "dummy values" that make logs easier to read and
			// debug within the reverse tunnel subsystem.
			From:     &utils.NetAddr{AddrNetwork: "tcp", Addr: "@web-proxy"},
			To:       &utils.NetAddr{AddrNetwork: "tcp", Addr: fmt.Sprintf("@app-%v", c.app.Name)},
			ServerID: fmt.Sprintf("%v.%v", c.server.GetName(), c.identity.RouteToApp.ClusterName),
			ConnType: services.AppTunnel,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}
}

// configureTLS creates and configures a *tls.Config will be used for
// mutual authentication.
func configureTLS(c *transportConfig) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(c.cipherSuites)

	// Configure the pool of certificates that will be used to verify the
	// identity of the server. This allows the client to verify the identity of
	// the server it is connecting to.
	ca, err := c.accessPoint.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
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
	certificate, err := tls.X509KeyPair(c.ws.GetTLSCert(), c.ws.GetPriv())
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse certificate or key")
	}
	tlsConfig.Certificates = []tls.Certificate{certificate}

	// Make sure the server this client connects to presents a certificate for
	// hostUUID.clusterName.
	tlsConfig.ServerName = fmt.Sprintf("%v.%v", c.server.GetName(), c.identity.RouteToApp.ClusterName)

	// This is a hack used elsewhere within Teleport to ensure that a client
	// always sends it's clients certificate when establishing a connection. It's
	// unclear if it's still needed in recent versions of Go.
	cert := tlsConfig.Certificates[0]
	tlsConfig.Certificates = nil
	tlsConfig.GetClientCertificate = func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return &cert, nil
	}

	return tlsConfig, nil
}
