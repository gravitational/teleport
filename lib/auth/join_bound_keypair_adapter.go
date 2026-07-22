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

package auth

import (
	"context"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/join"
)

// RegisterUsingBoundKeypairMethod handles bound keypair joining for the legacy
// join service and accepts the legacy protobuf message types. It calls into
// the common logic implemented in lib/join.
//
// TODO(nklaassen): DELETE IN 20 when removing the legacy join service.
func (a *Server) RegisterUsingBoundKeypairMethod(
	ctx context.Context,
	req *proto.RegisterUsingBoundKeypairInitialRequest,
	challengeResponse client.RegisterUsingBoundKeypairChallengeResponseFunc,
) (*client.BoundKeypairRegistrationResponse, error) {
	return join.AdaptRegisterUsingBoundKeypairMethod(ctx, a, a.createBoundKeypairValidator, req, challengeResponse)
}
