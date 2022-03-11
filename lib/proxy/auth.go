// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/credentials"
)

// newProxyCredentials creates new proxyCredentials from the given transport credentials.
func newProxyCredentials(creds credentials.TransportCredentials) credentials.TransportCredentials {
	return &proxyCredentials{
		creds,
	}
}

// proxyCredentials wraps TransportCredentials server and client handshakes
// to ensure the credentials contain the proxy system role.
type proxyCredentials struct {
	credentials.TransportCredentials
}

// ServerHandshake wraps a server handshake with an additional check for the
// proxy role.
func (c *proxyCredentials) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, authInfo, err := c.TransportCredentials.ServerHandshake(conn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	err = checkProxyRole(authInfo)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return conn, authInfo, nil
}

// ClientHandshake wraps a client handshake with an additional check for the
// proxy role.
func (c *proxyCredentials) ClientHandshake(ctx context.Context, laddr string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, authInfo, err := c.TransportCredentials.ClientHandshake(ctx, laddr, conn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	err = checkProxyRole(authInfo)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return conn, authInfo, nil
}

// checkProxyRole checks the authInfo for a certificate with the role types.RoleProxy.
func checkProxyRole(authInfo credentials.AuthInfo) error {
	tlsInfo, ok := authInfo.(credentials.TLSInfo)
	if !ok {
		return trace.AccessDenied("missing authentication")
	}

	certs := tlsInfo.State.PeerCertificates
	if len(certs) == 0 {
		return trace.AccessDenied("missing authentication")
	}

	clientCert := certs[0]
	identity, err := tlsca.FromSubject(clientCert.Subject, clientCert.NotAfter)
	if err != nil {
		return trace.Wrap(err)
	}

	// Ensure the proxy system role is present.
	for _, role := range identity.Groups {
		if types.SystemRole(role) == types.RoleProxy {
			return nil
		}
	}

	return trace.AccessDenied("proxy system role required")
}

// getConfigForClient clones and updates the server's tls config with the
// appropriate client certificate authorities.
func getConfigForClient(tlsConfig *tls.Config, ap auth.AccessCache, log logrus.FieldLogger) func(*tls.ClientHelloInfo) (*tls.Config, error) {
	return func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		pool, err := getCertPool(ap)
		if err != nil {
			log.WithError(err).Error("Failed to retrieve client CA pool.")
			return tlsConfig, nil
		}

		tlsCopy := tlsConfig.Clone()
		tlsCopy.ClientAuth = tls.RequireAndVerifyClientCert
		tlsCopy.ClientCAs = pool
		return tlsCopy, nil
	}
}

// getConfigForServer clones and updates the client's tls config with the
// appropriate server certificate authorities.
func getConfigForServer(tlsConfig *tls.Config, ap auth.AccessCache, log logrus.FieldLogger) func() (*tls.Config, error) {
	return func() (*tls.Config, error) {
		pool, err := getCertPool(ap)
		if err != nil {
			log.WithError(err).Error("Failed to retrieve server CA pool.")
			return tlsConfig, nil
		}

		tlsCopy := tlsConfig.Clone()
		tlsCopy.RootCAs = pool
		return tlsCopy, nil
	}
}

// getCertPool returns a new cert pool from cache if any.
func getCertPool(ap auth.AccessCache) (*x509.CertPool, error) {
	clusterName, err := ap.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool, err := auth.ClientCertPool(ap, clusterName.GetClusterName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return pool, nil
}
