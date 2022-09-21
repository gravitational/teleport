package auth

import (
	"context"
	"github.com/coreos/go-oidc"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/githubactions"
	"github.com/gravitational/trace"
)

func (a *Server) checkGitHubJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) error {
	if req.IDToken == "" {
		return trace.BadParameter("IDToken not provided for Github join request")
	}
	tokenName := req.Token
	pt, err := a.GetToken(ctx, tokenName)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO: Extract this so we aren't producing a new provider for each thing
	p, err := oidc.NewProvider(ctx, githubactions.IssuerURL)
	if err != nil {
		return trace.Wrap(err)
	}

	verifier := p.Verifier(&oidc.Config{
		// TODO: Ensure this matches the cluster name once we start injecting
		// that into the token.
		SkipClientIDCheck: true,
		Now:               a.clock.Now,
	})

	idToken, err := verifier.Verify(ctx, req.IDToken)
	if err != nil {
		return trace.Wrap(err)
	}

	claims := githubactions.IDTokenClaims{}
	if err := idToken.Claims(&claims); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(checkGithubAllowRules(pt, claims))
}

func checkGithubAllowRules(pt types.ProvisionToken, claims githubactions.IDTokenClaims) error {
	token := pt.V3()

	// If a single rule passes, accept the IDToken
	for _, rule := range token.Spec.GitHub.Allow {
		if rule.Sub != "" && claims.Sub != rule.Sub {
			continue
		}
		if rule.RepositoryOwner != "" && claims.RepositoryOwner != rule.RepositoryOwner {
			continue
		}
		if rule.Repository != "" && claims.Repository != rule.Repository {
			continue
		}

		// All provided rules met.
		return nil
	}

	return trace.AccessDenied("provided ID token's claims did not match any configured rules")
}
