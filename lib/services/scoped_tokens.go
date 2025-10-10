// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

	"github.com/gravitational/trace"

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
)

// ScopedTokenFilters allow for filtering [joiningv1.ScopedToken] values when calling
// the ListScopedTokens method.
type ScopedTokenFilters struct {
	Roles         types.SystemRoles
	ResourceScope *scopesv1.Filter
	AssignedScope *scopesv1.Filter
	Labels        map[string]string
}

// ScopedTokenService handles CRUD operations for the ScopedToken resource.
type ScopedTokenService interface {
	// CreateScopedToken creates a scoped join token.
	CreateScopedToken(ctx context.Context, token *joiningv1.ScopedToken) (*joiningv1.ScopedToken, error)

	// UpdateScopedToken updates a scoped join token.
	UpdateScopedToken(ctx context.Context, token *joiningv1.ScopedToken) (*joiningv1.ScopedToken, error)

	// UpsertScopedToken upserts a scoped join token
	UpsertScopedToken(ctx context.Context, token *joiningv1.ScopedToken) (*joiningv1.ScopedToken, error)

	// GetScopedToken fetches a scoped join token by unique name
	GetScopedToken(ctx context.Context, name string) (*joiningv1.ScopedToken, error)

	// ListScopedTokens retrieves a paginated list of scoped join tokens
	ListScopedTokens(ctx context.Context, pageSize int, pageToken string, filters *ScopedTokenFilters) ([]*joiningv1.ScopedToken, string, error)

	// DeleteScopedToken deletes a named scoped join token. Imlementations must guarantee that
	// this returns trace.NotFound error if the token doesn't exist
	DeleteScopedToken(ctx context.Context, name string) error
}

// ValidateScopedToken checks that the given [joiningv1.ScopedToken] meets
// the constraints required to be written to the backend.
func ValidateScopedToken(token *joiningv1.ScopedToken) error {
	if expected, actual := types.KindScopedToken, token.GetKind(); expected != actual {
		return trace.BadParameter("expected kind %v, got %q", expected, actual)
	}
	if expected, actual := types.V1, token.GetVersion(); expected != actual {
		return trace.BadParameter("expected version %v, got %q", expected, actual)
	}
	if expected, actual := "", token.GetSubKind(); expected != actual {
		return trace.BadParameter("expected sub_kind %v, got %q", expected, actual)
	}
	if name := token.GetMetadata().GetName(); name == "" {
		return trace.BadParameter("missing name")
	}

	spec := token.GetSpec()
	if spec == nil {
		return trace.BadParameter("spec must not be nil")
	}

	if token.GetScope() == "" {
		return trace.BadParameter("scoped token must have a scope assigned")
	}

	if err := scopes.StrongValidate(token.GetScope()); err != nil {
		return trace.Wrap(err, "validating scoped token resource scope")
	}

	if spec.AssignedScope != "" {
		if err := scopes.StrongValidate(spec.AssignedScope); err != nil {
			return trace.Wrap(err, "validating scoped token assigned scope")
		}
		if !scopes.ResourceScope(spec.AssignedScope).IsSubjectToPolicyScope(token.GetScope()) {
			return trace.BadParameter("scoped token assigned scope must be descendant of its resource scope")
		}
	}

	if len(spec.Roles) == 0 {
		return trace.BadParameter("scoped token must have at least one role")
	}

	roles, err := types.NewTeleportRoles(spec.Roles)
	if err != nil {
		return trace.Wrap(err, "validating scoped token roles")
	}

	hasBotRole := roles.Include(types.RoleBot)
	if hasBotRole && spec.BotName == "" {
		return trace.BadParameter("scoped token with role %q must set bot_name", types.RoleBot)
	}

	if spec.BotName != "" && !hasBotRole {
		return trace.BadParameter("can only set bot_name on scoped token with role %q", types.RoleBot)
	}

	return nil
}
