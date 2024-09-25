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
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/plugin"
)

// MinimalAuthHelper implements the AuthHelper interface.
// It starts an Auth server and a TLS server. This is not a full-featured
// Teleport process.
type MinimalAuthHelper struct {
	server *libauth.TestTLSServer
	// dir is where we put identity files, and start the auth server
	// (unless AuthConfig.Dir is manually set).
	dir            string
	AuthConfig     libauth.TestAuthServerConfig
	PluginRegistry plugin.Registry
}

// StartServer implements the AuthHelper interface.
// The function takes care of registering client and server close function
// on the t.Cleanup() stack.
func (a *MinimalAuthHelper) StartServer(t *testing.T) *client.Client {
	a.dir = t.TempDir()
	if a.AuthConfig.Dir == "" {
		a.AuthConfig.Dir = a.dir
	}

	// If there's no auth preference spec, we turn 2FA on as it is mandatory since v16.
	if a.AuthConfig.AuthPreferenceSpec == nil {
		a.AuthConfig.AuthPreferenceSpec = &types.AuthPreferenceSpecV2{
			SecondFactor: constants.SecondFactorOn,
			Webauthn: &types.Webauthn{
				RPID: "localhost",
			},
		}
	}

	authServer, err := libauth.NewTestAuthServer(a.AuthConfig)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, authServer.Close())
	})

	server, err := libauth.NewTestTLSServer(libauth.TestTLSServerConfig{
		APIConfig: &libauth.APIConfig{
			AuthServer:     authServer.AuthServer,
			Authorizer:     authServer.Authorizer,
			AuditLog:       authServer.AuditLog,
			Emitter:        authServer.AuditLog,
			PluginRegistry: a.PluginRegistry,
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
func (a *MinimalAuthHelper) ServerAddr() string {
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
func (a *MinimalAuthHelper) getUserCerts(t *testing.T, user types.User) userCerts {
	auth := a.server.Auth()

	clusterName, err := auth.GetClusterName()
	require.NoError(t, err)
	// Get user certs
	userKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(userKey.Public())
	require.NoError(t, err)
	tlsPub, err := keys.MarshalPublicKey(userKey.Public())
	require.NoError(t, err)
	testCertsReq := libauth.GenerateUserTestCertsRequest{
		SSHPubKey:      ssh.MarshalAuthorizedKey(sshPub),
		TLSPubKey:      tlsPub,
		Username:       user.GetName(),
		TTL:            time.Hour,
		Compatibility:  constants.CertificateFormatStandard,
		RouteToCluster: clusterName.GetClusterName(),
	}
	sshCert, tlsCert, err := auth.GenerateUserTestCerts(testCertsReq)
	require.NoError(t, err)

	// Build credentials from the certs
	keyPEM, err := keys.MarshalPrivateKey(userKey)
	require.NoError(t, err)

	return userCerts{keyPEM, sshCert, tlsCert}
}

// CredentialsForUser implements the AuthHelper interface.
// It builds TLS client credentials for the given user.
func (a *MinimalAuthHelper) CredentialsForUser(t *testing.T, ctx context.Context, user types.User) client.Credentials {
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
func (a *MinimalAuthHelper) SignIdentityForUser(t *testing.T, ctx context.Context, user types.User) string {
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

func (a *MinimalAuthHelper) Auth() *libauth.Server {
	return a.server.Auth()
}
