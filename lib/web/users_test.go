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

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
)

func TestRequestParameters(t *testing.T) {
	tests := []struct {
		name         string
		username     string
		role         []string
		traitsPreset *traitsPreset
		allTraits    map[string][]string
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name:         "empty request",
			username:     "",
			role:         nil,
			traitsPreset: nil,
			allTraits:    nil,
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("missing user name"))
			},
		},
		{
			name:         "empty name",
			username:     "",
			role:         []string{"testrole"},
			traitsPreset: nil,
			allTraits:    nil,
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("missing user name"))
			},
		},
		{
			name:         "empty role",
			username:     "testuser",
			role:         nil,
			traitsPreset: nil,
			allTraits:    nil,
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("missing roles"))
			},
		},
		{
			name:         "both traitsPreset and allTraits",
			username:     "testuser",
			role:         []string{"testrole"},
			traitsPreset: &traitsPreset{Logins: &[]string{"root"}},
			allTraits:    map[string][]string{"logins": {"root"}},
			errAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("either traits or allTraits must be provided"))
			},
		},
		{
			name:         "user without traits",
			username:     "testuser",
			role:         []string{"testrole"},
			traitsPreset: nil,
			allTraits:    nil,
			errAssertion: require.NoError,
		},
		{
			name:         "user with traitsPreset",
			username:     "testuser",
			role:         []string{"testrole"},
			traitsPreset: &traitsPreset{Logins: &[]string{"root"}},
			allTraits:    map[string][]string{},
			errAssertion: require.NoError,
		},
		{
			name:         "user with allTraits",
			username:     "testuser",
			role:         []string{"testrole"},
			traitsPreset: nil,
			allTraits:    map[string][]string{"logins": {"root"}},
			errAssertion: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := saveUserRequest{
				Name:         test.username,
				Roles:        test.role,
				TraitsPreset: test.traitsPreset,
				AllTraits:    test.allTraits,
			}

			err := r.checkAndSetDefaults()
			test.errAssertion(t, err)
		})
	}
}

func TestCRUDs(t *testing.T) {
	u := saveUserRequest{
		Name:         "testname",
		Roles:        []string{"testrole"},
		TraitsPreset: nil,
	}

	m := &mockedUserAPIGetter{}
	m.mockCreateUser = func(ctx context.Context, user types.User) (types.User, error) {
		return user, nil
	}

	m.mockGetUser = func(ctx context.Context, name string, withSecrets bool) (types.User, error) {
		return types.NewUser(name)
	}

	m.mockUpdateUser = func(ctx context.Context, user types.User) (types.User, error) {
		return user, nil
	}

	m.mockGetUsers = func(ctx context.Context, withSecrets bool) ([]types.User, error) {
		u, err := types.NewUser("testname")
		return []types.User{u}, err
	}

	m.mockDeleteUser = func(ctx context.Context, user string) error {
		return nil
	}

	// test create
	user, err := createUser(newRequest(t, u), m, "")
	require.NoError(t, err)
	require.Equal(t, "testname", user.Name)
	require.Equal(t, "local", user.AuthType)
	require.Contains(t, user.Roles, "testrole")

	// test update
	u.Roles = []string{"newrole"}
	user, err = updateUser(newRequest(t, u), m)
	require.NoError(t, err)
	require.Contains(t, user.Roles, "newrole")

	// test list
	users, err := getUsers(context.Background(), m)
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, "testname", users[0].Name)

	// test delete
	param := httprouter.Params{httprouter.Param{Key: "username", Value: "testname"}}
	req, err := http.NewRequest("", "/:username", nil)
	require.NoError(t, err)

	err = deleteUser(req, param, m, "self")
	require.NoError(t, err)
}

