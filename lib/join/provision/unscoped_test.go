// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package provision_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/provision"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/joining"
)

func newProvisionTokenV2(t *testing.T, spec types.ProvisionTokenSpecV2) *types.ProvisionTokenV2 {
	t.Helper()
	tok, err := types.NewProvisionTokenFromSpec("test-token", time.Time{}, spec)
	require.NoError(t, err)
	ptv2, ok := tok.(*types.ProvisionTokenV2)
	require.True(t, ok)
	return ptv2
}

func TestUnscopedTokenGetBot(t *testing.T) {
	botToken := newProvisionTokenV2(t, types.ProvisionTokenSpecV2{
		Roles:      types.SystemRoles{types.RoleBot},
		JoinMethod: types.JoinMethodIAM,
		BotName:    "test-bot",
		Allow:      []*types.TokenRule{{AWSAccount: "1234"}},
	})
	require.Equal(t,
		scopes.QualifiedName{Name: "test-bot"},
		provision.UnscopedToken{ProvisionToken: botToken}.GetBot(),
	)

	nonBotToken := newProvisionTokenV2(t, types.ProvisionTokenSpecV2{
		Roles:      types.SystemRoles{types.RoleNode},
		JoinMethod: types.JoinMethodToken,
	})
	require.Zero(t, provision.UnscopedToken{ProvisionToken: nonBotToken}.GetBot())
}

func TestAsProvisionTokenV2(t *testing.T) {
	ptv2 := newProvisionTokenV2(t, types.ProvisionTokenSpecV2{
		Roles:      types.SystemRoles{types.RoleNode},
		JoinMethod: types.JoinMethodToken,
	})

	unwrapped, ok := provision.AsProvisionTokenV2(provision.UnscopedToken{ProvisionToken: ptv2})
	require.True(t, ok)
	require.Same(t, ptv2, unwrapped)

	scoped, err := joining.NewToken(joiningv1.ScopedToken_builder{
		Kind:    types.KindScopedToken,
		Scope:   "/aa",
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: "test-scoped-token",
		}.Build(),
		Spec: joiningv1.ScopedTokenSpec_builder{
			Roles:         []string{types.RoleNode.String()},
			AssignedScope: "/aa",
			JoinMethod:    string(types.JoinMethodToken),
			UsageMode:     string(joining.TokenUsageModeUnlimited),
		}.Build(),
	}.Build())
	require.NoError(t, err)

	unwrapped, ok = provision.AsProvisionTokenV2(scoped)
	require.False(t, ok)
	require.Nil(t, unwrapped)
}
