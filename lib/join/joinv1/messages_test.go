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
			req: joinv1.JoinRequest_builder{
				ClientInit: &joinv1.ClientInit{},
			}.Build(),
		},
		{
			desc: "empty TokenInit",
			req: joinv1.JoinRequest_builder{
				TokenInit: &joinv1.TokenInit{},
			}.Build(),
		},
		{
			desc: "empty BoundKeypairInit",
			req: joinv1.JoinRequest_builder{
				BoundKeypairInit: &joinv1.BoundKeypairInit{},
			}.Build(),
		},
		{
			desc: "empty IAMInit",
			req: joinv1.JoinRequest_builder{
				IamInit: &joinv1.IAMInit{},
			}.Build(),
		},
		{
			desc: "empty EC2Init",
			req: joinv1.JoinRequest_builder{
				Ec2Init: &joinv1.EC2Init{},
			}.Build(),
		},
		{
			desc: "empty OIDCInit",
			req: joinv1.JoinRequest_builder{
				OidcInit: &joinv1.OIDCInit{},
			}.Build(),
		},
		{
			desc: "empty OracleInit",
			req: joinv1.JoinRequest_builder{
				OracleInit: &joinv1.OracleInit{},
			}.Build(),
		},
		{
			desc: "empty TpmInit",
			req: joinv1.JoinRequest_builder{
				TpmInit: &joinv1.TPMInit{},
			}.Build(),
		},
		{
			desc: "empty AzureInit",
			req: joinv1.JoinRequest_builder{
				AzureInit: &joinv1.AzureInit{},
			}.Build(),
		},
		{
			desc: "empty HostParams",
			req: joinv1.JoinRequest_builder{
				TokenInit: joinv1.TokenInit_builder{
					ClientParams: joinv1.ClientParams_builder{
						HostParams: &joinv1.HostParams{},
					}.Build(),
				}.Build(),
			}.Build(),
		},
		{
			desc: "empty BotParams",
			req: joinv1.JoinRequest_builder{
				TokenInit: joinv1.TokenInit_builder{
					ClientParams: joinv1.ClientParams_builder{
						BotParams: &joinv1.BotParams{},
					}.Build(),
				}.Build(),
			}.Build(),
		},
		{
			desc: "empty Solution",
			req: joinv1.JoinRequest_builder{
				Solution: &joinv1.ChallengeSolution{},
			}.Build(),
		},
		{
			desc: "empty BoundKeypairChallengeSolution",
			req: joinv1.JoinRequest_builder{
				Solution: joinv1.ChallengeSolution_builder{
					BoundKeypairChallengeSolution: &joinv1.BoundKeypairChallengeSolution{},
				}.Build(),
			}.Build(),
		},
		{
			desc: "empty BoundKeypairRotationResponse",
			req: joinv1.JoinRequest_builder{
				Solution: joinv1.ChallengeSolution_builder{
					BoundKeypairRotationResponse: &joinv1.BoundKeypairRotationResponse{},
				}.Build(),
			}.Build(),
		},
		{
			desc: "empty IamChallengeSolution",
			req: joinv1.JoinRequest_builder{
				Solution: joinv1.ChallengeSolution_builder{
					IamChallengeSolution: &joinv1.IAMChallengeSolution{},
				}.Build(),
			}.Build(),
		},
		{
			desc: "empty OracleChallengeSolution",
			req: joinv1.JoinRequest_builder{
				Solution: joinv1.ChallengeSolution_builder{
					OracleChallengeSolution: &joinv1.OracleChallengeSolution{},
				}.Build(),
			}.Build(),
		},
		{
			desc: "empty TpmSolution",
			req: joinv1.JoinRequest_builder{
				Solution: joinv1.ChallengeSolution_builder{
					TpmSolution: &joinv1.TPMSolution{},
				}.Build(),
			}.Build(),
		},
		{
			desc: "empty AzureSolution",
			req: joinv1.JoinRequest_builder{
				Solution: joinv1.ChallengeSolution_builder{
					AzureChallengeSolution: &joinv1.AzureChallengeSolution{},
				}.Build(),
			}.Build(),
		},
		{
			desc: "empty GivingUp",
			req: joinv1.JoinRequest_builder{
				GivingUp: &joinv1.GivingUp{},
			}.Build(),
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
