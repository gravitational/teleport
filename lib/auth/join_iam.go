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
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/aws"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
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
	expectedSTSIdentityRequestBody = "Action=GetCallerIdentity&Version=2011-06-15"

	// Used to check if we were unable to resolve the regional STS endpoint.
	globalSTSEndpoint = "https://sts.amazonaws.com"

	// AWS SignedHeaders will always be lowercase
	// https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-auth-using-authorization-header.html#sigv4-auth-header-overview
	challengeHeaderKey = "x-teleport-challenge"
)

// validateSTSHost returns an error if the given stsHost is not a valid regional
// endpoint for the AWS STS service, or nil if it is valid.
//
// This is a security-critical check: we are allowing the client to tell us
// which URL we should use to validate their identity. If the client could pass
// off an attacker-controlled URL as the STS endpoint, the entire security
// mechanism of the IAM join method would be compromised.
//
// To keep this validation simple and secure, we check the given endpoint
// against a static list of known valid endpoints. We will need to update this
// list as AWS adds new regions.
func validateSTSHost(stsHost string) error {
	valid := apiutils.SliceContainsStr(validSTSEndpoints, stsHost)
	if valid {
		return nil
	}

	return trace.AccessDenied("IAM join request uses unknown STS host %q. "+
		"This could mean that the Teleport Node attempting to join the cluster is "+
		"running in a new AWS region which is unknown to this Teleport auth server. "+
		"Alternatively, if this URL looks suspicious, an attacker may be attempting to "+
		"join your Teleport cluster. "+
		"Following is the list of valid STS endpoints known to this auth server. "+
		"If a legitimate STS endpoint is not included, please file an issue at "+
		"https://github.com/gravitational/teleport. %v",
		stsHost, validSTSEndpoints)
}

// validateSTSIdentityRequest checks that a received sts:GetCallerIdentity
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
func validateSTSIdentityRequest(req *http.Request, challenge string) (err error) {
	defer func() {
		// Always log a warning on the Auth server if the function detects an
		// invalid sts:GetCallerIdentity request, it's either going to be caused
		// by a node in a unknown region or an attacker.
		if err != nil {
			log.WithError(err).Warn("Detected an invalid sts:GetCallerIdentity used by a client attempting to use the IAM join method.")
		}
	}()

	if err := validateSTSHost(req.Host); err != nil {
		return trace.Wrap(err)
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
	if !apiutils.SliceContainsStr(sigV4.SignedHeaders, challengeHeaderKey) {
		return trace.AccessDenied("sts identity request auth header %q does not include "+
			challengeHeaderKey+" as a signed header", authHeader)
	}

	body, err := aws.GetAndReplaceReqBody(req)
	if err != nil {
		return trace.Wrap(err)
	}
	if !bytes.Equal([]byte(expectedSTSIdentityRequestBody), body) {
		return trace.BadParameter("sts request body %q does not equal expected %q", string(body), expectedSTSIdentityRequestBody)
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
		Host:   httpReq.Host,
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

// executeSTSIdentityRequest sends the sts:GetCallerIdentity HTTP request to the
// AWS API, parses the response, and returns the awsIdentity
func executeSTSIdentityRequest(ctx context.Context, req *http.Request) (*awsIdentity, error) {
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
	if err := validateSTSIdentityRequest(identityRequest, challenge); err != nil {
		return trace.Wrap(err)
	}

	// send the signed request to the public AWS API and get the node identity
	// from the response
	identity, err := executeSTSIdentityRequest(ctx, identityRequest)
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

// createSignedSTSIdentityRequest is called on the client side and returns an
// sts:GetCallerIdentity request signed with the local AWS credentials
func createSignedSTSIdentityRequest(ctx context.Context, endpointOption stsEndpointOption, challenge string) ([]byte, error) {
	stsClient, err := endpointOption(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, _ := stsClient.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
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

type stsEndpointOption func(context.Context) (*sts.STS, error)

var (
	stsEndpointOptionGlobal   = newGlobalSTSClient
	stsEndpointOptionRegional = newRegionalSTSClient
)

// newRegionalSTSClient returns an STS client will resolve the "global" endpoint
// for the STS service.
func newGlobalSTSClient(ctx context.Context) (*sts.STS, error) {
	// sess will be used as a ConfigProvider to be passed to sts.New. It will
	// load AWS configuration options from the environment, which means that AWS
	// credentials may come from environment variables, files in ~/.aws/, or
	// from the attached role on an EC2 instance.
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sts.New(sess), nil
}

// newRegionalSTSClient returns an STS client which attempts to resolve the local
// regional endpoint for the STS service, rather than the "global" endpoint
// which is not supported in non-default AWS partitions.
func newRegionalSTSClient(ctx context.Context) (*sts.STS, error) {
	// sess will be used as a ConfigProvider to be passed to sts.New. It will
	// load AWS configuration options from the environment, which means that AWS
	// credentials may come from environment variables, files in ~/.aws/, or
	// from the attached role on an EC2 instance. The regional STS endpoint will
	// be used instead of the global endopint if the local (or preferred) region
	// can be resolved from the environment.
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config:            *awssdk.NewConfig().WithSTSRegionalEndpoint(endpoints.RegionalSTSEndpoint),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// will set the local region on extraConfigOptions if we can find it from
	// the environment or IMDS
	extraConfigOptions := awssdk.NewConfig()

	// If the region was not resolved from the environment the client will try to
	// use the global STS endpoint, which will not be supported if the AWS identity
	// being used is for a non-default AWS partition (such as China or
	// GovCloud.) This is the default behavior on EC2, so let's try to find the
	// region from the IMDS.
	if clientConfig := sess.ClientConfig(sts.ServiceName); clientConfig.Endpoint == globalSTSEndpoint {
		region, err := getEC2LocalRegion(ctx)
		if trace.IsNotFound(err) {
			// Unfortunately we could not find the region from the IMDS, go with
			// the default global endpoint and hope it works.
			log.Info("Unable to find the local AWS region from the environment or IMDSv2. " +
				"Attempting to use the global STS endpoint for the IAM join method. " +
				"This will probably fail in non-default AWS partitions such as China or GovCloud. " +
				"Consider setting the AWS_REGION environment variable, setting the region in ~/.aws/config, or enabling the IMDSv2.")
		} else if err != nil {
			// Return the unexpected error.
			return nil, trace.Wrap(err)
		} else {
			// Found the region, set it on the config.
			extraConfigOptions.Region = &region
		}
	}

	return sts.New(sess, extraConfigOptions), nil
}

// getEC2LocalRegion returns the AWS region this EC2 instance is running in, or
// a NotFound error if the EC2 IMDS is unavailable.
func getEC2LocalRegion(ctx context.Context) (string, error) {
	imdsClient, err := utils.NewInstanceMetadataClient(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if !imdsClient.IsAvailable(ctx) {
		return "", trace.NotFound("IMDS is unavailable")
	}

	region, err := imdsClient.GetRegion(ctx)
	return region, trace.Wrap(err)
}
