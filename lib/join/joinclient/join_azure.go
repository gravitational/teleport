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

	"github.com/gravitational/teleport/lib/cloud/imds/azure"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

func azureJoin(ctx context.Context, stream messages.ClientStream, joinParams JoinParams, clientParams messages.ClientParams) (messages.Response, error) {
	// The Azure join method involves the following messages:
	//
	// client->server ClientInit
	// client<-server ServerInit
	// client->server AzureInit
	// client<-server AzureChallenge
	// client->server AzureChallengeSolution
	// client<-server Result
	//
	// At this point the ServerInit messages has already been received, what's
	// left is to send the AzureInit message, handle the challenge-response, and
	// receive and return the final result.
	if err := stream.Send(&messages.AzureInit{
		ClientParams: clientParams,
	}); err != nil {
		return nil, trace.Wrap(err, "sending AzureInit")
	}

	challenge, err := messages.RecvResponse[*messages.AzureChallenge](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving AzureChallenge")
	}

	imds := joinParams.AzureParams.IMDSClient
	if imds == nil {
		imds = azure.NewInstanceMetadataClient()
	}
	if !imds.IsAvailable(ctx) {
		return nil, trace.AccessDenied("could not reach instance metadata. Is Teleport running on an Azure VM?")
	}
	ad, err := imds.GetAttestedData(ctx, challenge.Challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessToken, err := imds.GetAccessToken(ctx, joinParams.AzureParams.ClientID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := stream.Send(&messages.AzureChallengeSolution{
		AttestedData: ad,
		AccessToken:  accessToken,
	}); err != nil {
		return nil, trace.Wrap(err, "sending AzureChallengeSolution")
	}

	result, err := stream.Recv()
	return result, trace.Wrap(err, "receiving join result")
}
