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
