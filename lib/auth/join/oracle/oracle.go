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
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/oracle/oci-go-sdk/v65/common"

	"github.com/gravitational/teleport/api"
)

const teleportUserAgent = "teleport/" + api.Version

const (
	dateHeader      = "x-date"
	challengeHeader = "x-teleport-challenge"
)

const (
	tenancyClaim     = "opc-tenancy-id"
	compartmentClaim = "opc-compartment-id"
	cnstanceClaim    = "opc-instance-id"
)

func formatDateHeader(d time.Time) string {
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

func newAuthenticateClientRequest(time time.Time, challenge string, headers http.Header) authenticateClientRequest {
	req := authenticateClientRequest{
		Date:      formatDateHeader(time),
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

func createAuthenticationRequest(region string, auth authenticateClientRequest) (*http.Request, error) {
	req, err := common.MakeDefaultHTTPRequestWithTaggedStruct(
		http.MethodPost,
		"/v1/authentication/authenticateClient",
		auth,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.URL.Host = fmt.Sprintf("https://auth.%s.oraclecloud.com", region)
	return &req, nil
}

// CreateSignedRequest creates a signed HTTP request to
// https://auth.<region>.oraclecloud.com/v1/authentication/authenticateClient.
// The returned headers should be sent to an auth server as part of
// RegisterUsingOracleMethod.
func CreateSignedRequest(provider common.ConfigurationProvider, challenge string) (innerHeaders, outerHeaders http.Header, err error) {
	signedHeaders := append(common.DefaultGenericHeaders(), dateHeader, challengeHeader)
	signer := common.RequestSigner(provider, signedHeaders, common.DefaultBodyHeaders())
	region, err := provider.Region()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	now := time.Now().UTC()
	innerReq, err := createAuthenticationRequest(region, newAuthenticateClientRequest(now, challenge, nil))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	signer.Sign(innerReq)

	outerReq, err := createAuthenticationRequest(region, newAuthenticateClientRequest(now, challenge, innerReq.Header))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	signer.Sign(outerReq)
	return innerReq.Header, outerReq.Header, nil
}

func getAuthorizationHeaderValues(header http.Header) map[string]string {
	rawValues := strings.TrimPrefix(header.Get("Authorization"), "Signature ")
	keyValuePairs := strings.Split(rawValues, ",")
	values := make(map[string]string, len(keyValuePairs))
	for _, pair := range keyValuePairs {
		k, v, _ := strings.Cut(pair, "=")
		values[k] = strings.Trim(v, "\"")
	}
	return values
}
