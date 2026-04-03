/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package issuancev1_test

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/durationpb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	issuancev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/issuance/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

func newTestTLSServer(t testing.TB) *authtest.TLSServer {
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, as.Close()) })

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

func TestIssueScopedBotCerts(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	t.Setenv("TELEPORT_UNSTABLE_SCOPES_MWI", "yes")

	ctx := t.Context()
	srv := newTestTLSServer(t)

	const botScope = "/test-scope"

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminClient.Close() })

	// Create a scoped role.
	scopedSvc := adminClient.ScopedAccessServiceClient()
	_, err = scopedSvc.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "bot-role",
			},
			Scope: botScope,
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{botScope},
			},
		},
	})
	require.NoError(t, err)

	// Create a scoped bot.
	bot, err := adminClient.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "test-bot",
			},
			Scope: botScope,
			Spec:  &machineidv1pb.BotSpec{},
		},
	})
	require.NoError(t, err)

	// Create a scoped role assignment for the bot.
	_, err = scopedSvc.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.NewString(),
			},
			Scope: botScope,
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				BotName:  bot.Metadata.Name,
				BotScope: botScope,
				Assignments: []*scopedaccessv1.Assignment{
					{Role: "bot-role", Scope: botScope},
				},
			},
		},
	})
	require.NoError(t, err)

	// Create a client with a scoped bot internal identity.
	botClient, err := srv.NewClient(
		authtest.TestScopedBot(bot.Metadata.Name, botScope, true),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = botClient.Close() })

	// Generate a key pair for the request.
	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPubKeyPEM, err := keys.MarshalPublicKey(key.Public())
	require.NoError(t, err)
	sshPubKey, err := ssh.NewPublicKey(key.Public())
	require.NoError(t, err)
	sshPubKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)

	// Call IssueScopedBotCerts.
	issuanceClient := issuancev1pb.NewIssuanceServiceClient(botClient.GetConnection())
	resp, err := issuanceClient.IssueScopedBotCerts(ctx, &issuancev1pb.IssueScopedBotCertsRequest{
		SshPublicKey: sshPubKeyBytes,
		TlsPublicKey: tlsPubKeyPEM,
		Ttl:          durationpb.New(time.Hour),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Certs)
	require.NotEmpty(t, resp.Certs.Ssh)
	require.NotEmpty(t, resp.Certs.Tls)

	// Parse the returned TLS cert and verify identity properties.
	tlsCert, err := tlsca.ParseCertificatePEM(resp.Certs.Tls)
	require.NoError(t, err)
	identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
	require.NoError(t, err)
	require.False(t, identity.BotInternal, "output cert should not be bot-internal")
	require.True(t, identity.DisallowReissue, "output cert should disallow reissue")
	require.Equal(t, "test-bot", identity.BotName)
	require.Equal(t, key.Public(), tlsCert.PublicKey, "cert public key should match requested key")

	// Verify the SSH cert is valid (it's in authorized_keys format).
	sshParsedKey, _, _, _, err := ssh.ParseAuthorizedKey(resp.Certs.Ssh)
	require.NoError(t, err)
	sshCert, ok := sshParsedKey.(*ssh.Certificate)
	require.True(t, ok, "parsed SSH key should be a certificate")
	require.NotNil(t, sshCert)

	// Verify that a TTL exceeding the maximum is rejected.
	_, err = issuanceClient.IssueScopedBotCerts(ctx, &issuancev1pb.IssueScopedBotCertsRequest{
		SshPublicKey: sshPubKeyBytes,
		TlsPublicKey: tlsPubKeyPEM,
		Ttl:          durationpb.New(defaults.MaxRenewableCertTTL + time.Hour),
	})
	require.True(t, trace.IsBadParameter(err), "expected bad parameter for excessive TTL, got: %v", err)
}

func TestIssueScopedBotCerts_FeatureFlagRequired(t *testing.T) {
	// Do NOT set the feature flag env vars.
	ctx := t.Context()
	srv := newTestTLSServer(t)

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminClient.Close() })

	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPubKeyPEM, err := keys.MarshalPublicKey(key.Public())
	require.NoError(t, err)
	sshPubKey, err := ssh.NewPublicKey(key.Public())
	require.NoError(t, err)

	issuanceClient := issuancev1pb.NewIssuanceServiceClient(adminClient.GetConnection())
	_, err = issuanceClient.IssueScopedBotCerts(ctx, &issuancev1pb.IssueScopedBotCertsRequest{
		SshPublicKey: ssh.MarshalAuthorizedKey(sshPubKey),
		TlsPublicKey: tlsPubKeyPEM,
		Ttl:          durationpb.New(time.Hour),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "MWI scoping features are not enabled")
}

