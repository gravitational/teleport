// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package clientcache

import (
	"context"
	"crypto/x509/pkix"
	"log/slog"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	apiprofile "github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestClearingClientsWithStaleCert(t *testing.T) {
	privateKey, err := keys.ParsePrivateKey(fixtures.PEMBytes["rsa"])
	require.NoError(t, err)
	tlsCert, sshCert, err := makeCerts(privateKey)
	require.NoError(t, err)

	keyRing := client.NewKeyRing(privateKey, privateKey)
	keyRing.KeyRingIndex = client.KeyRingIndex{
		ProxyHost:   "localhost",
		Username:    "testuser",
		ClusterName: "root",
	}
	keyRing.Cert = sshCert
	keyRing.TLSCert = tlsCert

	profile := &apiprofile.Profile{
		WebProxyAddr: keyRing.ProxyHost,
		Username:     keyRing.Username,
		SiteName:     keyRing.ClusterName,
	}
	clientStore := client.NewFSClientStore(t.TempDir())
	err = clientStore.SaveProfile(profile, true)
	require.NoError(t, err)
	err = clientStore.AddKeyRing(keyRing)
	require.NoError(t, err)

	cache, err := New(Config{
		NewClientFunc: func(ctx context.Context, profileName, leafClusterName string) (*client.TeleportClient, error) {
			config := &client.Config{
				ClientStore:  clientStore,
				SSHProxyAddr: "localhost:3080",
				WebProxyAddr: "localhost:3080",
				Username:     "testuser",
				Tracer:       tracing.NoopProvider().Tracer("test"),
				SiteName:     "root",
			}
			if leafClusterName != "" {
				config.SiteName = leafClusterName
			}
			tc, err := client.NewClient(config)

			return tc, err
		},
		RetryWithReloginFunc: func(ctx context.Context, tc *client.TeleportClient, fn func() error, opts ...client.RetryWithReloginOption) error {
			return fn()
		},
		Logger: slog.New(slog.DiscardHandler),
	})
	require.NoError(t, err)

	// Get clients.
	rootClient, err := cache.Get(t.Context(), "root", "")
	require.NoError(t, err)
	leaf1Client, err := cache.Get(t.Context(), "root", "leaf1")
	require.NoError(t, err)

	// Update the TLS cert.
	tlsCert, _, err = makeCerts(privateKey)
	require.NoError(t, err)
	keyRing.TLSCert = tlsCert
	err = clientStore.AddKeyRing(keyRing)
	require.NoError(t, err)

	// Get the client for a new leaf after the cert has been updated.
	leaf2Client, err := cache.Get(t.Context(), "root", "leaf2")
	require.NoError(t, err)

	// Clear stale clients.
	err = cache.ClearForRoot("root", WithClearingOnlyClientsWithStaleCert())
	require.NoError(t, err)

	newRootClient, err := cache.Get(t.Context(), "root", "")
	require.NoError(t, err)
	newLeaf1Client, err := cache.Get(t.Context(), "root", "leaf1")
	require.NoError(t, err)
	newLeaf2Client, err := cache.Get(t.Context(), "root", "leaf2")
	require.NoError(t, err)
	// Clients opened before updating the cert should be reopened.
	require.NotEqual(t, newRootClient, rootClient)
	require.NotEqual(t, newLeaf1Client, leaf1Client)
	// The client opened after updating the cert should be untouched.
	require.Equal(t, newLeaf2Client, leaf2Client)
}

// makeCerts makes TSL and SSH certs.
func makeCerts(privateKey *keys.PrivateKey) ([]byte, []byte, error) {
	cert, err := tlsca.GenerateSelfSignedCAWithSigner(privateKey, pkix.Name{
		CommonName:   "root",
		Organization: []string{"localhost"},
	}, nil, defaults.CATTL)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	ca, err := tlsca.FromCertAndSigner(cert, privateKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	clock := clockwork.NewRealClock()
	identity := tlsca.Identity{
		Username: "testuser",
	}

	subject, err := identity.Subject()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	tlsCert, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subject,
		NotAfter:  clock.Now().UTC().Add(defaults.CATTL),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	signer, err := keys.ParsePrivateKey([]byte(fixtures.SSHCAPrivateKey))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	caSigner, err := ssh.NewSignerFromKey(signer)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	sshCert, err := testauthority.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:      caSigner,
		PublicUserKey: ssh.MarshalAuthorizedKey(privateKey.SSHPublicKey()),
		Identity: sshca.Identity{
			Username:   "testuser",
			Principals: []string{"testuser"},
		},
	})

	return tlsCert, sshCert, trace.Wrap(err)
}
