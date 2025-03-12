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

package local

import (
	"context"
	"encoding/base32"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
)

func TestCreateResourcesProvisionToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tt := setupServicesContext(ctx, t)

	token, err := types.NewProvisionToken(
		"foo",
		types.SystemRoles{types.RoleNode},
		time.Time{},
	)
	require.NoError(t, err)
	require.NoError(t, CreateResources(ctx, tt.bk, token))

	s := NewProvisioningService(tt.bk)
	fetchedToken, err := s.GetToken(ctx, "foo")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(token, fetchedToken, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

func TestCreateResource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tt := setupServicesContext(ctx, t)
	cap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type: constants.Local,
	})
	require.NoError(t, err)

	// Check that the initial call to CreateResources creates the given resources.
	s, err := NewClusterConfigurationService(tt.bk)
	require.NoError(t, err)
	err = CreateResources(ctx, tt.bk, cap)
	require.NoError(t, err)
	got, err := s.GetAuthPreference(ctx)
	require.NoError(t, err)
	require.Equal(t, cap.GetType(), got.GetType())

	// Check that already exists errors are ignored and the resource is not
	// updated.
	cap.SetType(constants.SAML)
	err = CreateResources(ctx, tt.bk, cap)
	require.NoError(t, err)
	got, err = s.GetAuthPreference(ctx)
	require.NoError(t, err)
	require.NotEqual(t, cap.GetType(), got.GetType())
}

func TestUserResource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tt := setupServicesContext(ctx, t)
	runUserResourceTest(ctx, t, tt, false)
}

func TestUserResourceWithSecrets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tt := setupServicesContext(ctx, t)
	runUserResourceTest(ctx, t, tt, true)
}

func runUserResourceTest(
	ctx context.Context,
	t *testing.T,
	tt *servicesContext,
	withSecrets bool,
) {
	expiry := tt.bk.Clock().Now().Add(time.Minute)

	alice := newUserTestCase(t, "alice", nil, withSecrets, expiry)
	bob := newUserTestCase(t, "bob", nil, withSecrets, expiry)

	// Check basic dynamic item creation
	err := CreateResources(ctx, tt.bk, alice, bob)
	require.NoError(t, err)

	// Check that dynamically created item is compatible with service
	s, err := NewTestIdentityService(tt.bk)
	require.NoError(t, err)
	b, err := s.GetUser(ctx, "bob", withSecrets)
	require.NoError(t, err)
	require.True(t, services.UsersEquals(bob, b), "dynamically inserted user does not match")
	allUsers, err := s.GetUsers(ctx, withSecrets)
	require.NoError(t, err)
	require.Len(t, allUsers, 2, "expected exactly two users")
	for _, user := range allUsers {
		switch user.GetName() {
		case "alice":
			require.True(t, services.UsersEquals(alice, user), "alice does not match")
		case "bob":
			require.True(t, services.UsersEquals(bob, user), "bob does not match")
		default:
			t.Errorf("Unexpected user %q", user.GetName())
		}
	}

	// Advance the clock to let the users to expire.
	tt.bk.Clock().(*clockwork.FakeClock).Advance(2 * time.Minute)
	allUsers, err = s.GetUsers(ctx, withSecrets)
	require.NoError(t, err)
	require.Empty(t, allUsers, "expected all users to expire")
}

func TestCertAuthorityResource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tt := setupServicesContext(ctx, t)

	userCA := suite.NewTestCA(types.UserCA, "example.com")
	hostCA := suite.NewTestCA(types.HostCA, "example.com")

	// Check basic dynamic item creation
	err := CreateResources(ctx, tt.bk, userCA, hostCA)
	require.NoError(t, err)

	// Check that dynamically created item is compatible with service
	s := NewCAService(tt.bk)
	err = s.CompareAndSwapCertAuthority(userCA, userCA)
	require.NoError(t, err)
}

func TestTrustedClusterResource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tt := setupServicesContext(ctx, t)

	foo, err := types.NewTrustedCluster("foo", types.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{"bar", "baz"},
		Token:                "qux",
		ProxyAddress:         "quux",
		ReverseTunnelAddress: "quuz",
	})
	require.NoError(t, err)

	bar, err := types.NewTrustedCluster("bar", types.TrustedClusterSpecV2{
		Enabled:              false,
		Roles:                []string{"baz", "aux"},
		Token:                "quux",
		ProxyAddress:         "quuz",
		ReverseTunnelAddress: "corge",
	})
	require.NoError(t, err)

	// Check basic dynamic item creation
	err = CreateResources(ctx, tt.bk, foo, bar)
	require.NoError(t, err)

	s := NewCAService(tt.bk)
	_, err = s.GetTrustedCluster(ctx, "foo")
	require.NoError(t, err)
	_, err = s.GetTrustedCluster(ctx, "bar")
	require.NoError(t, err)
}

func TestGithubConnectorResource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tt := setupServicesContext(ctx, t)

	connector := &types.GithubConnectorV3{
		Kind:    types.KindGithubConnector,
		Version: types.V3,
		Metadata: types.Metadata{
			Name:      "github",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.GithubConnectorSpecV3{
			ClientID:     "aaa",
			ClientSecret: "bbb",
			RedirectURL:  "https://localhost:3080/v1/webapi/github/callback",
			Display:      "GitHub",
			TeamsToLogins: []types.TeamMapping{
				{
					Organization: "gravitational",
					Team:         "admins",
					Logins:       []string{"admin"},
					KubeGroups:   []string{"system:masters"},
				},
			},
		},
	}

	// Check basic dynamic item creation
	err := CreateResources(ctx, tt.bk, connector)
	require.NoError(t, err)

	s, err := NewTestIdentityService(tt.bk)
	require.NoError(t, err)
	_, err = s.GetGithubConnector(ctx, "github", true)
	require.NoError(t, err)
}

func localAuthSecretsTestCase(t *testing.T) types.LocalAuthSecrets {
	var auth types.LocalAuthSecrets
	var err error
	auth.PasswordHash, err = bcrypt.GenerateFromPassword([]byte("insecure"), bcrypt.MinCost)
	require.NoError(t, err)

	dev, err := services.NewTOTPDevice("otp", base32.StdEncoding.EncodeToString([]byte("abc123")), time.Now())
	require.NoError(t, err)
	auth.MFA = append(auth.MFA, dev)

	return auth
}

func newUserTestCase(t *testing.T, name string, roles []string, withSecrets bool, expires time.Time) types.User {
	user := types.UserV2{
		Kind:    types.KindUser,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      name,
			Namespace: apidefaults.Namespace,
			Expires:   &expires,
		},
		Spec: types.UserSpecV2{
			Roles: roles,
		},
	}
	if withSecrets {
		auth := localAuthSecretsTestCase(t)
		user.SetLocalAuth(&auth)
		user.SetWeakestDevice(types.MFADeviceKind_MFA_DEVICE_KIND_TOTP)
	}
	return &user
}

func TestBootstrapLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tt := setupServicesContext(ctx, t)

	nl, err := types.NewLock("test", types.LockSpecV2{
		Target: types.LockTarget{
			User: "user",
		},
		Message: "lock test",
	})
	require.NoError(t, err)
	require.NoError(t, CreateResources(ctx, tt.bk, nl))

	l, err := tt.suite.Access.GetLock(ctx, "test")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(nl, l, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}
