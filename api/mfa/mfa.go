/*
Copyright 2023 Gravitational, Inc.

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

package mfa

import (
	"bytes"
	"context"
	"encoding/base64"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	"github.com/gravitational/teleport/api/client/proto"
)

const mfaResponseToken = "mfa_challenge_response"

// ErrAdminActionMFARequired is an error indicating that an admin-level
// API request failed due to missing MFA verification.
var ErrAdminActionMFARequired = trace.AccessDeniedError{Message: "admin-level API request requires MFA verification"}

// WithCredentials can be called on a GRPC client request to attach
// MFA credentials to the GRPC metadata for requests that require MFA,
// like admin-level requests.
func WithCredentials(resp *proto.MFAAuthenticateResponse) grpc.CallOption {
	return grpc.PerRPCCredentials(&perRPCCredentials{MFAChallengeResponse: resp})
}

// CredentialsFromContext can be called from a GRPC server method to return
// MFA credentials added to the GRPC metadata for requests that require MFA,
// like admin-level requests. If no MFA credentials are found, an
// ErrAdminActionMFARequired will be returned, aggregated with any other errors
// encountered.
func CredentialsFromContext(ctx context.Context) (*proto.MFAAuthenticateResponse, error) {
	resp, err := getMFACredentialsFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func getMFACredentialsFromContext(ctx context.Context) (*proto.MFAAuthenticateResponse, error) {
	values := metadata.ValueFromIncomingContext(ctx, mfaResponseToken)
	if len(values) == 0 {
		return nil, trace.BadParameter("request metadata missing MFA credentials")
	}
	mfaChallengeResponseEnc := values[0]

	mfaChallengeResponseJSON, err := base64.StdEncoding.DecodeString(mfaChallengeResponseEnc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var mfaChallengeResponse proto.MFAAuthenticateResponse
	if err := jsonpb.Unmarshal(bytes.NewReader(mfaChallengeResponseJSON), &mfaChallengeResponse); err != nil {
		return nil, trace.Wrap(err)
	}

	return &mfaChallengeResponse, nil
}

// perRPCCredentials supplies perRPCCredentials from an MFA challenge response.
type perRPCCredentials struct {
	MFAChallengeResponse *proto.MFAAuthenticateResponse
}

// GetRequestMetadata gets the request metadata as a map from a TokenSource.
func (mc *perRPCCredentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	ri, _ := credentials.RequestInfoFromContext(ctx)
	if err := credentials.CheckSecurityLevel(ri.AuthInfo, credentials.PrivacyAndIntegrity); err != nil {
		return nil, trace.BadParameter("unable to transfer MFA PerRPCCredentials: %v", err)
	}

	challengeJSON, err := (&jsonpb.Marshaler{}).MarshalToString(mc.MFAChallengeResponse)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	enc := base64.StdEncoding.EncodeToString([]byte(challengeJSON))
	return map[string]string{
		mfaResponseToken: enc,
	}, nil
}

// RequireTransportSecurity indicates whether the credentials requires transport security.
func (mc *perRPCCredentials) RequireTransportSecurity() bool {
	return true
}
