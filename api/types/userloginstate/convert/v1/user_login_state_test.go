/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/types/userloginstate"
)

func TestRoundtrip(t *testing.T) {
	uls := newUserLoginState(t, "user-login-state")

	converted, err := FromProto(ToProto(uls))
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(uls, converted))
}

// Make sure that we don't panic if any of the message fields are missing.
func TestFromProtoNils(t *testing.T) {
	// Spec is nil
	uls := ToProto(newUserLoginState(t, "user-login-state"))
	uls.Spec = nil

	_, err := FromProto(uls)
	require.Error(t, err)

	// Roles is nil
	uls = ToProto(newUserLoginState(t, "user-login-state"))
	uls.Spec.Roles = nil

	_, err = FromProto(uls)
	require.NoError(t, err)

	// Traits is nil
	uls = ToProto(newUserLoginState(t, "user-login-state"))
	uls.Spec.Traits = nil

	_, err = FromProto(uls)
	require.NoError(t, err)

	// UserType is empty
	uls = ToProto(newUserLoginState(t, "user-login-state"))
	uls.Spec.UserType = ""

	fromProto, err := FromProto(uls)
	require.NoError(t, err)
	require.Equal(t, types.UserTypeLocal, fromProto.GetUserType())

	// GitHub identity is nil
	uls = ToProto(newUserLoginState(t, "user-login-state"))
	uls.Spec.GitHubIdentity = nil

	_, err = FromProto(uls)
	require.NoError(t, err)
}

func newUserLoginState(t *testing.T, name string) *userloginstate.UserLoginState {
	t.Helper()

	uls, err := userloginstate.New(
		header.Metadata{
			Name: name,
		},
		userloginstate.Spec{
			OriginalRoles: []string{"role1"},
			OriginalTraits: trait.Traits{
				"key1": []string{"value1"},
			},
			Roles: []string{"role1", "role2"},
			Traits: trait.Traits{
				"key1": []string{"value1"},
				"key2": []string{"value2"},
			},
			UserType: types.UserTypeSSO,
			GitHubIdentity: &userloginstate.ExternalIdentity{
				Username: "my-github-username",
				UserID:   "1234567",
			},
		},
	)
	require.NoError(t, err)
	return uls
}
