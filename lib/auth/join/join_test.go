// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package join

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

func newTestTLSServer(t testing.TB) *auth.TestTLSServer {
	as, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
	})
	require.NoError(t, err)

	srv, err := as.NewTestTLSServer()
	require.NoError(t, err)

	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	return srv
}

// TestRegisterWithAlgorithmSuite tests that the registration generates keys
// with the correct algorithm based on the cluster's configured signature
// algorithm suite.
func TestRegisterWithAlgorithmSuite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := newTestTLSServer(t)

	token, err := types.NewProvisionToken("foo", []types.SystemRole{types.RoleNode}, time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.NoError(t, srv.Auth().UpsertToken(ctx, token))

	authPref, err := srv.Auth().GetAuthPreference(ctx)
	require.NoError(t, err)

	for _, tc := range []struct {
		suite            types.SignatureAlgorithmSuite
		expectPubKeyType crypto.PublicKey
	}{
		{
			suite:            types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED,
			expectPubKeyType: &rsa.PublicKey{},
		},
		{
			suite:            types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
			expectPubKeyType: &rsa.PublicKey{},
		},
		{
			suite:            types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
			expectPubKeyType: &ecdsa.PublicKey{},
		},
		{
			suite:            types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
			expectPubKeyType: &ecdsa.PublicKey{},
		},
		{
			suite:            types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
			expectPubKeyType: &ecdsa.PublicKey{},
		},
	} {
		name, err := tc.suite.MarshalText()
		require.NoError(t, err)
		t.Run(string(name), func(t *testing.T) {
			authPref.SetSignatureAlgorithmSuite(tc.suite)
			_, err := srv.Auth().UpsertAuthPreference(ctx, authPref)
			require.NoError(t, err)

			result, err := Register(ctx, RegisterParams{
				Token: token.GetName(),
				ID: state.IdentityID{
					HostUUID: "testhostid",
					NodeName: "testhostname",
					Role:     types.RoleNode,
				},
				AuthServers: []utils.NetAddr{*utils.MustParseAddr(srv.Addr().String())},
			})
			require.NoError(t, err, trace.DebugReport(err))
			require.IsType(t, tc.expectPubKeyType, result.PrivateKey.Public())

			// Sanity check ssh cert subject pubkey matches returned private key.
			expectSSHPub, err := ssh.NewPublicKey(result.PrivateKey.Public())
			require.NoError(t, err)
			sshCert, err := sshutils.ParseCertificate(result.Certs.SSH)
			require.NoError(t, err)
			require.Equal(t, expectSSHPub, sshCert.Key)

			// Sanity check TLS cert subject pubkey matches returned private key.
			tlsCert, err := tlsca.ParseCertificatePEM(result.Certs.TLS)
			require.NoError(t, err)
			require.Equal(t, result.PrivateKey.Public(), tlsCert.PublicKey)
		})
	}
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
			result, err := Register(ctx, RegisterParams{
				Token: test.token.GetName(),
				ID: state.IdentityID{
					Role: types.RoleBot,
				},
				AuthServers: []utils.NetAddr{*utils.MustParseAddr(srv.Addr().String())},
			})
			test.assertErr(t, err)

			if err == nil {
				require.NotEmpty(t, result.Certs.SSH)
				require.NotEmpty(t, result.Certs.TLS)

				// ensure token was removed
				_, err = srv.Auth().GetToken(ctx, test.token.GetName())
				require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

				// ensure cert is renewable
				x509, err := tlsca.ParseCertificatePEM(result.Certs.TLS)
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
			botName := uuid.NewString()
			_, err := machineidv1.UpsertBot(ctx, srv.Auth(), &machineidv1pb.Bot{
				Metadata: &headerv1.Metadata{
					Name: botName,
				},
				Spec: &machineidv1pb.BotSpec{
					Roles:  []string{},
					Traits: []*machineidv1pb.Trait{},
				},
			}, srv.Clock().Now(), "")
			require.NoError(t, err)
			tok := newBotToken(t, uuid.NewString(), botName, types.RoleBot, srv.Clock().Now().Add(time.Hour))
			require.NoError(t, srv.Auth().UpsertToken(ctx, tok))

			result, err := Register(ctx, RegisterParams{
				Token: tok.GetName(),
				ID: state.IdentityID{
					Role: types.RoleBot,
				},
				AuthServers: []utils.NetAddr{*utils.MustParseAddr(srv.Addr().String())},
				Expires:     tt.requestExpires,
			})
			require.NoError(t, err)
			x509, err := tlsca.ParseCertificatePEM(result.Certs.TLS)
			require.NoError(t, err)
			id, err := tlsca.FromSubject(x509.Subject, x509.NotAfter)
			require.NoError(t, err)

			ttl := id.Expires.Sub(srv.Clock().Now())
			require.Equal(t, tt.expectTTL, ttl)
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

type authJoinClientMock struct {
	AuthJoinClient
	registerUsingToken func(ctx context.Context, req *types.RegisterUsingTokenRequest) (*proto.Certs, error)
}

func (a *authJoinClientMock) RegisterUsingToken(ctx context.Context, req *types.RegisterUsingTokenRequest) (*proto.Certs, error) {
	return a.registerUsingToken(ctx, req)
}

func (a *authJoinClientMock) Ping(_ context.Context) (proto.PingResponse, error) {
	return proto.PingResponse{
		SignatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
	}, nil
}

// TestRegisterWithAuthClient is a unit test to validate joining using a
// auth client supplied via RegisterParams
func TestRegisterWithAuthClient(t *testing.T) {
	ctx := context.Background()
	expectedCerts := &proto.Certs{
		SSH: []byte("ssh-cert"),
	}
	expectedToken := "test-token"
	expectedRole := types.RoleBot
	called := false
	m := &authJoinClientMock{
		registerUsingToken: func(ctx context.Context, req *types.RegisterUsingTokenRequest) (*proto.Certs, error) {
			assert.Empty(t, cmp.Diff(
				&types.RegisterUsingTokenRequest{
					Token: expectedToken,
					Role:  expectedRole,
				},
				req,
				cmpopts.IgnoreFields(types.RegisterUsingTokenRequest{}, "PublicSSHKey", "PublicTLSKey"),
			))
			called = true
			return expectedCerts, nil
		},
	}

	gotResult, gotErr := Register(ctx, RegisterParams{
		Token: expectedToken,
		ID: state.IdentityID{
			Role: expectedRole,
		},
		AuthClient: m,
	})
	require.NoError(t, gotErr)
	assert.True(t, called)
	assert.Equal(t, expectedCerts, gotResult.Certs)
}
