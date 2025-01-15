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

package oracle

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/oracle/oci-go-sdk/v65/common"

	"github.com/gravitational/teleport/api"
)

const teleportUserAgent = "teleport/" + api.Version

const (
	DateHeader      = "x-date"
	ChallengeHeader = "x-teleport-challenge"
)

const (
	TenancyClaim     = "opc-tenant"
	CompartmentClaim = "opc-compartment"
	InstanceClaim    = "opc-instance"
)

func FormatDateHeader(d time.Time) string {
	return d.UTC().Format(http.TimeFormat)
}

type authenticateClientDetails struct {
	RequestHeaders http.Header `json:"requestHeaders"`
}

type authenticateClientRequest struct {
	Date      string                    `contributesTo:"header" name:"x-date"`
	Challenge string                    `contributesTo:"header" name:"x-teleport-challenge"`
	UserAgent string                    `contributesTo:"header" name:"User-Agent"`
	Details   authenticateClientDetails `contributesTo:"body"`
}

type Claims struct {
	TenancyID     string
	CompartmentID string
	InstanceID    string
}

func (c Claims) Region() string {
	// OCID format: ocid1.<RESOURCE TYPE>.<REALM>.[REGION][.FUTURE USE].<UNIQUE ID>
	idParts := strings.Split(c.InstanceID, ".")
	switch len(idParts) {
	case 5, 6:
		return string(common.StringToRegion(idParts[3]))
	default:
		return ""
	}
}

type Claim struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Principal struct {
	Claims []Claim `json:"claims"`
}

func (p Principal) GetClaims() Claims {
	claims := Claims{}
	for _, claim := range p.Claims {
		switch claim.Key {
		case TenancyClaim:
			claims.TenancyID = claim.Value
		case CompartmentClaim:
			claims.CompartmentID = claim.Value
		case InstanceClaim:
			claims.InstanceID = claim.Value
		}
	}
	return claims
}

type AuthenticateClientResult struct {
	ErrorMessage string    `json:"errorMessage"`
	Principal    Principal `json:"principal"`
}

type AuthenticateClientResponse struct {
	AuthenticateClientResult `presentIn:"body"`
}

func newAuthenticateClientRequest(time time.Time, challenge string, headers http.Header) authenticateClientRequest {
	req := authenticateClientRequest{
		Date:      FormatDateHeader(time),
		Challenge: challenge,
		UserAgent: teleportUserAgent,
		Details: authenticateClientDetails{
			RequestHeaders: headers,
		},
	}
	if len(headers) == 0 {
		req.Details.RequestHeaders = http.Header{}
	}
	return req
}

func createAuthenticationRequest(endpoint string, auth authenticateClientRequest) (*http.Request, error) {
	req, err := common.MakeDefaultHTTPRequestWithTaggedStruct(
		http.MethodPost,
		"",
		auth,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	endpointURL.Path = "/v1/authentication/authenticateClient"
	req.Header.Set("Host", endpointURL.Host)
	req.Host = endpointURL.Host
	req.URL = endpointURL
	if len(auth.Details.RequestHeaders) == 0 {
		req.Header.Set("(request-target)", fmt.Sprintf("%s %s", strings.ToLower(req.Method), endpointURL.RequestURI()))
	}
	return &req, nil
}

// CreateSignedRequest creates a signed HTTP request to
// https://auth.<region>.oraclecloud.com/v1/authentication/authenticateClient.
// The returned headers should be sent to an auth server as part of
// RegisterUsingOracleMethod.
func CreateSignedRequest(provider common.ConfigurationProvider, challenge string) (innerHeaders, outerHeaders http.Header, err error) {
	signedHeaders := append(common.DefaultGenericHeaders(), DateHeader, ChallengeHeader)
	signer := common.RequestSigner(provider, signedHeaders, common.DefaultBodyHeaders())
	region, err := provider.Region()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	endpoint := fmt.Sprintf("https://auth.%s.oraclecloud.com", region)
	now := time.Now().UTC()
	innerReq, err := createAuthenticationRequest(endpoint, newAuthenticateClientRequest(now, challenge, nil))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	signer.Sign(innerReq)

	outerReq, err := createAuthenticationRequest(endpoint, newAuthenticateClientRequest(now, challenge, innerReq.Header))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	signer.Sign(outerReq)
	return innerReq.Header, outerReq.Header, nil
}

func GetAuthorizationHeaderValues(header http.Header) map[string]string {
	rawValues := strings.TrimPrefix(header.Get("Authorization"), "Signature ")
	keyValuePairs := strings.Split(rawValues, ",")
	values := make(map[string]string, len(keyValuePairs))
	for _, pair := range keyValuePairs {
		k, v, _ := strings.Cut(pair, "=")
		values[k] = strings.Trim(v, "\"")
	}
	return values
}

func CreateRequestFromHeaders(endpoint string, innerHeaders, outerHeaders http.Header) (*http.Request, error) {
	req, err := createAuthenticationRequest(endpoint, authenticateClientRequest{
		Date:      outerHeaders.Get(DateHeader),
		Challenge: outerHeaders.Get(ChallengeHeader),
		UserAgent: teleportUserAgent,
		Details: authenticateClientDetails{
			RequestHeaders: innerHeaders,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header = outerHeaders
	return req, nil
}
