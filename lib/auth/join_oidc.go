package auth

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

type oidcTokenChecker interface {
	JoinMethod() types.JoinMethod

	// Check ensures that the provided token is valid, signed by the provider,
	// and that the rules provided in types.ProvisionToken allow the token
	// to be used.
	//
	// Check returns an error if the token is not valid
	Check(ctx context.Context, pt types.ProvisionToken, token string) error
}

var ErrTokenNotMatchAllow = trace.AccessDenied("token did not match any allow rules")

func (a *Server) checkOIDCJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest, checker oidcTokenChecker) error {
	if req.OIDCJWT == "" {
		return trace.BadParameter("Request must contain OIDCJWT value when using OIDC based joining.")
	}
	tokenResource, err := a.GetToken(ctx, req.Token)
	if err != nil {
		return trace.Wrap(err)
	}

	return checker.Check(ctx, tokenResource, req.OIDCJWT)
}
