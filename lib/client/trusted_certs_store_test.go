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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/fixtures"
)

func newTestFSTrustedCertsStore(t *testing.T) TrustedCertsStore {
	fsTrustedCertsStore := NewFSTrustedCertsStore(t.TempDir())
	return fsTrustedCertsStore
}

func testEachTrustedCertsStore(t *testing.T, testFunc func(t *testing.T, TrustedCertsStore TrustedCertsStore)) {
	t.Run("FS", func(t *testing.T) {
		testFunc(t, newTestFSTrustedCertsStore(t))
	})

	t.Run("Mem", func(t *testing.T) {
		testFunc(t, NewMemTrustedCertsStore())
	})
}

func TestTrustedCertsStore(t *testing.T) {
	t.Parallel()
	a := newTestAuthority(t)

	testEachTrustedCertsStore(t, func(t *testing.T, trustedCertsStore TrustedCertsStore) {
		t.Parallel()

		pemBytes, ok := fixtures.PEMBytes["rsa"]
		require.True(t, ok)

		ca, rootCluster, err := newSelfSignedCA(pemBytes, "root")
		require.NoError(t, err)
		_, rootClusterSecondCert, err := newSelfSignedCA(pemBytes, "root")
		require.NoError(t, err)
		_, leafCluster, err := newSelfSignedCA(pemBytes, "leaf")
		require.NoError(t, err)

		caHostKey, err := ssh.NewPublicKey(ca.Signer.Public())
		require.NoError(t, err)

		// Add trusted certs to the store.
		proxy := "proxy.example.com"
		trustedCerts := []authclient.TrustedCerts{
			{
				ClusterName:     rootCluster.ClusterName,
				TLSCertificates: append(rootCluster.TLSCertificates, rootClusterSecondCert.TLSCertificates...),
				AuthorizedKeys:  rootCluster.AuthorizedKeys,
			}, {
				ClusterName:     leafCluster.ClusterName,
				TLSCertificates: leafCluster.TLSCertificates,
				AuthorizedKeys:  leafCluster.AuthorizedKeys,
			},
		}
		err = trustedCertsStore.SaveTrustedCerts(proxy, trustedCerts)
		require.NoError(t, err)

		// GetTrustedCerts should return the trusted certs.
		retrievedTrustedCerts, err := trustedCertsStore.GetTrustedCerts(proxy)
		require.NoError(t, err)
		require.ElementsMatch(t, trustedCerts, retrievedTrustedCerts)

		// Check against duplicates (no change).
		err = trustedCertsStore.SaveTrustedCerts(proxy, trustedCerts)
		require.NoError(t, err)
		retrievedTrustedCerts, err = trustedCertsStore.GetTrustedCerts(proxy)
		require.NoError(t, err)
		require.ElementsMatch(t, trustedCerts, retrievedTrustedCerts)

		// GetTrustedCerts with a different proxy should not return the trusted certs.
		retrievedTrustedCerts, err = trustedCertsStore.GetTrustedCerts("other.example.com")
		require.NoError(t, err)
		require.Empty(t, retrievedTrustedCerts)

		// GetTrustedCertsPEM should returns the trusted TLS certificates.
		retrievedCerts, err := trustedCertsStore.GetTrustedCertsPEM(proxy)
		require.NoError(t, err)
		expectCerts := append(
			append(
				rootCluster.TLSCertificates,
				rootClusterSecondCert.TLSCertificates...),
			leafCluster.TLSCertificates...,
		)
		require.ElementsMatch(t, expectCerts, retrievedCerts)

		// GetTrustedHostKeys should return each CA's public host key. We should
		// find a host key for each cluster, which in this case is the same host key.
		hostKeys, err := trustedCertsStore.GetTrustedHostKeys(rootCluster.ClusterName, leafCluster.ClusterName)
		require.NoError(t, err)
		require.ElementsMatch(t, []ssh.PublicKey{caHostKey, caHostKey}, hostKeys)

		// Saving a new trusted certs entry should overwrite existing TLS certificates.
		// Host keys shouldn't be overwritten.
		err = trustedCertsStore.SaveTrustedCerts(proxy, []authclient.TrustedCerts{
			{
				ClusterName:     rootCluster.ClusterName,
				TLSCertificates: rootCluster.TLSCertificates,
			},
		})
		require.NoError(t, err)
		trustedCerts[0].TLSCertificates = rootCluster.TLSCertificates
		retrievedTrustedCerts, err = trustedCertsStore.GetTrustedCerts(proxy)
		require.NoError(t, err)
		require.ElementsMatch(t, trustedCerts, retrievedTrustedCerts)

		// Adding a new trusted certs with host keys should append to existing entry.
		// TLS certs shouldn't be overwritten if not provided.
		_, publicKey, err := a.keygen.GenerateKeyPair()
		require.NoError(t, err)
		sshPub, _, _, _, err := ssh.ParseAuthorizedKey(publicKey)
		require.NoError(t, err)
		trustedCertsStore.SaveTrustedCerts(proxy, []authclient.TrustedCerts{{
			ClusterName:    rootCluster.ClusterName,
			AuthorizedKeys: [][]byte{ssh.MarshalAuthorizedKey(sshPub)},
		}})
		require.NoError(t, err)
		trustedCerts[0].AuthorizedKeys = append(trustedCerts[0].AuthorizedKeys, ssh.MarshalAuthorizedKey(sshPub))
		retrievedTrustedCerts, err = trustedCertsStore.GetTrustedCerts(proxy)
		require.NoError(t, err)
		require.ElementsMatch(t, trustedCerts, retrievedTrustedCerts)
	})
}

