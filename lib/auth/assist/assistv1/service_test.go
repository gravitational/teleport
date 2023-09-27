/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package assistv1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	assistpb "github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/assist"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
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
		name     string
		username string
		req      *assistpb.DeleteAssistantConversationRequest
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "success",
			username: defaultUser,
			req: &assistpb.DeleteAssistantConversationRequest{
				Username: defaultUser,
			},
			wantErr: assert.NoError,
		},
		{
			name:     "access denies - RBAC",
			username: noAccessUser,
			req: &assistpb.DeleteAssistantConversationRequest{
				Username: noAccessUser,
			},
			wantErr: assert.Error,
		},
		{
			name:     "access denied - different user",
			username: defaultUser,
			req: &assistpb.DeleteAssistantConversationRequest{
				Username: noAccessUser,
			},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxs, svc := initSvc(t)

			// Create a conversation that we can remove, so we don't hit "conversation doesn't exist" error
			convMsg, err := svc.backend.CreateAssistantConversation(ctxs[tt.username], &assistpb.CreateAssistantConversationRequest{
				Username:    tt.username,
				CreatedTime: timestamppb.Now(),
			})
			require.NoError(t, err)

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
		name     string
		username string
		req      *assistpb.CreateAssistantMessageRequest
		wantErr  assert.ErrorAssertionFunc
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
			wantErr: assert.NoError,
		},
		{
			name:     "access denies - RBAC",
			username: noAccessUser,
			req: &assistpb.CreateAssistantMessageRequest{
				Username: noAccessUser,
			},
			wantErr: assert.Error,
		},
		{
			name:     "access denied - different user",
			username: defaultUser,
			req: &assistpb.CreateAssistantMessageRequest{
				Username: noAccessUser,
			},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxs, svc := initSvc(t)

			// Create a conversation that we can remove, so we don't hit "conversation doesn't exist" error
			convMsg, err := svc.backend.CreateAssistantConversation(ctxs[tt.username], &assistpb.CreateAssistantConversationRequest{
				Username:    tt.username,
				CreatedTime: timestamppb.Now(),
			})
			require.NoError(t, err)

			conversationID := convMsg.GetId()

			tt.req.ConversationId = conversationID

			_, err = svc.CreateAssistantMessage(ctxs[tt.username], tt.req)
			tt.wantErr(t, err)
		})
	}
}

func initSvc(t *testing.T) (map[string]context.Context, *Service) {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	trustSvc := local.NewCAService(backend)
	roleSvc := local.NewAccessService(backend)
	userSvc := local.NewIdentityService(backend)

	require.NoError(t, clusterConfigSvc.SetAuthPreference(ctx, types.DefaultAuthPreference()))
	require.NoError(t, clusterConfigSvc.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	require.NoError(t, clusterConfigSvc.SetClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig()))
	require.NoError(t, clusterConfigSvc.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig()))

	accessPoint := struct {
		services.ClusterConfiguration
		services.Trust
		services.RoleGetter
		services.UserGetter
	}{
		ClusterConfiguration: clusterConfigSvc,
		Trust:                trustSvc,
		RoleGetter:           roleSvc,
		UserGetter:           userSvc,
	}

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
			Rules: []types.Rule{
				{
					Resources: []string{types.KindAssistant},
					Verbs:     []string{types.VerbList, types.VerbRead, types.VerbUpdate, types.VerbCreate, types.VerbDelete},
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
		err = roleSvc.CreateRole(ctx, role)
		require.NoError(t, err)

		user, err := types.NewUser(username)
		user.AddRole(role.GetName())
		require.NoError(t, err)

		err = userSvc.CreateUser(user)
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

	svc, err := NewService(&ServiceConfig{
		Backend:        local.NewAssistService(backend),
		Authorizer:     authorizer,
		Embeddings:     &ai.SimpleRetriever{},
		ResourceGetter: &nodeGetterFake{},
	})
	require.NoError(t, err)

	return ctxs, svc
}

type nodeGetterFake struct {
}

func (g *nodeGetterFake) GetNode(ctx context.Context, namespace, name string) (types.Server, error) {
	return nil, nil
}
