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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

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
		idx := KeyIndex{"test.proxy.com", "test-user", "root"}
		key := s.makeSignedKey(t, idx, false)

		// add the test key to the memory store
		err := keyStore.AddKey(key)
		require.NoError(t, err)

		// check that the key exists in the store and is the same,
		// except the key's trusted certs should be empty, to be
		// filled in by a trusted certs store.
		retrievedKey, err := keyStore.GetKey(idx, WithAllCerts...)
		require.NoError(t, err)
		key.TrustedCerts = nil
		require.Equal(t, key, retrievedKey)

		// Delete just the db cert, reload & verify it's gone
		err = keyStore.DeleteUserCerts(idx, WithDBCerts{})
		require.NoError(t, err)
		retrievedKey, err = keyStore.GetKey(idx, WithSSHCerts{}, WithDBCerts{})
		require.NoError(t, err)
		expectKey := key.Copy()
		expectKey.DBTLSCerts = make(map[string][]byte)
		require.Equal(t, expectKey, retrievedKey)

		// check for the key, now without cluster name
		retrievedKey, err = keyStore.GetKey(KeyIndex{idx.ProxyHost, idx.Username, ""})
		require.NoError(t, err)
		expectKey.ClusterName = ""
		expectKey.Cert = nil
		require.Equal(t, expectKey, retrievedKey)

		// delete the key
		err = keyStore.DeleteKey(idx)
		require.NoError(t, err)

		// check that the key doesn't exist in the store
		retrievedKey, err = keyStore.GetKey(idx)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
		require.Nil(t, retrievedKey)

		// Delete non-existing
		err = keyStore.DeleteKey(idx)
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
		keys := make([]Key, keyNum)
		for i := 0; i < keyNum; i++ {
			idx := KeyIndex{fmt.Sprintf("host-%v", i), "bob", "root"}
			key := auth.makeSignedKey(t, idx, false)
			require.NoError(t, keyStore.AddKey(key))
			keys[i] = *key
		}
		// add 1 key for "sam"
		samIdx := KeyIndex{"sam.host", "sam", "root"}
		samKey := auth.makeSignedKey(t, samIdx, false)
		require.NoError(t, keyStore.AddKey(samKey))

		// read all bob keys:
		for i := 0; i < keyNum; i++ {
			key, err := keyStore.GetKey(keys[i].KeyIndex, WithSSHCerts{}, WithDBCerts{})
			require.NoError(t, err)
			key.TrustedCerts = keys[i].TrustedCerts
			require.Equal(t, &keys[i], key)
		}

		// read sam's key and make sure it's the same:
		skey, err := keyStore.GetKey(samIdx, WithSSHCerts{})
		require.NoError(t, err)
		require.Equal(t, samKey.Cert, skey.Cert)
		require.Equal(t, samKey.MarshalSSHPublicKey(), skey.MarshalSSHPublicKey())
	})
}

func TestGetCertificates(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)

	testEachKeyStore(t, func(t *testing.T, keyStore KeyStore) {
		const keyNum = 3

		// add keys for 3 different clusters with the same user and proxy.
		keys := make([]Key, keyNum)
		certs := make([]*ssh.Certificate, keyNum)
		var proxy = "proxy.example.com"
		var user = "bob"
		for i := 0; i < keyNum; i++ {
			idx := KeyIndex{proxy, user, fmt.Sprintf("cluster-%v", i)}
			key := auth.makeSignedKey(t, idx, false)
			err := keyStore.AddKey(key)
			require.NoError(t, err)
			keys[i] = *key
			certs[i], err = key.SSHCert()
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
		idxFoo := KeyIndex{"proxy.example.com", "foo", "root"}
		keyFoo := auth.makeSignedKey(t, idxFoo, false)
		idxBar := KeyIndex{"proxy.example.com", "bar", "root"}
		keyBar := auth.makeSignedKey(t, idxBar, false)

		// add keys
		err := keyStore.AddKey(keyFoo)
		require.NoError(t, err)
		err = keyStore.AddKey(keyBar)
		require.NoError(t, err)

		// check keys exist
		_, err = keyStore.GetKey(idxFoo)
		require.NoError(t, err)
		_, err = keyStore.GetKey(idxBar)
		require.NoError(t, err)

		// delete all keys
		err = keyStore.DeleteKeys()
		require.NoError(t, err)

		// verify keys are gone
		_, err = keyStore.GetKey(idxFoo)
		require.True(t, trace.IsNotFound(err))
		_, err = keyStore.GetKey(idxBar)
		require.Error(t, err)
	})
}

// TestCheckKey makes sure Teleport clients can load non-RSA algorithms in
// normal operating mode.
func TestCheckKey(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)

	testEachKeyStore(t, func(t *testing.T, keyStore KeyStore) {
		idx := KeyIndex{"host.a", "bob", "root"}
		key := auth.makeSignedKey(t, idx, false)

		// Swap out the key with a ECDSA SSH key.
		ellipticCertificate, _, err := cert.CreateEllipticCertificate("foo", ssh.UserCert)
		require.NoError(t, err)
		key.Cert = ssh.MarshalAuthorizedKey(ellipticCertificate)

		err = keyStore.AddKey(key)
		require.NoError(t, err)

		_, err = keyStore.GetKey(idx)
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
		idx := KeyIndex{"host.a", "bob", "root"}
		key := auth.makeSignedKey(t, idx, false)

		// Swap out the key with a ECDSA SSH key.
		ellipticCertificate, _, err := cert.CreateEllipticCertificate("foo", ssh.UserCert)
		require.NoError(t, err)
		key.Cert = ssh.MarshalAuthorizedKey(ellipticCertificate)

		err = keyStore.AddKey(key)
		require.NoError(t, err)

		// Should return trace.BadParameter error because only RSA keys are supported.
		_, err = keyStore.GetKey(idx)
		require.True(t, trace.IsBadParameter(err))
	})
}

func TestAddKey_withoutSSHCert(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)
	keyStore := newTestFSKeyStore(t)

	// without ssh cert, db certs only
	idx := KeyIndex{"host.a", "bob", "root"}
	key := auth.makeSignedKey(t, idx, false)
	key.Cert = nil
	require.NoError(t, keyStore.AddKey(key))

	// ssh cert path should NOT exist
	sshCertPath := keyStore.sshCertPath(key.KeyIndex)
	_, err := os.Stat(sshCertPath)
	require.ErrorIs(t, err, os.ErrNotExist)

	// check db certs
	keyCopy, err := keyStore.GetKey(idx, WithDBCerts{})
	require.NoError(t, err)
	require.Len(t, keyCopy.DBTLSCerts, 1)
}

func TestProtectedDirsNotDeleted(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)
	keyStore := newTestFSKeyStore(t)

	idx := KeyIndex{"host.a", "bob", "root"}
	keyStore.AddKey(auth.makeSignedKey(t, idx, false))
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
