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

func boundKeypairInitToMessage(req *joinv1.BoundKeypairInit) (*messages.BoundKeypairInit, error) {
	clientParams, err := clientParamsToMessage(req.ClientParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &messages.BoundKeypairInit{
		ClientParams:      clientParams,
		InitialJoinSecret: req.InitialJoinSecret,
		PreviousJoinState: req.PreviousJoinState,
	}, nil
}

func boundKeypairInitFromMessage(msg *messages.BoundKeypairInit) (*joinv1.BoundKeypairInit, error) {
	clientParams, err := clientParamsFromMessage(msg.ClientParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &joinv1.BoundKeypairInit{
		ClientParams:      clientParams,
		InitialJoinSecret: msg.InitialJoinSecret,
		PreviousJoinState: msg.PreviousJoinState,
	}, nil
}

func boundKeypairChallengeToMessage(resp *joinv1.BoundKeypairChallenge) *messages.BoundKeypairChallenge {
	return &messages.BoundKeypairChallenge{
		PublicKey: resp.PublicKey,
		Challenge: resp.Challenge,
	}
}

func boundKeypairChallengeFromMessage(msg *messages.BoundKeypairChallenge) *joinv1.BoundKeypairChallenge {
	return &joinv1.BoundKeypairChallenge{
		PublicKey: msg.PublicKey,
		Challenge: msg.Challenge,
	}
}

func boundKeypairChallengeSolutionToMessage(req *joinv1.BoundKeypairChallengeSolution) *messages.BoundKeypairChallengeSolution {
	return &messages.BoundKeypairChallengeSolution{
		Solution: req.Solution,
	}
}

func boundKeypairRotationRequestToMessage(resp *joinv1.BoundKeypairRotationRequest) *messages.BoundKeypairRotationRequest {
	return &messages.BoundKeypairRotationRequest{
		SignatureAlgorithmSuite: resp.SignatureAlgorithmSuite,
	}
}

func boundKeypairRotationRequestFromMessage(resp *messages.BoundKeypairRotationRequest) *joinv1.BoundKeypairRotationRequest {
	return &joinv1.BoundKeypairRotationRequest{
		SignatureAlgorithmSuite: resp.SignatureAlgorithmSuite,
	}
}

func boundKeypairChallengeSolutionFromMessage(msg *messages.BoundKeypairChallengeSolution) *joinv1.BoundKeypairChallengeSolution {
	return &joinv1.BoundKeypairChallengeSolution{
		Solution: msg.Solution,
	}
}

func boundKeypairRotationResponseToMessage(req *joinv1.BoundKeypairRotationResponse) *messages.BoundKeypairRotationResponse {
	return &messages.BoundKeypairRotationResponse{
		PublicKey: req.PublicKey,
	}
}

func boundKeypairRotationResponseFromMessage(msg *messages.BoundKeypairRotationResponse) *joinv1.BoundKeypairRotationResponse {
	return &joinv1.BoundKeypairRotationResponse{
		PublicKey: msg.PublicKey,
	}
}

func boundKeypairResultToMessage(req *joinv1.BoundKeypairResult) *messages.BoundKeypairResult {
	if req == nil {
		return nil
	}
	return &messages.BoundKeypairResult{
		JoinState: req.JoinState,
		PublicKey: req.PublicKey,
	}
}

func boundKeypairResultFromMessage(msg *messages.BoundKeypairResult) *joinv1.BoundKeypairResult {
	if msg == nil {
		return nil
	}
	return &joinv1.BoundKeypairResult{
		JoinState: msg.JoinState,
		PublicKey: msg.PublicKey,
	}
}
