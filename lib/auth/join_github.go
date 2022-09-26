package auth

import (
	"context"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/githubactions"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
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

	validator := githubactions.NewIDTokenValidator(a.clock)
	claims, err := validator.Validate(ctx, req.IDToken)
	if err != nil {
		return trace.Wrap(err)
	}

	log.WithFields(logrus.Fields{
		"claims": claims,
		"token":  tokenName,
	}).Info("Github actions run trying to join cluster")

	return trace.Wrap(checkGithubAllowRules(pt, claims))
}

func checkGithubAllowRules(pt types.ProvisionToken, claims *githubactions.IDTokenClaims) error {
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
