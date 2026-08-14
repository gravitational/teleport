package accessgraph

import (
	"context"

	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/msgraph/models"
)

// GraphClient is an interface for interacting with the Microsoft Graph API.
type GraphClient interface {
	IterateApplications(ctx context.Context, f func(*models.Application) bool, opts ...msgraph.IterateOpt) error
}
