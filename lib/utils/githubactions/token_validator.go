package githubactions

import (
	"context"
	"github.com/coreos/go-oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

type IDTokenValidator struct {
	clock clockwork.Clock
}

func NewIDTokenValidator(clock clockwork.Clock) *IDTokenValidator {
	return &IDTokenValidator{
		clock: clock,
	}
}

func (id *IDTokenValidator) Validate(ctx context.Context, token string) (*IDTokenClaims, error) {
	// TODO: Extract this so we aren't producing a new provider for each attempt
	p, err := oidc.NewProvider(ctx, IssuerURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	verifier := p.Verifier(&oidc.Config{
		// TODO: Ensure this matches the cluster name once we start injecting
		// that into the token.
		ClientID: "teleport.cluster.local",
		Now:      id.clock.Now,
	})

	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	claims := IDTokenClaims{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, trace.Wrap(err)
	}
	return &claims, nil
}
