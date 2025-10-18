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

package join

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/ec2join"
	"github.com/gravitational/teleport/lib/join/internal/authz"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

// handleEC2Join handles join attempts for the IAM join method.
//
// The EC2 join method involves the following messages:
//
// client->server ClientInit
// client<-server ServerInit
// client->server EC2Init
// client<-server Result
//
// At this point the ServerInit message has already been sent, what's left is
// to receive the EC2Init message, check the request EC2 join request
// invariants, and send the final result if everything checks out.
func (s *Server) handleEC2Join(
	stream messages.ServerStream,
	authCtx *authz.Context,
	clientInit *messages.ClientInit,
	provisionToken types.ProvisionToken,
) (messages.Response, error) {
	// Receive the EC2Init message from the client.
	ec2Init, err := messages.RecvRequest[*messages.EC2Init](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving EC2Init message")
	}
	// Set any diagnostic info from the ClientParams.
	setDiagnosticClientParams(stream.Diagnostic(), &ec2Init.ClientParams)

	var requestedHostID *string
	if authCtx.HostID != "" {
		requestedHostID = &authCtx.HostID
	}

	hostID, err := ec2join.CheckEC2Request(stream.Context(), &ec2join.CheckEC2RequestParams{
		ProvisionToken:  provisionToken,
		Role:            types.SystemRole(clientInit.SystemRole),
		RequestedHostID: requestedHostID,
		Document:        ec2Init.Document,
		Presence:        s.cfg.AuthService,
		EC2Client:       s.cfg.AuthService.GetEC2ClientForEC2JoinMethod(),
		Clock:           s.cfg.AuthService.GetClock(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "checking EC2 join attempt")
	}

	// At this point the request has been authenticated and the host ID
	// returned by CheckEC2Request is legitimate. Set authCtx.HostID so that
	// s.makeResult uses the EC2-format host ID instead of generating a UUID.
	authCtx.HostID = hostID

	// Make and return the final result message.
	result, err := s.makeResult(
		stream.Context(),
		stream.Diagnostic(),
		authCtx,
		clientInit,
		&ec2Init.ClientParams,
		provisionToken,
		nil, // ec2 method has no claims
		nil, // ec2 method has no attrs
	)
	return result, trace.Wrap(err)
}
