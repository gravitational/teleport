/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/trace"
)

func (a *Server) RegisterUsingKubernetesRemoteMethod(ctx context.Context, req *types.RegisterUsingTokenRequest, solver client.KubernetesRemoteChallengeSolver) (*proto.Certs, error) {
	clientAddr, err := authz.ClientAddrFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req.RemoteAddr = clientAddr.String()
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	provisionToken, err := a.checkTokenJoinRequestCommon(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	solutionJWT, err := solver("challenge")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if provisionToken.GetVersion() != types.V2 {
		panic("version mitcmathc")
	}
	if provisionToken.GetJoinMethod() != types.JoinMethodKubernetesRemote {
		panic("mismtatch join methiod")
	}

	// TODO: Evaluate solutionJWT

	if req.Role == types.RoleBot {
		certs, err := a.generateCertsBot(
			ctx,
			provisionToken,
			req,
			nil,
		)
		return certs, trace.Wrap(err)
	}
	certs, err := a.generateCerts(
		ctx,
		provisionToken,
		req,
		nil,
	)
	return certs, trace.Wrap(err)
}
