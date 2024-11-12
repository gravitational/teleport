// Teleport
// Copyright (C) 2023  Gravitational, Inc.
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

package okta

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
)

func userWithOrigin(t *testing.T, origin string) types.User {
	user, err := types.NewUser(uuid.NewString())
	require.NoError(t, err)
	if origin != "" {
		user.SetOrigin(origin)
	}
	return user
}

func TestCheckOrigin(t *testing.T) {
	t.Parallel()

	oktaCtx, err := authz.ContextForBuiltinRole(authz.BuiltinRole{
		Role:     types.RoleOkta,
		Username: string(types.RoleOkta),
	}, nil)
	require.NoError(t, err)

	nonOktaCtx, err := authz.ContextForBuiltinRole(authz.BuiltinRole{
		Role:     types.RoleAdmin,
		Username: string(types.RoleAdmin),
	}, nil)
	require.NoError(t, err)

	oktaResource := userWithOrigin(t, types.OriginOkta)
	nonOktaResource := userWithOrigin(t, "")

	tests := []struct {
		name       string
		authCtx    *authz.Context
		resource   types.ResourceWithLabels
		checkError require.ErrorAssertionFunc
	}{
		{
			name:       "resources created by okta service with origin supplied is allowed ",
			authCtx:    oktaCtx,
			resource:   oktaResource,
			checkError: require.NoError,
		}, {
			name:       "resources created by okta service without okta origin ia an error",
			authCtx:    oktaCtx,
			resource:   nonOktaResource,
			checkError: require.Error,
		}, {
			name:       "resources created by non-okta service must not supply okta origin",
			authCtx:    nonOktaCtx,
			resource:   oktaResource,
			checkError: require.Error,
		}, {
			name:       "resources created by non-okta service omit origin",
			authCtx:    nonOktaCtx,
			resource:   nonOktaResource,
			checkError: require.NoError,
		}, {
			name:       "resources created by non-okta service can supply non-okta origin",
			authCtx:    nonOktaCtx,
			resource:   userWithOrigin(t, types.OriginCloud),
			checkError: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.checkError(t,
				CheckOrigin(test.authCtx, test.resource))
		})
	}
}

func TestCheckAccess(t *testing.T) {
	t.Parallel()

	oktaCtx, err := authz.ContextForBuiltinRole(authz.BuiltinRole{
		Role:     types.RoleOkta,
		Username: string(types.RoleOkta),
	}, nil)
	require.NoError(t, err)

	nonOktaCtx, err := authz.ContextForBuiltinRole(authz.BuiltinRole{
		Role:     types.RoleAdmin,
		Username: string(types.RoleAdmin),
	}, nil)
	require.NoError(t, err)

	oktaResource, err := types.NewUser("okta-user")
	require.NoError(t, err)
	oktaResource.SetOrigin(types.OriginOkta)

	nonOktaResource, err := types.NewUser("not-an-okta-user")
	require.NoError(t, err)

	tests := []struct {
		name       string
		authCtx    *authz.Context
		resource   types.ResourceWithLabels
		verb       string
		checkError require.ErrorAssertionFunc
	}{
		{
			name:       "okta service creating okta resource is allowed",
			authCtx:    oktaCtx,
			resource:   oktaResource,
			verb:       types.VerbCreate,
			checkError: require.NoError,
		}, {
			name:       "okta service creating non-okta resource is an error",
			authCtx:    oktaCtx,
			resource:   nonOktaResource,
			verb:       types.VerbCreate,
			checkError: require.Error,
		}, {
			name:       "non-okta service creating okta resource is an error",
			authCtx:    nonOktaCtx,
			resource:   oktaResource,
			verb:       types.VerbCreate,
			checkError: require.Error,
		}, {
			name:       "non-okta service creating non-okta resource is allowed",
			authCtx:    nonOktaCtx,
			resource:   nonOktaResource,
			verb:       types.VerbCreate,
			checkError: require.NoError,
		}, {
			name:       "okta service updating okta resource is allowed",
			authCtx:    oktaCtx,
			resource:   oktaResource,
			verb:       types.VerbUpdate,
			checkError: require.NoError,
		}, {
			name:       "okta service updating non-okta resource is an error",
			authCtx:    oktaCtx,
			resource:   nonOktaResource,
			verb:       types.VerbUpdate,
			checkError: require.Error,
		}, {
			name:       "non-okta service updating okta resource is an error",
			authCtx:    nonOktaCtx,
			resource:   oktaResource,
			verb:       types.VerbUpdate,
			checkError: require.Error,
		}, {
			name:       "non-okta service updating non-okta resource is allowed",
			authCtx:    nonOktaCtx,
			resource:   nonOktaResource,
			verb:       types.VerbUpdate,
			checkError: require.NoError,
		}, {
			name:       "okta service deleting okta resource is allowed",
			authCtx:    oktaCtx,
			resource:   oktaResource,
			verb:       types.VerbDelete,
			checkError: require.NoError,
		}, {
			name:       "okta service deleting non-okta resource is an error",
			authCtx:    oktaCtx,
			resource:   nonOktaResource,
			verb:       types.VerbDelete,
			checkError: require.Error,
		}, {
			name:       "non-okta service deleting non-okta resource is allowed",
			authCtx:    nonOktaCtx,
			resource:   nonOktaResource,
			verb:       types.VerbDelete,
			checkError: require.NoError,
		}, {
			name:       "non-okta service deleting okta resource is allowed",
			authCtx:    nonOktaCtx,
			resource:   oktaResource,
			verb:       types.VerbDelete,
			checkError: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.checkError(t,
				CheckAccess(test.authCtx, test.resource, test.verb))
		})
	}
}
