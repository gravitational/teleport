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

package assistv1_test

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	assistpb "github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/assist"
	"github.com/gravitational/teleport/lib/auth/assist/assistv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	defaultUser  = "test-user"
	noAccessUser = "user-no-access"
)

func TestService_CreateAssistantConversation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		username       string
		req            *assistpb.CreateAssistantConversationRequest
		wantErr        assert.ErrorAssertionFunc
		assertResponse func(t *testing.T, resp *assistpb.CreateAssistantConversationResponse)
	}{
		{
			name:     "success",
			username: defaultUser,
			req: &assistpb.CreateAssistantConversationRequest{
				Username:    defaultUser,
				CreatedTime: timestamppb.Now(),
			},
			wantErr: assert.NoError,
			assertResponse: func(t *testing.T, resp *assistpb.CreateAssistantConversationResponse) {
				require.NotEmpty(t, resp.GetId())
			},
		},
		{
			name:     "access denies - RBAC",
			username: noAccessUser,
			req: &assistpb.CreateAssistantConversationRequest{
				Username:    noAccessUser,
				CreatedTime: timestamppb.Now(),
			},
			wantErr: assert.Error,
		},
		{
			name:     "access denied - different user",
			username: defaultUser,
			req: &assistpb.CreateAssistantConversationRequest{
				Username:    noAccessUser,
				CreatedTime: timestamppb.Now(),
			},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxs, svc := initSvc(t)

			got, err := svc.CreateAssistantConversation(ctxs[tt.username], tt.req)
			tt.wantErr(t, err)

			if tt.assertResponse != nil {
				tt.assertResponse(t, got)
			}
		})
	}
}

func TestService_GetAssistantConversations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		username       string
		req            *assistpb.GetAssistantConversationsRequest
		wantErr        assert.ErrorAssertionFunc
		assertResponse func(t *testing.T, resp *assistpb.CreateAssistantConversationResponse)
	}{
		{
			name:     "success",
			username: defaultUser,
			req: &assistpb.GetAssistantConversationsRequest{
				Username: defaultUser,
			},
			wantErr: assert.NoError,
		},
		{
			name:     "access denies - RBAC",
			username: noAccessUser,
			req: &assistpb.GetAssistantConversationsRequest{
				Username: noAccessUser,
			},
			wantErr: assert.Error,
		},
		{
			name:     "access denied - different user",
			username: defaultUser,
			req: &assistpb.GetAssistantConversationsRequest{
				Username: noAccessUser,
			},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxs, svc := initSvc(t)

			_, err := svc.GetAssistantConversations(ctxs[tt.username], tt.req)
			tt.wantErr(t, err)
		})
	}
}

func TestService_DeleteAssistantConversations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		username    string
		req         *assistpb.DeleteAssistantConversationRequest
		wantConvErr assert.ErrorAssertionFunc
		wantErr     assert.ErrorAssertionFunc
	}{
		{
			name:     "success",
			username: defaultUser,
			req: &assistpb.DeleteAssistantConversationRequest{
				Username: defaultUser,
			},
			wantConvErr: assert.NoError,
			wantErr:     assert.NoError,
		},
		{
			name:     "access denies - RBAC",
			username: noAccessUser,
			req: &assistpb.DeleteAssistantConversationRequest{
				Username: noAccessUser,
			},
			wantConvErr: assert.Error,
			wantErr:     assert.Error,
		},
		{
			name:     "access denied - different user",
			username: defaultUser,
			req: &assistpb.DeleteAssistantConversationRequest{
				Username: noAccessUser,
			},
			wantConvErr: assert.NoError,
			wantErr:     assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxs, svc := initSvc(t)

			// Create a conversation that we can remove, so we don't hit "conversation doesn't exist" error
			convMsg, err := svc.CreateAssistantConversation(ctxs[tt.username], &assistpb.CreateAssistantConversationRequest{
				Username:    tt.username,
				CreatedTime: timestamppb.Now(),
			})
			tt.wantConvErr(t, err)

			conversationID := convMsg.GetId()

			tt.req.ConversationId = conversationID

			_, err = svc.DeleteAssistantConversation(ctxs[tt.username], tt.req)
			tt.wantErr(t, err)
		})
	}
}

