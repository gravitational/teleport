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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/durationpb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	issuancev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/issuance/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	scopedapp "github.com/gravitational/teleport/lib/scopes/app"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

func newTestTLSServer(t testing.TB) *authtest.TLSServer {
	return newTestTLSServerWithScopesFeatures(t, scopes.Features{})
}

func newTestTLSServerWithScopesFeatures(t testing.TB, scopesFeatures scopes.Features) *authtest.TLSServer {
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:            t.TempDir(),
		Clock:          clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
		ScopesFeatures: scopesFeatures,
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

func TestIssueScopedBotCerts_UsageIdentity(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServerWithScopesFeatures(t, scopes.Features{Enabled: true})

	const botScope = "/test-scope"

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminClient.Close() })

	// Create a scoped role.
	scopedSvc := adminClient.ScopedAccessServiceClient()
	_, err = scopedSvc.CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
		Role: scopedaccessv1.ScopedRole_builder{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: "bot-role",
			}.Build(),
			Scope: botScope,
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{botScope},
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)

	// Create a scoped bot.
	bot, err := adminClient.BotServiceClient().CreateBot(ctx, machineidv1pb.CreateBotRequest_builder{
		Bot: machineidv1pb.Bot_builder{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: "test-bot",
			}.Build(),
			Scope: botScope,
			Spec:  &machineidv1pb.BotSpec{},
		}.Build(),
	}.Build())
	require.NoError(t, err)

	// Create a scoped role assignment for the bot.
	sraResp, err := scopedSvc.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: uuid.NewString(),
			}.Build(),
			Scope: botScope,
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				Bot: scopes.QualifiedName{Scope: botScope, Name: bot.GetMetadata().GetName()}.String(),
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{Role: botScope + "::bot-role", Scope: botScope}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)
	waitForSRACache(t, srv, sraResp)

	// Create a client with a scoped bot internal identity.
	botClient, err := srv.NewClient(
		authtest.TestScopedBot(t, scopes.QualifiedName{Scope: botScope, Name: bot.GetMetadata().GetName()}, true),
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

	issuanceClient := issuancev1pb.NewIssuanceServiceClient(botClient.GetConnection())
	requestedTTL := time.Hour
	now := srv.Clock().Now()

	t.Run("success", func(t *testing.T) {
		resp, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			SshPublicKey: sshPubKeyBytes,
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			Identity:     &issuancev1pb.UsageIdentity{},
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, resp.GetCerts())
		require.NotEmpty(t, resp.GetCerts().GetSsh())
		require.NotEmpty(t, resp.GetCerts().GetTls())

		// Parse the returned TLS cert and verify identity properties.
		tlsCert, err := tlsca.ParseCertificatePEM(resp.GetCerts().GetTls())
		require.NoError(t, err)
		identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
		require.NoError(t, err)
		require.False(t, identity.BotInternal, "output cert should not be bot-internal")
		require.True(t, identity.DisallowReissue, "output cert should disallow reissue")
		require.Equal(t, "test-bot", identity.BotName)
		require.Equal(t, key.Public(), tlsCert.PublicKey, "cert public key should match requested key")

		// Verify TLS cert validity matches the requested TTL.
		require.WithinDuration(t, now.Add(requestedTTL), tlsCert.NotAfter, time.Minute)

		// Verify the SSH cert is valid.
		sshParsedKey, _, _, _, err := ssh.ParseAuthorizedKey(resp.GetCerts().GetSsh())
		require.NoError(t, err)
		sshCert, ok := sshParsedKey.(*ssh.Certificate)
		require.True(t, ok, "parsed SSH key should be a certificate")
		require.NotNil(t, sshCert)

		// Verify SSH cert validity matches the requested TTL.
		sshNotAfter := time.Unix(int64(sshCert.ValidBefore), 0)
		require.WithinDuration(t, now.Add(requestedTTL), sshNotAfter, time.Minute)
	})

	t.Run("excessive TTL rejected", func(t *testing.T) {
		_, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			SshPublicKey: sshPubKeyBytes,
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(defaults.MaxRenewableCertTTL + time.Hour),
			Identity:     &issuancev1pb.UsageIdentity{},
		}.Build())
		require.True(t, trace.IsBadParameter(err), "expected bad parameter for excessive TTL, got: %v", err)
	})

	t.Run("missing usage rejected", func(t *testing.T) {
		_, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			SshPublicKey: sshPubKeyBytes,
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(time.Hour),
		}.Build())
		require.ErrorContains(t, err, "unsupported or unspecified usage variant")
	})

	t.Run("ssh only", func(t *testing.T) {
		resp, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			SshPublicKey: sshPubKeyBytes,
			Ttl:          durationpb.New(requestedTTL),
			Identity:     &issuancev1pb.UsageIdentity{},
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, resp.GetCerts())
		require.NotEmpty(t, resp.GetCerts().GetSsh())
		require.Empty(t, resp.GetCerts().GetTls())
	})

	t.Run("tls only", func(t *testing.T) {
		resp, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			Identity:     &issuancev1pb.UsageIdentity{},
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, resp.GetCerts())
		require.Empty(t, resp.GetCerts().GetSsh())
		require.NotEmpty(t, resp.GetCerts().GetTls())
	})

	t.Run("missing keys rejected", func(t *testing.T) {
		_, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			Ttl:      durationpb.New(requestedTTL),
			Identity: &issuancev1pb.UsageIdentity{},
		}.Build())
		require.True(t, trace.IsBadParameter(err), "expected bad parameter, got: %v", err)
		require.ErrorContains(t, err, "at least one of ssh_public_key or tls_public_key is required")
	})

	t.Run("zero ttl rejected", func(t *testing.T) {
		_, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			SshPublicKey: sshPubKeyBytes,
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(0),
			Identity:     &issuancev1pb.UsageIdentity{},
		}.Build())
		require.True(t, trace.IsBadParameter(err), "expected bad parameter for zero TTL, got: %v", err)
		require.ErrorContains(t, err, "must be greater than zero")
	})

	t.Run("negative ttl rejected", func(t *testing.T) {
		_, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			SshPublicKey: sshPubKeyBytes,
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(-time.Hour),
			Identity:     &issuancev1pb.UsageIdentity{},
		}.Build())
		require.True(t, trace.IsBadParameter(err), "expected bad parameter for negative TTL, got: %v", err)
		require.ErrorContains(t, err, "must be greater than zero")
	})
}

