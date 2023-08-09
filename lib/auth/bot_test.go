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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/utilsaddr"
)

// TestServerCreateBot ensures that the create bot RPC creates the appropriate
// role and users.
//
// TODO: We should add more cases to this to properly exercise the token
// creation elements of createBot.
func TestServerCreateBot(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()
	testRole := "test-role"
	_, err := CreateRole(ctx, srv.Auth(), testRole, types.RoleSpecV6{})
	require.NoError(t, err)

	tests := []struct {
		name    string
		request *proto.CreateBotRequest

		checkErr func(*testing.T, error)

		checkUser func(*testing.T, types.User)
		checkRole func(*testing.T, types.Role)
	}{
		{
			name: "no roles",
			request: &proto.CreateBotRequest{
				Name: "no-roles",
			},
			checkErr: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "at least one role is required")
				require.True(t, trace.IsBadParameter(err))
			},
		},
		{
			name: "success",
			request: &proto.CreateBotRequest{
				Name:  "success",
				Roles: []string{testRole},
				Traits: wrappers.Traits{
					constants.TraitLogins: []string{
						"a-principal",
					},
				},
			},
			checkUser: func(t *testing.T, got types.User) {
				require.Equal(t, []string{"bot-success"}, got.GetRoles())
				require.Equal(t, map[string]string{
					types.BotLabel:           "success",
					types.BotGenerationLabel: "0",
				}, got.GetMetadata().Labels)
				// Ensure bot user receives requested traits
				require.Equal(
					t,
					[]string{"a-principal"},
					got.GetTraits()[constants.TraitLogins],
				)
			},
			checkRole: func(t *testing.T, got types.Role) {
				require.Equal(
					t, "success", got.GetMetadata().Labels[types.BotLabel],
				)
				require.Equal(
					t,
					[]string{testRole},
					got.GetImpersonateConditions(types.Allow).Roles,
				)
				require.Equal(
					t,
					types.Duration(12*time.Hour),
					got.GetOptions().MaxSessionTTL,
				)
				// Ensure bot will be able to read the cert authorities
				require.Equal(
					t,
					[]types.Rule{
						types.NewRule(
							types.KindCertAuthority,
							[]string{types.VerbReadNoSecrets},
						),
					},
					got.GetRules(types.Allow),
				)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			res, err := srv.Auth().createBot(ctx, tt.request)
			if tt.checkErr != nil {
				tt.checkErr(t, err)
				return
			}
			require.NoError(t, err)

			// Ensure createBot produces the expected role and user.
			resourceName := BotResourceName(tt.request.Name)
			usr, err := srv.Auth().Services.GetUser(resourceName, false)
			require.NoError(t, err)
			tt.checkUser(t, usr)
			role, err := srv.Auth().Services.GetRole(ctx, resourceName)
			require.NoError(t, err)
			tt.checkRole(t, role)

			// Ensure response includes the correct details
			require.Equal(t, resourceName, res.UserName)
			require.Equal(t, resourceName, res.RoleName)
			require.Equal(t, types.JoinMethodToken, res.JoinMethod)

			// Check generated token exists
			token, err := srv.Auth().Services.GetToken(ctx, res.TokenID)
			require.NoError(t, err)
			require.Equal(t, tt.request.Name, token.GetBotName())
			require.Equal(t, types.JoinMethodToken, token.GetJoinMethod())
			require.Equal(t, types.SystemRoles{types.RoleBot}, token.GetRoles())
		})
	}
}

func TestBotResourceName(t *testing.T) {
	require.Equal(t, "bot-name", BotResourceName("name"))
	require.Equal(t, "bot-name-with-spaces", BotResourceName("name with spaces"))
}

func renewBotCerts(
	ctx context.Context,
	srv *TestTLSServer,
	tlsCert tls.Certificate,
	botUser string,
	publicKey []byte,
	privateKey []byte,
) (*Client, *proto.Certs, tls.Certificate, error) {
	client := srv.NewClientWithCert(tlsCert)

	certs, err := client.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: publicKey,
		Username:  botUser,
		Expires:   time.Now().Add(time.Hour),
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

// TestRegisterBotCertificateGenerationCheck ensures bot cert generation checks
// work in ordinary conditions, with several rapid renewals.
func TestRegisterBotCertificateGenerationCheck(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	_, err := CreateRole(ctx, srv.Auth(), "example", types.RoleSpecV6{})
	require.NoError(t, err)

	// Create a new bot.
	bot, err := srv.Auth().createBot(ctx, &proto.CreateBotRequest{
		Name:  "test",
		Roles: []string{"example"},
	})
	require.NoError(t, err)

	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
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
		AuthServers:  []utilsaddr.NetAddr{*utilsaddr.MustParseAddr(srv.Addr().String())},
		PublicTLSKey: tlsPublicKey,
		PublicSSHKey: publicKey,
	})
	require.NoError(t, err)

	tlsCert, err := tls.X509KeyPair(certs.TLS, privateKey)
	require.NoError(t, err)

	// Renew the cert a bunch of times.
	for i := 0; i < 10; i++ {
		_, certs, tlsCert, err = renewBotCerts(ctx, srv, tlsCert, bot.UserName, publicKey, privateKey)
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

// TestRegisterBotCertificateGenerationStolen simulates a stolen renewable
// certificate where a generation check is expected to fail.
func TestRegisterBotCertificateGenerationStolen(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()
	_, err := CreateRole(ctx, srv.Auth(), "example", types.RoleSpecV6{})
	require.NoError(t, err)

	// Create a new bot.
	bot, err := srv.Auth().createBot(ctx, &proto.CreateBotRequest{
		Name:  "test",
		Roles: []string{"example"},
	})
	require.NoError(t, err)

	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
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
		AuthServers:  []utilsaddr.NetAddr{*utilsaddr.MustParseAddr(srv.Addr().String())},
		PublicTLSKey: tlsPublicKey,
		PublicSSHKey: publicKey,
	})
	require.NoError(t, err)

	tlsCert, err := tls.X509KeyPair(certs.TLS, privateKey)
	require.NoError(t, err)

	// Renew the certs once (e.g. this is the actual bot process)
	_, certsReal, _, err := renewBotCerts(ctx, srv, tlsCert, bot.UserName, publicKey, privateKey)
	require.NoError(t, err)

	// Check the generation, it should be 2.
	impersonatedTLSCert, err := tlsca.ParseCertificatePEM(certsReal.TLS)
	require.NoError(t, err)
	impersonatedIdent, err := tlsca.FromSubject(impersonatedTLSCert.Subject, impersonatedTLSCert.NotAfter)
	require.NoError(t, err)
	require.Equal(t, uint64(2), impersonatedIdent.Generation)

	// Meanwhile, the initial set of certs was stolen. Let's try to renew those.
	_, _, _, err = renewBotCerts(ctx, srv, tlsCert, bot.UserName, publicKey, privateKey)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// The user should now be locked.
	locks, err := srv.Auth().GetLocks(ctx, true, types.LockTarget{
		User: "bot-test",
	})
	require.NoError(t, err)
	require.NotEmpty(t, locks)
}
