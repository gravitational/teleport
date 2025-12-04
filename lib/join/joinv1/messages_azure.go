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
	"github.com/gravitational/trace"

	joinv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/join/v1"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

func azureInitToMessage(req *joinv1.AzureInit) (*messages.AzureInit, error) {
	clientParams, err := clientParamsToMessage(req.GetClientParams())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &messages.AzureInit{
		ClientParams: clientParams,
	}, nil
}

func azureInitFromMessage(msg *messages.AzureInit) (*joinv1.AzureInit, error) {
	clientParams, err := clientParamsFromMessage(msg.ClientParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &joinv1.AzureInit{
		ClientParams: clientParams,
	}, nil
}

func azureChallengeToMessage(req *joinv1.AzureChallenge) *messages.AzureChallenge {
	return &messages.AzureChallenge{
		Challenge: req.GetChallenge(),
	}
}

func azureChallengeFromMessage(msg *messages.AzureChallenge) *joinv1.AzureChallenge {
	return &joinv1.AzureChallenge{
		Challenge: msg.Challenge,
	}
}

func azureChallengeSolutionToMessage(req *joinv1.AzureChallengeSolution) *messages.AzureChallengeSolution {
	return &messages.AzureChallengeSolution{
		AttestedData: req.GetAttestedData(),
		Intermediate: req.GetIntermediate(),
		AccessToken:  req.GetAccessToken(),
	}
}

func azureChallengeSolutionFromMessage(msg *messages.AzureChallengeSolution) *joinv1.AzureChallengeSolution {
	return &joinv1.AzureChallengeSolution{
		AttestedData: msg.AttestedData,
		Intermediate: msg.Intermediate,
		AccessToken:  msg.AccessToken,
	}
}
