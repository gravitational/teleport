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

package aws

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/cloud/awsconfig"
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
	// SessionProvider is a provider for AWS Sessions.
	SessionProvider AWSSessionProvider
	// Clock is used to override time in tests.
	Clock clockwork.Clock
	// CredentialsGetter is used to obtain STS credentials.
	CredentialsGetter CredentialsGetter
	// AWSConfigProvider is a provider for AWS configs.
	AWSConfigProvider awsconfig.Provider
}

// CheckAndSetDefaults validates the SigningServiceConfig config.
func (s *SigningServiceConfig) CheckAndSetDefaults() error {
	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}

	if s.AWSConfigProvider == nil {
		if s.SessionProvider == nil {
			return trace.BadParameter("session provider or config provider is required")
		}
		if s.CredentialsGetter == nil {
			// Use cachedCredentialsGetter by default. cachedCredentialsGetter
			// caches the credentials for one minute.
			cachedGetter, err := NewCachedCredentialsGetter(CachedCredentialsGetterConfig{
				Clock: s.Clock,
			})
			if err != nil {
				return trace.Wrap(err)
			}

			s.CredentialsGetter = cachedGetter
		}
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
	// BaseAWSRoleARN is the AWS ARN of the role as a base to the assumed roles.
	BaseAWSRoleARN string
	// BaseAWSRoleARN is an optional external ID used on base assumed role.
	BaseAWSExternalID string
	// AWSRoleArn is the AWS ARN of the role to assume for signing requests,
	// chained with BaseAWSRoleARN.
	AWSRoleArn string
	// AWSExternalID is an optional external ID used when getting sts credentials.
	AWSExternalID string
	// SessionTags is a list of AWS STS session tags.
	SessionTags map[string]string
	// Integration is the Integration name to use to generate credentials.
	// If empty, it will use ambient credentials
	Integration string
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
	signer, err := s.newSigner(ctx, signCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = signer.Sign(reqCopy, bytes.NewReader(payload), signCtx.SigningName, signCtx.SigningRegion, s.Clock.Now())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// copy removed headers back to the request after signing it, but don't copy the old Authorization header.
	copyHeaders(reqCopy, req, utils.RemoveFromSlice(unsignedHeaders, "Authorization"))
	return reqCopy, nil
}

// TODO(gabrielcorado): once all service callers are updated to use
// AWSConfigProvider, make it required and remove session provider and
// credentials getter fallback.
func (s *SigningService) newSigner(ctx context.Context, signCtx *SigningCtx) (*v4.Signer, error) {
	if s.AWSConfigProvider != nil {
		awsCfg, err := s.AWSConfigProvider.GetConfig(ctx, signCtx.SigningRegion,
			awsconfig.WithAssumeRole(signCtx.BaseAWSRoleARN, signCtx.BaseAWSExternalID),
			awsconfig.WithAssumeRole(signCtx.AWSRoleArn, signCtx.AWSExternalID),
			awsconfig.WithCredentialsMaybeIntegration(signCtx.Integration),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return NewSignerV2(awsCfg.Credentials, signCtx.SigningName), nil
	}

	session, err := s.SessionProvider(ctx, signCtx.SigningRegion, signCtx.Integration)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credentials, err := s.CredentialsGetter.Get(ctx, GetCredentialsRequest{
		Provider:    session,
		Expiry:      signCtx.Expiry,
		SessionName: signCtx.SessionName,
		RoleARN:     signCtx.AWSRoleArn,
		ExternalID:  signCtx.AWSExternalID,
		Tags:        signCtx.SessionTags,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewSigner(credentials, signCtx.SigningName), nil
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