func TestService_InsertAssistantMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		username    string
		req         *assistpb.CreateAssistantMessageRequest
		wantConvErr assert.ErrorAssertionFunc
		wantErr     assert.ErrorAssertionFunc
	}{
		{
			name:     "success",
			username: defaultUser,
			req: &assistpb.CreateAssistantMessageRequest{
				Username: defaultUser,
				Message: &assistpb.AssistantMessage{
					Type:        string(assist.MessageKindAssistantMessage),
					CreatedTime: timestamppb.Now(),
					Payload:     "Blah",
				},
			},
			wantConvErr: assert.NoError,
			wantErr:     assert.NoError,
		},
		{
			name:     "access denies - RBAC",
			username: noAccessUser,
			req: &assistpb.CreateAssistantMessageRequest{
				Username: noAccessUser,
			},
			wantConvErr: assert.Error,
			wantErr:     assert.Error,
		},
		{
			name:     "access denied - different user",
			username: defaultUser,
			req: &assistpb.CreateAssistantMessageRequest{
				Username: noAccessUser,
			},
			wantConvErr: assert.NoError,
			wantErr:     assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxs, svc := initSvc(t)

			// Create a conversation that we can remove, so we don't hit "conversation doesn't exist" error
			convMsg, err := svc.CreateAssistantConversation(ctxs[tt.username], &assistpb.CreateAssistantConversationRequest{
				Username:    tt.username,
				CreatedTime: timestamppb.Now(),
			})
			tt.wantConvErr(t, err)

			conversationID := convMsg.GetId()

			tt.req.ConversationId = conversationID

			_, err = svc.CreateAssistantMessage(ctxs[tt.username], tt.req)
			tt.wantErr(t, err)
		})
	}
}

func TestService_SearchUnifiedResources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		username    string
		req         *assistpb.SearchUnifiedResourcesRequest
		returnedLen int
	}{
		{
			username: defaultUser,
			req: &assistpb.SearchUnifiedResourcesRequest{
				Kinds: []string{types.KindNode},
			},
			returnedLen: 2,
		},
		{
			username: noAccessUser,
			req: &assistpb.SearchUnifiedResourcesRequest{
				Kinds: []string{types.KindNode},
			},
			returnedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			ctxs, svc := initSvc(t)
			require.Eventually(t, func() bool {
				resp, err := svc.SearchUnifiedResources(ctxs[tt.username], tt.req)
				require.NoError(t, err)
				return tt.returnedLen == len(resp.GetResources())
			}, 5*time.Second, 100*time.Millisecond)
		})
	}
}

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.RoleGetter
	services.UserGetter
}

