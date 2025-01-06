package auth

import (
	"context"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/trace"
)

func (a *Server) RegisterUsingOracleMethod(ctx context.Context, challengeResponse client.RegisterOracleChallengeResponseFunc) (*proto.Certs, error) {
	return nil, trace.NotImplemented("")
}
