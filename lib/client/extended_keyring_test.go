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

// This file was partially copied from x/crypto/ssh/agent.keyring.go

// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/profile"
)

func newTestExtendedKeyring(t *testing.T, opts ...ExtendedKeyringOpt) *extendedKeyring {
	keyring, err := NewExtendedKeyring(opts...)
	require.NoError(t, err)

	extendedKeyring, ok := keyring.(*extendedKeyring)
	require.True(t, ok, "expected *extendedKeyring but got %T", keyring)

	return extendedKeyring
}

func TestExtendedKeyringAddRemove(t *testing.T) {
	keyring := newTestExtendedKeyring(t)
	key := newTestKey(t)
	agentKey, err := key.AsAgentKey()
	require.NoError(t, err)

	// Add, Remove, and RemoveAll should add/remove crypto signers.
	err = keyring.Add(agentKey)
	require.NoError(t, err)
	require.Len(t, keyring.cryptoSigners, 1)

	sshCert, err := key.SSHCert()
	require.NoError(t, err)
	err = keyring.Remove(sshCert)
	require.NoError(t, err)
	require.Empty(t, keyring.cryptoSigners)

	err = keyring.Add(agentKey)
	require.NoError(t, err)

	err = keyring.RemoveAll()
	require.NoError(t, err)
	require.Empty(t, keyring.cryptoSigners)
}

func TestExtendedKeyringLock(t *testing.T) {
	// Lock should apply to extensions.
	extensionName := "test"
	keyring := newTestExtendedKeyring(t, func(r *extendedKeyring) {
		r.extensionHandlers[extensionName] = func(contents []byte) ([]byte, error) { return nil, nil }
	})

	passphrase := "password"
	err := keyring.Lock([]byte(passphrase))
	require.NoError(t, err)
	require.True(t, keyring.locked)

	_, err = keyring.Extension(extensionName, nil)
	require.Equal(t, errLocked, err)

	err = keyring.Unlock([]byte(passphrase))
	require.NoError(t, err)
	require.False(t, keyring.locked)

	_, err = keyring.Extension(extensionName, nil)
	require.NoError(t, err)
}

func TestExtendedKeyringQueryExtension(t *testing.T) {
	keyring := newTestExtendedKeyring(t, WithKeyExtension(NewMemClientStore()), WithSignExtension())
	supportedExtensions, err := callQueryExtension(keyring)
	require.NoError(t, err)
	expectSupportedExtension := map[string]bool{
		signAgentExtension: true,
		keyAgentExtension:  true,
	}
	require.Equal(t, expectSupportedExtension, supportedExtensions)
}

func TestExtendedKeyringSignExtension(t *testing.T) {
	keyring := newTestExtendedKeyring(t, WithSignExtension())
	key := newTestKey(t)
	agentKey, err := key.AsAgentKey()
	require.NoError(t, err)
	sshCert, err := key.SSHCert()
	require.NoError(t, err)

	_, err = callSignExtension(keyring, sshCert, []byte{}, nil)
	require.True(t, trace.IsNotFound(err), "Expected not found error but got %v", err)

	err = keyring.Add(agentKey)
	require.NoError(t, err)

	// Test sign extension with each hash function supported by the crypto/rsa package.
	for _, hashFunc := range []crypto.Hash{
		crypto.MD5,
		crypto.SHA1,
		crypto.SHA224,
		crypto.SHA256,
		crypto.SHA384,
		crypto.SHA512,
	} {
		t.Run(hashFunc.String(), func(t *testing.T) {
			digest := make([]byte, 100)
			if hashFunc != 0 {
				_, err = rand.Read(digest)
				require.NoError(t, err)
				h := hashFunc.New()
				h.Write(digest)
				digest = h.Sum(nil)
			}

			sig, err := callSignExtension(keyring, sshCert, digest, hashFunc)
			require.NoError(t, err)
			rsaPub, ok := key.Public().(*rsa.PublicKey)
			require.True(t, ok)
			err = rsa.VerifyPKCS1v15(rsaPub, hashFunc, digest, sig)
			require.NoError(t, err)

			t.Run("PSS", func(t *testing.T) {
				for _, tt := range []struct {
					name string
					opts *rsa.PSSOptions
				}{
					{
						name: "salt 1",
						opts: &rsa.PSSOptions{
							Hash:       hashFunc,
							SaltLength: 1,
						},
					}, {
						name: "salt auto",
						opts: &rsa.PSSOptions{
							Hash:       hashFunc,
							SaltLength: rsa.PSSSaltLengthAuto,
						},
					}, {
						name: "salt hash length",
						opts: &rsa.PSSOptions{
							Hash:       hashFunc,
							SaltLength: rsa.PSSSaltLengthEqualsHash,
						},
					},
				} {
					t.Run(tt.name, func(t *testing.T) {
						sig, err := callSignExtension(keyring, sshCert, digest, tt.opts)
						require.NoError(t, err)
						rsaPub, ok := key.Public().(*rsa.PublicKey)
						require.True(t, ok)
						err = rsa.VerifyPSS(rsaPub, hashFunc, digest, sig, tt.opts)
						require.NoError(t, err)
					})
				}
			})
		})
	}
}

func TestExtendedKeyringKeyExtension(t *testing.T) {
	clientStore := NewMemClientStore()
	keyring := newTestExtendedKeyring(t, WithKeyExtension(clientStore))
	key := newTestKey(t)
	agentKey, err := key.AsAgentKey()
	require.NoError(t, err)
	err = keyring.Add(agentKey)
	require.NoError(t, err)
	sshCert, err := key.SSHCert()
	require.NoError(t, err)

	// Key extension should return not found.
	_, _, err = callKeyExtension(keyring)
	require.True(t, trace.IsNotFound(err), "Expected not found error but got %v", err)

	// Add the key and profile to to client store.
	err = clientStore.AddKey(key)
	require.NoError(t, err)
	profile := &profile.Profile{
		WebProxyAddr: key.ProxyHost + ":3080",
		SiteName:     key.ClusterName,
		Username:     key.Username,
	}
	err = clientStore.SaveProfile(profile, true)
	require.NoError(t, err)

	// Key extension should return the key.
	forwardedProfile, forwardedKey, err := callKeyExtension(keyring)
	require.NoError(t, err)
	require.Equal(t, forwardedProfile, profile)
	require.Equal(t, forwardedKey, &ForwardedKey{
		KeyIndex:       key.KeyIndex,
		SSHCertificate: key.Cert,
		TLSCertificate: key.TLSCert,
		TrustedCerts:   key.TrustedCerts,
	})

	// Key extension should return not found if the agent key is missing.
	err = keyring.Remove(sshCert)
	require.NoError(t, err)
	_, _, err = callKeyExtension(keyring)
	require.True(t, trace.IsNotFound(err), "Expected not found error but got %v", err)
}
