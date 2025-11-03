/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/ec2join"
)

// SetEC2ClientForEC2JoinMethod sets an AWS EC2 client that will be used to
// check hosts joining via EC2 join method.
func (a *Server) SetEC2ClientForEC2JoinMethod(clt ec2join.EC2Client) {
	a.ec2ClientForEC2JoinMethod = clt
}

// GetEC2ClientForEC2JoinMethod returns an AWS EC2 client that should be used to
// check hosts joining via EC2 join method.
func (a *Server) GetEC2ClientForEC2JoinMethod() ec2join.EC2Client {
	return a.ec2ClientForEC2JoinMethod
}

// checkEC2JoinRequest checks register requests which use EC2 Simplified Node
// Joining. This method checks that:
//  1. The given Instance Identity Document has a valid signature (signed by AWS).
//  2. There is no obvious signs that a node already joined the cluster from this EC2 instance (to
//     reduce the risk of re-use of a stolen Instance Identity Document).
//  3. The signed instance attributes match one of the allow rules for the
//     corresponding token.
//
// If the request does not include an Instance Identity Document, and the
// token does not include any allow rules, this method returns nil and the
// normal token checking logic resumes.
//
// TODO(nklaassen): DELETE IN 20 when removing the legacy join service.
func (a *Server) checkEC2JoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) error {
	tokenName := req.Token
	provisionToken, err := a.GetToken(ctx, tokenName)
	if err != nil {
		return trace.Wrap(err)
	}

	a.logger.DebugContext(ctx, "Received Simplified Node Joining request", "host_id", req.HostID)

	if len(req.EC2IdentityDocument) == 0 {
		return trace.AccessDenied("this token is only valid for the EC2 join " +
			"method but the node has not included an EC2 Instance Identity " +
			"Document, make sure your node is configured to use the EC2 join method")
	}

	_, err = ec2join.CheckEC2Request(ctx, &ec2join.CheckEC2RequestParams{
		ProvisionToken:  provisionToken,
		Role:            req.Role,
		Document:        req.EC2IdentityDocument,
		RequestedHostID: &req.HostID,
		Presence:        a,
		EC2Client:       a.GetEC2ClientForEC2JoinMethod(),
		// Hosts calling this legacy join RPC may join with the Instance role
		// after already joining with another role, so the Instance identity
		// reuse check must be skipped.
		SkipInstanceReuseCheck: true,
		Clock:                  a.GetClock(),
	})
	return trace.Wrap(err)
}
