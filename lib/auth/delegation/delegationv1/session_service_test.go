/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package delegationv1_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/delegation/delegationv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func sessionServiceTestPack(t *testing.T) (*delegationv1.SessionService, *sessionTestPack) {
	t.Helper()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	profileUpstream, err := local.NewDelegationProfileService(backend)
	require.NoError(t, err)

	sessionUpstream, err := local.NewDelegationSessionService(backend)
	require.NoError(t, err)

	accessService := local.NewAccessService(backend)
	require.NoError(t, err)

	presenceService := local.NewPresenceService(backend)
	require.NoError(t, err)

	identityService, err := local.NewIdentityService(backend)
	require.NoError(t, err)

	appServer, err := types.NewAppServerV3(
		types.Metadata{Name: "hr-system"},
		types.AppServerSpecV3{
			HostID: uuid.NewString(),
			App: &types.AppV3{
				Metadata: types.Metadata{Name: "hr-system"},
				Spec:     types.AppSpecV3{URI: "https://hr-system"},
			},
		},
	)
	require.NoError(t, err)

	_, err = presenceService.UpsertApplicationServer(t.Context(), appServer)
	require.NoError(t, err)

	pack := &sessionTestPack{
		profiles: profileUpstream,
		sessions: sessionUpstream,
		access:   accessService,
		presence: presenceService,
		identity: identityService,
	}

	service, err := delegationv1.NewSessionService(delegationv1.SessionServiceConfig{
		Authorizer: authz.AuthorizerFunc(func(context.Context) (*authz.Context, error) {
			checker, err := services.NewAccessChecker(
				&services.AccessInfo{
					Roles: pack.user.GetRoles(),
				},
				"test.teleport.sh",
				accessService,
			)
			require.NoError(t, err)

			return &authz.Context{
				User:                 pack.user,
				AdminActionAuthState: pack.adminActionAuthState,
				Checker:              checker,
			}, nil
		}),
		ProfileReader:  profileUpstream,
		SessionWriter:  sessionUpstream,
		ResourceLister: presenceService,
		RoleGetter:     accessService,
		UserGetter:     identityService,
		Logger:         logtest.NewLogger(),
	})

	return service, pack
}

type sessionTestPack struct {
	profiles services.DelegationProfiles
	sessions services.DelegationSessions
	access   services.Access
	presence services.Presence
	identity services.Identity

	user                 types.User
	adminActionAuthState authz.AdminActionAuthState
}

func (p *sessionTestPack) authenticate(
	t *testing.T,
	name string,
	mfaState authz.AdminActionAuthState,
	roleSpec types.RoleSpecV6,
) {
	t.Helper()

	user, err := types.NewUser(name)
	require.NoError(t, err)

	role, err := types.NewRole(name, roleSpec)
	require.NoError(t, err)
	user.AddRole(role.GetName())

	_, err = p.access.CreateRole(t.Context(), role)
	require.NoError(t, err)

	_, err = p.identity.CreateUser(t.Context(), user)
	require.NoError(t, err)

	p.user = user
	p.adminActionAuthState = mfaState
}