func TestIssueScopedBotCerts_FeatureFlagRequired(t *testing.T) {
	t.Parallel()
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
	_, err = issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
		SshPublicKey: ssh.MarshalAuthorizedKey(sshPubKey),
		TlsPublicKey: tlsPubKeyPEM,
		Ttl:          durationpb.New(time.Hour),
		Identity:     &issuancev1pb.UsageIdentity{},
	}.Build())
	require.Error(t, err)
	require.Contains(t, err.Error(), "scoping features are not enabled")
}

func TestIssueScopedBotCerts_Unauthorized(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServerWithScopesFeatures(t, scopes.Features{Enabled: true})

	const testScope = "/test-scope"

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminClient.Close() })

	// Create a scoped role (needed for scoped identity generation).
	scopedSvc := adminClient.ScopedAccessServiceClient()
	_, err = scopedSvc.CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
		Role: scopedaccessv1.ScopedRole_builder{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: "test-role",
			}.Build(),
			Scope: testScope,
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{testScope},
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)

	// Generate a key pair shared across subtests.
	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPubKeyPEM, err := keys.MarshalPublicKey(key.Public())
	require.NoError(t, err)
	sshPubKey, err := ssh.NewPublicKey(key.Public())
	require.NoError(t, err)
	sshPubKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)

	scopedBot, err := adminClient.BotServiceClient().CreateBot(ctx, machineidv1pb.CreateBotRequest_builder{
		Bot: machineidv1pb.Bot_builder{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: "scoped-bot",
			}.Build(),
			Scope: testScope,
			Spec:  &machineidv1pb.BotSpec{},
		}.Build(),
	}.Build())
	require.NoError(t, err)

	sraResp, err := scopedSvc.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: uuid.NewString(),
			}.Build(),
			Scope: testScope,
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				Bot: scopes.QualifiedName{Scope: testScope, Name: scopedBot.GetMetadata().GetName()}.String(),
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{Role: testScope + "::test-role", Scope: testScope}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)
	waitForSRACache(t, srv, sraResp)

	req := issuancev1pb.IssueScopedBotCertsRequest_builder{
		SshPublicKey: sshPubKeyBytes,
		TlsPublicKey: tlsPubKeyPEM,
		Ttl:          durationpb.New(time.Hour),
		Identity:     &issuancev1pb.UsageIdentity{},
	}.Build()

	t.Run("non-bot user without scope", func(t *testing.T) {
		_, err := adminClient.IssuanceClient().IssueScopedBotCerts(ctx, req)
		require.True(
			t,
			trace.IsAccessDenied(err),
			"expected access denied, got: %v", err,
		)
	})

	t.Run("non-bot user with scope", func(t *testing.T) {
		// Create a regular user with a scoped role assignment.
		user, err := authtest.CreateUser(ctx, srv.Auth(), "scoped-user")
		require.NoError(t, err)

		userSRAResp, err := scopedSvc.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
			Assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    scopedaccess.KindScopedRoleAssignment,
				Version: types.V1,
				SubKind: scopedaccess.SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.NewString(),
				}.Build(),
				Scope: testScope,
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: user.GetName(),
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{Role: testScope + "::test-role", Scope: testScope}.Build(),
					},
				}.Build(),
			}.Build(),
		}.Build())
		require.NoError(t, err)
		waitForSRACache(t, srv, userSRAResp)

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
		unscopedBot, err := adminClient.BotServiceClient().CreateBot(ctx, machineidv1pb.CreateBotRequest_builder{
			Bot: machineidv1pb.Bot_builder{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "unscoped-bot",
				}.Build(),
				Spec: &machineidv1pb.BotSpec{},
			}.Build(),
		}.Build())
		require.NoError(t, err)

		botClient, err := srv.NewClient(
			authtest.TestBot(unscopedBot.GetMetadata().GetName(), true),
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
			authtest.TestScopedBot(t, scopes.QualifiedName{Scope: testScope, Name: scopedBot.GetMetadata().GetName()}, false),
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
		ident := authtest.TestScopedBot(t, scopes.QualifiedName{Scope: testScope, Name: scopedBot.GetMetadata().GetName()}, true)
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

func TestIssueScopedBotCerts_UsageApp(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServerWithScopesFeatures(t, scopes.Features{Enabled: true})

	const botScope = "/test-scope"

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminClient.Close() })

	// Create a scoped role with app access.
	scopedSvc := adminClient.ScopedAccessServiceClient()
	_, err = scopedSvc.CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
		Role: scopedaccessv1.ScopedRole_builder{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: "bot-role",
			}.Build(),
			Scope: botScope,
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{botScope},
				App: scopedaccessv1.ScopedRoleApp_builder{
					Labels: []*labelv1.Label{
						labelv1.Label_builder{
							Name:   types.Wildcard,
							Values: []string{types.Wildcard},
						}.Build(),
					},
				}.Build(),
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)

	// Create a scoped bot.
	bot, err := adminClient.BotServiceClient().CreateBot(ctx, machineidv1pb.CreateBotRequest_builder{
		Bot: machineidv1pb.Bot_builder{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: "test-bot",
			}.Build(),
			Scope: botScope,
			Spec:  &machineidv1pb.BotSpec{},
		}.Build(),
	}.Build())
	require.NoError(t, err)

	// Create a scoped role assignment for the bot.
	sraResp, err := scopedSvc.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: uuid.NewString(),
			}.Build(),
			Scope: botScope,
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				Bot: scopes.QualifiedName{Scope: botScope, Name: bot.GetMetadata().GetName()}.String(),
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{Role: botScope + "::bot-role", Scope: botScope}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)
	waitForSRACache(t, srv, sraResp)

	// Create a scoped app and register it.
	app, err := types.NewAppV3(types.Metadata{
		Name: "test-app",
	}, types.AppSpecV3{
		URI:        "http://localhost:8080",
		PublicAddr: scopedapp.ScopedAppPublicAddr(botScope, "test-app", "proxy.example.com"),
	})
	require.NoError(t, err)
	app.Scope = botScope
	appServer, err := types.NewAppServerV3FromApp(app, "test-app-host", "test-app-hostid")
	require.NoError(t, err)
	appServer.Scope = botScope
	_, err = srv.Auth().UpsertApplicationServer(ctx, appServer)
	require.NoError(t, err)

	// Create a client with a scoped bot internal identity.
	botClient, err := srv.NewClient(
		authtest.TestScopedBot(bot.GetMetadata().GetName(), botScope, true),
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

	issuanceClient := issuancev1pb.NewIssuanceServiceClient(botClient.GetConnection())
	requestedTTL := time.Hour

	t.Run("success", func(t *testing.T) {
		resp, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			SshPublicKey: sshPubKeyBytes,
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			App: issuancev1pb.UsageApp_builder{
				Name:        "test-app",
				PublicAddr:  app.GetPublicAddr(),
				ClusterName: srv.ClusterName(),
				Scope:       botScope,
			}.Build(),
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, resp.GetCerts())
		require.NotEmpty(t, resp.GetCerts().GetTls())

		// Parse the returned TLS cert and verify app-specific identity properties.
		tlsCert, err := tlsca.ParseCertificatePEM(resp.GetCerts().GetTls())
		require.NoError(t, err)
		identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
		require.NoError(t, err)

		assert.False(t, identity.BotInternal, "output cert should not be bot-internal")
		assert.True(t, identity.DisallowReissue, "output cert should disallow reissue")
		assert.Equal(t, "test-bot", identity.BotName)
		assert.Equal(t, "test-app", identity.RouteToApp.Name)
		assert.Equal(t, app.GetPublicAddr(), identity.RouteToApp.PublicAddr)
		assert.Equal(t, srv.ClusterName(), identity.RouteToApp.ClusterName)
		assert.NotEmpty(t, identity.RouteToApp.SessionID, "app session should have been created")
		assert.Equal(t, botScope, identity.RouteToApp.Scope)
	})

	t.Run("missing app name rejected", func(t *testing.T) {
		_, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			App: issuancev1pb.UsageApp_builder{
				Scope: botScope,
			}.Build(),
		}.Build())
		require.True(t, trace.IsBadParameter(err), "expected bad parameter, got: %v", err)
		require.ErrorContains(t, err, "app.name: is required")
	})

	t.Run("invalid scope rejected", func(t *testing.T) {
		_, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			App: issuancev1pb.UsageApp_builder{
				Name:  "test-app",
				Scope: "not-a-scope",
			}.Build(),
		}.Build())
		require.True(t, trace.IsBadParameter(err), "expected bad parameter, got: %v", err)
		require.ErrorContains(t, err, "app.scope")
	})

	t.Run("tls public key required for app usage", func(t *testing.T) {
		_, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			SshPublicKey: sshPubKeyBytes,
			Ttl:          durationpb.New(requestedTTL),
			App: issuancev1pb.UsageApp_builder{
				Name:        "test-app",
				PublicAddr:  app.GetPublicAddr(),
				ClusterName: srv.ClusterName(),
				Scope:       botScope,
			}.Build(),
		}.Build())
		require.True(t, trace.IsBadParameter(err), "expected bad parameter, got: %v", err)
		require.ErrorContains(t, err, "tls_public_key is required for app usage")
	})

	t.Run("app scope outside pinned scope rejected", func(t *testing.T) {
		_, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			App: issuancev1pb.UsageApp_builder{
				Name:        "test-app",
				PublicAddr:  app.GetPublicAddr(),
				ClusterName: srv.ClusterName(),
				Scope:       "/other-scope",
			}.Build(),
		}.Build())
		require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
	})

	t.Run("app not found in scope rejected", func(t *testing.T) {
		// Request an app that does not exist at all.
		_, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			App: issuancev1pb.UsageApp_builder{
				Name:        "nonexistent-app",
				PublicAddr:  "nonexistent-app.example.com",
				ClusterName: srv.ClusterName(),
				Scope:       botScope,
			}.Build(),
		}.Build())
		require.True(t, trace.IsNotFound(err), "expected not found, got: %v", err)
		require.ErrorContains(t, err, "nonexistent-app")
	})

	t.Run("app exists in sibling child scope rejected by lookup", func(t *testing.T) {
		// Register an app in /test-scope/team-b (a child scope of the bot's
		// pin). Then request it claiming scope /test-scope/team-a (a different
		// child). The scope pin check passes for both (both are descendants
		// of /test-scope), but the lookup must reject because the app server
		// is registered in /test-scope/team-b, not /test-scope/team-a.
		const (
			actualScope    = "/test-scope/team-b"
			requestedScope = "/test-scope/team-a"
		)
		siblingApp, err := types.NewAppV3(types.Metadata{
			Name: "team-b-app",
		}, types.AppSpecV3{
			URI:        "http://localhost:9090",
			PublicAddr: scopedapp.ScopedAppPublicAddr(actualScope, "team-b-app", "proxy.example.com"),
		})
		require.NoError(t, err)
		siblingApp.Scope = actualScope
		siblingAppServer, err := types.NewAppServerV3FromApp(siblingApp, "team-b-host", "team-b-hostid")
		require.NoError(t, err)
		siblingAppServer.Scope = actualScope
		_, err = srv.Auth().UpsertApplicationServer(ctx, siblingAppServer)
		require.NoError(t, err)

		// Request the app in the wrong sibling scope — scope pin passes
		// but the app lookup finds no server matching /test-scope/team-a.
		_, err = issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			App: issuancev1pb.UsageApp_builder{
				Name:        "team-b-app",
				PublicAddr:  siblingApp.GetPublicAddr(),
				ClusterName: srv.ClusterName(),
				Scope:       requestedScope,
			}.Build(),
		}.Build())
		require.True(t, trace.IsNotFound(err), "expected not found when app is in sibling scope, got: %v", err)
		require.ErrorContains(t, err, "team-b-app")
		require.ErrorContains(t, err, requestedScope)
	})

	t.Run("app in scope but requested with wrong scope rejected", func(t *testing.T) {
		// The app "test-app" lives in /test-scope. Request it claiming
		// /test-scope/team-a (a child scope that the app is not in).
		// The scope pin check passes (child of pinned scope), but the
		// app lookup must fail because no app server matches that scope.
		const childScope = "/test-scope/team-a"
		_, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			App: issuancev1pb.UsageApp_builder{
				Name:        "test-app",
				PublicAddr:  app.GetPublicAddr(),
				ClusterName: srv.ClusterName(),
				Scope:       childScope,
			}.Build(),
		}.Build())
		require.True(t, trace.IsNotFound(err), "expected not found when scope doesn't match app's actual scope, got: %v", err)
		require.ErrorContains(t, err, "test-app")
		require.ErrorContains(t, err, childScope)
	})

	t.Run("identity center account app rejected", func(t *testing.T) {
		// Register an Identity Center account app in the bot's scope.
		icApp, err := types.NewAppV3(types.Metadata{
			Name: "ic-account-app",
		}, types.AppSpecV3{
			URI:        "http://localhost:7070",
			PublicAddr: scopedapp.ScopedAppPublicAddr(botScope, "ic-account-app", "proxy.example.com"),
		})
		require.NoError(t, err)
		icApp.SetSubKind(types.KindIdentityCenterAccount)
		icApp.Scope = botScope
		icAppServer, err := types.NewAppServerV3FromApp(icApp, "ic-host", "ic-hostid")
		require.NoError(t, err)
		icAppServer.Scope = botScope
		_, err = srv.Auth().UpsertApplicationServer(ctx, icAppServer)
		require.NoError(t, err)

		_, err = issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			App: issuancev1pb.UsageApp_builder{
				Name:        "ic-account-app",
				PublicAddr:  icApp.GetPublicAddr(),
				ClusterName: srv.ClusterName(),
				Scope:       botScope,
			}.Build(),
		}.Build())
		require.True(t, trace.IsBadParameter(err), "expected bad parameter for IC account app, got: %v", err)
		require.ErrorContains(t, err, "Identity Center")
	})
}

