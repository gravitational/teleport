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

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
)

// ScopedTokenFilters
type ScopedTokenFilters struct {
	Roles         types.SystemRoles
	ResourceScope *scopesv1.Filter
	AssignedScope *scopesv1.Filter
}

// ScopedTokenService
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
