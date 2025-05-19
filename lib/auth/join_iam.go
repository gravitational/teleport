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

package auth

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/join/iam"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/aws"
)

const (
	// Hardcoding the sts API version here may be more strict than necessary,
	// but this is set by the Teleport node and can only be changed when we
	// update our AWS SDK dependency. Since Auth should always be upgraded
	// before nodes, we will have a chance to update the check on Auth if we
	// ever have a need to allow a newer API version.
	expectedSTSIdentityRequestBody = "Action=GetCallerIdentity&Version=2011-06-15"

	// AWS SignedHeaders will always be lowercase
	// https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-auth-using-authorization-header.html#sigv4-auth-header-overview
	challengeHeaderKey = "x-teleport-challenge"
)

// validateSTSHost returns an error if the given stsHost is not a valid regional
// endpoint for the AWS STS service, or nil if it is valid. If fips is true, the
// endpoint must be a valid FIPS endpoint.
//
// This is a security-critical check: we are allowing the client to tell us
// which URL we should use to validate their identity. If the client could pass
// off an attacker-controlled URL as the STS endpoint, the entire security
// mechanism of the IAM join method would be compromised.
//
// To keep this validation simple and secure, we check the given endpoint
// against a static list of known valid endpoints. We will need to update this
// list as AWS adds new regions.
func validateSTSHost(stsHost string, cfg *iamRegisterConfig) error {
	valid := slices.Contains(iam.ValidSTSEndpoints(), stsHost)
	if !valid {
		return trace.AccessDenied("IAM join request uses unknown STS host %q. "+
			"This could mean that the Teleport Node attempting to join the cluster is "+
			"running in a new AWS region which is unknown to this Teleport auth server. "+
			"Alternatively, if this URL looks suspicious, an attacker may be attempting to "+
			"join your Teleport cluster. "+
			"Following is the list of valid STS endpoints known to this auth server. "+
			"If a legitimate STS endpoint is not included, please file an issue at "+
			"https://github.com/gravitational/teleport. %v",
			stsHost, iam.ValidSTSEndpoints())
	}

	if cfg.fips && !slices.Contains(iam.FIPSSTSEndpoints(), stsHost) {
		return trace.AccessDenied("node selected non-FIPS STS endpoint (%s) for the IAM join method", stsHost)
	}

	return nil
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
func validateSTSIdentityRequest(req *http.Request, challenge string, cfg *iamRegisterConfig) (err error) {
	defer func() {
		// Always log a warning on the Auth server if the function detects an
		// invalid sts:GetCallerIdentity request, it's either going to be caused
		// by a node in a unknown region or an attacker.
		if err != nil {
			logger.WarnContext(req.Context(), "Detected an invalid sts:GetCallerIdentity used by a client attempting to use the IAM join method", "error", err)
		}
	}()

	if err := validateSTSHost(req.Host, cfg); err != nil {
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
	if !slices.Contains(sigV4.SignedHeaders, challengeHeaderKey) {
		return trace.AccessDenied("sts identity request auth header %q does not include "+
			challengeHeaderKey+" as a signed header", authHeader)
	}

	body, err := utils.GetAndReplaceRequestBody(req)
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

// JoinAttrs returns the protobuf representation of the attested identity.
// This is used for auditing and for evaluation of WorkloadIdentity rules and
// templating.
func (c *awsIdentity) JoinAttrs() *workloadidentityv1pb.JoinAttrsAWSIAM {
	attrs := &workloadidentityv1pb.JoinAttrsAWSIAM{
		Account: c.Account,
		Arn:     c.Arn,
	}

	return attrs
}

// getCallerIdentityReponse is used for JSON parsing
type getCallerIdentityResponse struct {
	GetCallerIdentityResult awsIdentity `json:"GetCallerIdentityResult"`
}

// stsIdentityResponse is used for JSON parsing
type stsIdentityResponse struct {
	GetCallerIdentityResponse getCallerIdentityResponse `json:"GetCallerIdentityResponse"`
}

// executeSTSIdentityRequest sends the sts:GetCallerIdentity HTTP request to the
// AWS API, parses the response, and returns the awsIdentity
func executeSTSIdentityRequest(ctx context.Context, client utils.HTTPDoClient, req *http.Request) (*awsIdentity, error) {
	if client == nil {
		client = http.DefaultClient
	}

	// set the http request context so it can be canceled
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
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
	return globMatch(pattern, arn)
}

// checkIAMAllowRules checks if the given identity matches any of the given
// allowRules.
func checkIAMAllowRules(identity *awsIdentity, token string, allowRules []*types.TokenRule) error {
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
	return trace.AccessDenied("instance %v did not match any allow rules in token %v", identity.Arn, token)
}

// checkIAMRequest checks if the given request satisfies the token rules and
// included the required challenge.
//
// If the joining entity presents a valid IAM identity, this will be returned,
// even if the identity does not match the token's allow rules. This is to
// support inclusion in audit logs.
func (a *Server) checkIAMRequest(ctx context.Context, challenge string, req *proto.RegisterUsingIAMMethodRequest, cfg *iamRegisterConfig) (*awsIdentity, error) {
	tokenName := req.RegisterUsingTokenRequest.Token
	provisionToken, err := a.GetToken(ctx, tokenName)
	if err != nil {
		return nil, trace.Wrap(err, "getting token")
	}
	if provisionToken.GetJoinMethod() != types.JoinMethodIAM {
		return nil, trace.AccessDenied("this token does not support the IAM join method")
	}

	// parse the incoming http request to the sts:GetCallerIdentity endpoint
	identityRequest, err := parseSTSRequest(req.StsIdentityRequest)
	if err != nil {
		return nil, trace.Wrap(err, "parsing STS request")
	}

	// validate that the host, method, and headers are correct and the expected
	// challenge is included in the signed portion of the request
	if err := validateSTSIdentityRequest(identityRequest, challenge, cfg); err != nil {
		return nil, trace.Wrap(err, "validating STS request")
	}

	// send the signed request to the public AWS API and get the node identity
	// from the response
	identity, err := executeSTSIdentityRequest(ctx, a.httpClientForAWSSTS, identityRequest)
	if err != nil {
		return nil, trace.Wrap(err, "executing STS request")
	}

	// check that the node identity matches an allow rule for this token
	if err := checkIAMAllowRules(identity, provisionToken.GetName(), provisionToken.GetAllowRules()); err != nil {
		// We return the identity since it's "validated" but does not match the
		// rules. This allows us to include it in a failed join audit event
		// as additional context to help the user understand why the join failed.
		return identity, trace.Wrap(err, "checking allow rules")
	}

	return identity, nil
}

func generateIAMChallenge() (string, error) {
	challenge, err := generateChallenge(base64.RawStdEncoding, 32)
	return challenge, trace.Wrap(err)
}

type iamRegisterConfig struct {
	authVersion *semver.Version
	fips        bool
}

func defaultIAMRegisterConfig(fips bool) *iamRegisterConfig {
	return &iamRegisterConfig{
		authVersion: teleport.SemVer(),
		fips:        fips,
	}
}

type iamRegisterOption func(cfg *iamRegisterConfig)

func withAuthVersion(v *semver.Version) iamRegisterOption {
	return func(cfg *iamRegisterConfig) {
		cfg.authVersion = v
	}
}

func withFips(fips bool) iamRegisterOption {
	return func(cfg *iamRegisterConfig) {
		cfg.fips = fips
	}
}

// RegisterUsingIAMMethodWithOpts registers the caller using the IAM join method and
// returns signed certs to join the cluster.
//
// The caller must provide a ChallengeResponseFunc which returns a
// *types.RegisterUsingTokenRequest with a signed sts:GetCallerIdentity request
// including the challenge as a signed header.
func (a *Server) RegisterUsingIAMMethodWithOpts(
	ctx context.Context,
	challengeResponse client.RegisterIAMChallengeResponseFunc,
	opts ...iamRegisterOption,
) (certs *proto.Certs, err error) {
	var provisionToken types.ProvisionToken
	var joinRequest *types.RegisterUsingTokenRequest
	var joinFailureMetadata any
	defer func() {
		// Emit a log message and audit event on join failure.
		if err != nil {
			a.handleJoinFailure(
				ctx, err, provisionToken, joinFailureMetadata, joinRequest,
			)
		}
	}()

	cfg := defaultIAMRegisterConfig(a.fips)
	for _, opt := range opts {
		opt(cfg)
	}

	challenge, err := generateIAMChallenge()
	if err != nil {
		return nil, trace.Wrap(err, "generating IAM challenge")
	}

	req, err := challengeResponse(challenge)
	if err != nil {
		return nil, trace.Wrap(err, "getting challenge response")
	}
	joinRequest = req.RegisterUsingTokenRequest

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "validating request parameters")
	}

	// perform common token checks
	provisionToken, err = a.checkTokenJoinRequestCommon(ctx, req.RegisterUsingTokenRequest)
	if err != nil {
		return nil, trace.Wrap(err, "completing common token checks")
	}

	// check that the GetCallerIdentity request is valid and matches the token
	verifiedIdentity, err := a.checkIAMRequest(ctx, challenge, req, cfg)
	if verifiedIdentity != nil {
		joinFailureMetadata = verifiedIdentity
	}
	if err != nil {
		return nil, trace.Wrap(err, "checking iam request")
	}

	if req.RegisterUsingTokenRequest.Role == types.RoleBot {
		certs, _, err := a.generateCertsBot(
			ctx,
			provisionToken,
			req.RegisterUsingTokenRequest,
			verifiedIdentity,
			&workloadidentityv1pb.JoinAttrs{
				Iam: verifiedIdentity.JoinAttrs(),
			},
		)
		return certs, trace.Wrap(err, "generating bot certs")
	}
	certs, err = a.generateCerts(ctx, provisionToken, req.RegisterUsingTokenRequest, verifiedIdentity)
	return certs, trace.Wrap(err, "generating certs")
}

// RegisterUsingIAMMethod registers the caller using the IAM join method and
// returns signed certs to join the cluster.
//
// The caller must provide a ChallengeResponseFunc which returns a
// *types.RegisterUsingTokenRequest with a signed sts:GetCallerIdentity request
// including the challenge as a signed header.
func (a *Server) RegisterUsingIAMMethod(
	ctx context.Context,
	challengeResponse client.RegisterIAMChallengeResponseFunc,
) (certs *proto.Certs, err error) {
	return a.RegisterUsingIAMMethodWithOpts(ctx, challengeResponse)
}
