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
	"crypto/tls"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

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
				ID: state.IdentityID{
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
				ID: state.IdentityID{
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

func newBotToken(t *testing.T, tokenName, botName string, role types.SystemRole, expiry time.Time) types.ProvisionToken {
	t.Helper()
	token, err := types.NewProvisionTokenFromSpec(tokenName, expiry, types.ProvisionTokenSpecV2{
		Roles:   []types.SystemRole{role},
		BotName: botName,
	})
	require.NoError(t, err, "could not create bot token")
	return token
}

func TestVerifyALPNUpgradedConn(t *testing.T) {
	t.Parallel()

	srv := newTestTLSServer(t)
	proxy, err := auth.NewServerIdentity(srv.Auth(), "test-proxy", types.RoleProxy)
	require.NoError(t, err)

	tests := []struct {
		name       string
		serverCert []byte
		clock      clockwork.Clock
		checkError require.ErrorAssertionFunc
	}{
		{
			name:       "proxy verified",
			serverCert: proxy.TLSCertBytes,
			clock:      srv.Clock(),
			checkError: require.NoError,
		},
		{
			name:       "proxy expired",
			serverCert: proxy.TLSCertBytes,
			clock:      clockwork.NewFakeClockAt(srv.Clock().Now().Add(defaults.CATTL + time.Hour)),
			checkError: require.Error,
		},
		{
			name:       "not proxy",
			serverCert: []byte(fixtures.TLSCACertPEM),
			clock:      srv.Clock(),
			checkError: require.Error,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			serverCert, err := utils.ReadCertificates(test.serverCert)
			require.NoError(t, err)

			test.checkError(t, verifyALPNUpgradedConn(test.clock)(tls.ConnectionState{
				PeerCertificates: serverCert,
			}))
		})
	}
}