func TestUpdateUser_updateUserTraitsPreset(t *testing.T) {
	defaultRoles := []string{"role1"}
	defaultLogins := []string{"login1"}
	tests := []struct {
		name           string
		updateReq      saveUserRequest
		expectedTraits map[string][]string
	}{
		{
			name: "Logins",
			updateReq: saveUserRequest{
				Name:         "setlogins",
				Roles:        defaultRoles,
				TraitsPreset: &traitsPreset{Logins: &[]string{"login1", "login2"}},
			},
			expectedTraits: map[string][]string{
				constants.TraitLogins: {"login1", "login2"},
			},
		},
		{
			name: "DB",
			updateReq: saveUserRequest{
				Name:  "setdb",
				Roles: defaultRoles,
				TraitsPreset: &traitsPreset{
					Logins:        &defaultLogins,
					DatabaseUsers: &[]string{"dbuser1", "dbuser2"},
					DatabaseNames: &[]string{"dbname1", "dbname2"},
				},
			},
			expectedTraits: map[string][]string{
				constants.TraitDBUsers: {"dbuser1", "dbuser2"},
				constants.TraitDBNames: {"dbname1", "dbname2"},
				constants.TraitLogins:  defaultLogins,
			},
		},
		{
			name: "Kube",
			updateReq: saveUserRequest{
				Name:  "setkube",
				Roles: defaultRoles,
				TraitsPreset: &traitsPreset{
					Logins:     &defaultLogins,
					KubeUsers:  &[]string{"kubeuser1", "kubeuser2"},
					KubeGroups: &[]string{"kubegroup1", "kubegroup2"},
				},
			},
			expectedTraits: map[string][]string{
				constants.TraitKubeUsers:  {"kubeuser1", "kubeuser2"},
				constants.TraitKubeGroups: {"kubegroup1", "kubegroup2"},
				constants.TraitLogins:     defaultLogins,
			},
		},
		{
			name: "WindowsLogins",
			updateReq: saveUserRequest{
				Name:  "setwindowslogins",
				Roles: defaultRoles,
				TraitsPreset: &traitsPreset{
					Logins:        &defaultLogins,
					WindowsLogins: &[]string{"login1", "login2"},
				},
			},
			expectedTraits: map[string][]string{
				constants.TraitWindowsLogins: {"login1", "login2"},
				constants.TraitLogins:        defaultLogins,
			},
		},
		{
			name: "AWSRoleARNs",
			updateReq: saveUserRequest{
				Name:  "setawsrolearns",
				Roles: defaultRoles,
				TraitsPreset: &traitsPreset{
					Logins:      &defaultLogins,
					AWSRoleARNs: &[]string{"arn1", "arn2"},
				},
			},
			expectedTraits: map[string][]string{
				constants.TraitAWSRoleARNs: {"arn1", "arn2"},
				constants.TraitLogins:      defaultLogins,
			},
		},
		{
			name: "Deduplicates",
			updateReq: saveUserRequest{
				Name:         "deduplicates",
				Roles:        defaultRoles,
				TraitsPreset: &traitsPreset{Logins: &[]string{"login1", "login2", "login1"}},
			},
			expectedTraits: map[string][]string{
				constants.TraitLogins: {"login1", "login2"},
			},
		},
		{
			name: "RemovesAll",
			updateReq: saveUserRequest{
				Name:         "removesall",
				Roles:        defaultRoles,
				TraitsPreset: &traitsPreset{Logins: &[]string{}},
			},
			expectedTraits: map[string][]string{
				constants.TraitLogins: {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			user, err := types.NewUser(tt.name)
			require.NoError(t, err)
			user.SetRoles(defaultRoles)
			user.SetLogins(defaultLogins)

			m := &mockedUserAPIGetter{}
			m.mockGetUser = func(ctx context.Context, name string, withSecrets bool) (types.User, error) {
				return user, nil
			}
			m.mockUpdateUser = func(ctx context.Context, user types.User) (types.User, error) {
				return user, nil
			}

			_, err = updateUser(newRequest(t, tt.updateReq), m)
			require.NoError(t, err)

			// The traits match
			require.Equal(t, tt.expectedTraits, user.GetTraits())

			// Other fields dont't change
			require.ElementsMatch(t, user.GetRoles(), defaultRoles)

			// We can read back the user traits
			uiUser, err := getUser(context.Background(), tt.name, m)
			require.NoError(t, err)

			require.ElementsMatch(t, uiUser.Traits.Logins, tt.expectedTraits[constants.TraitLogins])
			require.ElementsMatch(t, uiUser.Traits.DatabaseUsers, tt.expectedTraits[constants.TraitDBUsers])
			require.ElementsMatch(t, uiUser.Traits.DatabaseNames, tt.expectedTraits[constants.TraitDBNames])
			require.ElementsMatch(t, uiUser.Traits.KubeUsers, tt.expectedTraits[constants.TraitKubeUsers])
			require.ElementsMatch(t, uiUser.Traits.KubeGroups, tt.expectedTraits[constants.TraitKubeGroups])
			require.ElementsMatch(t, uiUser.Traits.WindowsLogins, tt.expectedTraits[constants.TraitWindowsLogins])
			require.ElementsMatch(t, uiUser.Traits.AWSRoleARNs, tt.expectedTraits[constants.TraitAWSRoleARNs])
		})
	}
}

func TestUpdateUser_setTraitsWithAllTraits(t *testing.T) {
	defaultRoles := []string{"role1"}

	// create user
	user, err := types.NewUser("alice")
	require.NoError(t, err)
	user.SetRoles(defaultRoles)

	m := &mockedUserAPIGetter{}
	m.mockGetUser = func(ctx context.Context, name string, withSecrets bool) (types.User, error) {
		return user, nil
	}
	m.mockUpdateUser = func(ctx context.Context, user types.User) (types.User, error) {
		return user, nil
	}

	// update user with AllTraits
	allTraitsWithValue := saveUserRequest{
		Name:      "setlogins",
		Roles:     defaultRoles,
		AllTraits: map[string][]string{"logins": {"root", "admin"}},
	}
	_, err = updateUser(newRequest(t, allTraitsWithValue), m)
	require.NoError(t, err)
	require.Equal(t, allTraitsWithValue.AllTraits, user.GetTraits())

	// verify other fields dont't change
	require.ElementsMatch(t, user.GetRoles(), defaultRoles)

	// update user with empty AllTraits
	emptyAllTraits := saveUserRequest{
		Name:      "setlogins",
		Roles:     defaultRoles,
		AllTraits: map[string][]string{},
	}
	_, err = updateUser(newRequest(t, emptyAllTraits), m)
	require.NoError(t, err)
	// empty AllTraits field should delete existing traits
	require.Equal(t, emptyAllTraits.AllTraits, user.GetTraits())
}

func TestCRUDErrors(t *testing.T) {
	m := &mockedUserAPIGetter{}
	m.mockCreateUser = func(ctx context.Context, user types.User) (types.User, error) {
		return nil, trace.AlreadyExists("")
	}

	m.mockGetUser = func(ctx context.Context, name string, withSecrets bool) (types.User, error) {
		return nil, trace.NotFound("")
	}

	m.mockUpdateUser = func(ctx context.Context, user types.User) (types.User, error) {
		return nil, trace.NotFound("")
	}

	m.mockGetUsers = func(ctx context.Context, withSecrets bool) ([]types.User, error) {
		return nil, trace.AccessDenied("")
	}

	m.mockDeleteUser = func(ctx context.Context, user string) error {
		return trace.NotFound("")
	}

	u := saveUserRequest{
		Name:         "testname",
		Roles:        []string{"testrole"},
		TraitsPreset: &traitsPreset{Logins: nil},
	}

	// update errors
	user, err := updateUser(newRequest(t, u), m)
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, user)

	// create errors
	user, err = createUser(newRequest(t, u), m, "")
	require.True(t, trace.IsAlreadyExists(err))
	require.Nil(t, user)

	users, err := getUsers(context.Background(), m)
	require.True(t, trace.IsAccessDenied(err))
	require.Nil(t, users)

	// delete errors
	param := httprouter.Params{httprouter.Param{Key: "username", Value: "testname"}}
	req, err := http.NewRequest("", "/:username", nil)
	require.NoError(t, err)

	err = deleteUser(req, param, m, "self")
	require.True(t, trace.IsNotFound(err))

	// deleting self error
	param = httprouter.Params{httprouter.Param{Key: "username", Value: "self"}}
	req, err = http.NewRequest("", "/:username", nil)
	require.NoError(t, err)

	err = deleteUser(req, param, m, "self")
	require.True(t, trace.IsBadParameter(err))
}

