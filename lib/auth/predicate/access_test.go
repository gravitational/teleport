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

package predicate

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

func TestCheckAccessToNode(t *testing.T) {
	withNameAsLogin := types.NewPolicy("allow", types.PolicySpecV1{
		Allow: map[string]string{
			"node": "(node.login == user.name) || (add(user.name, \"-admin\") == node.login)",
		},
	})

	denyMike := types.NewPolicy("allow", types.PolicySpecV1{
		Deny: map[string]string{
			"node": "node.login == \"mike\"",
		},
	})

	checker := NewPredicateAccessChecker([]types.Policy{withNameAsLogin})
	access, err := checker.CheckAccessToNode(&Node{Login: "mike"}, &User{Name: "mike"})
	require.NoError(t, err)
	require.True(t, access)

	access, err = checker.CheckAccessToNode(&Node{Login: "alice"}, &User{Name: "bob"})
	require.NoError(t, err)
	require.False(t, access)

	access, err = checker.CheckAccessToNode(&Node{Login: "bob-admin"}, &User{Name: "bob"})
	require.NoError(t, err)
	require.True(t, access)

	checkerWithDeny := NewPredicateAccessChecker([]types.Policy{withNameAsLogin, denyMike})
	access, err = checkerWithDeny.CheckAccessToNode(&Node{Login: "mike"}, &User{Name: "mike"})
	require.NoError(t, err)
	require.False(t, access)

	access, err = checkerWithDeny.CheckAccessToNode(&Node{Login: "bob"}, &User{Name: "bob"})
	require.NoError(t, err)
	require.True(t, access)
}
