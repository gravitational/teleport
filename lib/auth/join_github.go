package auth

import (
	"context"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

func (a *Server) checkGitHubJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) error {
	tokenName := req.Token
	_, err := a.GetToken(ctx, tokenName)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
