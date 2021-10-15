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

package auth

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/gravitational/trace"
	"google.golang.org/grpc/peer"
)

const (
	expectedSTSIdentityRequestBody = "Action=GetCallerIdentity&Version=2011-06-15"
	stsHost                        = "sts.amazonaws.com"
	challengeHeaderKey             = "X-Teleport-Challenge"
	normalizedChallengeHeaderKey   = "x-teleport-challenge"
	authHeaderKey                  = "Authorization"
	acceptHeaderKey                = "Accept"
	acceptJSON                     = "application/json"
)

var signedHeadersRe = regexp.MustCompile(`^AWS4-HMAC-SHA256 Credential=\S+, SignedHeaders=(\S+), Signature=\S+$`)

// validateSTSIdentityRequest checks that a received sts:GetCallerIdentity
// request is valid and includes the challenge as a signed header. An example of
// a valid request looks like:
/*
POST / HTTP/1.1
Host: sts.amazonaws.com
Accept: application/json
Authorization: AWS4-HMAC-SHA256 Credential=AAAAAAAAAAAAAAAAAAAA/20211108/us-east-1/sts/aws4_request, SignedHeaders=accept;content-length;content-type;host;x-amz-date;x-amz-security-token;x-teleport-challenge, Signature=9999999999999999999999999999999999999999999999999999999999999999
Content-Length: 43
Content-Type: application/x-www-form-urlencoded; charset=utf-8
User-Agent: aws-sdk-go/1.37.17 (go1.17.1; darwin; amd64)
X-Amz-Date: 20211108T190420Z
X-Amz-Security-Token: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa=
X-Teleport-Challenge: 0ezlc3usTAkXeZTcfOazUq0BGrRaKmb4EwODk8U7J5A

Action=GetCallerIdentity&Version=2011-06-15
*/
func validateSTSIdentityRequest(req *http.Request, challenge string) error {
	if req.Host != stsHost {
		return trace.AccessDenied("sts identity request is for unknown host %q", req.Host)
	}

	if req.Method != http.MethodPost {
		return trace.AccessDenied("sts identity request method %q does not match expected method %q", req.RequestURI, http.MethodPost)
	}

	if req.Header.Get(challengeHeaderKey) != challenge {
		return trace.AccessDenied("sts identity request does not include challenge header or it does not match")
	}

	authHeader := req.Header.Get(authHeaderKey)
	matches := signedHeadersRe.FindStringSubmatch(authHeader)
	// first match should be the full header, second is the SignedHeaders
	if len(matches) < 2 {
		return trace.AccessDenied("sts identity request Authorization header is invalid")
	}
	signedHeadersString := matches[1]
	signedHeaders := strings.Split(signedHeadersString, ";")
	if !utils.SliceContainsStr(signedHeaders, normalizedChallengeHeaderKey) {
		return trace.AccessDenied("sts identity request auth header %q does not include "+
			normalizedChallengeHeaderKey+" as a signed header", authHeader)
	}

	// read the request body to make sure it matches
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	if !bytes.Equal([]byte(expectedSTSIdentityRequestBody), body) {
		return trace.BadParameter("sts request body %q does not equal expected %q", string(body), expectedSTSIdentityRequestBody)
	}

	// replace the request body since it was "drained" when read above
	req.Body = io.NopCloser(bytes.NewBuffer(body))

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

type stsIdentityResponse struct {
	GetCallerIdentityResponse struct {
		GetCallerIdentityResult awsIdentity
	}
}

type awsIdentity struct {
	Account string
	Arn     string
}

type stsClient interface {
	Do(*http.Request) (*http.Response, error)
}

func executeSTSIdentityRequest(ctx context.Context, client stsClient, req *http.Request) (identity awsIdentity, err error) {
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return identity, trace.Wrap(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return identity, trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return identity, trace.AccessDenied("aws sts api returned status: %q body: %q",
			resp.Status, body)
	}

	var identityResponse stsIdentityResponse
	if err := json.Unmarshal(body, &identityResponse); err != nil {
		return identity, trace.Wrap(err)
	}
	return identityResponse.GetCallerIdentityResponse.GetCallerIdentityResult, nil
}

// arnMatches returns true if arn matches the pattern. pattern should be an AWS
// ARN which may include "*" to match any combination of zero or more characters
// and "?" to match any single character, see https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_resource.html
func arnMatches(pattern, arn string) (bool, error) {
	// asterisk should match zero or
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	pattern = strings.ReplaceAll(pattern, "?", ".")
	pattern = "^" + pattern + "$"
	matched, err := regexp.MatchString(pattern, arn)
	return matched, trace.Wrap(err)
}

func checkIAMAllowRules(identity awsIdentity, provisionToken types.ProvisionToken) error {
	allowRules := provisionToken.GetAllowRules()
	for _, rule := range allowRules {
		// If this rule specifies an AWS account, the identity must match
		if len(rule.AWSAccount) > 0 {
			if rule.AWSAccount != identity.Account {
				// account doesn't match, continue to check the next rule
				continue
			}
		}
		// If this rule specifies an AWS ARN, the identity must match
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

func (a *Server) checkIAMRequest(ctx context.Context, client stsClient, challenge string, req *types.RegisterUsingTokenRequest) error {
	tokenName := req.Token
	provisionToken, err := a.GetCache().GetToken(ctx, tokenName)
	if err != nil {
		return trace.Wrap(err)
	}
	if provisionToken.GetJoinMethod() != types.JoinMethodIAM {
		return trace.AccessDenied("this token does not support the IAM join method")
	}

	// parse the incoming http request to the sts:GetCallerIdentity endpoint
	identityRequest, err := parseSTSRequest(req.STSIdentityRequest)
	if err != nil {
		return trace.Wrap(err)
	}

	// validate that the host, method, and headers are correct and the expected
	// challenge is included in the signed portion of the request
	if err := validateSTSIdentityRequest(identityRequest, challenge); err != nil {
		return trace.Wrap(err)
	}

	// send the signed request to the public AWS API and get the node identity
	// from the response
	identity, err := executeSTSIdentityRequest(ctx, client, identityRequest)
	if err != nil {
		return trace.Wrap(err)
	}

	// check that the node identity matches an allow rule for this token
	if err := checkIAMAllowRules(identity, provisionToken); err != nil {
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
	encoding := base64.RawStdEncoding
	challengeBase64 := make([]byte, encoding.EncodedLen(len(challengeRawBytes)))
	encoding.Encode(challengeBase64, challengeRawBytes)
	return string(challengeBase64), nil
}

// RegisterUsingIAMMethod is used to register new nodes to the cluster using the
// IAM join method.
func (a *Server) RegisterUsingIAMMethod(srv proto.AuthService_RegisterUsingIAMMethodServer) error {
	ctx := srv.Context()

	p, ok := peer.FromContext(ctx)
	if !ok {
		return trace.AccessDenied("failed to read peer information from gRPC context")
	}
	remoteAddr := p.Addr.String()

	challenge, err := generateChallenge()
	if err != nil {
		return trace.Wrap(err)
	}

	// send the challenge to the node
	if err := srv.Send(&proto.RegisterUsingIAMMethodResponse{
		Challenge: challenge,
	}); err != nil {
		return trace.Wrap(err)
	}

	req, err := srv.Recv()
	if err != nil {
		return trace.Wrap(err)
	}

	// fill in the client remote addr to the register request
	req.RemoteAddr = remoteAddr

	// check that the GetCallerIdentity request is valid and matches the token
	if err := a.checkIAMRequest(ctx, http.DefaultClient, challenge, req); err != nil {
		return trace.Wrap(err)
	}

	// pass on to the regular token checking logic
	certs, err := a.RegisterUsingToken(*req)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(srv.Send(&proto.RegisterUsingIAMMethodResponse{
		Certs: certs,
	}))
}

// createSignedSTSIdentityRequest is called on the client side and returns an
// sts:GetCallerIdentity request signed with the local AWS credentials
func createSignedSTSIdentityRequest(challenge string) ([]byte, error) {
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
	req.HTTPRequest.Header.Set(acceptHeaderKey, acceptJSON)
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
