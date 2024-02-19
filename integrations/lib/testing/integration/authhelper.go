/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package integration

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	libauth "github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
)

// OSSAuthHelper implements the AuthHelper interface.
// It starts an OSS Auth server and exposes the functions required for plugin
// integration tests to build teleport clients for each user/plugin/bot.
type OSSAuthHelper struct {
	server *libauth.TestTLSServer
	dir    string
}

// StartServer implements the AuthHelper interface.
// The function takes care of registering client and server close function
// on the t.Cleanup() stack.
func (a *OSSAuthHelper) StartServer(t *testing.T) *client.Client {
	a.dir = t.TempDir()
	authServer, err := libauth.NewTestAuthServer(libauth.TestAuthServerConfig{
		Dir: a.dir,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, authServer.Close())
	})

	server, err := libauth.NewTestTLSServer(libauth.TestTLSServerConfig{
		APIConfig: &libauth.APIConfig{
			AuthServer: authServer.AuthServer,
			Authorizer: authServer.Authorizer,
			AuditLog:   authServer.AuditLog,
			Emitter:    authServer.AuditLog,
		},
		AuthServer:    authServer,
		AcceptedUsage: authServer.AcceptedUsage,
	})
	require.NoError(t, err)
	a.server = server

	t.Cleanup(func() {
		require.NoError(t, server.Close())
	})

	authClient, err := server.NewClient(libauth.TestAdmin())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authClient.Close())
	})

	return authClient.APIClient
}

// ServerAddr implements the AuthHelper interface.
// It returns the server address, including the port.
// For example, "192.0.2.1:25" or "[2001:db8::1]:80"
func (a *OSSAuthHelper) ServerAddr() string {
	return a.server.Addr().String()
}

type userCerts struct {
	// PEM-encoded private key (the same private key is used for both SSH and TLS)
	privateKey []byte
	// SSH certs formatted following the Authorized-key format
	ssh []byte
	// PEM-encoded TLS certs
	tls []byte
}

// getUserCerts generates a private key for the user and has the auth sign a TLS and an SSH certificate.
// The function returns three values:
// - the PEM-encoded private key
// - the Authorized-key formatted SSH cert
// - the PEM-encoded TLS cert
func (a *OSSAuthHelper) getUserCerts(t *testing.T, user types.User) userCerts {
	auth := a.server.Auth()

	clusterName, err := auth.GetClusterName()
	require.NoError(t, err)
	// Get user certs
	userKey, err := native.GenerateRSAPrivateKey()
	require.NoError(t, err)
	userPubKey, err := ssh.NewPublicKey(&userKey.PublicKey)
	require.NoError(t, err)
	testCertsReq := libauth.GenerateUserTestCertsRequest{
		Key:            ssh.MarshalAuthorizedKey(userPubKey),
		Username:       user.GetName(),
		TTL:            time.Hour,
		Compatibility:  constants.CertificateFormatStandard,
		RouteToCluster: clusterName.GetClusterName(),
	}
	sshCert, tlsCert, err := auth.GenerateUserTestCerts(testCertsReq)
	require.NoError(t, err)

	// Build credentials from the certs
	pemKey := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(userKey),
		},
	)

	return userCerts{pemKey, sshCert, tlsCert}
}

// CredentialsForUser implements the AuthHelper interface.
// It builds TLS client credentials for the given user.
func (a *OSSAuthHelper) CredentialsForUser(t *testing.T, ctx context.Context, user types.User) client.Credentials {
	auth := a.server.Auth()
	clusterName, err := auth.GetClusterName()
	require.NoError(t, err)

	certs := a.getUserCerts(t, user)
	cert, err := keys.X509KeyPair(certs.tls, certs.privateKey)
	require.NoError(t, err)

	pool := x509.NewCertPool()
	ca, err := auth.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, false)
	require.NoError(t, err)

	tlsKeys := ca.GetActiveKeys().TLS
	require.NotEmpty(t, tlsKeys)
	for _, key := range tlsKeys {
		pool.AppendCertsFromPEM(key.Cert)
	}

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}
	return client.LoadTLS(tlsConf)
}

// SignIdentityForUser implements the AuthHelper interface.
// It signs an identity, write it to a temporary directory, and returns its path.
func (a *OSSAuthHelper) SignIdentityForUser(t *testing.T, ctx context.Context, user types.User) string {
	auth := a.server.Auth()
	clusterName, err := auth.GetClusterName()
	require.NoError(t, err)

	certs := a.getUserCerts(t, user)

	var tlsCAs, sshCAs [][]byte
	ca, err := auth.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, false)
	require.NoError(t, err)

	activeKeys := ca.GetActiveKeys()
	require.NotEmpty(t, activeKeys)

	for _, key := range activeKeys.TLS {
		tlsCAs = append(tlsCAs, key.Cert)
	}
	for _, key := range activeKeys.SSH {
		sshCAs = append(sshCAs, key.PublicKey)
	}

	id := &identityfile.IdentityFile{
		PrivateKey: certs.privateKey,
		Certs: identityfile.Certs{
			TLS: certs.tls,
			SSH: certs.ssh,
		},
		CACerts: identityfile.CACerts{
			TLS: tlsCAs,
			SSH: sshCAs,
		},
	}

	path := fmt.Sprintf("%s/%s-identity.pem", a.dir, user.GetName())
	require.NoError(t, identityfile.Write(id, path))
	return path
}
