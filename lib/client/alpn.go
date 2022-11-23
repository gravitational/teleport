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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpn "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

// ALPNAuthClient contains the required methods to create a local ALPN proxy.
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
}

// ALPNAuthTunnelConfig contains the required fields used to create an authed ALPN Proxy
type ALPNAuthTunnelConfig struct {
	// Client is the client that's used to interact with the cluster and obtain Certificates.
	AuthClient ALPNAuthClient

	// Listener to be used to accept connections that will go trough the tunnel.
	Listener net.Listener

	// InsecureSkipTLSVerify turns off verification for x509 upstream ALPN proxy service certificate.
	InsecureSkipVerify bool

	// Protocol name
	// Examples for databases: "postgres", "mysql"
	// This protocol must map to a Teleport ALPN protocol [lib/srv/alpnproxy/common.alpnToALPNProtocol]
	Protocol string

	// WebProxyAddr is the proxy addr to
	WebProxyAddr string

	// ConnectionDiagnosticID contains the ID to be used to store Connection Diagnostic checks.
	// Can be empty.
	ConnectionDiagnosticID string

	// RouteToDatabase contains the destination server that must receive the connection.
	// Specific for database proxying.
	RouteToDatabase proto.RouteToDatabase
}

// RunALPNAuthTunnel runs a local authenticated ALPN proxy to another service.
// At least one Route (which defines the service) must be defined
func RunALPNAuthTunnel(ctx context.Context, cfg ALPNAuthTunnelConfig) error {
	alpnProtocol, err := alpn.ToALPNProtocol(cfg.Protocol)
	if err != nil {
		return trace.Wrap(err)
	}

	protocols := []alpn.Protocol{alpnProtocol}
	if alpn.HasPingSupport(alpnProtocol) {
		protocols = append(alpn.ProtocolsWithPing(alpnProtocol), protocols...)
	}

	pool := x509.NewCertPool()

	alpnUpgradeRequired := alpnproxy.IsALPNConnUpgradeRequired(cfg.WebProxyAddr, cfg.InsecureSkipVerify)

	if alpnUpgradeRequired {
		caCert, err := cfg.AuthClient.GetClusterCACert(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		if ok := pool.AppendCertsFromPEM(caCert.GetTLSCA()); !ok {
			return fmt.Errorf("failed to append cert from cluster's TLS CA Cert")
		}
	}

	address, err := utils.ParseAddr(cfg.WebProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	tlsCert, err := tlsCertForUser(ctx, cfg.AuthClient, cfg.RouteToDatabase, cfg.ConnectionDiagnosticID)
	if err != nil {
		return trace.BadParameter("failed to parse private key: %v", err)
	}

	lp, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		InsecureSkipVerify:      cfg.InsecureSkipVerify,
		RemoteProxyAddr:         cfg.WebProxyAddr,
		Protocols:               protocols,
		Listener:                cfg.Listener,
		ParentContext:           ctx,
		SNI:                     address.Host(),
		Certs:                   []tls.Certificate{tlsCert},
		RootCAs:                 pool,
		ALPNConnUpgradeRequired: alpnUpgradeRequired,
		Middleware:              nil,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		defer cfg.Listener.Close()
		if err := lp.Start(ctx); err != nil {
			log.WithError(err).Info("ALPN proxy stopped.")
		}
	}()

	return nil
}

func tlsCertForUser(ctx context.Context, client ALPNAuthClient, routeToDatabase proto.RouteToDatabase, connectionDiagnosticID string) (tls.Certificate, error) {
	key, err := GenerateRSAKey()
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	currentUser, err := client.GetCurrentUser(ctx)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	certs, err := client.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey:              key.MarshalSSHPublicKey(),
		Username:               currentUser.GetName(),
		Expires:                time.Now().Add(time.Minute).UTC(),
		ConnectionDiagnosticID: connectionDiagnosticID,
		RouteToDatabase:        routeToDatabase,
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	tlsCert, err := keys.X509KeyPair(certs.TLS, key.PrivateKeyPEM())
	if err != nil {
		return tls.Certificate{}, trace.BadParameter("failed to parse private key: %v", err)
	}

	return tlsCert, err
}
