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

package hardwarekeyagent_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekeyagent"
)

func TestHardwareKeyAgentService(t *testing.T) {
	ctx := context.Background()

	// Mock known keys. Usually the server's login session storage would be used to check for known keys.
	var serverKnownKeySlots []hardwarekey.PIVSlotKey
	knownKeyFn := func(ref *hardwarekey.PrivateKeyRef, _ hardwarekey.ContextualKeyInfo) (bool, error) {
		return slices.ContainsFunc(serverKnownKeySlots, func(s hardwarekey.PIVSlotKey) bool {
			return ref.SlotKey == s
		}), nil
	}

	// Prepare the agent server
	mockService := hardwarekey.NewMockHardwareKeyService(nil /*prompt*/)
	server, err := hardwarekeyagent.NewServer(mockService, insecure.NewCredentials(), knownKeyFn)
	require.NoError(t, err)
	t.Cleanup(server.Stop)

	agentDir := t.TempDir()
	socketPath := filepath.Join(agentDir, "agent.sock")
	l, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve(l)
	}()

	// Prepare the agent client
	agentClient, err := hardwarekeyagent.NewClient(socketPath, insecure.NewCredentials())
	require.NoError(t, err)

	unusedService := hardwarekey.NewMockHardwareKeyService(nil /*prompt*/)
	agentServiceNoFallback := hardwarekeyagent.NewService(agentClient, unusedService)
	agentServiceWithFallback := hardwarekeyagent.NewService(agentClient, mockService)

	for _, tc := range []struct {
		name      string
		algorithm hardwarekey.SignatureAlgorithm
		opts      crypto.SignerOpts
		expectErr bool
	}{
		{
			name:      "EC256 Unsupported hash",
			algorithm: hardwarekey.SignatureAlgorithmEC256,
			opts:      crypto.MD5,
			expectErr: true, // unsupported hash
		},
		{
			name:      "EC256 No hash",
			algorithm: hardwarekey.SignatureAlgorithmEC256,
			opts:      crypto.Hash(0),
		},
		{
			name:      "EC256 SHA256",
			algorithm: hardwarekey.SignatureAlgorithmEC256,
			opts:      crypto.SHA256,
		},
		{
			name:      "EC256 SHA512",
			algorithm: hardwarekey.SignatureAlgorithmEC256,
			opts:      crypto.SHA512,
		},
		{
			name:      "ED25519 No hash",
			algorithm: hardwarekey.SignatureAlgorithmEd25519,
			opts:      crypto.Hash(0),
		},
		{
			name:      "ED25519 SHA256",
			algorithm: hardwarekey.SignatureAlgorithmEd25519,
			opts:      crypto.SHA256,
			expectErr: true, // sha256 not supported
		},
		{
			name:      "ED25519 SHA512",
			algorithm: hardwarekey.SignatureAlgorithmEd25519,
			opts:      crypto.SHA512,
		},
		{
			name:      "RSA2048 No hash",
			algorithm: hardwarekey.SignatureAlgorithmRSA2048,
			opts:      crypto.Hash(0),
		},
		{
			name:      "RSA2048 SHA256",
			algorithm: hardwarekey.SignatureAlgorithmRSA2048,
			opts:      crypto.SHA256,
		},
		{
			name:      "RSA2048 SHA512",
			algorithm: hardwarekey.SignatureAlgorithmRSA2048,
			opts:      crypto.SHA512,
		},
		{
			name:      "RSA2048 No hash PSS signature",
			algorithm: hardwarekey.SignatureAlgorithmRSA2048,
			opts: &rsa.PSSOptions{
				SaltLength: 10,
				Hash:       crypto.Hash(0),
			},
			expectErr: true, // hash required for pss signature
		},
		{
			name:      "RSA2048 SHA256 PSS signature",
			algorithm: hardwarekey.SignatureAlgorithmRSA2048,
			opts: &rsa.PSSOptions{
				SaltLength: 10,
				Hash:       crypto.SHA256,
			},
		},
		{
			name:      "RSA2048 SHA256 PSSSaltLengthAuto",
			algorithm: hardwarekey.SignatureAlgorithmRSA2048,
			opts: &rsa.PSSOptions{
				SaltLength: rsa.PSSSaltLengthAuto,
				Hash:       crypto.SHA256,
			},
		},
		{
			name:      "RSA2048 SHA256 PSSSaltLengthEqualsHash",
			algorithm: hardwarekey.SignatureAlgorithmRSA2048,
			opts: &rsa.PSSOptions{
				SaltLength: rsa.PSSSaltLengthEqualsHash,
				Hash:       crypto.SHA256,
			},
		},
		{
			name:      "RSA2048 SHA512 PSS signature",
			algorithm: hardwarekey.SignatureAlgorithmRSA2048,
			opts: &rsa.PSSOptions{
				SaltLength: 10,
				Hash:       crypto.SHA512,
			},
		},
		{
			name:      "RSA2048 SHA512 PSSSaltLengthAuto",
			algorithm: hardwarekey.SignatureAlgorithmRSA2048,
			opts: &rsa.PSSOptions{
				SaltLength: rsa.PSSSaltLengthAuto,
				Hash:       crypto.SHA512,
			},
		},
		{
			name:      "RSA2048 SHA512 PSSSaltLengthEqualsHash",
			algorithm: hardwarekey.SignatureAlgorithmRSA2048,
			opts: &rsa.PSSOptions{
				SaltLength: rsa.PSSSaltLengthEqualsHash,
				Hash:       crypto.SHA512,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(mockService.Reset)

			// Create a new key directly in the service.
			hwSigner, err := mockService.NewPrivateKey(ctx, hardwarekey.PrivateKeyConfig{
				Algorithm: tc.algorithm,
			})
			require.NoError(t, err)

			// Mock a hashed digest.
			digest := make([]byte, 100)
			if hash := tc.opts.HashFunc(); hash != 0 {
				digest = make([]byte, hash.Size())
			}

			// Perform a signature over the agent.
			_, err = agentServiceNoFallback.Sign(ctx, hwSigner.Ref, hwSigner.KeyInfo, rand.Reader, digest, tc.opts)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}

	t.Run("unknown agent key", func(t *testing.T) {
		mockService.Reset()

		hwSigner, err := mockService.NewPrivateKey(ctx, hardwarekey.PrivateKeyConfig{})
		require.NoError(t, err)

		// Mark the hardware key as unknown by the Hardware Key Service.
		mockService.AddUnknownAgentKey(hwSigner.Ref)
		_, err = agentServiceNoFallback.Sign(ctx, hwSigner.Ref, hwSigner.KeyInfo, rand.Reader, []byte{}, crypto.Hash(0))
		require.Error(t, err)

		// Make the hardware key as known by the  Hardware Key Agent Server.
		serverKnownKeySlots = append(serverKnownKeySlots, hwSigner.Ref.SlotKey)
		_, err = agentServiceNoFallback.Sign(ctx, hwSigner.Ref, hwSigner.KeyInfo, rand.Reader, []byte{}, crypto.Hash(0))
		require.NoError(t, err)
	})

	t.Run("fallback", func(t *testing.T) {
		mockService.Reset()

		// Create a new key.
		hwSigner, err := agentServiceWithFallback.NewPrivateKey(ctx, hardwarekey.PrivateKeyConfig{})
		require.NoError(t, err)

		// If the server stops, the service should fallback to the fallback service.
		server.Stop()
		err = hwSigner.WarmupHardwareKey(ctx)
		require.NoError(t, err)
	})
}
