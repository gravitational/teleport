package githubactions

import (
	"context"
	"github.com/coreos/go-oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

type IDTokenValidatorConfig struct {
	// Clock is used by the validator when checking expiry and issuer times of
	// tokens. If omitted, a real clock will be used.
	Clock clockwork.Clock
	// IssuerURL is the URL of the OIDC token issuer, on which the
	// /well-known/openid-configuration endpoint can be found.
	// If this is omitted, a default value will be set.
	IssuerURL string
}

type IDTokenValidator struct {
	IDTokenValidatorConfig
}

func NewIDTokenValidator(cfg IDTokenValidatorConfig) *IDTokenValidator {
	if cfg.IssuerURL == "" {
		cfg.IssuerURL = IssuerURL
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return &IDTokenValidator{
		IDTokenValidatorConfig: cfg,
	}
}

func (id *IDTokenValidator) Validate(ctx context.Context, token string) (*IDTokenClaims, error) {
	// TODO: Extract this so we aren't producing a new provider for each attempt
	p, err := oidc.NewProvider(ctx, id.IssuerURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	verifier := p.Verifier(&oidc.Config{
		// TODO: Ensure this matches the cluster name once we start injecting
		// that into the token.
		ClientID: "teleport.cluster.local",
		Now:      id.Clock.Now,
	})

	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: NBF check or implement directly into go-oidc fork

	claims := IDTokenClaims{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, trace.Wrap(err)
	}
	return &claims, nil
}
