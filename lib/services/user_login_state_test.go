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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/utils"
)

// TestUserLoginStateUnmarshal verifies a user login state resource can be unmarshaled.
func TestUserLoginState(t *testing.T) {
	expected, err := userloginstate.New(
		header.Metadata{
			Name: "test-user",
		},
		userloginstate.Spec{
			Roles: []string{
				"role1",
				"role2",
				"role3",
			},
			Traits: map[string][]string{
				"trait1": {"value1", "value2"},
				"trait2": {"value3", "value4"},
			},
		},
	)
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(userLoginStateYAML))
	require.NoError(t, err)
	actual, err := UnmarshalUserLoginState(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestUserLoginStateMarshal verifies a marshaled user login state resource can be unmarshaled back.
func TestUserLoginStateMarshal(t *testing.T) {
	expected, err := userloginstate.New(
		header.Metadata{
			Name: "test-user",
		},
		userloginstate.Spec{
			Roles: []string{
				"role1",
				"role2",
				"role3",
			},
			Traits: map[string][]string{
				"trait1": {"value1", "value2"},
				"trait2": {"value3", "value4"},
			},
		},
	)
	require.NoError(t, err)
	data, err := MarshalUserLoginState(expected)
	require.NoError(t, err)
	actual, err := UnmarshalUserLoginState(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

var userLoginStateYAML = `---
kind: user_login_state
version: v1
metadata:
  name: test-user
spec:
  roles:
  - role1
  - role2
  - role3
  traits:
    trait1:
    - value1
    - value2
    trait2:
    - value3
    - value4
`