func TestIssueScopedBotCerts_Unauthorized(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	t.Setenv("TELEPORT_UNSTABLE_SCOPES_MWI", "yes")

	ctx := t.Context()
	srv := newTestTLSServer(t)

	const testScope = "/test-scope"

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminClient.Close() })

	// Create a scoped role (needed for scoped identity generation).
	scopedSvc := adminClient.ScopedAccessServiceClient()
	_, err = scopedSvc.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "test-role",
			},
			Scope: testScope,
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{testScope},
			},
		},
	})
	require.NoError(t, err)

	// Generate a key pair shared across subtests.
	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPubKeyPEM, err := keys.MarshalPublicKey(key.Public())
	require.NoError(t, err)
	sshPubKey, err := ssh.NewPublicKey(key.Public())
	require.NoError(t, err)
	sshPubKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)

	scopedBot, err := adminClient.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "scoped-bot",
			},
			Scope: testScope,
			Spec:  &machineidv1pb.BotSpec{},
		},
	})
	require.NoError(t, err)

	_, err = scopedSvc.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.NewString(),
			},
			Scope: testScope,
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				BotName:  scopedBot.Metadata.Name,
				BotScope: testScope,
				Assignments: []*scopedaccessv1.Assignment{
					{Role: "test-role", Scope: testScope},
				},
			},
		},
	})
	require.NoError(t, err)

	req := &issuancev1pb.IssueScopedBotCertsRequest{
		SshPublicKey: sshPubKeyBytes,
		TlsPublicKey: tlsPubKeyPEM,
		Ttl:          durationpb.New(time.Hour),
	}

	t.Run("non-bot user without scope", func(t *testing.T) {
		_, err := adminClient.IssuanceClient().IssueScopedBotCerts(ctx, req)
		require.True(
			t,
			trace.IsAccessDenied(err),
			"expected bad parameter, got: %v", err,
		)
	})

	t.Run("non-bot user with scope", func(t *testing.T) {
		// Create a regular user with a scoped role assignment.
		user, err := authtest.CreateUser(ctx, srv.Auth(), "scoped-user")
		require.NoError(t, err)

		_, err = scopedSvc.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
			Assignment: &scopedaccessv1.ScopedRoleAssignment{
				Kind:    scopedaccess.KindScopedRoleAssignment,
				Version: types.V1,
				SubKind: scopedaccess.SubKindDynamic,
				Metadata: &headerv1.Metadata{
					Name: uuid.NewString(),
				},
				Scope: testScope,
				Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
					User: user.GetName(),
					Assignments: []*scopedaccessv1.Assignment{
						{Role: "test-role", Scope: testScope},
					},
				},
			},
		})
		require.NoError(t, err)

		scopedUserClient, err := srv.NewClient(
			authtest.TestScopedUser(user.GetName(), testScope),
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = scopedUserClient.Close() })

		_, err = scopedUserClient.IssuanceClient().IssueScopedBotCerts(ctx, req)
		require.True(
			t,
			trace.IsAccessDenied(err),
			"expected access denied, got: %v", err,
		)
	})

	t.Run("unscoped bot", func(t *testing.T) {
		// Create an unscoped bot.
		unscopedBot, err := adminClient.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
			Bot: &machineidv1pb.Bot{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "unscoped-bot",
				},
				Spec: &machineidv1pb.BotSpec{},
			},
		})
		require.NoError(t, err)

		botClient, err := srv.NewClient(
			authtest.TestBot(unscopedBot.Metadata.Name, true),
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = botClient.Close() })

		_, err = botClient.IssuanceClient().IssueScopedBotCerts(ctx, req)
		require.True(
			t,
			trace.IsAccessDenied(err),
			"expected access denied, got: %v", err,
		)
	})

	t.Run("scoped bot without BotInternal", func(t *testing.T) {
		botClient, err := srv.NewClient(
			authtest.TestScopedBot(scopedBot.Metadata.Name, testScope, false),
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = botClient.Close() })

		_, err = botClient.IssuanceClient().IssueScopedBotCerts(ctx, req)
		require.True(
			t,
			trace.IsAccessDenied(err),
			"expected access denied, got: %v", err,
		)
	})

	t.Run("scoped bot with DisallowReissue", func(t *testing.T) {
		ident := authtest.TestScopedBot(scopedBot.Metadata.Name, testScope, true)
		lu := ident.I.(authz.LocalUser)
		lu.Identity.DisallowReissue = true
		ident.I = lu

		botClient, err := srv.NewClient(ident)
		require.NoError(t, err)
		t.Cleanup(func() { _ = botClient.Close() })

		_, err = botClient.IssuanceClient().IssueScopedBotCerts(ctx, req)
		require.True(
			t,
			trace.IsAccessDenied(err),
			"expected access denied, got: %v", err,
		)
	})
}
