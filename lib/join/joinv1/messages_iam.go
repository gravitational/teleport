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

func iamInitToMessage(req *joinv1.IAMInit) (*messages.IAMInit, error) {
	clientParams, err := clientParamsToMessage(req.ClientParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &messages.IAMInit{
		ClientParams: clientParams,
	}, nil
}

func iamInitFromMessage(msg *messages.IAMInit) (*joinv1.IAMInit, error) {
	clientParams, err := clientParamsFromMessage(msg.ClientParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &joinv1.IAMInit{
		ClientParams: clientParams,
	}, nil
}

func iamChallengeToMessage(req *joinv1.IAMChallenge) *messages.IAMChallenge {
	return &messages.IAMChallenge{
		Challenge: req.Challenge,
	}
}

func iamChallengeFromMessage(msg *messages.IAMChallenge) *joinv1.IAMChallenge {
	return &joinv1.IAMChallenge{
		Challenge: msg.Challenge,
	}
}

func iamChallengeSolutionToMessage(req *joinv1.IAMChallengeSolution) *messages.IAMChallengeSolution {
	return &messages.IAMChallengeSolution{
		STSIdentityRequest: req.StsIdentityRequest,
	}
}

func iamChallengeSolutionFromMessage(msg *messages.IAMChallengeSolution) *joinv1.IAMChallengeSolution {
	return &joinv1.IAMChallengeSolution{
		StsIdentityRequest: msg.STSIdentityRequest,
	}
}
