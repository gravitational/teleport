/**
 * Copyright 2021 Gravitational, Inc.
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

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
)

func TestRequestParameters(t *testing.T) {
	r := saveUserRequest{
		Name:   "",
		Roles:  nil,
		Traits: userTraits{},
	}
	require.True(t, trace.IsBadParameter(r.checkAndSetDefaults()))

	r = saveUserRequest{
		Name:   "",
		Roles:  []string{"testrole"},
		Traits: userTraits{},
	}
	require.True(t, trace.IsBadParameter(r.checkAndSetDefaults()))

	r = saveUserRequest{
		Name:   "username",
		Roles:  nil,
		Traits: userTraits{},
	}
	require.True(t, trace.IsBadParameter(r.checkAndSetDefaults()))

	r = saveUserRequest{
		Name:   "username",
		Roles:  []string{"testrole"},
		Traits: userTraits{},
	}
	require.Nil(t, r.checkAndSetDefaults())
}

func TestCRUDs(t *testing.T) {
	u := saveUserRequest{
		Name:   "testname",
		Roles:  []string{"testrole"},
		Traits: userTraits{},
	}

	m := &mockedUserAPIGetter{}
	m.mockCreateUser = func(ctx context.Context, user types.User) error {
		return nil
	}

	m.mockGetUser = func(name string, withSecrets bool) (types.User, error) {
		return types.NewUser(name)
	}

	m.mockUpdateUser = func(ctx context.Context, user types.User) error {
		return nil
	}

	m.mockGetUsers = func(withSecrets bool) ([]types.User, error) {
		u, err := types.NewUser("testname")
		return []types.User{u}, err
	}

	m.mockDeleteUser = func(ctx context.Context, user string) error {
		return nil
	}

	// test create
	user, err := createUser(newRequest(t, u), m, "")
	require.Nil(t, err)
	require.Equal(t, "testname", user.Name)
	require.Equal(t, "local", user.AuthType)
	require.Contains(t, user.Roles, "testrole")

	// test update
	u.Roles = []string{"newrole"}
	user, err = updateUser(newRequest(t, u), m, "")
	require.Nil(t, err)
	require.Contains(t, user.Roles, "newrole")

	// test list
	users, err := getUsers(m)
	require.Nil(t, err)
	require.Len(t, users, 1)
	require.Equal(t, "testname", users[0].Name)

	// test delete
	param := httprouter.Params{httprouter.Param{Key: "username", Value: "testname"}}
	req, err := http.NewRequest("", "/:username", nil)
	require.Nil(t, err)

	err = deleteUser(req, param, m, "self")
	require.Nil(t, err)
}

func TestUpdateUser_setTraits(t *testing.T) {
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
				Name:   "setlogins",
				Roles:  defaultRoles,
				Traits: userTraits{Logins: &[]string{"login1", "login2"}},
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
				Traits: userTraits{
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
				Traits: userTraits{
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
				Traits: userTraits{
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
				Traits: userTraits{
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
				Name:   "deduplicates",
				Roles:  defaultRoles,
				Traits: userTraits{Logins: &[]string{"login1", "login2", "login1"}},
			},
			expectedTraits: map[string][]string{
				constants.TraitLogins: {"login1", "login2"},
			},
		},
		{
			name: "RemovesAll",
			updateReq: saveUserRequest{
				Name:   "removesall",
				Roles:  defaultRoles,
				Traits: userTraits{Logins: &[]string{}},
			},
			expectedTraits: map[string][]string{
				constants.TraitLogins: {},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			user, err := types.NewUser(tt.name)
			require.NoError(t, err)
			user.SetRoles(defaultRoles)
			user.SetLogins(defaultLogins)

			m := &mockedUserAPIGetter{}
			m.mockGetUser = func(name string, withSecrets bool) (types.User, error) {
				return user, nil
			}
			m.mockUpdateUser = func(ctx context.Context, user types.User) error {
				return nil
			}

			_, err = updateUser(newRequest(t, tt.updateReq), m, "")
			require.NoError(t, err)

			// The traits match
			require.Equal(t, tt.expectedTraits, user.GetTraits())

			// Other fields dont't change
			require.ElementsMatch(t, user.GetRoles(), defaultRoles)

			// We can read back the user traits
			uiUser, err := getUser(tt.name, m)
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

func TestCRUDErrors(t *testing.T) {
	m := &mockedUserAPIGetter{}
	m.mockCreateUser = func(ctx context.Context, user types.User) error {
		return trace.AlreadyExists("")
	}

	m.mockGetUser = func(name string, withSecrets bool) (types.User, error) {
		return nil, trace.NotFound("")
	}

	m.mockUpdateUser = func(ctx context.Context, user types.User) error {
		return trace.NotFound("")
	}

	m.mockGetUsers = func(withSecrets bool) ([]types.User, error) {
		return nil, trace.AccessDenied("")
	}

	m.mockDeleteUser = func(ctx context.Context, user string) error {
		return trace.NotFound("")
	}

	u := saveUserRequest{
		Name:   "testname",
		Roles:  []string{"testrole"},
		Traits: userTraits{Logins: nil},
	}

	// update errors
	user, err := updateUser(newRequest(t, u), m, "")
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, user)

	// create errors
	user, err = createUser(newRequest(t, u), m, "")
	require.True(t, trace.IsAlreadyExists(err))
	require.Nil(t, user)

	users, err := getUsers(m)
	require.True(t, trace.IsAccessDenied(err))
	require.Nil(t, users)

	// delete errors
	param := httprouter.Params{httprouter.Param{Key: "username", Value: "testname"}}
	req, err := http.NewRequest("", "/:username", nil)
	require.Nil(t, err)

	err = deleteUser(req, param, m, "self")
	require.True(t, trace.IsNotFound(err))

	// deleting self error
	param = httprouter.Params{httprouter.Param{Key: "username", Value: "self"}}
	req, err = http.NewRequest("", "/:username", nil)
	require.Nil(t, err)

	err = deleteUser(req, param, m, "self")
	require.True(t, trace.IsBadParameter(err))
}

// newRequest creates http request with given body
func newRequest(t *testing.T, body interface{}) *http.Request {
	reqBody, err := json.Marshal(body)
	require.Nil(t, err)

	req, err := http.NewRequest("", "", bytes.NewBuffer(reqBody))
	require.Nil(t, err)
	req.Header.Add("Content-Type", "application/json")

	return req
}

type mockedUserAPIGetter struct {
	mockGetUser    func(name string, withSecrets bool) (types.User, error)
	mockCreateUser func(ctx context.Context, user types.User) error
	mockUpdateUser func(ctx context.Context, user types.User) error
	mockGetUsers   func(withSecrets bool) ([]types.User, error)
	mockDeleteUser func(ctx context.Context, user string) error
}

func (m *mockedUserAPIGetter) GetUser(name string, withSecrets bool) (types.User, error) {
	if m.mockGetUser != nil {
		return m.mockGetUser(name, withSecrets)
	}
	return nil, trace.NotImplemented("mockGetUser not implemented")
}

func (m *mockedUserAPIGetter) CreateUser(ctx context.Context, user types.User) error {
	if m.mockCreateUser != nil {
		return m.mockCreateUser(ctx, user)
	}
	return trace.NotImplemented("mockCreateUser not implemented")
}

func (m *mockedUserAPIGetter) UpdateUser(ctx context.Context, user types.User) error {
	if m.mockUpdateUser != nil {
		return m.mockUpdateUser(ctx, user)
	}
	return trace.NotImplemented("mockUpdateUser not implemented")
}

func (m *mockedUserAPIGetter) GetUsers(withSecrets bool) ([]types.User, error) {
	if m.mockGetUsers != nil {
		return m.mockGetUsers(withSecrets)
	}
	return nil, trace.NotImplemented("mockGetUsers not implemented")
}

func (m *mockedUserAPIGetter) DeleteUser(ctx context.Context, name string) error {
	if m.mockDeleteUser != nil {
		return m.mockDeleteUser(ctx, name)
	}

	return trace.NotImplemented("mockDeleteUser not implemented")
}
