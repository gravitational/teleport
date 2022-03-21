/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/trace"
)

// JoinServiceClient is a client for the JoinService, which runs on both the
// auth and proxy.
type JoinServiceClient struct {
	grpcClient proto.JoinServiceClient
}

// NewJoinServiceClient returns a new JoinServiceClient wrapping the given grpc
// client.
func NewJoinServiceClient(grpcClient proto.JoinServiceClient) *JoinServiceClient {
	return &JoinServiceClient{
		grpcClient: grpcClient,
	}
}

// RegisterChallengeResponseFunc is a function type meant to be passed to
// RegisterUsingIAMMethod. It must return a *types.RegisterUsingTokenRequest for
// a given challenge, or an error.
type RegisterChallengeResponseFunc func(challenge string) (*proto.RegisterUsingIAMMethodRequest, error)

// RegisterUsingIAMMethod registers the caller using the IAM join method and
// returns signed certs to join the cluster.
//
// The caller must provide a ChallengeResponseFunc which returns a
// *types.RegisterUsingTokenRequest with a signed sts:GetCallerIdentity request
// including the challenge as a signed header.
func (c *JoinServiceClient) RegisterUsingIAMMethod(ctx context.Context, challengeResponse RegisterChallengeResponseFunc) (*proto.Certs, error) {
	// initiate the streaming rpc
	iamJoinClient, err := c.grpcClient.RegisterUsingIAMMethod(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// wait for the challenge string from auth
	challenge, err := iamJoinClient.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get challenge response from the caller
	req, err := challengeResponse(challenge.Challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// forward the challenge response from the caller to auth
	if err := iamJoinClient.Send(req); err != nil {
		return nil, trace.Wrap(err)
	}

	// wait for the certs from auth and return to the caller
	certsResp, err := iamJoinClient.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certsResp.Certs, nil
}
