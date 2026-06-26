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

package gitserverv1

import (
	"context"
	"crypto/x509"
	"iter"
	"testing"
	"time"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	userexternalcredentialsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/integration/credentials"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

type fakeEmitter struct {
	events []apievents.AuditEvent
}

func (e *fakeEmitter) EmitAuditEvent(_ context.Context, event apievents.AuditEvent) error {
	e.events = append(e.events, event)
	return nil
}

type credentialsTestBackend struct {
	gitServers              *local.GitServerService
	integrations            map[string]types.Integration
	staticCreds             []types.PluginStaticCredentials
	userExternalCredentials map[string]*userexternalcredentialsv1.UserExternalCredentials
}

func (b *credentialsTestBackend) GetGitServer(ctx context.Context, name string) (types.Server, error) {
	return b.gitServers.GetGitServer(ctx, name)
}

func (b *credentialsTestBackend) CreateGitServer(ctx context.Context, server types.Server) (types.Server, error) {
	return b.gitServers.CreateGitServer(ctx, server)
}

func (b *credentialsTestBackend) UpdateGitServer(ctx context.Context, server types.Server) (types.Server, error) {
	return b.gitServers.UpdateGitServer(ctx, server)
}

func (b *credentialsTestBackend) UpsertGitServer(ctx context.Context, server types.Server) (types.Server, error) {
	return b.gitServers.UpsertGitServer(ctx, server)
}

func (b *credentialsTestBackend) DeleteGitServer(ctx context.Context, name string) error {
	return b.gitServers.DeleteGitServer(ctx, name)
}

func (b *credentialsTestBackend) ListGitServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	return b.gitServers.ListGitServers(ctx, pageSize, pageToken)
}

func (b *credentialsTestBackend) GetIntegration(_ context.Context, name string) (types.Integration, error) {
	ig, ok := b.integrations[name]
	if !ok {
		return nil, trace.NotFound("integration %q not found", name)
	}
	return ig, nil
}

func (b *credentialsTestBackend) ListIntegrations(_ context.Context, _ int, _ string) ([]types.Integration, string, error) {
	var result []types.Integration
	for _, ig := range b.integrations {
		result = append(result, ig)
	}
	return result, "", nil
}

func (b *credentialsTestBackend) GetPluginStaticCredentialsByLabels(_ context.Context, _ map[string]string) ([]types.PluginStaticCredentials, error) {
	return b.staticCreds, nil
}

func (b *credentialsTestBackend) GetUserExternalCredentials(_ context.Context, username, clientID string) (*userexternalcredentialsv1.UserExternalCredentials, error) {
	key := username + "/" + clientID
	creds, ok := b.userExternalCredentials[key]
	if !ok {
		return nil, trace.NotFound("credentials not found for %s", key)
	}
	return creds, nil
}

func (b *credentialsTestBackend) UpsertUserExternalCredentials(_ context.Context, creds *userexternalcredentialsv1.UserExternalCredentials) (*userexternalcredentialsv1.UserExternalCredentials, error) {
	key := creds.GetSpec().GetUser() + "/" + creds.GetMetadata().GetName()
	b.userExternalCredentials[key] = creds
	return creds, nil
}

func (b *credentialsTestBackend) DeleteUserExternalCredentials(_ context.Context, username, clientID string) error {
	key := username + "/" + clientID
	if _, ok := b.userExternalCredentials[key]; !ok {
		return trace.NotFound("credentials not found for %s", key)
	}
	delete(b.userExternalCredentials, key)
	return nil
}

func (b *credentialsTestBackend) ListUserExternalCredentials(_ context.Context, username string) ([]*userexternalcredentialsv1.UserExternalCredentials, error) {
	var out []*userexternalcredentialsv1.UserExternalCredentials
	prefix := username + "/"
	for key, creds := range b.userExternalCredentials {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			out = append(out, creds)
		}
	}
	return out, nil
}

func (b *credentialsTestBackend) IterateUserExternalCredentials(_ context.Context, req services.IterateUserExternalCredentialsRequest) iter.Seq2[*userexternalcredentialsv1.UserExternalCredentials, error] {
	return func(yield func(*userexternalcredentialsv1.UserExternalCredentials, error) bool) {
		for _, creds := range b.userExternalCredentials {
			if req.User != "" && creds.GetSpec().GetUser() != req.User {
				continue
			}
			if req.Name != "" && creds.GetMetadata().GetName() != req.Name {
				continue
			}
			if !yield(creds, nil) {
				return
			}
		}
	}
}

