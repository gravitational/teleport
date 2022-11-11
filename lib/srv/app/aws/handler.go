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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/oxy/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/app/common"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// NewSigningService creates a new instance of SigningService.
func NewSigningService(config SigningServiceConfig) (*SigningService, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	svc := &SigningService{
		SigningServiceConfig: config,
	}

	fwd, err := forward.New(
		forward.RoundTripper(svc),
		forward.ErrorHandler(utils.ErrorHandlerFunc(svc.formatForwardResponseError)),
		forward.PassHostHeader(true),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	svc.fwd = fwd
	return svc, nil
}

// SigningService is an AWS CLI proxy service that signs AWS requests
// based on user identity.
type SigningService struct {
	// SigningServiceConfig is the SigningService configuration.
	SigningServiceConfig

	// fwd signs and forwards the request to AWS API.
	fwd *forward.Forwarder
}

// SigningServiceConfig is the SigningService configuration.
type SigningServiceConfig struct {
	// Client is an HTTP client instance used for HTTP calls.
	Client *http.Client
	// Log is the Logger.
	Log logrus.FieldLogger
	// Session is AWS session.
	Session *awssession.Session
	// Clock is used to override time in tests.
	Clock clockwork.Clock

	// getSigningCredentials allows so set the function responsible for obtaining STS credentials.
	// Used in tests to set static AWS credentials and skip API call.
	getSigningCredentials getSigningCredentialsFunc
}

// CheckAndSetDefaults validates the SigningServiceConfig config.
func (s *SigningServiceConfig) CheckAndSetDefaults() error {
	if s.Client == nil {
		tr, err := defaults.Transport()
		if err != nil {
			return trace.Wrap(err)
		}
		s.Client = &http.Client{
			Transport: tr,
		}
	}
	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}
	if s.Log == nil {
		s.Log = logrus.WithField(trace.Component, "aws:signer")
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
	if s.getSigningCredentials == nil {
		s.getSigningCredentials = getAWSCredentialsFromSTSAPI
	}
	return nil
}

// Handle handles the AWS CLI request.
func (s *SigningService) Handle(rw http.ResponseWriter, r *http.Request) {
	s.fwd.ServeHTTP(rw, r)
}

// RoundTrip handles incoming requests and forwards them to the proper AWS API.
// Handling steps:
// 1) Decoded Authorization Header. Authorization Header example:
//
//    Authorization: AWS4-HMAC-SHA256
//    Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,
//    SignedHeaders=host;range;x-amz-date,
//    Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024
//
// 2) Extract credential section from credential Authorization Header.
// 3) Extract aws-region and aws-service from the credential section.
// 4) Build AWS API endpoint based on extracted aws-region and aws-service fields.
//    Not that for endpoint resolving the https://github.com/aws/aws-sdk-go/aws/endpoints/endpoints.go
//    package is used and when Amazon releases a new API the dependency update is needed.
// 5) Sign HTTP request.
// 6) Forward the signed HTTP request to the AWS API.
func (s *SigningService) RoundTrip(req *http.Request) (*http.Response, error) {
	sessionCtx, err := common.GetSessionContext(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resolvedEndpoint, err := resolveEndpoint(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signedReq, err := s.prepareSignedRequest(req, resolvedEndpoint, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := s.Client.Do(signedReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (s *SigningService) formatForwardResponseError(rw http.ResponseWriter, r *http.Request, err error) {
	switch trace.Unwrap(err).(type) {
	case *trace.BadParameterError:
		s.Log.Debugf("Failed to process request: %v.", err)
		rw.WriteHeader(http.StatusBadRequest)
	case *trace.AccessDeniedError:
		s.Log.Infof("Failed to process request: %v.", err)
		rw.WriteHeader(http.StatusForbidden)
	default:
		s.Log.Warnf("Failed to process request: %v.", err)
		rw.WriteHeader(http.StatusInternalServerError)
	}
}

// prepareSignedRequest creates a new HTTP request and rewrites the header from the original request and returns a new
// HTTP request signed by STS AWS API.
func (s *SigningService) prepareSignedRequest(r *http.Request, re *endpoints.ResolvedEndpoint, sessionCtx *common.SessionContext) (*http.Request, error) {
	payload, err := awsutils.GetAndReplaceReqBody(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	url := fmt.Sprintf("%s%s", re.URL, r.URL.Opaque)
	reqCopy, err := http.NewRequest(r.Method, url, bytes.NewReader(payload))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rewriteHeaders(r, reqCopy)
	// Sign the copy of the request.
	signer := awsutils.NewSigner(s.getSigningCredentials(s.Session, sessionCtx), re.SigningName)
	_, err = signer.Sign(reqCopy, bytes.NewReader(payload), re.SigningName, re.SigningRegion, s.Clock.Now())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return reqCopy, nil
}

func rewriteHeaders(r *http.Request, reqCopy *http.Request) {
	for key, values := range r.Header {
		// Remove Teleport app headers.
		if appcommon.IsReservedHeader(key) {
			continue
		}
		for _, v := range values {
			reqCopy.Header.Add(key, v)
		}
	}
	reqCopy.Header.Del("Content-Length")
}

type getSigningCredentialsFunc func(c client.ConfigProvider, sessionCtx *common.SessionContext) *credentials.Credentials

func getAWSCredentialsFromSTSAPI(provider client.ConfigProvider, sessionCtx *common.SessionContext) *credentials.Credentials {
	return stscreds.NewCredentials(provider, sessionCtx.Identity.RouteToApp.AWSRoleARN,
		func(cred *stscreds.AssumeRoleProvider) {
			cred.RoleSessionName = sessionCtx.Identity.Username
			cred.Expiry.SetExpiration(sessionCtx.Identity.Expires, 0)

			if externalID := sessionCtx.App.GetAWSExternalID(); externalID != "" {
				cred.ExternalID = aws.String(externalID)
			}
		},
	)
}
