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
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
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

	// GetSigningCredentials allows so set the function responsible for obtaining STS credentials.
	// Used in tests to set static AWS credentials and skip API call.
	GetSigningCredentials getSigningCredentialsFunc
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
		s.GetSigningCredentials = getAWSCredentialsFromSTSAPI
	}
	return nil
}

type SigningCtx struct {
	Expiry        time.Time
	SessionName   string
	AWSRoleArn    string
	AWSExternalID string
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
func (s *SigningService) SignRequest(req *http.Request, sc SigningCtx) (*http.Request, []byte, *endpoints.ResolvedEndpoint, error) {
	payload, err := GetAndReplaceReqBody(req)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	re, err := resolveEndpoint(req)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	url := fmt.Sprintf("%s%s", re.URL, req.URL.Opaque)
	reqCopy, err := http.NewRequest(req.Method, url, bytes.NewReader(payload))
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	rewriteHeaders(req, reqCopy)
	credentials := s.GetSigningCredentials(s.Session, sc.Expiry, sc.SessionName, sc.AWSRoleArn, sc.AWSExternalID)
	signer := NewSigner(credentials, re.SigningName)
	_, err = signer.Sign(reqCopy, bytes.NewReader(payload), re.SigningName, re.SigningRegion, s.Clock.Now())
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	return reqCopy, payload, re, nil
}

// ReservedHeaders is a list of headers injected by Teleport.
var ReservedHeaders = []string{
	teleport.AppJWTHeader,
	teleport.AppCFHeader,
	forward.XForwardedFor,
	forward.XForwardedHost,
	forward.XForwardedProto,
	forward.XForwardedServer,
}

// IsReservedHeader returns true if the provided header is one of headers
// injected by Teleport.
func IsReservedHeader(header string) bool {
	for _, h := range ReservedHeaders {
		if http.CanonicalHeaderKey(header) == http.CanonicalHeaderKey(h) {
			return true
		}
	}
	return false
}

func rewriteHeaders(r *http.Request, reqCopy *http.Request) {
	for key, values := range r.Header {
		// Remove Teleport app headers.
		if IsReservedHeader(key) {
			continue
		}
		for _, v := range values {
			reqCopy.Header.Add(key, v)
		}
	}
	reqCopy.Header.Del("Content-Length")
}

type getSigningCredentialsFunc func(provider client.ConfigProvider, expiry time.Time, sessName, roleARN, externalID string) *credentials.Credentials

func getAWSCredentialsFromSTSAPI(provider client.ConfigProvider, expiry time.Time, sessName, roleARN, externalID string) *credentials.Credentials {
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
