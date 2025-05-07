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

// ResponseMetadataKey is the context metadata key for an MFA response in a gRPC request.
const ResponseMetadataKey = "mfa_challenge_response"

var (
	// ErrAdminActionMFARequired is an error indicating that an admin-level
	// API request failed due to missing MFA verification.
	ErrAdminActionMFARequired = trace.AccessDeniedError{Message: "admin-level API request requires MFA verification"}

	// ErrMFANotRequired is returned by MFA ceremonies when it is discovered or
	// inferred that an MFA ceremony is not required by the server.
	ErrMFANotRequired = trace.BadParameterError{Message: "re-authentication with MFA is not required"}

	// ErrMFANotSupported is returned by MFA ceremonies when the client does not
	// support MFA ceremonies, or the server does not support MFA ceremonies for
	// the client user.
	ErrMFANotSupported = trace.BadParameterError{Message: "re-authentication with MFA is not supported for this client"}

	// ErrExpiredReusableMFAResponse is returned by Auth APIs like
	// GenerateUserCerts when an expired reusable MFA response is provided.
	ErrExpiredReusableMFAResponse = trace.AccessDeniedError{
		Message: "Reusable MFA response validation failed and possibly expired",
	}
)

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
	values := metadata.ValueFromIncomingContext(ctx, ResponseMetadataKey)
	if len(values) == 0 {
		return nil, trace.NotFound("request metadata missing MFA credentials")
	}
	mfaChallengeResponseEnc := values[0]

	mfaChallengeResponseJSON, err := base64.StdEncoding.DecodeString(mfaChallengeResponseEnc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var mfaChallengeResponse proto.MFAAuthenticateResponse
	if err := (&jsonpb.Unmarshaler{AllowUnknownFields: true}).Unmarshal(bytes.NewReader(mfaChallengeResponseJSON), &mfaChallengeResponse); err != nil {
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

	enc, err := EncodeMFAChallengeResponseCredentials(mc.MFAChallengeResponse)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return map[string]string{
		ResponseMetadataKey: enc,
	}, nil
}

// RequireTransportSecurity indicates whether the credentials requires transport security.
func (mc *perRPCCredentials) RequireTransportSecurity() bool {
	return true
}

// EncodeMFAChallengeResponseCredentials encodes the given MFA challenge response into a string.
func EncodeMFAChallengeResponseCredentials(mfaResp *proto.MFAAuthenticateResponse) (string, error) {
	challengeJSON, err := (&jsonpb.Marshaler{}).MarshalToString(mfaResp)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return base64.StdEncoding.EncodeToString([]byte(challengeJSON)), nil
}

type mfaResponseContextKey struct{}

// ContextWithMFAResponse embeds the MFA response in the context.
func ContextWithMFAResponse(ctx context.Context, mfaResp *proto.MFAAuthenticateResponse) context.Context {
	return context.WithValue(ctx, mfaResponseContextKey{}, mfaResp)
}

// MFAResponseFromContext returns the MFA response from the context.
func MFAResponseFromContext(ctx context.Context) (*proto.MFAAuthenticateResponse, error) {
	if val := ctx.Value(mfaResponseContextKey{}); val != nil {
		mfaResp, ok := val.(*proto.MFAAuthenticateResponse)
		if !ok {
			return nil, trace.BadParameter("unexpected context value type %T", val)
		}
		if mfaResp == nil {
			return nil, trace.NotFound("mfa response not found in the context")
		}
		return mfaResp, nil
	}
	return nil, trace.NotFound("mfa response not found in the context")
}
