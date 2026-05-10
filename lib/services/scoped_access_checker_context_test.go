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

package services

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
)

func TestScopedAccessCheckerContextRiskyAuthorizeUnpinnedRead(t *testing.T) {
	ctx := context.Background()
	checkerContext, err := NewScopedAccessCheckerContext(ctx, &AccessInfo{
		Username: "alice",
		ScopePin: &scopesv1.Pin{
			Scope: "/test/scope",
		},
	}, "test-cluster", emptyScopedRoleReader{})
	require.NoError(t, err)

	ruleCtx := &Context{}

	// A normal decision for a root-scoped resource is denied because the identity
	// is pinned away from root before any checker, including the default implicit
	// role checker, is evaluated.
	err = checkerContext.Decision(ctx, scopes.Root, func(checker *ScopedAccessChecker) error {
		return checker.CheckAccessToRules(ruleCtx, types.KindCertAuthority, types.VerbReadNoSecrets)
	})
	require.ErrorAs(t, err, new(*trace.AccessDeniedError))

	// RiskyAuthorizeUnpinnedRead bypasses pin enforcement but still requires the
	// underlying RBAC permission. The default implicit role grants CA
	// read_no_secrets, so this succeeds.
	err = checkerContext.RiskyAuthorizeUnpinnedRead(ctx, UnpinnedReadCertAuthority, ruleCtx)
	require.NoError(t, err)

	// Using an empty UnpinnedReadAuthorization is not allowed.
	err = checkerContext.RiskyAuthorizeUnpinnedRead(ctx, UnpinnedReadAuthorization{}, ruleCtx)
	require.ErrorAs(t, err, new(*trace.BadParameterError))
}

type emptyScopedRoleReader struct{}

func (emptyScopedRoleReader) GetScopedRole(context.Context, *scopedaccessv1.GetScopedRoleRequest) (*scopedaccessv1.GetScopedRoleResponse, error) {
	return nil, trace.NotFound("scoped role not found")
}

func (emptyScopedRoleReader) ListScopedRoles(context.Context, *scopedaccessv1.ListScopedRolesRequest) (*scopedaccessv1.ListScopedRolesResponse, error) {
	return &scopedaccessv1.ListScopedRolesResponse{}, nil
}
