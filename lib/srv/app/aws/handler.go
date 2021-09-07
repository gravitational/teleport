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

package appaws

import (
	"bytes"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/oxy/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/auth"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
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

	fwd *forward.Forwarder
}

// SigningServiceConfig is
type SigningServiceConfig struct {
	// Client is an HTTP client instance used for HTTP calls.
	Client *http.Client
	// Log is the Logger.
	Log logrus.FieldLogger
	// Session is AWS session.
	Session *awssession.Session
	// Clock is used to override time in
	Clock clockwork.Clock

	getSigningCredentials getSigningCredentialsFunc
}

// CheckAndSetDefaults validates the config.
func (s *SigningServiceConfig) CheckAndSetDefaults() error {
	if s.Client == nil {
		s.Client = http.DefaultClient
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
	req.RequestURI = ""
	ctxUser := req.Context().Value(auth.ContextUser)
	userI, ok := ctxUser.(auth.IdentityGetter)
	if !ok {
		return nil, trace.BadParameter("failed to get user identity")
	}
	identity := userI.GetIdentity()
	resolvedEndpoint, err := resolveEndpoint(req)
	if err != nil {
		return nil, err
	}
	signedReq, err := s.paperSignedRequest(req, resolvedEndpoint, &identity)
	if err != nil {
		return nil, err
	}
	resp, err := s.Client.Do(signedReq)
	if err != nil {
		return nil, err
	}
	return resp, err
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

func resolveEndpoint(r *http.Request) (*endpoints.ResolvedEndpoint, error) {
	awsAuthHeader, err := ParseSigV4(r.Header.Get(authorizationHeader))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resolvedEndpoint, err := endpoints.DefaultResolver().EndpointFor(awsAuthHeader.Service, awsAuthHeader.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &resolvedEndpoint, nil
}

// paperSignedRequest creates a new HTTP request and rewrites the header from the original request and returns a new
// HTTP request signed by STS AWS API.
func (s *SigningService) paperSignedRequest(r *http.Request, resolvedEndpoint *endpoints.ResolvedEndpoint, identity *tlsca.Identity) (*http.Request, error) {
	payload, err := GetAndReplaceReqBody(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reqCopy, err := http.NewRequest(r.Method, resolvedEndpoint.URL+r.URL.Opaque, bytes.NewReader(payload))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for k, kv := range r.Header {
		// Remove Teleport app headers.
		if appcommon.IsReservedHeader(k) {
			continue
		}
		for _, v := range kv {
			reqCopy.Header.Add(k, v)
		}
	}
	err = s.sign(
		reqCopy,
		bytes.NewReader(payload),
		resolvedEndpoint.SigningName,
		resolvedEndpoint.SigningRegion,
		identity,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return reqCopy, nil
}

type getSigningCredentialsFunc func(c client.ConfigProvider, identity *tlsca.Identity) *credentials.Credentials

func getAWSCredentialsFromSTSAPI(provider client.ConfigProvider, identity *tlsca.Identity) *credentials.Credentials {
	return stscreds.NewCredentials(provider, identity.RouteToApp.AWSRoleARN,
		func(cred *stscreds.AssumeRoleProvider) {
			cred.RoleSessionName = identity.Username
			cred.Expiry.SetExpiration(identity.Expires, 0)
		},
	)
}

func (s *SigningService) sign(r *http.Request, b io.ReadSeeker, service, region string, identity *tlsca.Identity) error {
	signer := v4.NewSigner(s.getSigningCredentials(s.Session, identity))
	_, err := signer.Sign(r, b, service, region, s.Clock.Now())
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
