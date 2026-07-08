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
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
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
				ClientStore:    clientStore,
				SSHProxyAddr:   "localhost:3080",
				WebProxyAddr:   "localhost:3080",
				Username:       "testuser",
				Tracer:         tracing.NoopProvider().Tracer("test"),
				SiteName:       "root",
				AddKeysToAgent: client.AddKeysToAgentNo,
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
	err = cache.ClearStaleClientsForRoot("root")
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

	// Verify that the client is considered "stale" after logging out.
	err = clientStore.DeleteKeyRing(keyRing.KeyRingIndex)
	require.NoError(t, err)
	err = cache.ClearStaleClientsForRoot("root")
	require.NoError(t, err)
	_, err = cache.Get(t.Context(), "root", "")
	// Getting the client should return an error because we are logged out.
	require.ErrorContains(t, err, "are you logged in?")
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

// TestGetClosesSystemAgentConnection verifies that the cache closes the
// connection each built client opens to the system SSH agent, both when the
// build fails and when a cached client is evicted. Leaking these connections
// (as happened while VNet rebuilt clients on a loop with expired certs)
// eventually exhausts the system agent and wedges every caller on an
// unresponsive agent.List.
func TestGetClosesSystemAgentConnection(t *testing.T) {
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
	require.NoError(t, clientStore.SaveProfile(profile, true))
	require.NoError(t, clientStore.AddKeyRing(keyRing))

	// AddKeysToAgentYes makes each built client open a connection to the system
	// agent advertised by SSH_AUTH_SOCK.
	newClientFunc := func(ctx context.Context, profileName, leafClusterName string) (*client.TeleportClient, error) {
		config := &client.Config{
			ClientStore:    clientStore,
			SSHProxyAddr:   "localhost:3080",
			WebProxyAddr:   "localhost:3080",
			Username:       "testuser",
			Tracer:         tracing.NoopProvider().Tracer("test"),
			SiteName:       "root",
			AddKeysToAgent: client.AddKeysToAgentYes,
		}
		if leafClusterName != "" {
			config.SiteName = leafClusterName
		}
		return client.NewClient(config)
	}

	t.Run("connection is closed when building the client fails", func(t *testing.T) {
		conns := startFakeSystemAgent(t)
		cache, err := New(Config{
			NewClientFunc: newClientFunc,
			// Fail the build after the TeleportClient (and its system agent
			// connection) has been created, as happens with expired certs.
			RetryWithReloginFunc: func(ctx context.Context, tc *client.TeleportClient, fn func() error, opts ...client.RetryWithReloginOption) error {
				return trace.AccessDenied("cluster unreachable")
			},
			Logger: slog.New(slog.DiscardHandler),
		})
		require.NoError(t, err)

		_, err = cache.Get(t.Context(), "root", "")
		require.Error(t, err)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			// The failed build opened a connection to the system agent, and the
			// cache must have closed it again.
			assert.Positive(t, conns.opened.Load(), "the failed build should have opened a system agent connection")
			assert.Zero(t, conns.open.Load(), "the system agent connection of a client the cache fails to build must be closed")
		}, 5*time.Second, 10*time.Millisecond)
	})

	t.Run("connection is closed when the cache is cleared", func(t *testing.T) {
		conns := startFakeSystemAgent(t)
		cache, err := New(Config{
			NewClientFunc: newClientFunc,
			RetryWithReloginFunc: func(ctx context.Context, tc *client.TeleportClient, fn func() error, opts ...client.RetryWithReloginOption) error {
				return fn()
			},
			Logger: slog.New(slog.DiscardHandler),
		})
		require.NoError(t, err)

		_, err = cache.Get(t.Context(), "root", "")
		require.NoError(t, err)
		require.Positive(t, conns.open.Load(), "building the client should have opened a system agent connection")

		require.NoError(t, cache.Clear())

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.Zero(t, conns.open.Load())
		}, 5*time.Second, 10*time.Millisecond,
			"Clear must close the system agent connections of cached clients")
	})
}

// systemAgentConns counts connections made to a fake system SSH agent: open is
// the number currently established, opened the number ever established.
type systemAgentConns struct {
	open   atomic.Int64
	opened atomic.Int64
}

// startFakeSystemAgent starts an SSH agent on a unix socket, points
// SSH_AUTH_SOCK at it, and returns counters over the client connections it
// receives so a test can assert those connections get closed.
func startFakeSystemAgent(t *testing.T) *systemAgentConns {
	t.Helper()

	// Use a short temp dir rather than t.TempDir: the latter embeds the (long)
	// test name in the path, and unix socket paths are capped at 104 chars on
	// macOS.
	socketDir, err := os.MkdirTemp("", "teleport-test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(socketDir) })

	socketPath := filepath.Join(socketDir, "agent.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })
	t.Setenv(teleport.SSHAuthSock, socketPath)

	conns := &systemAgentConns{}
	keyring := agent.NewKeyring()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conns.open.Add(1)
			conns.opened.Add(1)
			go func() {
				defer conns.open.Add(-1)
				// ServeAgent returns once the client closes the connection.
				_ = agent.ServeAgent(keyring, conn)
			}()
		}
	}()
	return conns
}
