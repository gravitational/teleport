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
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/api/profile"
)

func newTestExtendedAgent(t *testing.T, extensions ...AgentExtension) *extendedAgent {
	agent, ok := agent.NewKeyring().(agent.ExtendedAgent)
	require.True(t, ok, "unexpected agent type: %T, expected agent.ExtendedAgent", agent)

	agent, err := NewExtendedAgent(agent, extensions...)
	require.NoError(t, err)

	extendedAgent, ok := agent.(*extendedAgent)
	require.True(t, ok, "unexpected agent type: %T, expected *ExtendedAgent", agent)

	return extendedAgent
}

func TestExtendedAgentAddRemove(t *testing.T) {
	agent := newTestExtendedAgent(t)
	key := newTestKey(t)
	agentKey, err := key.AsAgentKey()
	require.NoError(t, err)

	// Add, Remove, and RemoveAll should add/remove crypto signers.
	err = agent.Add(agentKey)
	require.NoError(t, err)
	require.Len(t, agent.cryptoSigners, 1)

	sshCert, err := key.SSHCert()
	require.NoError(t, err)
	err = agent.Remove(sshCert)
	require.NoError(t, err)
	require.Empty(t, agent.cryptoSigners)

	err = agent.Add(agentKey)
	require.NoError(t, err)

	err = agent.RemoveAll()
	require.NoError(t, err)
	require.Empty(t, agent.cryptoSigners)
}

func TestExtendedAgentLock(t *testing.T) {
	// Lock should apply to extensions.
	extensionName := "test"
	agent := newTestExtendedAgent(t, AgentExtension{extensionName, func(a *extendedAgent, contents []byte) ([]byte, error) { return nil, nil }})

	passphrase := "password"
	err := agent.Lock([]byte(passphrase))
	require.NoError(t, err)
	require.True(t, agent.locked)

	_, err = agent.Extension(extensionName, nil)
	require.Equal(t, errLocked, err)

	err = agent.Unlock([]byte(passphrase))
	require.NoError(t, err)
	require.False(t, agent.locked)

	_, err = agent.Extension(extensionName, nil)
	require.NoError(t, err)
}

func TestQueryExtension(t *testing.T) {
	agent := newTestExtendedAgent(t, WithKeyExtension(NewMemClientStore()), WithSignExtension())
	supportedExtensions, err := callQueryExtension(agent)
	require.NoError(t, err)
	expectSupportedExtension := map[string]bool{
		signAgentExtension: true,
		keyAgentExtension:  true,
	}
	require.Equal(t, expectSupportedExtension, supportedExtensions)
}

func TestSignExtension(t *testing.T) {
	agent := newTestExtendedAgent(t, WithSignExtension())
	key := newTestKey(t)
	agentKey, err := key.AsAgentKey()
	require.NoError(t, err)
	sshCert, err := key.SSHCert()
	require.NoError(t, err)

	_, err = callSignExtension(agent, sshCert, []byte{}, nil)
	require.True(t, trace.IsNotFound(err), "Expected not found error but got %v", err)

	err = agent.Add(agentKey)
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

			sig, err := callSignExtension(agent, sshCert, digest, hashFunc)
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
						sig, err := callSignExtension(agent, sshCert, digest, tt.opts)
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

func TestExtensionWithConfirmation(t *testing.T) {
	clientStore := NewMemClientStore()
	agent := newTestExtendedAgent(t, WithKeyExtension(clientStore))
	key := newTestKey(t)
	agentKey, err := key.AsAgentKey()
	require.NoError(t, err)
	err = agent.Add(agentKey)
	require.NoError(t, err)
	sshCert, err := key.SSHCert()
	require.NoError(t, err)

	// Key extension should return not found.
	_, _, err = callKeyExtension(agent)
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
	forwardedProfile, forwardedKey, err := callKeyExtension(agent)
	require.NoError(t, err)
	require.Equal(t, forwardedProfile, profile)
	require.Equal(t, forwardedKey, &ForwardedKey{
		KeyIndex:       key.KeyIndex,
		SSHCertificate: key.Cert,
		TLSCertificate: key.TLSCert,
		TrustedCerts:   key.TrustedCerts,
	})

	// Key extension should return not found if the agent key is missing.
	err = agent.Remove(sshCert)
	require.NoError(t, err)
	_, _, err = callKeyExtension(agent)
	require.True(t, trace.IsNotFound(err), "Expected not found error but got %v", err)
}

func TestExtendedAgentKeyExtension(t *testing.T) {
	ctx := context.Background()

	// create a dummy extension.
	extension := AgentExtension{
		name:    "dummy",
		handler: func(a *extendedAgent, contents []byte) ([]byte, error) { return nil, nil },
	}

	// wrap the dummy extension with confirmation.
	inBuf := []byte{}
	in := bytes.NewBuffer(inBuf)
	outBuf := []byte{}
	out := bytes.NewBuffer(outBuf)
	extension = extension.WithConfirmation(ctx, in, out, "prompt")

	agent := newTestExtendedAgent(t, extension)

	_, err := in.WriteString("y")
	require.NoError(t, err)
	_, err = agent.Extension(extension.name, nil)
	require.NoError(t, err)
	require.Contains(t, out.String(), "prompt")

	_, err = in.WriteString("N")
	require.NoError(t, err)
	_, err = agent.Extension(extension.name, nil)
	require.Error(t, err)
}
