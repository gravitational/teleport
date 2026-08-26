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

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpn "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

// ALPNAuthClient contains the required auth.ClientI methods to create a local ALPN proxy.
type ALPNAuthClient interface {
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

	// GetAuthPreference returns the current cluster auth preference.
	GetAuthPreference(context.Context) (types.AuthPreference, error)
}

// ALPNAuthTunnelConfig contains the required fields used to create an authed ALPN Proxy
type ALPNAuthTunnelConfig struct {
	// MFAAuthenticateResponse is a response to MFAAuthenticateChallenge using one
	// of the MFA devices registered for a user.
	MFAResponse *proto.MFAAuthenticateResponse

	// AuthClient is the client that's used to interact with the cluster and obtain Certificates.
	AuthClient ALPNAuthClient

	// Listener to be used to accept connections that will go trough the tunnel.
	Listener net.Listener

	// InsecureSkipTLSVerify turns off verification for x509 upstream ALPN proxy service certificate.
	InsecureSkipVerify bool

	// Expires is a desired time of the expiry of the certificate.
	Expires time.Time

	// Protocol name.
	Protocol alpn.Protocol

	// PublicProxyAddr is public address of the proxy
	PublicProxyAddr string

	// ConnectionDiagnosticID contains the ID to be used to store Connection Diagnostic checks.
	// Can be empty.
	ConnectionDiagnosticID string

	// RouteToDatabase contains the destination server that must receive the connection.
	// Specific for database proxying.
	RouteToDatabase proto.RouteToDatabase

	// TLSCert specifies the TLS certificate used on the proxy connection.
	TLSCert *tls.Certificate
}

func (c *ALPNAuthTunnelConfig) CheckAndSetDefaults(ctx context.Context) error {
	if c.AuthClient == nil {
		return trace.BadParameter("missing auth client")
	}

	if c.TLSCert == nil {
		tlsCert, err := getUserCerts(ctx, c.AuthClient, c.MFAResponse, c.Expires, c.RouteToDatabase, c.ConnectionDiagnosticID)
		if err != nil {
			return trace.BadParameter("failed to parse private key: %v", err)
		}

		c.TLSCert = &tlsCert
	}

	return nil
}

// RunALPNAuthTunnel runs a local authenticated ALPN proxy to another service.
// At least one Route (which defines the service) must be defined
func RunALPNAuthTunnel(ctx context.Context, cfg ALPNAuthTunnelConfig) error {
	if err := cfg.CheckAndSetDefaults(ctx); err != nil {
		return trace.Wrap(err)
	}

	lp, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		RemoteProxyAddr:    cfg.PublicProxyAddr,
		Protocols:          []alpn.Protocol{cfg.Protocol},
		Listener:           cfg.Listener,
		ParentContext:      ctx,
		Cert:               *cfg.TLSCert,
	}, alpnproxy.WithALPNConnUpgradeTest(ctx, getClusterCACertPool(cfg.AuthClient)))
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		defer cfg.Listener.Close()
		if err := lp.Start(ctx); err != nil {
			log.InfoContext(ctx, "ALPN proxy stopped", "error", err)
		}
	}()

	return nil
}

func getUserCerts(ctx context.Context, client ALPNAuthClient, mfaResponse *proto.MFAAuthenticateResponse, expires time.Time, routeToDatabase proto.RouteToDatabase, connectionDiagnosticID string) (tls.Certificate, error) {
	key, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(client),
		cryptosuites.UserTLS)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	publicKeyPEM, err := keys.MarshalPublicKey(key.Public())
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	currentUser, err := client.GetCurrentUser(ctx)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	certs, err := client.GenerateUserCerts(ctx, proto.UserCertsRequest{
		TLSPublicKey:           publicKeyPEM,
		Username:               currentUser.GetName(),
		Expires:                expires,
		ConnectionDiagnosticID: connectionDiagnosticID,
		RouteToDatabase:        routeToDatabase,
		MFAResponse:            mfaResponse,
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	tlsCert, err := keys.TLSCertificateForSigner(key, certs.TLS)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	return tlsCert, nil
}

func getClusterCACertPool(authClient ALPNAuthClient) alpnproxy.GetClusterCACertPoolFunc {
	return func(ctx context.Context) (*x509.CertPool, error) {
		caCert, err := authClient.GetClusterCACert(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		pool := x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM(caCert.GetTLSCA()); !ok {
			return nil, trace.BadParameter("failed to append cert from cluster's TLS CA Cert")
		}
		return pool, nil
	}
}
