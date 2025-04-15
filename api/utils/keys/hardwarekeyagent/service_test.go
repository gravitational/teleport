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

func TestHardwareKeyAgent_Service(t *testing.T) {
	ctx := context.Background()
	agentDir := t.TempDir()

	// Prepare the agent server
	mockService := hardwarekey.NewMockHardwareKeyService(nil /*prompt*/)
	s, err := hardwarekeyagent.NewServer(ctx, mockService, agentDir)
	require.NoError(t, err)
	t.Cleanup(s.Stop)

	go func() {
		s.Serve(ctx)
	}()

	// Prepare the agent client
	agentClient, err := hardwarekeyagent.NewClient(ctx, agentDir)
	require.NoError(t, err)

	// Prepare the agent service.
	agentService := hardwarekeyagent.NewService(agentClient, mockService)

	// Create a new key.
	hwSigner, err := agentService.NewPrivateKey(ctx, hardwarekey.PrivateKeyConfig{
		Policy: hardwarekey.PromptPolicyNone,
	})
	require.NoError(t, err)

	// Perform a signature using the service.
	hash := crypto.SHA512
	digest := make([]byte, hash.Size())
	_, err = hwSigner.Sign(nil, digest, crypto.SHA512)
	require.NoError(t, err)

	// If the server stops, the service should fallback to the fallback service.
	s.Stop()
	_, err = hwSigner.Sign(nil, digest, crypto.SHA512)
	require.NoError(t, err)
}
