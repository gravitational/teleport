// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package joinv1

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	joinv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/join/v1"
)

// TestRequestToMessage tests that parsing a gRPC [joinv1.JoinRequest] into a
// [messages.Request] does not trigger a panic on the server. These gRPC
// requests come in over the network from untrusted clients.
func TestRequestToMessage(t *testing.T) {
	for _, tc := range []struct {
		desc string
		req  *joinv1.JoinRequest
	}{
		{
			desc: "nil req",
		},
		{
			desc: "nil payload",
			req:  &joinv1.JoinRequest{},
		},
		{
			desc: "empty ClientInit",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_ClientInit{},
			},
		},
		{
			desc: "empty TokenInit",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_TokenInit{},
			},
		},
		{
			desc: "empty BoundKeypairInit",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_BoundKeypairInit{},
			},
		},
		{
			desc: "empty IAMInit",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_IamInit{},
			},
		},
		{
			desc: "empty EC2Init",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_Ec2Init{},
			},
		},
		{
			desc: "empty OIDCInit",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_OidcInit{},
			},
		},
		{
			desc: "empty OracleInit",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_OracleInit{},
			},
		},
		{
			desc: "empty TpmInit",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_TpmInit{},
			},
		},
		{
			desc: "empty AzureInit",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_AzureInit{},
			},
		},
		{
			desc: "empty HostParams",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_TokenInit{
					TokenInit: &joinv1.TokenInit{
						ClientParams: &joinv1.ClientParams{
							Payload: &joinv1.ClientParams_HostParams{},
						},
					},
				},
			},
		},
		{
			desc: "empty BotParams",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_TokenInit{
					TokenInit: &joinv1.TokenInit{
						ClientParams: &joinv1.ClientParams{
							Payload: &joinv1.ClientParams_BotParams{},
						},
					},
				},
			},
		},
		{
			desc: "empty Solution",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_Solution{},
			},
		},
		{
			desc: "empty BoundKeypairChallengeSolution",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_Solution{
					Solution: &joinv1.ChallengeSolution{
						Payload: &joinv1.ChallengeSolution_BoundKeypairChallengeSolution{},
					},
				},
			},
		},
		{
			desc: "empty BoundKeypairRotationResponse",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_Solution{
					Solution: &joinv1.ChallengeSolution{
						Payload: &joinv1.ChallengeSolution_BoundKeypairRotationResponse{},
					},
				},
			},
		},
		{
			desc: "empty IamChallengeSolution",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_Solution{
					Solution: &joinv1.ChallengeSolution{
						Payload: &joinv1.ChallengeSolution_IamChallengeSolution{},
					},
				},
			},
		},
		{
			desc: "empty OracleChallengeSolution",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_Solution{
					Solution: &joinv1.ChallengeSolution{
						Payload: &joinv1.ChallengeSolution_OracleChallengeSolution{},
					},
				},
			},
		},
		{
			desc: "empty TpmSolution",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_Solution{
					Solution: &joinv1.ChallengeSolution{
						Payload: &joinv1.ChallengeSolution_TpmSolution{},
					},
				},
			},
		},
		{
			desc: "empty AzureSolution",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_Solution{
					Solution: &joinv1.ChallengeSolution{
						Payload: &joinv1.ChallengeSolution_AzureChallengeSolution{},
					},
				},
			},
		},
		{
			desc: "empty GivingUp",
			req: &joinv1.JoinRequest{
				Payload: &joinv1.JoinRequest_GivingUp{},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// Put the request through a marshal/unmarshal to more closely
			// model a request that could actually come in over the network.
			// For example, in unmarshalled messages, the inner pointer of a
			// oneof can never be nil.
			buf, err := proto.Marshal(tc.req)
			require.NoError(t, err)
			var req joinv1.JoinRequest
			err = proto.Unmarshal(buf, &req)
			require.NoError(t, err)

			require.NotPanics(t, func() {
				_, _ = requestToMessage(&req)
			})
		})
	}
}