func initSvc(t *testing.T) (map[string]context.Context, *assistv1.Service) {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	trustSvc := local.NewCAService(backend)
	roleSvc := local.NewAccessService(backend)
	userSvc := local.NewTestIdentityService(backend)
	presenceSvc := local.NewPresenceService(backend)

	_, err = clusterConfigSvc.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	require.NoError(t, clusterConfigSvc.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	_, err = clusterConfigSvc.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)
	_, err = clusterConfigSvc.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	require.NoError(t, err)

	accessPoint := &testClient{
		ClusterConfiguration: clusterConfigSvc,
		Trust:                trustSvc,
		RoleGetter:           roleSvc,
		UserGetter:           userSvc,
	}

	n1, err := types.NewServer("node-1", types.KindNode, types.ServerSpecV2{})
	require.NoError(t, err)
	n2, err := types.NewServer("node-2", types.KindNode, types.ServerSpecV2{})
	require.NoError(t, err)
	_, err = presenceSvc.UpsertNode(ctx, n1)
	require.NoError(t, err)
	_, err = presenceSvc.UpsertNode(ctx, n2)
	require.NoError(t, err)

	accesslistSvc, err := local.NewAccessListService(backend, clockwork.NewFakeClock())
	require.NoError(t, err)

	accessService := local.NewAccessService(backend)
	eventService := local.NewEventsService(backend)
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Client:    eventService,
			Component: "test",
		},
		LockGetter: accessService,
	})
	require.NoError(t, err)

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: "test-cluster",
		AccessPoint: accessPoint,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	roles := map[string]types.Role{}

	role, err := types.NewRole("allow-rules", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Namespaces: []string{},
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			Rules: []types.Rule{
				{
					Resources: []string{types.KindAssistant},
					Verbs:     []string{types.VerbList, types.VerbRead, types.VerbUpdate, types.VerbCreate, types.VerbDelete},
				},
				{
					Resources: []string{types.KindNode},
					Verbs:     []string{types.VerbList, types.VerbRead},
				},
			},
		},
	})
	require.NoError(t, err)

	roles[defaultUser] = role

	roleNoAccess, err := types.NewRole("no-rules", types.RoleSpecV6{
		Allow: types.RoleConditions{},
	})
	require.NoError(t, err)
	roles["user-no-access"] = roleNoAccess

	ctxs := make(map[string]context.Context, len(roles))
	for username, role := range roles {
		role, err = roleSvc.CreateRole(ctx, role)
		require.NoError(t, err)

		user, err := types.NewUser(username)
		user.AddRole(role.GetName())
		require.NoError(t, err)

		user, err = userSvc.CreateUser(ctx, user)
		require.NoError(t, err)

		ctx = authz.ContextWithUser(ctx, authz.LocalUser{
			Username: user.GetName(),
			Identity: tlsca.Identity{
				Username: user.GetName(),
				Groups:   []string{role.GetName()},
			},
		})
		ctxs[user.GetName()] = ctx
	}

	embedder := ai.MockEmbedder{
		TimesCalled: make(map[string]int),
	}

	embeddings := &ai.SimpleRetriever{}
	embeddingSrv := local.NewEmbeddingsService(backend)
	svc, err := assistv1.NewService(&assistv1.ServiceConfig{
		Backend:    local.NewAssistService(backend),
		Authorizer: authorizer,
		Embeddings: embeddings,
		ResourceGetter: &resourceGetterAllImpl{
			PresenceService: presenceSvc,
			AccessLists:     accesslistSvc,
		},
		Embedder: &embedder,
	})
	require.NoError(t, err)

	unifiedResourcesCache, err := services.NewUnifiedResourceCache(ctx, services.UnifiedResourceCacheConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			QueueSize:    defaults.UnifiedResourcesQueueSize,
			Component:    teleport.ComponentUnifiedResource,
			Client:       eventService,
			MaxStaleness: time.Second,
		},
		ResourceGetter: &resourceGetterAllImpl{
			PresenceService: presenceSvc,
			AccessLists:     accesslistSvc,
		},
	})
	require.NoError(t, err)

	log.Debugf("Starting embedding watcher")
	embeddingProcessor := ai.NewEmbeddingProcessor(&ai.EmbeddingProcessorConfig{
		AIClient:            &embedder,
		EmbeddingsRetriever: embeddings,
		EmbeddingSrv:        embeddingSrv,
		NodeSrv:             unifiedResourcesCache,
		Jitter:              retryutils.NewFullJitter(),
		Log:                 log.NewEntry(log.StandardLogger()),
	})
	log.Debugf("Starting embedding processor")

	embeddingProcessorCtx, embeddingProcessorCancel := context.WithCancel(context.Background())
	go embeddingProcessor.Run(embeddingProcessorCtx, time.Millisecond*100, time.Millisecond*100)
	t.Cleanup(embeddingProcessorCancel)
	return ctxs, svc
}

type resourceGetterAllImpl struct {
	*local.PresenceService
	services.AccessLists
}

func (g *resourceGetterAllImpl) GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error) {
	kubeServers, err := g.PresenceService.GetKubernetesServers(ctx)
	if err != nil {
		return nil, err
	}

	for _, kubeServer := range kubeServers {
		if kubeServer.GetName() == name {
			return kubeServer.GetCluster(), nil
		}
	}

	return nil, trace.NotFound("cluster not found")
}

func (g *resourceGetterAllImpl) GetApp(ctx context.Context, name string) (types.Application, error) {
	return nil, nil
}

func (g *resourceGetterAllImpl) GetDatabase(ctx context.Context, name string) (types.Database, error) {
	return nil, nil
}

func (g *resourceGetterAllImpl) GetWindowsDesktops(ctx context.Context, _ types.WindowsDesktopFilter) ([]types.WindowsDesktop, error) {
	return nil, nil
}

func (m *resourceGetterAllImpl) GetDatabaseServers(_ context.Context, _ string, _ ...services.MarshalOption) ([]types.DatabaseServer, error) {
	return nil, nil
}

func (m *resourceGetterAllImpl) GetKubernetesServers(_ context.Context) ([]types.KubeServer, error) {
	return nil, nil
}

func (m *resourceGetterAllImpl) GetApplicationServers(_ context.Context, _ string) ([]types.AppServer, error) {
	return nil, nil
}

func (m *resourceGetterAllImpl) ListSAMLIdPServiceProviders(_ context.Context, _ int, _ string) ([]types.SAMLIdPServiceProvider, string, error) {
	return nil, "", nil
}
