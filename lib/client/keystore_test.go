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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/utils/cert"
)

func newTestFSKeyStore(t *testing.T) *FSKeyStore {
	fsKeyStore := NewFSKeyStore(t.TempDir())
	return fsKeyStore
}

func testEachKeyStore(t *testing.T, testFunc func(t *testing.T, keyStore KeyStore)) {
	t.Run("FS", func(t *testing.T) {
		testFunc(t, newTestFSKeyStore(t))
	})

	t.Run("Mem", func(t *testing.T) {
		testFunc(t, NewMemKeyStore())
	})
}

func TestKeyStore(t *testing.T) {
	t.Parallel()
	s := newTestAuthority(t)

	testEachKeyStore(t, func(t *testing.T, keyStore KeyStore) {
		t.Parallel()

		// create a test key
		idx := KeyRingIndex{"test.proxy.com", "test-user", "root"}
		keyRing := s.makeSignedKeyRing(t, idx, false)

		// add the test key to the memory store
		err := keyStore.AddKeyRing(keyRing)
		require.NoError(t, err)

		// check that the key exists in the store and is the same,
		// except the key's trusted certs should be empty, to be
		// filled in by a trusted certs store.
		retrievedKeyRing, err := keyStore.GetKeyRing(idx, WithAllCerts...)
		require.NoError(t, err)
		keyRing.TrustedCerts = nil
		assertEqualKeyRings(t, keyRing, retrievedKeyRing)

		// Delete just the db cred, reload & verify it's gone
		err = keyStore.DeleteUserCerts(idx, WithDBCerts{})
		require.NoError(t, err)
		retrievedKeyRing, err = keyStore.GetKeyRing(idx, WithSSHCerts{}, WithDBCerts{})
		require.NoError(t, err)
		expectKeyRing := keyRing.Copy()
		expectKeyRing.DBTLSCredentials = make(map[string]TLSCredential)
		assertEqualKeyRings(t, expectKeyRing, retrievedKeyRing)

		// check for the key, now without cluster name
		retrievedKeyRing, err = keyStore.GetKeyRing(KeyRingIndex{idx.ProxyHost, idx.Username, ""})
		require.NoError(t, err)
		expectKeyRing.ClusterName = ""
		expectKeyRing.Cert = nil
		assertEqualKeyRings(t, expectKeyRing, retrievedKeyRing)

		// delete the key
		err = keyStore.DeleteKeyRing(idx)
		require.NoError(t, err)

		// check that the key doesn't exist in the store
		retrievedKeyRing, err = keyStore.GetKeyRing(idx)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
		require.Nil(t, retrievedKeyRing)

		// Delete non-existing
		err = keyStore.DeleteKeyRing(idx)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}

func TestListKeys(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)

	testEachKeyStore(t, func(t *testing.T, keyStore KeyStore) {
		t.Parallel()
		const keyNum = 5

		// add 5 keys for "bob"
		keys := make([]KeyRing, keyNum)
		for i := 0; i < keyNum; i++ {
			idx := KeyRingIndex{fmt.Sprintf("host-%v", i), "bob", "root"}
			keyRing := auth.makeSignedKeyRing(t, idx, false)
			require.NoError(t, keyStore.AddKeyRing(keyRing))
			keys[i] = *keyRing
		}
		// add 1 key for "sam"
		samIdx := KeyRingIndex{"sam.host", "sam", "root"}
		samKeyRing := auth.makeSignedKeyRing(t, samIdx, false)
		require.NoError(t, keyStore.AddKeyRing(samKeyRing))

		// read all bob keys:
		for i := 0; i < keyNum; i++ {
			keyRing, err := keyStore.GetKeyRing(keys[i].KeyRingIndex, WithSSHCerts{}, WithDBCerts{})
			require.NoError(t, err)
			keyRing.TrustedCerts = keys[i].TrustedCerts
			assertEqualKeyRings(t, &keys[i], keyRing)
		}

		// read sam's key and make sure it's the same:
		skeyRing, err := keyStore.GetKeyRing(samIdx, WithSSHCerts{})
		require.NoError(t, err)
		require.Equal(t, samKeyRing.Cert, skeyRing.Cert)
		require.Equal(t, samKeyRing.TLSCert, skeyRing.TLSCert)
		require.Equal(t, samKeyRing.SSHPrivateKey.MarshalSSHPublicKey(), skeyRing.SSHPrivateKey.MarshalSSHPublicKey())
		require.Equal(t, samKeyRing.TLSPrivateKey.MarshalSSHPublicKey(), skeyRing.TLSPrivateKey.MarshalSSHPublicKey())
	})
}

func TestGetCertificates(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)

	testEachKeyStore(t, func(t *testing.T, keyStore KeyStore) {
		const keyNum = 3

		// add keys for 3 different clusters with the same user and proxy.
		keys := make([]KeyRing, keyNum)
		certs := make([]*ssh.Certificate, keyNum)
		var proxy = "proxy.example.com"
		var user = "bob"
		for i := 0; i < keyNum; i++ {
			idx := KeyRingIndex{proxy, user, fmt.Sprintf("cluster-%v", i)}
			keyRing := auth.makeSignedKeyRing(t, idx, false)
			err := keyStore.AddKeyRing(keyRing)
			require.NoError(t, err)
			keys[i] = *keyRing
			certs[i], err = keyRing.SSHCert()
			require.NoError(t, err)
		}

		retrievedCerts, err := keyStore.GetSSHCertificates(proxy, user)
		require.NoError(t, err)
		require.ElementsMatch(t, certs, retrievedCerts)
	})
}

