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

package userpreferencesv1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	defaultUser     = "test-user"
	nonExistingUser = "non-existing-user"
)

func TestService_GetUserPreferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		userName string
		req      *userpreferencesv1.GetUserPreferencesRequest
		want     *userpreferencesv1.GetUserPreferencesResponse
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "success",
			userName: defaultUser,
			req:      &userpreferencesv1.GetUserPreferencesRequest{},
			want: &userpreferencesv1.GetUserPreferencesResponse{
				Preferences: &userpreferencesv1.UserPreferences{
					Assist: &userpreferencesv1.AssistUserPreferences{
						PreferredLogins: []string{},
						ViewMode:        userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_DOCKED,
					},
					Theme: userpreferencesv1.Theme_THEME_LIGHT,
					Onboard: &userpreferencesv1.OnboardUserPreferences{
						PreferredResources: []userpreferencesv1.Resource{},
					},
				},
			},
			wantErr: assert.NoError,
		},
		{
			name:     "access denied - user doesn't exist",
			userName: nonExistingUser,
			req:      &userpreferencesv1.GetUserPreferencesRequest{},
			want:     nil,
			wantErr:  assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxs, svc := initSvc(t)

			got, err := svc.GetUserPreferences(ctxs[tt.userName], tt.req)
			tt.wantErr(t, err)

			if tt.want != nil {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestService_UpsertUserPreferences(t *testing.T) {
	t.Parallel()

	defaultPreferences := &userpreferencesv1.UserPreferences{
		Assist: &userpreferencesv1.AssistUserPreferences{
			PreferredLogins: []string{},
			ViewMode:        userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_DOCKED,
		},
		Theme: userpreferencesv1.Theme_THEME_LIGHT,
		Onboard: &userpreferencesv1.OnboardUserPreferences{
			PreferredResources: []userpreferencesv1.Resource{},
		},
	}

	tests := []struct {
		name     string
		userName string
		req      *userpreferencesv1.UpsertUserPreferencesRequest
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "success",
			userName: defaultUser,
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Preferences: defaultPreferences,
			},
			wantErr: assert.NoError,
		},
		{
			name:     "access denied - user doesn't exist",
			userName: nonExistingUser,
			req: &userpreferencesv1.UpsertUserPreferencesRequest{
				Preferences: defaultPreferences,
			},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxs, svc := initSvc(t)

			_, err := svc.UpsertUserPreferences(ctxs[tt.userName], tt.req)
			if tt.wantErr(t, err) {
				return
			}
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

	role, err := types.NewRole("allow-rules", types.RoleSpecV6{})
	require.NoError(t, err)

	roles[defaultUser] = role

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
		Backend:    local.NewUserPreferencesService(backend),
		Authorizer: authorizer,
	})
	require.NoError(t, err)

	return ctxs, svc
}
