package identitycenter

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/credprovider"
)

// tokenGenerator defines an interface matching the private credprovider.tokenGenerator
// interface
type tokenGenerator interface {
	GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error)
}

// functionTokenGenerator is a wrapper around a credprovider.GenerateOIDCTokenFn
// function that lets functions with a matching signature implement
// `tokenGenerator`
type functionTokenGenerator credprovider.GenerateOIDCTokenFn

// GenerateAWSOIDCToken implements `tokenGenerator` for functionTokenGenerator,
// passing the call straight through to the underlying function.
func (g functionTokenGenerator) GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error) {
	return credprovider.GenerateOIDCTokenFn(g)(ctx, integration)
}

// MakeTokenGenerator lightly wraps the `awsoidc.GenerateAWSOIDCToken` to create
// an OIDC token generator
func MakeTokenGenerator(auth *auth.Server) tokenGenerator {
	fn := func(ctx context.Context, integrationName string) (string, error) {
		token, err := awsoidc.GenerateAWSOIDCToken(ctx, auth, auth.GetKeyStore(), awsoidc.GenerateAWSOIDCTokenRequest{
			Integration: integrationName,
			Username:    auth.ServerID,
			Subject:     types.IntegrationAWSOIDCSubjectAuth,
			Clock:       auth.GetClock(),
		})
		if err != nil {
			return "", trace.Wrap(err)
		}

		return token, nil
	}
	return functionTokenGenerator(fn)
}