// TestIssueScopedBotCerts_UsageApp_ScopeHierarchy tests scope matching
// behavior in detail: exact matches, descendant scopes, and narrower bot
// scope scenarios.
func TestIssueScopedBotCerts_UsageApp_ScopeHierarchy(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServerWithScopesFeatures(t, scopes.Features{Enabled: true})

	const (
		parentScope = "/testing"
		childScope  = "/testing/team-a"
	)

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminClient.Close() })

	scopedSvc := adminClient.ScopedAccessServiceClient()

	// Create scoped roles at both levels with wildcard app access.
	for _, scope := range []string{parentScope, childScope} {
		_, err = scopedSvc.CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
			Role: scopedaccessv1.ScopedRole_builder{
				Kind:    scopedaccess.KindScopedRole,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "app-role",
				}.Build(),
				Scope: scope,
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{scope},
					App: scopedaccessv1.ScopedRoleApp_builder{
						Labels: []*labelv1.Label{
							labelv1.Label_builder{
								Name:   types.Wildcard,
								Values: []string{types.Wildcard},
							}.Build(),
						},
					}.Build(),
				}.Build(),
			}.Build(),
		}.Build())
		require.NoError(t, err)
	}

	// Register apps at both scope levels.
	parentApp, err := types.NewAppV3(types.Metadata{
		Name: "parent-app",
	}, types.AppSpecV3{
		URI:        "http://localhost:8081",
		PublicAddr: scopedapp.ScopedAppPublicAddr(parentScope, "parent-app", "proxy.example.com"),
	})
	require.NoError(t, err)
	parentApp.Scope = parentScope
	parentAppServer, err := types.NewAppServerV3FromApp(parentApp, "parent-host", "parent-hostid")
	require.NoError(t, err)
	parentAppServer.Scope = parentScope
	_, err = srv.Auth().UpsertApplicationServer(ctx, parentAppServer)
	require.NoError(t, err)

	childApp, err := types.NewAppV3(types.Metadata{
		Name: "child-app",
	}, types.AppSpecV3{
		URI:        "http://localhost:8082",
		PublicAddr: scopedapp.ScopedAppPublicAddr(childScope, "child-app", "proxy.example.com"),
	})
	require.NoError(t, err)
	childApp.Scope = childScope
	childAppServer, err := types.NewAppServerV3FromApp(childApp, "child-host", "child-hostid")
	require.NoError(t, err)
	childAppServer.Scope = childScope
	_, err = srv.Auth().UpsertApplicationServer(ctx, childAppServer)
	require.NoError(t, err)

	// Helper to create a bot+SRA+client pinned to a given scope.
	setupBot := func(t *testing.T, botName, botScope, roleScope string) issuancev1pb.IssuanceServiceClient {
		t.Helper()

		bot, err := adminClient.BotServiceClient().CreateBot(ctx, machineidv1pb.CreateBotRequest_builder{
			Bot: machineidv1pb.Bot_builder{
				Kind:    types.KindBot,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: botName,
				}.Build(),
				Scope: botScope,
				Spec:  &machineidv1pb.BotSpec{},
			}.Build(),
		}.Build())
		require.NoError(t, err)

		sraResp, err := scopedSvc.CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
			Assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    scopedaccess.KindScopedRoleAssignment,
				SubKind: scopedaccess.SubKindDynamic,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.NewString(),
				}.Build(),
				Scope: botScope,
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					Bot: scopes.QualifiedName{Scope: botScope, Name: bot.GetMetadata().GetName()}.String(),
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  scopes.QualifiedName{Scope: roleScope, Name: "app-role"}.String(),
							Scope: roleScope,
						}.Build(),
					},
				}.Build(),
			}.Build(),
		}.Build())
		require.NoError(t, err)
		waitForSRACache(t, srv, sraResp)

		botClient, err := srv.NewClient(
			authtest.TestScopedBot(bot.GetMetadata().GetName(), botScope, true),
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = botClient.Close() })

		return issuancev1pb.NewIssuanceServiceClient(botClient.GetConnection())
	}

	// Generate a key pair shared across sub-tests.
	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPubKeyPEM, err := keys.MarshalPublicKey(key.Public())
	require.NoError(t, err)
	requestedTTL := time.Hour

	t.Run("parent-scoped bot accesses parent-scope app (exact match)", func(t *testing.T) {
		issuanceClient := setupBot(t, "parent-bot-exact", parentScope, parentScope)

		resp, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			App: issuancev1pb.UsageApp_builder{
				Name:        "parent-app",
				PublicAddr:  parentApp.GetPublicAddr(),
				ClusterName: srv.ClusterName(),
				Scope:       parentScope,
			}.Build(),
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, resp.GetCerts())
		require.NotEmpty(t, resp.GetCerts().GetTls())

		tlsCert, err := tlsca.ParseCertificatePEM(resp.GetCerts().GetTls())
		require.NoError(t, err)
		identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
		require.NoError(t, err)
		assert.Equal(t, "parent-app", identity.RouteToApp.Name)
		assert.Equal(t, parentScope, identity.RouteToApp.Scope)
	})

	t.Run("parent-scoped bot accesses child-scope app (descendant)", func(t *testing.T) {
		// Bot pinned to /testing can access app in /testing/team-a
		// because /testing/team-a is a descendant of /testing.
		issuanceClient := setupBot(t, "parent-bot-child", parentScope, parentScope)

		resp, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			App: issuancev1pb.UsageApp_builder{
				Name:        "child-app",
				PublicAddr:  childApp.GetPublicAddr(),
				ClusterName: srv.ClusterName(),
				Scope:       childScope,
			}.Build(),
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, resp.GetCerts())
		require.NotEmpty(t, resp.GetCerts().GetTls())

		tlsCert, err := tlsca.ParseCertificatePEM(resp.GetCerts().GetTls())
		require.NoError(t, err)
		identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
		require.NoError(t, err)
		assert.Equal(t, "child-app", identity.RouteToApp.Name)
		assert.Equal(t, childScope, identity.RouteToApp.Scope)
	})

	t.Run("child-scoped bot rejected at scope pin for parent-scope app", func(t *testing.T) {
		// Bot pinned to /testing/team-a requests an app in /testing.
		// /testing is NOT a descendant of /testing/team-a, so the scope
		// pin check rejects immediately — the app is never fetched.
		issuanceClient := setupBot(t, "child-bot-parent", childScope, childScope)

		_, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			App: issuancev1pb.UsageApp_builder{
				Name:        "parent-app",
				PublicAddr:  parentApp.GetPublicAddr(),
				ClusterName: srv.ClusterName(),
				Scope:       parentScope,
			}.Build(),
		}.Build())
		require.True(t, trace.IsAccessDenied(err), "expected access denied at scope pin check, got: %v", err)
	})

	t.Run("child-scoped bot accesses child-scope app (exact match)", func(t *testing.T) {
		issuanceClient := setupBot(t, "child-bot-exact", childScope, childScope)

		resp, err := issuanceClient.IssueScopedBotCerts(ctx, issuancev1pb.IssueScopedBotCertsRequest_builder{
			TlsPublicKey: tlsPubKeyPEM,
			Ttl:          durationpb.New(requestedTTL),
			App: issuancev1pb.UsageApp_builder{
				Name:        "child-app",
				PublicAddr:  childApp.GetPublicAddr(),
				ClusterName: srv.ClusterName(),
				Scope:       childScope,
			}.Build(),
		}.Build())
		require.NoError(t, err)
		require.NotNil(t, resp.GetCerts())
		require.NotEmpty(t, resp.GetCerts().GetTls())

		tlsCert, err := tlsca.ParseCertificatePEM(resp.GetCerts().GetTls())
		require.NoError(t, err)
		identity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
		require.NoError(t, err)
		assert.Equal(t, "child-app", identity.RouteToApp.Name)
		assert.Equal(t, childScope, identity.RouteToApp.Scope)
	})
}

func waitForSRACache(t *testing.T, srv *authtest.TLSServer, resps ...*scopedaccessv1.CreateScopedRoleAssignmentResponse) {
	t.Helper()
	ctx := t.Context()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		for _, resp := range resps {
			_, err := srv.Auth().ScopedAccessCache.GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
				Name:    resp.GetAssignment().GetMetadata().GetName(),
				SubKind: resp.GetAssignment().GetSubKind(),
				Scope:   resp.GetAssignment().GetScope(),
			}.Build())
			require.NoError(t, err)
		}
	}, 10*time.Second, 100*time.Millisecond)
}
