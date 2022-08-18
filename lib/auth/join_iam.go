/*
Copyright 2021-2022 Gravitational, Inc.

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

package auth

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/utils/aws"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/gravitational/trace"
)

const (
	// Hardcoding the sts API version here may be more strict than necessary,
	// but this is set by the Teleport node and can only be changed when we
	// update our AWS SDK dependency. Since Auth should always be upgraded
	// before nodes, we will have a chance to update the check on Auth if we
	// ever have a need to allow a newer API version.
	expectedStsIdentityRequestBody = "Action=GetCallerIdentity&Version=2011-06-15"

	// Only allowing the global sts endpoint here, Teleport nodes will only send
	// requests for this endpoint. If we want to start using regional endpoints
	// we can update this check before updating the nodes.
	stsHost = "sts.amazonaws.com"

	// AWS SignedHeaders will always be lowercase
	// https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-auth-using-authorization-header.html#sigv4-auth-header-overview
	challengeHeaderKey = "x-teleport-challenge"
)

// validateStsIdentityRequest checks that a received sts:GetCallerIdentity
// request is valid and includes the challenge as a signed header. An example
// valid request looks like:
// ```
// POST / HTTP/1.1
// Host: sts.amazonaws.com
// Accept: application/json
// Authorization: AWS4-HMAC-SHA256 Credential=AAAAAAAAAAAAAAAAAAAA/20211108/us-east-1/sts/aws4_request, SignedHeaders=accept;content-length;content-type;host;x-amz-date;x-amz-security-token;x-teleport-challenge, Signature=999...
// Content-Length: 43
// Content-Type: application/x-www-form-urlencoded; charset=utf-8
// User-Agent: aws-sdk-go/1.37.17 (go1.17.1; darwin; amd64)
// X-Amz-Date: 20211108T190420Z
// X-Amz-Security-Token: aaa...
// X-Teleport-Challenge: 0ezlc3usTAkXeZTcfOazUq0BGrRaKmb4EwODk8U7J5A
//
// Action=GetCallerIdentity&Version=2011-06-15
// ```
func validateStsIdentityRequest(req *http.Request, challenge string) error {
	if req.Host != stsHost {
		return trace.AccessDenied("sts identity request is for unknown host %q", req.Host)
	}

	if req.Method != http.MethodPost {
		return trace.AccessDenied("sts identity request method %q does not match expected method %q", req.RequestURI, http.MethodPost)
	}

	if req.Header.Get(challengeHeaderKey) != challenge {
		return trace.AccessDenied("sts identity request does not include challenge header or it does not match")
	}

	authHeader := req.Header.Get(aws.AuthorizationHeader)

	sigV4, err := aws.ParseSigV4(authHeader)
	if err != nil {
		return trace.Wrap(err)
	}
	if !utils.SliceContainsStr(sigV4.SignedHeaders, challengeHeaderKey) {
		return trace.AccessDenied("sts identity request auth header %q does not include "+
			challengeHeaderKey+" as a signed header", authHeader)
	}

	body, err := aws.GetAndReplaceReqBody(req)
	if err != nil {
		return trace.Wrap(err)
	}
	if !bytes.Equal([]byte(expectedStsIdentityRequestBody), body) {
		return trace.BadParameter("sts request body %q does not equal expected %q", string(body), expectedStsIdentityRequestBody)
	}

	return nil
}

func parseSTSRequest(req []byte) (*http.Request, error) {
	httpReq, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(req)))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Unset RequestURI and set req.URL instead (necessary quirk of sending a
	// request parsed by http.ReadRequest). Also, force https here.
	if httpReq.RequestURI != "/" {
		return nil, trace.AccessDenied("unexpected sts identity request URI: %q", httpReq.RequestURI)
	}
	httpReq.RequestURI = ""
	httpReq.URL = &url.URL{
		Scheme: "https",
		Host:   stsHost,
	}
	return httpReq, nil
}

// awsIdentity holds aws Account and Arn, used for JSON parsing
type awsIdentity struct {
	Account string `json:"Account"`
	Arn     string `json:"Arn"`
}

// getCallerIdentityReponse is used for JSON parsing
type getCallerIdentityResponse struct {
	GetCallerIdentityResult awsIdentity `json:"GetCallerIdentityResult"`
}

// stsIdentityResponse is used for JSON parsing
type stsIdentityResponse struct {
	GetCallerIdentityResponse getCallerIdentityResponse `json:"GetCallerIdentityResponse"`
}

type stsClient interface {
	Do(*http.Request) (*http.Response, error)
}

type stsClientKey struct{}

// stsClientFromContext allows the default http client to be overridden for tests
func stsClientFromContext(ctx context.Context) stsClient {
	client, ok := ctx.Value(stsClientKey{}).(stsClient)
	if ok {
		return client
	}
	return http.DefaultClient
}

// executeStsIdentityRequest sends the sts:GetCallerIdentity HTTP request to the
// AWS API, parses the response, and returns the awsIdentity
func executeStsIdentityRequest(ctx context.Context, req *http.Request) (*awsIdentity, error) {
	client := stsClientFromContext(ctx)

	// set the http request context so it can be canceled
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, trace.AccessDenied("aws sts api returned status: %q body: %q",
			resp.Status, body)
	}

	var identityResponse stsIdentityResponse
	if err := json.Unmarshal(body, &identityResponse); err != nil {
		return nil, trace.Wrap(err)
	}

	id := &identityResponse.GetCallerIdentityResponse.GetCallerIdentityResult
	if id.Account == "" {
		return nil, trace.BadParameter("received empty AWS account ID from sts API")
	}
	if id.Arn == "" {
		return nil, trace.BadParameter("received empty AWS identity ARN from sts API")
	}
	return id, nil
}

// arnMatches returns true if arn matches the pattern.
// Pattern should be an AWS ARN which may include "*" to match any combination
// of zero or more characters and "?" to match any single character.
// See https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_resource.html
func arnMatches(pattern, arn string) (bool, error) {
	pattern = regexp.QuoteMeta(pattern)
	pattern = strings.ReplaceAll(pattern, `\*`, ".*")
	pattern = strings.ReplaceAll(pattern, `\?`, ".")
	pattern = "^" + pattern + "$"
	matched, err := regexp.MatchString(pattern, arn)
	return matched, trace.Wrap(err)
}

// checkIAMAllowRules checks if the given identity matches any of the given
// allowRules.
func checkIAMAllowRules(identity *awsIdentity, allowRules []*types.TokenRule) error {
	for _, rule := range allowRules {
		// if this rule specifies an AWS account, the identity must match
		if len(rule.AWSAccount) > 0 {
			if rule.AWSAccount != identity.Account {
				// account doesn't match, continue to check the next rule
				continue
			}
		}
		// if this rule specifies an AWS ARN, the identity must match
		if len(rule.AWSARN) > 0 {
			matches, err := arnMatches(rule.AWSARN, identity.Arn)
			if err != nil {
				return trace.Wrap(err)
			}
			if !matches {
				// arn doesn't match, continue to check the next rule
				continue
			}
		}
		// node identity matches this allow rule
		return nil
	}
	return trace.AccessDenied("instance did not match any allow rules")
}

// checkIAMRequest checks if the given request satisfies the token rules and
// included the required challenge.
func (a *Server) checkIAMRequest(ctx context.Context, challenge string, req *proto.RegisterUsingIAMMethodRequest) error {
	tokenName := req.RegisterUsingTokenRequest.Token
	provisionToken, err := a.GetToken(ctx, tokenName)
	if err != nil {
		return trace.Wrap(err)
	}
	if provisionToken.GetJoinMethod() != types.JoinMethodIAM {
		return trace.AccessDenied("this token does not support the IAM join method")
	}

	// parse the incoming http request to the sts:GetCallerIdentity endpoint
	identityRequest, err := parseSTSRequest(req.StsIdentityRequest)
	if err != nil {
		return trace.Wrap(err)
	}

	// validate that the host, method, and headers are correct and the expected
	// challenge is included in the signed portion of the request
	if err := validateStsIdentityRequest(identityRequest, challenge); err != nil {
		return trace.Wrap(err)
	}

	// send the signed request to the public AWS API and get the node identity
	// from the response
	identity, err := executeStsIdentityRequest(ctx, identityRequest)
	if err != nil {
		return trace.Wrap(err)
	}

	// check that the node identity matches an allow rule for this token
	if err := checkIAMAllowRules(identity, provisionToken.GetAllowRules()); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func generateChallenge() (string, error) {
	// read 32 crypto-random bytes to generate the challenge
	challengeRawBytes := make([]byte, 32)
	if _, err := rand.Read(challengeRawBytes); err != nil {
		return "", trace.Wrap(err)
	}

	// encode the challenge to base64 so it can be sent in an HTTP header
	return base64.RawStdEncoding.EncodeToString(challengeRawBytes), nil
}

// RegisterUsingIAMMethod registers the caller using the IAM join method and
// returns signed certs to join the cluster.
//
// The caller must provide a ChallengeResponseFunc which returns a
// *types.RegisterUsingTokenRequest with a signed sts:GetCallerIdentity request
// including the challenge as a signed header.
func (a *Server) RegisterUsingIAMMethod(ctx context.Context, challengeResponse client.RegisterChallengeResponseFunc) (*proto.Certs, error) {
	clientAddr, ok := ctx.Value(ContextClientAddr).(net.Addr)
	if !ok {
		return nil, trace.BadParameter("logic error: client address was not set")
	}

	challenge, err := generateChallenge()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := challengeResponse(challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// fill in the client remote addr to the register request
	req.RegisterUsingTokenRequest.RemoteAddr = clientAddr.String()
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// perform common token checks
	provisionToken, err := a.checkTokenJoinRequestCommon(ctx, req.RegisterUsingTokenRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// check that the GetCallerIdentity request is valid and matches the token
	if err := a.checkIAMRequest(ctx, challenge, req); err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := a.generateCerts(ctx, provisionToken, req.RegisterUsingTokenRequest)
	return certs, trace.Wrap(err)
}

// createSignedStsIdentityRequest is called on the client side and returns an
// sts:GetCallerIdentity request signed with the local AWS credentials
func createSignedStsIdentityRequest(challenge string) ([]byte, error) {
	// use the aws sdk to generate the request
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stsService := sts.New(sess)
	req, _ := stsService.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
	// set challenge header
	req.HTTPRequest.Header.Set(challengeHeaderKey, challenge)
	// request json for simpler parsing
	req.HTTPRequest.Header.Set("Accept", "application/json")
	// sign the request, including headers
	if err := req.Sign(); err != nil {
		return nil, trace.Wrap(err)
	}
	// write the signed HTTP request to a buffer
	var signedRequest bytes.Buffer
	if err := req.HTTPRequest.Write(&signedRequest); err != nil {
		return nil, trace.Wrap(err)
	}
	return signedRequest.Bytes(), nil
}
