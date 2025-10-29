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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/join/internal/messages"
)

// oidcJoin handles stream interactions for most OIDC join methods that provide
// a static OIDC assertion.
func oidcJoin(
	stream messages.ClientStream,
	joinParams JoinParams,
	clientParams messages.ClientParams,
) (messages.Response, error) {
	// The OIDC join methods involve the following messages:
	//
	// client->server ClientInit
	// client<-server ServerInit
	// client->server OIDCInit
	// client<-server Result
	//
	// At this point the ServerInit messages has already been received, what's
	// left is to send the OIDCInit message and receive and return the final
	// result.
	if err := stream.Send(&messages.OIDCInit{
		ClientParams: clientParams,
		IDToken:      []byte(joinParams.IDToken),
	}); err != nil {
		return nil, trace.Wrap(err, "sending OIDCInit")
	}

	result, err := stream.Recv()
	return result, trace.Wrap(err, "receiving join result")
}
