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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekeyagent"
)

func TestHardwareKeyAgent_Client(t *testing.T) {
	ctx := context.Background()
	agentDir := t.TempDir()

	// Prepare the agent server
	mockService := hardwarekey.NewMockHardwareKeyService(nil /*prompt*/)
	s, err := hardwarekeyagent.NewServer(ctx, mockService, agentDir)
	require.NoError(t, err)
	t.Cleanup(s.Stop)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- s.Serve(ctx)
	}()

	// Prepare the agent client
	agentClient, err := hardwarekeyagent.NewClient(ctx, agentDir)
	require.NoError(t, err)

	// Create a new key in the server
	hwSigner, err := mockService.NewPrivateKey(ctx, hardwarekey.PrivateKeyConfig{
		Policy: hardwarekey.PromptPolicyNone,
	})
	require.NoError(t, err)

	// Perform a signature over the agent.
	hash := crypto.SHA512
	digest := make([]byte, hash.Size())
	agentSignature, err := agentClient.Sign(ctx, hwSigner.Ref, hwSigner.KeyInfo, nil, digest, crypto.SHA512)
	require.NoError(t, err)

	// It should match the signature of a normal hardware key signature,
	// since we are using an empty buffer as the source of entropy (rand).
	normalSignature, err := hwSigner.Sign(nil, digest, crypto.SHA512)
	require.NoError(t, err)
	require.Equal(t, normalSignature, agentSignature)
}
