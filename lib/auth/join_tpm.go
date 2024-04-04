package auth

import (
	"context"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
)

func (a *Server) RegisterUsingTPMMethod(ctx context.Context, initReq *proto.RegisterUsingTPMMethodInitialRequest, solveChallenge client.RegisterTPMChallengeResponseFunc) (*proto.Certs, error) {

}
