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
					UnifiedResourcePreferences: &userpreferencesv1.UnifiedResourcePreferences{
						DefaultTab:     userpreferencesv1.DefaultTab_DEFAULT_TAB_ALL,
						ViewMode:       userpreferencesv1.ViewMode_VIEW_MODE_CARD,
						LabelsViewMode: userpreferencesv1.LabelsViewMode_LABELS_VIEW_MODE_COLLAPSED,
					},
					Onboard: &userpreferencesv1.OnboardUserPreferences{
						PreferredResources: []userpreferencesv1.Resource{},
						MarketingParams:    &userpreferencesv1.MarketingParams{},
					},
					ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
						PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{},
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
		ClusterPreferences: &userpreferencesv1.ClusterUserPreferences{
			PinnedResources: &userpreferencesv1.PinnedResourcesUserPreferences{
				ResourceIds: []string{"node1", "node2"},
			},
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

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.RoleGetter
	services.UserGetter
}

func initSvc(t *testing.T) (map[string]context.Context, *Service) {
	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	clusterConfigSvc, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	trustSvc := local.NewCAService(backend)
	roleSvc := local.NewAccessService(backend)
	userSvc := local.NewTestIdentityService(backend)

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

	svc, err := NewService(&ServiceConfig{
		Backend:    local.NewUserPreferencesService(backend),
		Authorizer: authorizer,
	})
	require.NoError(t, err)

	return ctxs, svc
}