func newCredentialsTestSetup(t *testing.T) (*CredentialsService, *credentialsTestBackend, *fakeEmitter) {
	t.Helper()

	b, err := memory.New(memory.Config{})
	require.NoError(t, err)

	gitServersService, err := local.NewGitServerService(b)
	require.NoError(t, err)

	server := newServer(t, "my-org")
	_, err = gitServersService.CreateGitServer(context.Background(), server)
	require.NoError(t, err)

	ig, err := types.NewIntegrationGitHub(
		types.Metadata{Name: "my-org"},
		&types.GitHubIntegrationSpecV1{
			Organization: "my-org",
		},
	)
	require.NoError(t, err)
	ig.SetCredentials(&types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
			StaticCredentialsRef: credentials.NewRef(),
		},
	})
	ig.SetStatus(types.IntegrationStatusV1{
		GitHub: &types.GitHubIntegrationStatusV1{
			ClientID: fakeClientID,
		},
	})

	oauthCred, err := types.NewPluginStaticCredentials(
		types.Metadata{
			Name:         "cred",
			Labels:       map[string]string{},
		},
		types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
				OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
					ClientId:     fakeClientID,
					ClientSecret: fakeClientSecret,
				},
			},
		},
	)
	require.NoError(t, err)

	backend := &credentialsTestBackend{
		gitServers:   gitServersService,
		integrations: map[string]types.Integration{"my-org": ig},
		staticCreds:  []types.PluginStaticCredentials{oauthCred},
		userExternalCredentials: map[string]*userexternalcredentialsv1.UserExternalCredentials{},
	}

	clock := clockwork.NewFakeClock()
	emitter := &fakeEmitter{}

	authorizer := authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
		user, err := types.NewUser(fakeTeleportUser)
		if err != nil {
			return nil, err
		}
		return &authz.Context{
			User:    user,
			Checker: &fakeAccessChecker{allowVerbs: []string{types.VerbRead, types.VerbList, types.VerbDelete}, allowResource: true},
			Identity: authz.WrapIdentity(tlsca.Identity{
				Expires: clock.Now().Add(fakeIdentityTTL),
			}),
		}, nil
	})

	service, err := NewCredentialsService(CredentialsServiceConfig{
		Authorizer: authorizer,
		Cache:      backend,
		Backend:    backend,
		Emitter:    emitter,
		CertVerifier: func(certDER []byte) (*x509.Certificate, error) {
			return x509.ParseCertificate(certDER)
		},
		Clock: clock,
	})
	require.NoError(t, err)
	return service, backend, emitter
}

func addTestCredentials(t *testing.T, backend *credentialsTestBackend, username, clientID string) {
	t.Helper()
	creds := &userexternalcredentialsv1.UserExternalCredentials{}
	creds.SetKind("user_external_credentials")
	creds.SetVersion("v1")
	creds.SetMetadata(&headerv1.Metadata{Name: clientID})
	creds.SetSpec(&userexternalcredentialsv1.UserExternalCredentialsSpec{})
	creds.GetSpec().SetUser(username)
	ghOAuth := &userexternalcredentialsv1.GitHubOAuthCredentials{}
	ghOAuth.SetAccessToken("test-access-token")
	ghOAuth.SetAccessTokenExpiry(timestamppb.Now())
	ghOAuth.SetRefreshToken("test-refresh-token")
	creds.GetSpec().SetGithubOauth(ghOAuth)
	backend.userExternalCredentials[username+"/"+clientID] = creds
}

func addTestCredentialsWithUsername(t *testing.T, backend *credentialsTestBackend, username, clientID, githubUsername, githubUserID string) {
	t.Helper()
	creds := &userexternalcredentialsv1.UserExternalCredentials{}
	creds.SetKind("user_external_credentials")
	creds.SetVersion("v1")
	creds.SetMetadata(&headerv1.Metadata{Name: clientID})
	creds.SetSpec(&userexternalcredentialsv1.UserExternalCredentialsSpec{})
	creds.GetSpec().SetUser(username)
	ghOAuth := &userexternalcredentialsv1.GitHubOAuthCredentials{}
	ghOAuth.SetAccessToken("test-access-token")
	ghOAuth.SetAccessTokenExpiry(timestamppb.Now())
	ghOAuth.SetRefreshToken("test-refresh-token")
	ghOAuth.SetUsername(githubUsername)
	ghOAuth.SetUserId(githubUserID)
	creds.GetSpec().SetGithubOauth(ghOAuth)
	backend.userExternalCredentials[username+"/"+clientID] = creds
}

