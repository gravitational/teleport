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
)

// ScopedTokenService handles CRUD operations for the ScopedToken resource.
type ScopedTokenService interface {
	// CreateScopedToken creates a scoped join token.
	CreateScopedToken(ctx context.Context, req *joiningv1.CreateScopedTokenRequest) (*joiningv1.CreateScopedTokenResponse, error)

	// GetScopedToken fetches a scoped join token by unique name
	GetScopedToken(ctx context.Context, req *joiningv1.GetScopedTokenRequest) (*joiningv1.GetScopedTokenResponse, error)

	// UseScopedToken attempts to use a scoped token to provision a resource. The given public
	// key should be used as an idempotency key in cases where the usage limits apply and retries
	// should not the token.
	UseScopedToken(ctx context.Context, token *joiningv1.ScopedToken, publicKey []byte) (*joiningv1.ScopedToken, error)

	// ListScopedTokens retrieves a paginated list of scoped join tokens
	ListScopedTokens(ctx context.Context, req *joiningv1.ListScopedTokensRequest) (*joiningv1.ListScopedTokensResponse, error)

	// DeleteScopedToken deletes a named scoped join token. Imlementations must guarantee that
	// this returns trace.NotFound error if the token doesn't exist
	DeleteScopedToken(ctx context.Context, req *joiningv1.DeleteScopedTokenRequest) (*joiningv1.DeleteScopedTokenResponse, error)

	// UpsertScopedToken updates or creates a scoped join token. If updating an existing token, the scope and status must not be modified.
	UpsertScopedToken(ctx context.Context, req *joiningv1.UpsertScopedTokenRequest) (*joiningv1.UpsertScopedTokenResponse, error)
}
