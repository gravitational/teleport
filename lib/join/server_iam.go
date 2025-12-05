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
	"context"

	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/join/iamjoin"
	"github.com/gravitational/teleport/lib/join/internal/authz"
	"github.com/gravitational/teleport/lib/join/internal/diagnostic"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/join/provision"
	"github.com/gravitational/teleport/lib/modules"
)

// handleIAMJoin handles join attempts for the IAM join method.
//
// The IAM join method involves the following messages:
//
// client->server ClientInit
// client<-server ServerInit
// client->server IAMInit
// client<-server IAMChallenge
// client->server IAMChallengeSolution
// client<-server Result
//
// At this point the ServerInit message has already been sent, what's left is
// to receive the IAMInit message, handle the challenge-response, and send the
// final result if everything checks out.
func (s *Server) handleIAMJoin(
	stream messages.ServerStream,
	authCtx *authz.Context,
	clientInit *messages.ClientInit,
	token provision.Token,
) (messages.Response, error) {
	// Receive the IAMInit message from the client.
	iamInit, err := messages.RecvRequest[*messages.IAMInit](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving IAMInit message")
	}
	// Set any diagnostic info from the ClientParams.
	setDiagnosticClientParams(stream.Diagnostic(), &iamInit.ClientParams)

	// Generate and send the challenge.
	challenge, err := iamjoin.GenerateIAMChallenge()
	if err != nil {
		return nil, trace.Wrap(err, "generating challenge")
	}
	if err := stream.Send(&messages.IAMChallenge{
		Challenge: challenge,
	}); err != nil {
		return nil, trace.Wrap(err, "sending challenge")
	}

	// Receive the solution from the client.
	solution, err := messages.RecvRequest[*messages.IAMChallengeSolution](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving challenge solution")
	}

	// Verify the sts:GetCallerIdentity request, send it to AWS, and make sure
	// the verified identity matches allow rules in the provision token.
	verifiedIdentity, err := iamjoin.CheckIAMRequest(stream.Context(), &iamjoin.CheckIAMRequestParams{
		Challenge:                   challenge,
		ProvisionToken:              token,
		STSIdentityRequest:          solution.STSIdentityRequest,
		HTTPClient:                  s.cfg.AuthService.GetHTTPClientForAWSSTS(),
		FIPS:                        s.cfg.FIPS,
		DescribeAccountClientGetter: s.awsDescribeAccountClientGetter(),
	})
	// An identity will be returned even on error if the sts:GetCallerIdentity
	// request was completed but no allow rules were matched, include it in the
	// diagnostic for debugging.
	stream.Diagnostic().Set(func(info *diagnostic.Info) {
		info.RawJoinAttrs = verifiedIdentity
	})
	if err != nil {
		return nil, trace.Wrap(err, "verifying challenge response")
	}

	// Make and return the final result message.
	result, err := s.makeResult(
		stream.Context(),
		stream.Diagnostic(),
		authCtx,
		clientInit,
		&iamInit.ClientParams,
		token,
		verifiedIdentity,
		&workloadidentityv1pb.JoinAttrs{
			Iam: verifiedIdentity.JoinAttrs(),
		},
	)
	return result, trace.Wrap(err)
}

func (s *Server) awsDescribeAccountClientGetter() func(ctx context.Context) (iamjoin.DescribeAccountAPIClient, error) {
	// For IAM Join flow, the token might allow all all the AWS Accounts under an Organization.
	// In order to validate the organization ID of the joining identity, a call to organizations:DescribeAccount is performed.
	// This requires AWS credentials to be accessible to the Auth Service.
	// Currently, only ambient credentails are supported.
	// When running within Teleport Cloud, the Auth Service is not able to access ambient credentials for target account.
	// In that scenario a NotImplemented error is returned.
	if modules.GetModules().Features().Cloud {
		return func(ctx context.Context) (iamjoin.DescribeAccountAPIClient, error) {
			return nil, trace.NotImplemented("IAM Joins based on AWS Organization ID are not supported in Teleport Cloud")
		}
	}

	return func(ctx context.Context) (iamjoin.DescribeAccountAPIClient, error) {
		// Use cached AWS config without specifying a region, as Organizations is a global service.
		noRegion := ""
		awsCfg, err := s.awsCachedProvider.GetConfig(ctx, noRegion, awsconfig.WithAmbientCredentials())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return organizations.NewFromConfig(awsCfg).DescribeAccount, nil
	}
}
