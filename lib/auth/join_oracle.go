// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package auth

import (
	"context"
	"encoding/base64"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/join/oracle"
)

func generateOracleChallenge() (string, error) {
	challenge, err := generateChallenge(base64.StdEncoding, 32)
	return challenge, trace.Wrap(err)
}

func checkHeaders(headers http.Header, challenge string, clock clockwork.Clock) error {
	// Check that required headers are signed.
	authHeader := oracle.GetAuthorizationHeaderValues(headers)
	rawSignedHeaders, ok := authHeader["headers"]
	if !ok {
		return trace.BadParameter("missing list of signed headers")
	}
	signedHeaders := strings.Split(rawSignedHeaders, " ")
	if !slices.Contains(signedHeaders, oracle.DateHeader) {
		return trace.BadParameter("header %s is not signed", oracle.DateHeader)
	}
	if !slices.Contains(signedHeaders, oracle.ChallengeHeader) {
		return trace.BadParameter("header %s is not signed", oracle.ChallengeHeader)
	}

	// Check X-Date.
	now := clock.Now().UTC()
	rawHeaderDate := headers.Get(oracle.DateHeader)
	if rawHeaderDate == "" {
		return trace.BadParameter("missing header %v", oracle.DateHeader)
	}
	headerDate, err := time.Parse(http.TimeFormat, rawHeaderDate)
	if err != nil {
		return trace.Wrap(err)
	}
	var timeDelta time.Duration
	if headerDate.After(now) {
		timeDelta = headerDate.Sub(now)
	} else {
		timeDelta = now.Sub(headerDate)
	}
	if timeDelta > 5*time.Minute {
		return trace.BadParameter("header time is more than 5 minutes from now")
	}

	// Check challenge.
	if headers.Get(oracle.ChallengeHeader) != challenge {
		return trace.BadParameter("challenge does not match")
	}

	return nil
}

func checkOracleAllowRules(claims oracle.Claims, token string, allowRules []*types.ProvisionTokenSpecV2Oracle_Rule) error {
	for _, rule := range allowRules {
		if rule.Tenancy != claims.TenancyID {
			continue
		}
		if len(rule.ParentCompartments) != 0 && !slices.Contains(rule.ParentCompartments, claims.CompartmentID) {
			continue
		}
		if len(rule.Regions) != 0 && !slices.ContainsFunc(rule.Regions, func(region string) bool {
			return string(oracle.ParseRegion(region)) == claims.Region()
		}) {
			continue
		}
		return nil
	}
	return trace.AccessDenied("instance %v did not match any allow rules in token %v", claims.InstanceID, token)
}

func formatHeaderFromMap(m map[string]string) http.Header {
	header := make(http.Header, len(m))
	for k, v := range m {
		header.Set(k, v)
	}
	return header
}

func getRegionFromHost(host string) (string, error) {
	regionEndpoint, foundAuth := strings.CutPrefix(host, "auth.")
	rawRegion, foundOracleCloud := strings.CutSuffix(regionEndpoint, ".oraclecloud.com")
	if !foundAuth || !foundOracleCloud {
		return "", trace.BadParameter("expected host auth.{region}.oraclecloud.com, got %s", host)
	}
	region := oracle.ParseRegion(rawRegion)
	if region == "" {
		return "", trace.BadParameter("missing or invalid region")
	}
	return region, nil
}

func (a *Server) checkOracleRequest(
	ctx context.Context,
	challenge string,
	req *proto.RegisterUsingOracleMethodRequest,
	fetchClaims oracleClaimsFetcher,
) (oracle.Claims, error) {
	// Check shared token parameters.
	tokenName := req.RegisterUsingTokenRequest.Token
	provisionToken, err := a.GetToken(ctx, tokenName)
	if err != nil {
		return oracle.Claims{}, trace.Wrap(err)
	}
	if provisionToken.GetJoinMethod() != types.JoinMethodOracle {
		return oracle.Claims{}, trace.AccessDenied("this token does not support the Oracle join method")
	}

	// Check signed headers.
	outerHeaders := formatHeaderFromMap(req.Headers)
	innerHeaders := formatHeaderFromMap(req.PayloadHeaders)
	if err := checkHeaders(outerHeaders, challenge, a.GetClock()); err != nil {
		return oracle.Claims{}, trace.Wrap(err)
	}
	if err := checkHeaders(innerHeaders, challenge, a.GetClock()); err != nil {
		return oracle.Claims{}, trace.Wrap(err)
	}

	// Authenticate request with Oracle.
	region, err := getRegionFromHost(outerHeaders.Get("host"))
	if err != nil {
		return oracle.Claims{}, trace.Wrap(err)
	}
	authReq, err := oracle.CreateRequestFromHeaders(region, innerHeaders, outerHeaders)
	if err != nil {
		return oracle.Claims{}, trace.Wrap(err)
	}
	claims, err := fetchClaims(ctx, authReq)
	if err != nil {
		return oracle.Claims{}, trace.Wrap(err)
	}

	// Check allow rules.
	token, ok := provisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return oracle.Claims{}, trace.BadParameter("oracle join method only supports ProvisionTokenV2")
	}
	if err := checkOracleAllowRules(claims, provisionToken.GetName(), token.Spec.Oracle.Allow); err != nil {
		return oracle.Claims{}, trace.Wrap(err)
	}

	return claims, nil
}

// RegisterUsingOracleMethod registers the caller using the Oracle join method and
// returns signed certs to join the cluster.
func (a *Server) RegisterUsingOracleMethod(
	ctx context.Context,
	challengeResponse client.RegisterOracleChallengeResponseFunc,
) (certs *proto.Certs, err error) {
	certs, err = a.registerUsingOracleMethod(ctx, challengeResponse, oracle.FetchOraclePrincipalClaims)
	return certs, trace.Wrap(err)
}

type oracleClaimsFetcher func(ctx context.Context, req *http.Request) (oracle.Claims, error)

func (a *Server) registerUsingOracleMethod(
	ctx context.Context,
	challengeResponse client.RegisterOracleChallengeResponseFunc,
	fetchClaims oracleClaimsFetcher,
) (certs *proto.Certs, err error) {
	var provisionToken types.ProvisionToken
	var joinRequest *types.RegisterUsingTokenRequest
	var claims oracle.Claims
	defer func() {
		// Emit a log message and audit event on join failure.
		if err != nil {
			a.handleJoinFailure(
				ctx, err, provisionToken, claims, joinRequest,
			)
		}
	}()

	challenge, err := generateOracleChallenge()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req, err := challengeResponse(challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	joinRequest = req.RegisterUsingTokenRequest
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	provisionToken, err = a.checkTokenJoinRequestCommon(ctx, req.RegisterUsingTokenRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	claims, err = a.checkOracleRequest(ctx, challenge, req, fetchClaims)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.RegisterUsingTokenRequest.Role == types.RoleBot {
		certs, err := a.generateCertsBot(
			ctx,
			provisionToken,
			req.RegisterUsingTokenRequest,
			claims,
			nil,
		)
		return certs, trace.Wrap(err)
	}
	certs, err = a.generateCerts(
		ctx,
		provisionToken,
		req.RegisterUsingTokenRequest,
		claims,
	)
	return certs, trace.Wrap(err)
}