// newRequest creates http request with given body
func newRequest(t *testing.T, body any) *http.Request {
	reqBody, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest("", "", bytes.NewBuffer(reqBody))
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/json")

	return req
}

type mockedUserAPIGetter struct {
	mockGetUser    func(ctx context.Context, name string, withSecrets bool) (types.User, error)
	mockCreateUser func(ctx context.Context, user types.User) (types.User, error)
	mockUpdateUser func(ctx context.Context, user types.User) (types.User, error)
	mockGetUsers   func(ctx context.Context, withSecrets bool) ([]types.User, error)
	mockDeleteUser func(ctx context.Context, user string) error
}

func (m *mockedUserAPIGetter) GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error) {
	if m.mockGetUser != nil {
		return m.mockGetUser(ctx, name, withSecrets)
	}
	return nil, trace.NotImplemented("mockGetUser not implemented")
}

func (m *mockedUserAPIGetter) CreateUser(ctx context.Context, user types.User) (types.User, error) {
	if m.mockCreateUser != nil {
		return m.mockCreateUser(ctx, user)
	}
	return nil, trace.NotImplemented("mockCreateUser not implemented")
}

func (m *mockedUserAPIGetter) UpdateUser(ctx context.Context, user types.User) (types.User, error) {
	if m.mockUpdateUser != nil {
		return m.mockUpdateUser(ctx, user)
	}
	return nil, trace.NotImplemented("mockUpdateUser not implemented")
}

func (m *mockedUserAPIGetter) GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error) {
	if m.mockGetUsers != nil {
		return m.mockGetUsers(ctx, withSecrets)
	}
	return nil, trace.NotImplemented("mockGetUsers not implemented")
}

func (m *mockedUserAPIGetter) DeleteUser(ctx context.Context, name string) error {
	if m.mockDeleteUser != nil {
		return m.mockDeleteUser(ctx, name)
	}

	return trace.NotImplemented("mockDeleteUser not implemented")
}
