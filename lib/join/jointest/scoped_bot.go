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

package jointest

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

// CreateScopedBot creates a scoped Bot in the test scope ("/test") with a
// scoped role and role assignment, then waits for cache propagation. Returns
// the qualified bot name (e.g., "/test::botName") for use in scoped token
// spec.bot.
func CreateScopedBot(t testing.TB, authServer *auth.Server, botName string) string {
	t.Helper()

	ctx := t.Context()
	scope := testTokenScope // "/test"
	roleName := "jointest-role-" + botName

	// Create a scoped role for the bot.
	_, err := authServer.ScopedAccess().CreateScopedRole(ctx, scopedaccessv1.CreateScopedRoleRequest_builder{
		Role: scopedaccessv1.ScopedRole_builder{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: roleName,
			}.Build(),
			Scope: scope,
			Spec: scopedaccessv1.ScopedRoleSpec_builder{
				AssignableScopes: []string{scope},
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)

	// Create the scoped bot.
	_, err = machineidv1.UpsertBot(ctx, authServer, machineidv1pb.Bot_builder{
		Kind:    types.KindBot,
		Version: types.V1,
		Scope:   scope,
		Metadata: headerv1.Metadata_builder{
			Name: botName,
		}.Build(),
		Spec: &machineidv1pb.BotSpec{},
	}.Build(), authServer.GetClock().Now(), "", scopes.Features{Enabled: true})
	require.NoError(t, err)

	// Create a scoped role assignment for the bot.
	qualifiedBotName := scopes.QualifiedName{Scope: scope, Name: botName}.String()
	resp, err := authServer.ScopedAccess().CreateScopedRoleAssignment(ctx, scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
		Assignment: scopedaccessv1.ScopedRoleAssignment_builder{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: uuid.NewString(),
			}.Build(),
			SubKind: scopedaccess.SubKindDynamic,
			Scope:   scope,
			Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
				Bot: qualifiedBotName,
				Assignments: []*scopedaccessv1.Assignment{
					scopedaccessv1.Assignment_builder{
						Role:  scope + "::" + roleName,
						Scope: scope,
					}.Build(),
				},
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)

	// Wait for the cache to propagate the assignment.
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		_, err := authServer.ScopedAccessCache.GetScopedRoleAssignment(ctx, scopedaccessv1.GetScopedRoleAssignmentRequest_builder{
			Name:    resp.GetAssignment().GetMetadata().GetName(),
			SubKind: resp.GetAssignment().GetSubKind(),
			Scope:   resp.GetAssignment().GetScope(),
		}.Build())
		assert.NoError(ct, err)
	}, time.Second*10, 100*time.Millisecond)

	return qualifiedBotName
}
