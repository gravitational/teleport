/*
Copyright 2022 Gravitational, Inc.

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

package ui

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

func TestNewUserContext(t *testing.T) {
	t.Parallel()

	user := &types.UserV2{
		Metadata: types.Metadata{
			Name: "root",
		},
	}

	// set some rules
	role1 := &types.RoleV6{}
	role1.SetNamespaces(types.Allow, []string{apidefaults.Namespace})

	role2 := &types.RoleV6{}
	role2.SetNamespaces(types.Allow, []string{apidefaults.Namespace})

	roleSet := []types.Role{role1, role2}
	userContext, err := NewUserContext(user, roleSet, proto.Features{}, true, false)
	require.NoError(t, err)

	// test user name
	require.Equal(t, userContext.Name, "root")
	require.Empty(t, cmp.Diff(userContext.AccessStrategy, accessStrategy{
		Type:   types.RequestStrategyOptional,
		Prompt: "",
	}))

	// test local auth type
	require.Equal(t, userContext.AuthType, authLocal)

	// test sso auth type
	user.Spec.GithubIdentities = []types.ExternalIdentity{{ConnectorID: "foo", Username: "bar"}}
	userContext, err = NewUserContext(user, roleSet, proto.Features{}, true, false)
	require.NoError(t, err)
	require.Equal(t, authSSO, userContext.AuthType)

	// test sso auth type for users with the CreatedBy.Connector field set.
	// Eg users import from okta do not have any <IdP>Identities, so the CreatedBy.Connector must be checked.
	userCreatedExternally := &types.UserV2{
		Metadata: types.Metadata{
			Name: "root",
		},
		Spec: types.UserSpecV2{
			CreatedBy: types.CreatedBy{
				Connector: &types.ConnectorRef{},
			},
		},
	}
	userContext, err = NewUserContext(userCreatedExternally, roleSet, proto.Features{}, true, false)
	require.NoError(t, err)
	require.Equal(t, authSSO, userContext.AuthType)
}

func TestNewUserContextCloud(t *testing.T) {
	t.Parallel()

	user := &types.UserV2{
		Metadata: types.Metadata{
			Name: "root",
		},
	}

	role := &types.RoleV6{}
	role.SetNamespaces(types.Allow, []string{"*"})

	roleSet := []types.Role{role}

	userContext, err := NewUserContext(user, roleSet, proto.Features{Cloud: true}, true, false)
	require.NoError(t, err)

	require.Equal(t, userContext.Name, "root")
	require.Empty(t, cmp.Diff(userContext.AccessStrategy, accessStrategy{
		Type:   types.RequestStrategyOptional,
		Prompt: "",
	}))
}
