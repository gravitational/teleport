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

package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAuth_RegisterUsingToken(t *testing.T) {
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	// create a static token
	staticToken := types.ProvisionTokenV1{
		Roles: []types.SystemRole{types.RoleNode},
		Token: "static_token",
	}
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{staticToken},
	})
	require.NoError(t, err)
	err = p.a.SetStaticTokens(staticTokens)
	require.NoError(t, err)

	// create a valid dynamic token
	dynamicToken := generateTestToken(
		ctx,
		t,
		types.SystemRoles{types.RoleNode},
		p.a.GetClock().Now().Add(time.Minute*30),
		p.a,
	)

	// create an expired dynamic token
	expiredDynamicToken := generateTestToken(
		ctx,
		t,
		types.SystemRoles{types.RoleNode},
		p.a.GetClock().Now().Add(-time.Minute*30),
		p.a,
	)

	sshPrivateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	testcases := []struct {
		desc             string
		req              *types.RegisterUsingTokenRequest
		certsAssertion   func(*proto.Certs)
		errorAssertion   func(error) bool
		waitTokenDeleted bool // Expired tokens are deleted in background, might need slight delay in relevant test
	}{
		{
			desc:           "reject empty",
			req:            &types.RegisterUsingTokenRequest{},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "reject no token",
			req: &types.RegisterUsingTokenRequest{
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "reject no HostID",
			req: &types.RegisterUsingTokenRequest{
				Token:        staticToken.Token,
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "allow no NodeName",
			req: &types.RegisterUsingTokenRequest{
				Token:        staticToken.Token,
				HostID:       "localhost",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
		},
		{
			desc: "reject no SSH pub",
			req: &types.RegisterUsingTokenRequest{
				Token:        staticToken.Token,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "reject no TLS pub",
			req: &types.RegisterUsingTokenRequest{
				Token:        staticToken.Token,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
			},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "reject bad token",
			req: &types.RegisterUsingTokenRequest{
				Token:        "not a token",
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsAccessDenied,
		},
		{
			desc: "allow static token",
			req: &types.RegisterUsingTokenRequest{
				Token:        staticToken.Token,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
		},
		{
			desc: "reject wrong role static",
			req: &types.RegisterUsingTokenRequest{
				Token:        staticToken.Token,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleProxy,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "allow dynamic token",
			req: &types.RegisterUsingTokenRequest{
				Token:        dynamicToken,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
		},
		{
			desc: "reject wrong role dynamic",
			req: &types.RegisterUsingTokenRequest{
				Token:        dynamicToken,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleProxy,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "check additional principals",
			req: &types.RegisterUsingTokenRequest{
				Token:                dynamicToken,
				HostID:               "localhost",
				NodeName:             "node-name",
				Role:                 types.RoleNode,
				PublicSSHKey:         sshPublicKey,
				PublicTLSKey:         tlsPublicKey,
				AdditionalPrincipals: []string{"example.com"},
			},
			certsAssertion: func(certs *proto.Certs) {
				hostCert, err := sshutils.ParseCertificate(certs.SSH)
				require.NoError(t, err)
				require.Contains(t, hostCert.ValidPrincipals, "example.com")
			},
		},
		{
			desc: "reject expired dynamic token",
			req: &types.RegisterUsingTokenRequest{
				Token:        expiredDynamicToken,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			waitTokenDeleted: true,
			errorAssertion:   trace.IsAccessDenied,
		},
		{
			// relies on token being deleted during previous testcase
			desc: "expired token should be gone",
			req: &types.RegisterUsingTokenRequest{
				Token:        expiredDynamicToken,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsAccessDenied,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			certs, err := p.a.RegisterUsingToken(ctx, tc.req)
			if tc.errorAssertion != nil {
				require.True(t, tc.errorAssertion(err))
				if tc.waitTokenDeleted {
					require.Eventually(t, func() bool {
						_, err := p.a.ValidateToken(ctx, tc.req.Token)
						return err != nil && strings.Contains(err.Error(), TokenExpiredOrNotFound)
					}, time.Millisecond*100, time.Millisecond*10)
				}
				return
			}
			require.NoError(t, err)
			if tc.certsAssertion != nil {
				tc.certsAssertion(certs)
			}

			// Check audit log event is emitted
			evt := p.mockEmitter.LastEvent()
			require.NotNil(t, evt)
			joinEvent, ok := evt.(*apievents.InstanceJoin)
			require.True(t, ok)
			require.Equal(t, events.InstanceJoinEvent, joinEvent.Type)
			require.Equal(t, events.InstanceJoinCode, joinEvent.Code)
			require.Equal(t, tc.req.NodeName, joinEvent.NodeName)
			require.Equal(t, tc.req.HostID, joinEvent.HostID)
			require.EqualValues(t, tc.req.Role, joinEvent.Role)
			require.EqualValues(t, types.JoinMethodToken, joinEvent.Method)
		})
	}
}

func newBotToken(t *testing.T, tokenName, botName string, role types.SystemRole, expiry time.Time) types.ProvisionToken {
	t.Helper()
	token, err := types.NewProvisionTokenFromSpec(tokenName, expiry, types.ProvisionTokenSpecV2{
		Roles:   []types.SystemRole{role},
		BotName: botName,
	})
	require.NoError(t, err, "could not create bot token")
	return token
}

// TestRegister_Bot tests that a provision token can be used to generate
// renewable certificates for a non-interactive user.
func TestRegister_Bot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := newTestTLSServer(t)

	bot, err := machineidv1.UpsertBot(ctx, srv.Auth(), &machineidv1pb.Bot{
		Metadata: &headerv1.Metadata{
			Name: "test",
		},
		Spec: &machineidv1pb.BotSpec{
			Roles: []string{},
		},
	}, srv.Clock().Now(), "")
	require.NoError(t, err)

	later := srv.Clock().Now().Add(4 * time.Hour)

	goodToken := newBotToken(t, "good-token", bot.Metadata.Name, types.RoleBot, later)
	expiredToken := newBotToken(t, "expired", bot.Metadata.Name, types.RoleBot, srv.Clock().Now().Add(-1*time.Hour))
	wrongKind := newBotToken(t, "wrong-kind", "", types.RoleNode, later)
	wrongUser := newBotToken(t, "wrong-user", "llama", types.RoleBot, later)
	invalidToken := newBotToken(t, "this-token-does-not-exist", bot.Metadata.Name, types.RoleBot, later)

	err = srv.Auth().UpsertToken(ctx, goodToken)
	require.NoError(t, err)
	err = srv.Auth().UpsertToken(ctx, expiredToken)
	require.NoError(t, err)
	err = srv.Auth().UpsertToken(ctx, wrongKind)
	require.NoError(t, err)
	err = srv.Auth().UpsertToken(ctx, wrongUser)
	require.NoError(t, err)

	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)
	sshPrivateKey, err := ssh.ParseRawPrivateKey(privateKey)
	require.NoError(t, err)
	tlsPublicKey, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(sshPrivateKey)
	require.NoError(t, err)

	for _, test := range []struct {
		desc      string
		token     types.ProvisionToken
		assertErr require.ErrorAssertionFunc
	}{
		{
			desc:      "OK good token",
			token:     goodToken,
			assertErr: require.NoError,
		},
		{
			desc:      "NOK expired token",
			token:     expiredToken,
			assertErr: require.Error,
		},
		{
			desc:      "NOK wrong token kind",
			token:     wrongKind,
			assertErr: require.Error,
		},
		{
			desc:      "NOK token for wrong user",
			token:     wrongUser,
			assertErr: require.Error,
		},
		{
			desc:      "NOK invalid token",
			token:     invalidToken,
			assertErr: require.Error,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			start := srv.Clock().Now()
			certs, err := Register(ctx, RegisterParams{
				Token: test.token.GetName(),
				ID: IdentityID{
					Role: types.RoleBot,
				},
				AuthServers:  []utils.NetAddr{*utils.MustParseAddr(srv.Addr().String())},
				PublicTLSKey: tlsPublicKey,
				PublicSSHKey: publicKey,
			})
			test.assertErr(t, err)

			if err == nil {
				require.NotEmpty(t, certs.SSH)
				require.NotEmpty(t, certs.TLS)

				// ensure token was removed
				_, err = srv.Auth().GetToken(ctx, test.token.GetName())
				require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

				// ensure cert is renewable
				x509, err := tlsca.ParseCertificatePEM(certs.TLS)
				require.NoError(t, err)
				id, err := tlsca.FromSubject(x509.Subject, later)
				require.NoError(t, err)
				require.True(t, id.Renewable)

				// Check audit event
				evts, _, err := srv.Auth().SearchEvents(ctx, events.SearchEventsRequest{
					From:       start,
					To:         srv.Clock().Now(),
					EventTypes: []string{events.BotJoinEvent},
					Limit:      1,
					Order:      types.EventOrderDescending,
				})
				require.NoError(t, err)
				require.Len(t, evts, 1)
				evt, ok := evts[0].(*apievents.BotJoin)
				require.True(t, ok)
				require.Equal(t, events.BotJoinEvent, evt.Type)
				require.Equal(t, events.BotJoinCode, evt.Code)
				require.EqualValues(t, types.JoinMethodToken, evt.Method)
			}
		})
	}
}

// TestRegister_Bot_Expiry checks that bot certificate expiry can be set, and
// does not exceed the limit.
func TestRegister_Bot_Expiry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := newTestTLSServer(t)
	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)
	sshPrivateKey, err := ssh.ParseRawPrivateKey(privateKey)
	require.NoError(t, err)
	tlsPublicKey, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(sshPrivateKey)
	require.NoError(t, err)

	validExpires := srv.Clock().Now().Add(time.Hour * 6)
	tooGreatExpires := srv.Clock().Now().Add(time.Hour * 24 * 365)
	tests := []struct {
		name           string
		requestExpires *time.Time
		expectTTL      time.Duration
	}{
		{
			name:           "unspecified defaults",
			requestExpires: nil,
			expectTTL:      defaults.DefaultRenewableCertTTL,
		},
		{
			name:           "valid value specified",
			requestExpires: &validExpires,
			expectTTL:      time.Hour * 6,
		},
		{
			name:           "value exceeding limit specified",
			requestExpires: &tooGreatExpires,
			// MaxSessionTTL set in createBotRole is 12 hours, so this cap will
			// apply instead of the defaults.MaxRenewableCertTTL specified
			// in generateInitialBotCerts.
			expectTTL: 12 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			botName := t.Name()
			_, err = machineidv1.UpsertBot(ctx, srv.Auth(), &machineidv1pb.Bot{
				Metadata: &headerv1.Metadata{
					Name: botName,
				},
				Spec: &machineidv1pb.BotSpec{
					Roles:  []string{},
					Traits: []*machineidv1pb.Trait{},
				},
			}, srv.Clock().Now(), "")
			require.NoError(t, err)
			tok := newBotToken(t, t.Name(), botName, types.RoleBot, srv.Clock().Now().Add(time.Hour))
			require.NoError(t, srv.Auth().UpsertToken(ctx, tok))

			certs, err := Register(ctx, RegisterParams{
				Token: tok.GetName(),
				ID: IdentityID{
					Role: types.RoleBot,
				},
				AuthServers:  []utils.NetAddr{*utils.MustParseAddr(srv.Addr().String())},
				PublicTLSKey: tlsPublicKey,
				PublicSSHKey: publicKey,
				Expires:      tt.requestExpires,
			})
			require.NoError(t, err)
			x509, err := tlsca.ParseCertificatePEM(certs.TLS)
			require.NoError(t, err)
			id, err := tlsca.FromSubject(x509.Subject, x509.NotAfter)
			require.NoError(t, err)

			ttl := id.Expires.Sub(srv.Clock().Now())
			require.Equal(t, tt.expectTTL, ttl)
		})
	}
}