func TestCheckGitCredentials(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("valid credentials", func(t *testing.T) {
		service, backend, _ := newCredentialsTestSetup(t)
		addTestCredentials(t, backend, fakeTeleportUser, fakeClientID)

		servers, _, err := backend.gitServers.ListGitServers(ctx, 0, "")
		require.NoError(t, err)
		require.Len(t, servers, 1)

		req := &pb.CheckGitCredentialsRequest{}
		req.SetGitServerName(servers[0].GetName())
		resp, err := service.CheckGitCredentials(ctx, req)
		require.NoError(t, err)
		require.True(t, resp.GetValid())
	})

	t.Run("no credentials", func(t *testing.T) {
		service, backend, _ := newCredentialsTestSetup(t)

		servers, _, err := backend.gitServers.ListGitServers(ctx, 0, "")
		require.NoError(t, err)

		req := &pb.CheckGitCredentialsRequest{}
		req.SetGitServerName(servers[0].GetName())
		resp, err := service.CheckGitCredentials(ctx, req)
		require.NoError(t, err)
		require.False(t, resp.GetValid())
	})

	t.Run("missing git server name", func(t *testing.T) {
		service, _, _ := newCredentialsTestSetup(t)

		req := &pb.CheckGitCredentialsRequest{}
		_, err := service.CheckGitCredentials(ctx, req)
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("unknown git server", func(t *testing.T) {
		service, _, _ := newCredentialsTestSetup(t)

		req := &pb.CheckGitCredentialsRequest{}
		req.SetGitServerName("nonexistent")
		_, err := service.CheckGitCredentials(ctx, req)
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("returns github username", func(t *testing.T) {
		service, backend, _ := newCredentialsTestSetup(t)
		addTestCredentialsWithUsername(t, backend, fakeTeleportUser, fakeClientID, "greedy52", "12345")

		servers, _, err := backend.gitServers.ListGitServers(ctx, 0, "")
		require.NoError(t, err)

		req := &pb.CheckGitCredentialsRequest{}
		req.SetGitServerName(servers[0].GetName())
		resp, err := service.CheckGitCredentials(ctx, req)
		require.NoError(t, err)
		require.True(t, resp.GetValid())
		require.Equal(t, "greedy52", resp.GetGithubUsername())
	})
}

func TestRevokeGitCredentials(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("revoke existing credentials", func(t *testing.T) {
		service, backend, emitter := newCredentialsTestSetup(t)
		addTestCredentials(t, backend, fakeTeleportUser, fakeClientID)

		servers, _, err := backend.gitServers.ListGitServers(ctx, 0, "")
		require.NoError(t, err)

		req := &pb.RevokeGitCredentialsRequest{}
		req.SetGitServerName(servers[0].GetName())
		_, err = service.RevokeGitCredentials(ctx, req)
		require.NoError(t, err)

		// Verify credentials are gone.
		checkReq := &pb.CheckGitCredentialsRequest{}
		checkReq.SetGitServerName(servers[0].GetName())
		resp, err := service.CheckGitCredentials(ctx, checkReq)
		require.NoError(t, err)
		require.False(t, resp.GetValid())

		// Verify audit event was emitted.
		require.Len(t, emitter.events, 1)
		event, ok := emitter.events[0].(*apievents.GitCredentialRevoke)
		require.True(t, ok)
		require.Equal(t, "my-org", event.GitMetadata.Organization)
	})

	t.Run("revoke nonexistent credentials", func(t *testing.T) {
		service, backend, _ := newCredentialsTestSetup(t)

		servers, _, err := backend.gitServers.ListGitServers(ctx, 0, "")
		require.NoError(t, err)

		req := &pb.RevokeGitCredentialsRequest{}
		req.SetGitServerName(servers[0].GetName())
		_, err = service.RevokeGitCredentials(ctx, req)
		require.NoError(t, err)
	})
}

func TestSaveRefreshedCredentials_MaxCredentialTTL(t *testing.T) {
	t.Parallel()

	t.Run("default TTL", func(t *testing.T) {
		service, backend, _ := newCredentialsTestSetup(t)
		addTestCredentials(t, backend, fakeTeleportUser, fakeClientID)

		ig := backend.integrations["my-org"]
		creds := backend.userExternalCredentials[fakeTeleportUser+"/"+fakeClientID]

		token := &oauth2.Token{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
		}
		service.saveRefreshedCredentials(context.Background(), ig, creds, token)

		updated := backend.userExternalCredentials[fakeTeleportUser+"/"+fakeClientID]
		require.NotNil(t, updated.GetMetadata().GetExpires())
		expectedExpiry := service.clock.Now().Add(defaultMaxCredentialTTL)
		require.WithinDuration(t, expectedExpiry, updated.GetMetadata().GetExpires().AsTime(), time.Second)
	})

	t.Run("custom TTL from integration", func(t *testing.T) {
		service, backend, _ := newCredentialsTestSetup(t)
		addTestCredentials(t, backend, fakeTeleportUser, fakeClientID)

		customTTL := 3 * 24 * time.Hour
		ig, err := types.NewIntegrationGitHub(
			types.Metadata{Name: "my-org"},
			&types.GitHubIntegrationSpecV1{
				Organization:    "my-org",
				MaxCredentialTTL: gogotypes.DurationProto(customTTL),
			},
		)
		require.NoError(t, err)
		backend.integrations["my-org"] = ig

		creds := backend.userExternalCredentials[fakeTeleportUser+"/"+fakeClientID]
		token := &oauth2.Token{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
		}
		service.saveRefreshedCredentials(context.Background(), ig, creds, token)

		updated := backend.userExternalCredentials[fakeTeleportUser+"/"+fakeClientID]
		require.NotNil(t, updated.GetMetadata().GetExpires())
		expectedExpiry := service.clock.Now().Add(customTTL)
		require.WithinDuration(t, expectedExpiry, updated.GetMetadata().GetExpires().AsTime(), time.Second)
	})

	t.Run("nil MaxCredentialTTL uses 7-day default", func(t *testing.T) {
		service, backend, _ := newCredentialsTestSetup(t)
		addTestCredentials(t, backend, fakeTeleportUser, fakeClientID)

		ig, err := types.NewIntegrationGitHub(
			types.Metadata{Name: "my-org"},
			&types.GitHubIntegrationSpecV1{
				Organization: "my-org",
			},
		)
		require.NoError(t, err)

		creds := backend.userExternalCredentials[fakeTeleportUser+"/"+fakeClientID]
		token := &oauth2.Token{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
		}
		service.saveRefreshedCredentials(context.Background(), ig, creds, token)

		updated := backend.userExternalCredentials[fakeTeleportUser+"/"+fakeClientID]
		require.NotNil(t, updated.GetMetadata().GetExpires())
		expectedExpiry := service.clock.Now().Add(7 * 24 * time.Hour)
		require.WithinDuration(t, expectedExpiry, updated.GetMetadata().GetExpires().AsTime(), time.Second)
	})

	t.Run("refreshed token is saved", func(t *testing.T) {
		service, backend, _ := newCredentialsTestSetup(t)
		addTestCredentials(t, backend, fakeTeleportUser, fakeClientID)

		ig := backend.integrations["my-org"]
		creds := backend.userExternalCredentials[fakeTeleportUser+"/"+fakeClientID]

		newExpiry := time.Now().Add(8 * time.Hour)
		token := &oauth2.Token{
			AccessToken:  "refreshed-token",
			RefreshToken: "refreshed-refresh",
			Expiry:       newExpiry,
		}
		service.saveRefreshedCredentials(context.Background(), ig, creds, token)

		updated := backend.userExternalCredentials[fakeTeleportUser+"/"+fakeClientID]
		require.Equal(t, "refreshed-token", updated.GetSpec().GetGithubOauth().GetAccessToken())
		require.Equal(t, "refreshed-refresh", updated.GetSpec().GetGithubOauth().GetRefreshToken())
		require.WithinDuration(t, newExpiry, updated.GetSpec().GetGithubOauth().GetAccessTokenExpiry().AsTime(), time.Second)
	})
}
