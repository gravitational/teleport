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
	"crypto/tls"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestServer_createBot_FeatureDisabled(t *testing.T) {
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

// TestServer_createBot_NoRoles attempts to create a bot with an empty role list.
func TestServer_createBot_NoRoles(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	// Create a new bot without roles specified. This should fail.
	_, err := srv.Auth().createBot(context.Background(), &proto.CreateBotRequest{
		Name: "test",
	})
	require.True(t, trace.IsBadParameter(err))
}

func TestRegister_BotOnboardFeatureDisabled(t *testing.T) {
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

	_, err = createBotUser(context.Background(), srv.Auth(), botName, botResourceName, wrappers.Traits{})
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

func renewBotCerts(
	srv *TestTLSServer, tlsCert tls.Certificate, botUser string,
	publicKey []byte, privateKey []byte,
) (*Client, *proto.Certs, tls.Certificate, error) {
	client := srv.NewClientWithCert(tlsCert)

	certs, err := client.GenerateUserCerts(context.Background(), proto.UserCertsRequest{
		PublicKey: publicKey,
		Username:  botUser,
		Expires:   time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		return nil, nil, tls.Certificate{}, trace.Wrap(err)
	}

	// Make sure to overwrite tlsCert with the new certs.
	tlsCert, err = tls.X509KeyPair(certs.TLS, privateKey)
	if err != nil {
		return nil, nil, tls.Certificate{}, trace.Wrap(err)
	}

	return client, certs, tlsCert, nil
}

// TestRegister_BotCertificateGenerationCheck ensures bot cert generation checks
// work in ordinary conditions, with several rapid renewals.
func TestRegister_BotCertificateGenerationCheck(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	_, err := CreateRole(context.Background(), srv.Auth(), "example", types.RoleSpecV5{})
	require.NoError(t, err)

	// Create a new bot.
	bot, err := srv.Auth().createBot(context.Background(), &proto.CreateBotRequest{
		Name:  "test",
		Roles: []string{"example"},
	})
	require.NoError(t, err)

	privateKey, publicKey, err := native.GenerateKeyPair()
	require.NoError(t, err)
	sshPrivateKey, err := ssh.ParseRawPrivateKey(privateKey)
	require.NoError(t, err)
	tlsPublicKey, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(sshPrivateKey)
	require.NoError(t, err)

	certs, err := Register(RegisterParams{
		Token: bot.TokenID,
		ID: IdentityID{
			Role: types.RoleBot,
		},
		Servers:      []utils.NetAddr{*utils.MustParseAddr(srv.Addr().String())},
		PublicTLSKey: tlsPublicKey,
		PublicSSHKey: publicKey,
	})
	require.NoError(t, err)

	tlsCert, err := tls.X509KeyPair(certs.TLS, privateKey)
	require.NoError(t, err)

	// Renew the cert a bunch of times.
	for i := 0; i < 10; i++ {
		_, certs, tlsCert, err = renewBotCerts(srv, tlsCert, bot.UserName, publicKey, privateKey)
		require.NoError(t, err)

		// Parse the Identity
		impersonatedTLSCert, err := tlsca.ParseCertificatePEM(certs.TLS)
		require.NoError(t, err)
		impersonatedIdent, err := tlsca.FromSubject(impersonatedTLSCert.Subject, impersonatedTLSCert.NotAfter)
		require.NoError(t, err)

		// Cert must be renewable.
		require.True(t, impersonatedIdent.Renewable)
		require.False(t, impersonatedIdent.DisallowReissue)

		// Initial certs have generation=1 and we start the loop with a renewal, so add 2
		require.Equal(t, uint64(i+2), impersonatedIdent.Generation)
	}
}

// TestRegister_BotCertificateGenerationStolen simulates a stolen renewable
// certificate where a generation check is expected to fail.
func TestRegister_BotCertificateGenerationStolen(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	_, err := CreateRole(context.Background(), srv.Auth(), "example", types.RoleSpecV5{})
	require.NoError(t, err)

	// Create a new bot.
	bot, err := srv.Auth().createBot(context.Background(), &proto.CreateBotRequest{
		Name:  "test",
		Roles: []string{"example"},
	})
	require.NoError(t, err)

	privateKey, publicKey, err := native.GenerateKeyPair()
	require.NoError(t, err)
	sshPrivateKey, err := ssh.ParseRawPrivateKey(privateKey)
	require.NoError(t, err)
	tlsPublicKey, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(sshPrivateKey)
	require.NoError(t, err)

	certs, err := Register(RegisterParams{
		Token: bot.TokenID,
		ID: IdentityID{
			Role: types.RoleBot,
		},
		Servers:      []utils.NetAddr{*utils.MustParseAddr(srv.Addr().String())},
		PublicTLSKey: tlsPublicKey,
		PublicSSHKey: publicKey,
	})
	require.NoError(t, err)

	tlsCert, err := tls.X509KeyPair(certs.TLS, privateKey)
	require.NoError(t, err)

	// Renew the certs once (e.g. this is the actual bot process)
	_, certsReal, _, err := renewBotCerts(srv, tlsCert, bot.UserName, publicKey, privateKey)
	require.NoError(t, err)

	// Check the generation, it should be 2.
	impersonatedTLSCert, err := tlsca.ParseCertificatePEM(certsReal.TLS)
	require.NoError(t, err)
	impersonatedIdent, err := tlsca.FromSubject(impersonatedTLSCert.Subject, impersonatedTLSCert.NotAfter)
	require.NoError(t, err)
	require.Equal(t, uint64(2), impersonatedIdent.Generation)

	// Meanwhile, the initial set of certs was stolen. Let's try to renew those.
	_, _, _, err = renewBotCerts(srv, tlsCert, bot.UserName, publicKey, privateKey)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// The user should now be locked.
	locks, err := srv.Auth().GetLocks(context.Background(), true, types.LockTarget{
		User: "bot-test",
	})
	require.NoError(t, err)
	require.NotEmpty(t, locks)
}
