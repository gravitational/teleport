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
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/oracle/oci-go-sdk/v65/common"

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
		return trace.BadParameter("missing header X-Date")
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
			return string(common.StringToRegion(region)) == claims.Region()
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

func (a *Server) checkOracleRequest(ctx context.Context, challenge string, req *proto.RegisterUsingOracleMethodRequest, endpoint string) (*oracle.Claims, error) {
	tokenName := req.RegisterUsingTokenRequest.Token
	provisionToken, err := a.GetToken(ctx, tokenName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if provisionToken.GetJoinMethod() != types.JoinMethodOracle {
		return nil, trace.AccessDenied("this token does not support the Oracle join method")
	}

	outerHeaders := formatHeaderFromMap(req.Headers)
	innerHeaders := formatHeaderFromMap(req.InnerHeaders)
	if err := checkHeaders(outerHeaders, challenge, a.GetClock()); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := checkHeaders(innerHeaders, challenge, a.GetClock()); err != nil {
		return nil, trace.Wrap(err)
	}

	// Check region.
	host := outerHeaders.Get("host")
	hostParts := strings.Split(host, ".")
	if len(hostParts) != 4 {
		return nil, trace.BadParameter("unexpected host: %v", host)
	}
	region := string(common.StringToRegion(hostParts[1]))
	if region == "" {
		return nil, trace.BadParameter("invalid region: %v", region)
	}
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://auth.%s.oraclecloud.com", region)
	}

	authReq, err := oracle.CreateRequestFromHeaders(endpoint, innerHeaders, outerHeaders)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	claims, err := oracle.FetchOraclePrincipalClaims(ctx, authReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, ok := provisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("oracle join method only supports ProvisionTokenV2")
	}
	if err := checkOracleAllowRules(claims, provisionToken.GetName(), token.Spec.Oracle.Allow); err != nil {
		return nil, trace.Wrap(err)
	}

	return &claims, nil
}

// RegisterUsingOracleMethod registers the caller using the Oracle join method and
// returns signed certs to join the cluster.
func (a *Server) RegisterUsingOracleMethod(
	ctx context.Context,
	challengeResponse client.RegisterOracleChallengeResponseFunc,
) (certs *proto.Certs, err error) {
	certs, err = a.registerUsingOracleMethod(ctx, challengeResponse, "" /* default endpoint */)
	return certs, trace.Wrap(err)
}

func (a *Server) registerUsingOracleMethod(
	ctx context.Context,
	challengeResponse client.RegisterOracleChallengeResponseFunc,
	endpoint string,
) (certs *proto.Certs, err error) {
	var provisionToken types.ProvisionToken
	var joinRequest *types.RegisterUsingTokenRequest
	defer func() {
		// Emit a log message and audit event on join failure.
		if err != nil {
			a.handleJoinFailure(
				ctx, err, provisionToken, nil, joinRequest,
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
	claims, err := a.checkOracleRequest(ctx, challenge, req, endpoint)
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