func TestDeleteAll(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)

	testEachKeyStore(t, func(t *testing.T, keyStore KeyStore) {
		// generate keys
		idxFoo := KeyRingIndex{"proxy.example.com", "foo", "root"}
		keyFoo := auth.makeSignedKeyRing(t, idxFoo, false)
		idxBar := KeyRingIndex{"proxy.example.com", "bar", "root"}
		keyBar := auth.makeSignedKeyRing(t, idxBar, false)

		// add keys
		err := keyStore.AddKeyRing(keyFoo)
		require.NoError(t, err)
		err = keyStore.AddKeyRing(keyBar)
		require.NoError(t, err)

		// check keys exist
		_, err = keyStore.GetKeyRing(idxFoo)
		require.NoError(t, err)
		_, err = keyStore.GetKeyRing(idxBar)
		require.NoError(t, err)

		// delete all keys
		err = keyStore.DeleteKeys()
		require.NoError(t, err)

		// verify keys are gone
		_, err = keyStore.GetKeyRing(idxFoo)
		require.True(t, trace.IsNotFound(err))
		_, err = keyStore.GetKeyRing(idxBar)
		require.Error(t, err)
	})
}

// TestCheckKey makes sure Teleport clients can load non-RSA algorithms in
// normal operating mode.
func TestCheckKey(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)

	testEachKeyStore(t, func(t *testing.T, keyStore KeyStore) {
		idx := KeyRingIndex{"host.a", "bob", "root"}
		keyRing := auth.makeSignedKeyRing(t, idx, false)

		// Swap out the key with a ECDSA SSH key.
		ellipticCertificate, _, err := cert.CreateTestECDSACertificate("foo", ssh.UserCert)
		require.NoError(t, err)
		keyRing.Cert = ssh.MarshalAuthorizedKey(ellipticCertificate)

		err = keyStore.AddKeyRing(keyRing)
		require.NoError(t, err)

		_, err = keyStore.GetKeyRing(idx)
		require.NoError(t, err)
	})
}

// TestCheckKeyFIPS makes sure Teleport clients don't load invalid
// certificates while in FIPS mode.
func TestCheckKeyFIPS(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)

	// This test only runs in FIPS mode.
	if !isFIPS() {
		t.Skip("This test only runs in FIPS mode.")
	}

	testEachKeyStore(t, func(t *testing.T, keyStore KeyStore) {
		idx := KeyRingIndex{"host.a", "bob", "root"}
		keyRing := auth.makeSignedKeyRing(t, idx, false)

		// Swap out the key with a ECDSA SSH key.
		ellipticCertificate, _, err := cert.CreateTestECDSACertificate("foo", ssh.UserCert)
		require.NoError(t, err)
		keyRing.Cert = ssh.MarshalAuthorizedKey(ellipticCertificate)

		err = keyStore.AddKeyRing(keyRing)
		require.NoError(t, err)

		// Should return trace.BadParameter error because only RSA keys are supported.
		_, err = keyStore.GetKeyRing(idx)
		require.True(t, trace.IsBadParameter(err))
	})
}

func TestAddKey_withoutSSHCert(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)
	keyStore := newTestFSKeyStore(t)

	// without ssh cert, db certs only
	idx := KeyRingIndex{"host.a", "bob", "root"}
	keyRing := auth.makeSignedKeyRing(t, idx, false)
	keyRing.Cert = nil
	require.NoError(t, keyStore.AddKeyRing(keyRing))

	// ssh cert path should NOT exist
	sshCertPath := keyStore.sshCertPath(keyRing.KeyRingIndex)
	_, err := os.Stat(sshCertPath)
	require.ErrorIs(t, err, os.ErrNotExist)

	// check db creds
	keyCopy, err := keyStore.GetKeyRing(idx, WithDBCerts{})
	require.NoError(t, err)
	require.Len(t, keyCopy.DBTLSCredentials, 1)
}

func TestProtectedDirsNotDeleted(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)
	keyStore := newTestFSKeyStore(t)

	idx := KeyRingIndex{"host.a", "bob", "root"}
	keyStore.AddKeyRing(auth.makeSignedKeyRing(t, idx, false))

	configPath := filepath.Join(keyStore.KeyDir, "config")
	require.NoError(t, os.Mkdir(configPath, 0700))

	azurePath := filepath.Join(keyStore.KeyDir, "azure")
	require.NoError(t, os.Mkdir(azurePath, 0700))

	binPath := filepath.Join(keyStore.KeyDir, "bin")
	require.NoError(t, os.Mkdir(binPath, 0700))

	testPath := filepath.Join(keyStore.KeyDir, "test")
	require.NoError(t, os.Mkdir(testPath, 0700))

	require.NoError(t, keyStore.DeleteKeys())
	require.DirExists(t, configPath)
	require.DirExists(t, azurePath)
	require.DirExists(t, binPath)
	require.NoDirExists(t, testPath)

	require.NoDirExists(t, filepath.Join(keyStore.KeyDir, "keys"))
}

func assertEqualKeyRings(t *testing.T, expected, actual *KeyRing) {
	t.Helper()
	// Ignore differences in unexported private key fields, for example keyPEM
	// may change after being serialized in OpenSSH format and then deserialized.
	require.Empty(t, cmp.Diff(expected, actual, cmpopts.IgnoreUnexported(keys.PrivateKey{})))
}
