/*
Copyright 2022 Gravitational, Inc.

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

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpn "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// ALPNAuthTunnel contains the required methods to create a local ALPN proxy.
type ALPNAuthTunnel interface {
	// GetClusterCACert returns the PEM-encoded TLS certs for the local cluster.
	// If the cluster has multiple TLS certs, they will all be concatenated.
	GetClusterCACert(ctx context.Context) (*proto.GetClusterCACertResponse, error)

	// GetCurrentUser returns current user as seen by the server.
	// Useful especially in the context of remote clusters which perform role and trait mapping.
	GetCurrentUser(ctx context.Context) (types.User, error)

	// GenerateUserCerts takes the public key in the OpenSSH `authorized_keys` plain
	// text format, signs it using User Certificate Authority signing key and
	// returns the resulting certificates.
	GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error)
}

// RunALPNAuthTunnelRequest contains the required fields used to create an authed ALPN Proxy
type RunALPNAuthTunnelRequest struct {
	// Client is the client that's used to interact with the cluster and obtain Certificates.
	Client ALPNAuthTunnel

	// Listener to be used to accept connections that will go trough the
	Listener net.Listener

	// Insecure turns off verification for x509 upstream ALPN proxy service certificate.
	Insecure bool

	// Protocol name
	// Examples for databases: "postgres", "mysql"
	// This protocol must map to a Teleport ALPN protocol [lib/srv/alpnproxy/common.alpnToALPNProtocol]
	Protocol string

	// WebProxyAddr is the proxy addr to
	WebProxyAddr string

	// ConnectionDiagnosticID contains the ID to be used to store Connection Diagnostic checks.
	// Can be empty.
	ConnectionDiagnosticID string

	// ProxyMiddleware is a middleware that ensures that the local proxy has valid TLS certs.
	ProxyMiddleware alpnproxy.LocalProxyMiddleware

	// RouteToDatabase contains the destination server that must receive the connection.
	// Specific for database proxying.
	RouteToDatabase proto.RouteToDatabase
}

// RunALPNAuthTunnel runs a local authenticated ALPN proxy to another service.
// At least one Route (which defines the service) must be defined
func RunALPNAuthTunnel(ctx context.Context, req RunALPNAuthTunnelRequest) error {
	alpnProtocol, err := alpn.ToALPNProtocol(req.Protocol)
	if err != nil {
		return trace.Wrap(err)
	}

	protocols := []alpn.Protocol{alpnProtocol, alpn.Protocol(req.Protocol)}
	if alpn.HasPingSupport(alpnProtocol) {
		protocols = append(alpn.ProtocolsWithPing(alpnProtocol), protocols...)
	}

	rootCAs := &x509.CertPool{}

	alpnUpgradeRequired := alpnproxy.IsALPNConnUpgradeRequired(req.WebProxyAddr, req.Insecure)

	if alpnUpgradeRequired {
		caCert, err := req.Client.GetClusterCACert(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		if ok := rootCAs.AppendCertsFromPEM(caCert.GetTLSCA()); !ok {
			return fmt.Errorf("failed to append cert from cluster's TLS CA Cert")
		}
	}

	address, err := utils.ParseAddr(req.WebProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	key, err := GenerateRSAKey()
	if err != nil {
		return trace.Wrap(err)
	}

	currentUser, err := req.Client.GetCurrentUser(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	certs, err := req.Client.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey:              key.MarshalSSHPublicKey(),
		Username:               currentUser.GetName(),
		Expires:                time.Now().Add(time.Minute).UTC(),
		ConnectionDiagnosticID: req.ConnectionDiagnosticID,
		RouteToDatabase:        req.RouteToDatabase,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	tlsCert, err := keys.X509KeyPair(certs.TLS, key.PrivateKeyPEM())
	if err != nil {
		return trace.BadParameter("failed to parse private key: %v", err)
	}

	lp, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		InsecureSkipVerify:      req.Insecure,
		RemoteProxyAddr:         req.WebProxyAddr,
		Protocols:               protocols,
		Listener:                req.Listener,
		ParentContext:           ctx,
		SNI:                     address.Host(),
		Certs:                   []tls.Certificate{tlsCert},
		RootCAs:                 rootCAs,
		ALPNConnUpgradeRequired: alpnUpgradeRequired,
		Middleware:              req.ProxyMiddleware,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		defer req.Listener.Close()
		if err := lp.Start(ctx); err != nil {
			log.WithError(err).Info("ALPN proxy stopped.")
		}
	}()

	return nil
}

// IdentityCheckerFunc defines function that validates an Identity.
type IdentityCheckerFunc func(*tlsca.Identity) error

// ALPNCertChecker implements alpnproxy.LocalProxyMiddleware.
// It has basic checks and supports adding custom checks based on the extracted Identity from the certificate.
type ALPNCertChecker struct {
	validators []IdentityCheckerFunc
}

// NewALPNCertChecker creates a new ALPNCertChecker.
func NewALPNCertChecker() *ALPNCertChecker {
	return &ALPNCertChecker{}
}

// WithIdentityCheckerFunc adds a new chcker func to the validation list.
func (c *ALPNCertChecker) WithIdentityCheckerFunc(f IdentityCheckerFunc) *ALPNCertChecker {
	c.validators = append(c.validators, f)
	return c
}

// OnNewConnection is a callback triggered when a new downstream connection is
// accepted by the local proxy.
func (c *ALPNCertChecker) OnNewConnection(ctx context.Context, lp *alpnproxy.LocalProxy, conn net.Conn) error {
	return trace.Wrap(c.checkCerts(lp))
}

// OnStart is a callback triggered when the local proxy starts.
func (c *ALPNCertChecker) OnStart(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	return trace.Wrap(c.checkCerts(lp))
}

// checkCerts checks if the local proxy TLS certs are configured and not expired
// It also runs the validations added with WithIdentityCheckerFunc
func (c *ALPNCertChecker) checkCerts(lp *alpnproxy.LocalProxy) error {
	log.Debug("checking local proxy database certs")
	certs := lp.GetCerts()
	if len(certs) == 0 {
		return trace.Wrap(trace.NotFound("local proxy has no TLS certificates configured"))
	}

	cert, err := utils.TLSCertToX509(certs[0])
	if err != nil {
		return trace.Wrap(err)
	}

	err = utils.VerifyCertificateExpiry(cert, clockwork.NewRealClock())
	if err != nil {
		return trace.Wrap(err)
	}

	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, v := range c.validators {
		if err := v(identity); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
