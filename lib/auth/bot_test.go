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

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestBotCreateFeatureDisabled(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			MachineID: false,
		},
	})

	srv := newTestTLSServer(t)
	_, err := CreateRole(context.Background(), srv.Auth(), "example", types.RoleSpecV5{})
	require.NoError(t, err)

	// Attempt to create a bot. This should fail immediately.
	_, err = srv.Auth().createBot(context.Background(), &proto.CreateBotRequest{
		Name:  "test",
		Roles: []string{"example"},
	})
	require.True(t, trace.IsAccessDenied(err))
	require.Contains(t, err.Error(), "not licensed")
}

func TestBotOnboardFeatureDisabled(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			MachineID: false,
		},
	})

	srv := newTestTLSServer(t)

	botName := "test"
	botResourceName := BotResourceName(botName)

	_, err := createBotRole(context.Background(), srv.Auth(), "test", "bot-test", []string{})
	require.NoError(t, err)

	_, err = createBotUser(context.Background(), srv.Auth(), botName, botResourceName)
	require.NoError(t, err)

	later := srv.Clock().Now().Add(4 * time.Hour)
	goodToken := newBotToken(t, "good-token", botName, types.RoleBot, later)

	err = srv.Auth().UpsertToken(context.Background(), goodToken)
	require.NoError(t, err)

	privateKey, publicKey, err := native.GenerateKeyPair()
	require.NoError(t, err)
	sshPrivateKey, err := ssh.ParseRawPrivateKey(privateKey)
	require.NoError(t, err)
	tlsPublicKey, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(sshPrivateKey)
	require.NoError(t, err)

	// Attempt to register a bot. This should fail even if a token was manually
	// created.
	_, err = Register(RegisterParams{
		Token: goodToken.GetName(),
		ID: IdentityID{
			Role: types.RoleBot,
		},
		Servers:      []utils.NetAddr{*utils.MustParseAddr(srv.Addr().String())},
		PublicTLSKey: tlsPublicKey,
		PublicSSHKey: publicKey,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not licensed")
}
