/*
Copyright 2021 Gravitational, Inc.

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

package aws

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/utils"
)

// NewSigningService creates a new instance of SigningService.
func NewSigningService(config SigningServiceConfig) (*SigningService, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &SigningService{
		SigningServiceConfig: config,
	}, nil
}

// SigningService is an AWS CLI proxy service that signs AWS requests
// based on user identity.
type SigningService struct {
	// SigningServiceConfig is the SigningService configuration.
	SigningServiceConfig
}

// SigningServiceConfig is the SigningService configuration.
type SigningServiceConfig struct {
	// Session is AWS session.
	Session *awssession.Session
	// Clock is used to override time in tests.
	Clock clockwork.Clock
	// GetSigningCredentials allows to set the function responsible for obtaining STS credentials.
	// Used in tests to set static AWS credentials and skip API call.
	GetSigningCredentials GetSigningCredentialsFunc
}

// CheckAndSetDefaults validates the SigningServiceConfig config.
func (s *SigningServiceConfig) CheckAndSetDefaults() error {
	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}
	if s.Session == nil {
		ses, err := awssession.NewSessionWithOptions(awssession.Options{
			SharedConfigState: awssession.SharedConfigEnable,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		s.Session = ses
	}
	if s.GetSigningCredentials == nil {
		s.GetSigningCredentials = GetAWSCredentialsFromSTSAPI
	}
	return nil
}

// SigningCtx contains AWS SigV4 signing context parameters.
type SigningCtx struct {
	// SigningName is the AWS signing service name.
	SigningName string
	// SigningRegion is the AWS region to sign a request for.
	SigningRegion string
	// Expiry is the expiration of the AWS credentials used to sign requests.
	Expiry time.Time
	// SessionName is role session name of AWS credentials used to sign requests.
	SessionName string
	// AWSRoleArn is the AWS ARN of the role to assume for signing requests.
	AWSRoleArn string
	// AWSExternalID is an optional external ID used when getting sts credentials.
	AWSExternalID string
}

// Check checks signing context parameters.
func (sc *SigningCtx) Check(clock clockwork.Clock) error {
	switch {
	case sc.SigningName == "":
		return trace.BadParameter("missing AWS signing name")
	case sc.SigningRegion == "":
		return trace.BadParameter("missing AWS signing region")
	case sc.SessionName == "":
		return trace.BadParameter("missing AWS session name")
	case sc.AWSRoleArn == "":
		return trace.BadParameter("missing AWS Role ARN")
	case sc.Expiry.Before(clock.Now()):
		return trace.BadParameter("AWS SigV4 expiry has already expired")
	}
	_, err := ParseRoleARN(sc.AWSRoleArn)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SignRequest creates a new HTTP request and rewrites the header from the original request and returns a new
// HTTP request signed by STS AWS API.
// Signing steps:
// 1) Decode Authorization Header. Authorization Header example:
//
//		Authorization: AWS4-HMAC-SHA256
//		Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,
//		SignedHeaders=host;range;x-amz-date,
//		Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024
//
//	 2. Extract credential section from credential Authorization Header.
//	 3. Extract aws-region and aws-service from the credential section.
//	 4. Build AWS API endpoint based on extracted aws-region and aws-service fields.
//	    Not that for endpoint resolving the https://github.com/aws/aws-sdk-go/aws/endpoints/endpoints.go
//	    package is used and when Amazon releases a new API the dependency update is needed.
//	 5. Sign HTTP request.
func (s *SigningService) SignRequest(ctx context.Context, req *http.Request, signCtx *SigningCtx) (*http.Request, error) {
	if signCtx == nil {
		return nil, trace.BadParameter("missing signing context")
	}
	if err := signCtx.Check(s.Clock); err != nil {
		return nil, trace.Wrap(err)
	}
	payload, err := utils.GetAndReplaceRequestBody(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reqCopy := req.Clone(ctx)
	reqCopy.Body = io.NopCloser(req.Body)

	// Only keep the headers signed in the original request for signing. This
	// not only avoids signing extra headers injected by Teleport along the
	// way, but also preserves the signing logic of the original AWS client.
	//
	// For example, Athena ODBC driver sends query requests with "Expect:
	// 100-continue" headers without being signed, otherwise the Athena service
	// would reject the requests.
	unsignedHeaders := removeUnsignedHeaders(reqCopy)
	credentials := s.GetSigningCredentials(s.Session, signCtx.Expiry, signCtx.SessionName, signCtx.AWSRoleArn, signCtx.AWSExternalID)
	signer := NewSigner(credentials, signCtx.SigningName)
	_, err = signer.Sign(reqCopy, bytes.NewReader(payload), signCtx.SigningName, signCtx.SigningRegion, s.Clock.Now())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// copy removed headers back to the request after signing it, but don't copy the old Authorization header.
	copyHeaders(reqCopy, req, utils.RemoveFromSlice(unsignedHeaders, "Authorization"))
	return reqCopy, nil
}

// GetSigningCredentialsFunc allows to set the function responsible for obtaining STS credentials.
// Used in tests to set static AWS credentials and skip API call.
type GetSigningCredentialsFunc func(provider client.ConfigProvider, expiry time.Time, sessName, roleARN, externalID string) *credentials.Credentials

// GetAWSCredentialsFromSTSAPI obtains STS credentials.
func GetAWSCredentialsFromSTSAPI(provider client.ConfigProvider, expiry time.Time, sessName, roleARN, externalID string) *credentials.Credentials {
	return stscreds.NewCredentials(provider, roleARN,
		func(cred *stscreds.AssumeRoleProvider) {
			cred.RoleSessionName = sessName
			cred.Expiry.SetExpiration(expiry, 0)

			if externalID != "" {
				cred.ExternalID = aws.String(externalID)
			}
		},
	)
}

// removeUnsignedHeaders removes and returns header keys that are not included in SigV4 SignedHeaders.
// If the request is not already signed, then no headers are removed.
func removeUnsignedHeaders(reqCopy *http.Request) []string {
	// check if the request is already signed.
	authHeader := reqCopy.Header.Get("Authorization")
	sig, err := ParseSigV4(authHeader)
	if err != nil {
		return nil
	}
	return filterHeaders(reqCopy, sig.SignedHeaders)
}

// copyHeaders copies headers from src request to dst request, using a list of header keys to copy.
func copyHeaders(dst *http.Request, src *http.Request, keys []string) {
	for _, k := range keys {
		if vals, ok := src.Header[k]; ok {
			dst.Header[k] = vals
		}
	}
}