func TestAddTrustedHostKeys(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)

	testEachClientStore(t, func(t *testing.T, clientStore *Store) {
		t.Parallel()

		pub, _, _, _, err := ssh.ParseAuthorizedKey(CAPub)
		require.NoError(t, err)

		_, p2, _ := auth.keygen.GenerateKeyPair()
		pub2, _, _, _, _ := ssh.ParseAuthorizedKey(p2)

		err = clientStore.AddTrustedHostKeys("proxy.example.com", "root", pub)
		require.NoError(t, err)
		err = clientStore.AddTrustedHostKeys("proxy.example.com", "root", pub2)
		require.NoError(t, err)
		err = clientStore.AddTrustedHostKeys("leaf.example.com", "leaf", pub2)
		require.NoError(t, err)

		keys, err := clientStore.GetTrustedHostKeys()
		require.NoError(t, err)
		require.Len(t, keys, 3)
		require.ElementsMatch(t, keys, []ssh.PublicKey{pub, pub2, pub2})

		// check against dupes:
		before, _ := clientStore.GetTrustedHostKeys()
		err = clientStore.AddTrustedHostKeys("leaf.example.com", "leaf", pub2)
		require.NoError(t, err)
		err = clientStore.AddTrustedHostKeys("leaf.example.com", "leaf", pub2)
		require.NoError(t, err)
		after, _ := clientStore.GetTrustedHostKeys()
		require.Len(t, before, len(after))

		// check by hostname:
		keys, _ = clientStore.GetTrustedHostKeys("nocluster")
		require.Empty(t, keys)
		keys, _ = clientStore.GetTrustedHostKeys("leaf")
		require.Len(t, keys, 1)
		require.True(t, apisshutils.KeysEqual(keys[0], pub2))

		// check for proxy and wildcard as well:
		keys, _ = clientStore.GetTrustedHostKeys("leaf.example.com")
		require.Len(t, keys, 1)
		require.True(t, apisshutils.KeysEqual(keys[0], pub2))
		keys, _ = clientStore.GetTrustedHostKeys("*.leaf")
		require.Len(t, keys, 1)
		require.True(t, apisshutils.KeysEqual(keys[0], pub2))
		keys, _ = clientStore.GetTrustedHostKeys("prefix.leaf")
		require.Len(t, keys, 1)
		require.True(t, apisshutils.KeysEqual(keys[0], pub2))
	})
}

// Test that we can write keys to known_hosts in parallel without corrupting
// content of the file when using file based client store.
func TestParallelKnownHostsFileWrite(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)
	clientStore := newTestFSClientStore(t)

	pub, _, _, _, err := ssh.ParseAuthorizedKey(CAPub)
	require.NoError(t, err)

	err = clientStore.AddTrustedHostKeys("proxy.example1.com", "example1.com", pub)
	require.NoError(t, err)

	_, p2, _ := auth.keygen.GenerateKeyPair()
	tmpPub, _, _, _, _ := ssh.ParseAuthorizedKey(p2)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			err := clientStore.AddTrustedHostKeys("proxy.example2.com", "example2.com", tmpPub)
			assert.NoError(t, err)

			_, err = clientStore.GetTrustedHostKeys("")
			assert.NoError(t, err)

			wg.Done()
		}()
	}

	wg.Wait()

	keys, err := clientStore.GetTrustedHostKeys()
	require.NoError(t, err)
	require.Len(t, keys, 2)
}
