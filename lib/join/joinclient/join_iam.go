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

package joinclient

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/join/iam"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

func iamJoin(
	ctx context.Context,
	stream messages.ClientStream,
	joinParams JoinParams,
	clientParams messages.ClientParams,
) (messages.Response, error) {
	// The IAM join method involves the following messages:
	//
	// client->server ClientInit
	// client<-server ServerInit
	// client->server IAMInit
	// client<-server IAMChallenge
	// client->server IAMChallengeSolution
	// client<-server Result
	//
	// At this point the ServerInit messages has already been received, what's
	// left is to send the IAMInit message, handle the challenge-response, and
	// receive and return the final result.
	if err := stream.Send(&messages.IAMInit{
		ClientParams: clientParams,
	}); err != nil {
		return nil, trace.Wrap(err, "sending IAMInit")
	}

	challenge, err := messages.RecvResponse[*messages.IAMChallenge](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving IAMChallenge")
	}

	signedRequest, err := joinParams.CreateSignedSTSIdentityRequestFunc(ctx, challenge.Challenge,
		iam.WithFIPSEndpoint(joinParams.FIPS),
	)
	if err != nil {
		err = trace.Wrap(err, "creating signed sts:GetCallerIdentity request")
		sendGivingUpErr := stream.Send(&messages.GivingUp{
			Reason: messages.GivingUpReasonChallengeSolutionFailed,
			Msg:    err.Error(),
		})
		return nil, trace.NewAggregate(
			err,
			trace.Wrap(sendGivingUpErr, "sending GivingUp message to server"),
		)

	}

	if err := stream.Send(&messages.IAMChallengeSolution{
		STSIdentityRequest: signedRequest,
	}); err != nil {
		return nil, trace.Wrap(err, "sending IAMChallengeSolution")
	}

	result, err := stream.Recv()
	return result, trace.Wrap(err, "receiving join result")
}
