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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/fixtures"
)

func TestCreateSignedRequest(t *testing.T) {
	t.Parallel()

	pemBytes, ok := fixtures.PEMBytes["rsa"]
	require.True(t, ok)

	provider := common.NewRawConfigurationProvider(
		"ocid1.tenancy.oc1..abcd1234",
		"ocid1.user.oc1..abcd1234",
		"us-ashburn-1",
		"fingerprint",
		string(pemBytes),
		nil,
	)

	innerHeader, outerHeader, err := CreateSignedRequest(provider, "challenge")
	require.NoError(t, err)

	expectedHeaders := map[string]string{
		"Accept":           "*/*",
		"Authorization":    "",
		"Content-Length":   "",
		"Content-Type":     "application/json",
		"Host":             "auth.us-ashburn-1.oraclecloud.com",
		"User-Agent":       teleportUserAgent,
		"X-Content-Sha256": "",
		DateHeader:         "",
		ChallengeHeader:    "challenge",
	}
	expectedAuthHeader := map[string]string{
		"version":   "1",
		"headers":   "date (request-target) host x-date x-teleport-challenge content-length content-type x-content-sha256",
		"keyId":     "ocid1.tenancy.oc1..abcd1234/ocid1.user.oc1..abcd1234/fingerprint",
		"algorithm": "rsa-sha256",
		"signature": "",
	}

	for _, header := range []http.Header{innerHeader, outerHeader} {
		for k, v := range expectedHeaders {
			if v == "" {
				assert.NotEmpty(t, header.Get(k), "missing header: %s", k)
			} else {
				assert.Equal(t, v, header.Get(k))
			}
		}
		authValues := GetAuthorizationHeaderValues(header)
		for k, v := range expectedAuthHeader {
			if v == "" {
				assert.NotEmpty(t, authValues[k], "missing auth header value: %s", k)
			} else {
				assert.Equal(t, v, authValues[k])
			}
		}
	}
}

func TestFetchOraclePrincipalClaims(t *testing.T) {
	t.Parallel()

	defaultTenancyID := "tenancy-id"
	defaultCompartmentID := "compartment-id"
	defaultInstanceID := "instance-id"

	defaultHandle := func(code int, responseBody any) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			body, err := json.Marshal(responseBody)
			assert.NoError(t, err)
			_, err = w.Write(body)
			assert.NoError(t, err)
		})
	}

	tests := []struct {
		name           string
		handler        http.Handler
		assert         assert.ErrorAssertionFunc
		expectedClaims Claims
	}{
		{
			name: "ok",
			handler: defaultHandle(http.StatusOK, AuthenticateClientResult{
				Principal: Principal{
					Claims: []Claim{
						{
							Key:   TenancyClaim,
							Value: defaultTenancyID,
						},
						{
							Key:   CompartmentClaim,
							Value: defaultCompartmentID,
						},
						{
							Key:   InstanceClaim,
							Value: defaultInstanceID,
						},
					},
				},
			}),
			assert: assert.NoError,
			expectedClaims: Claims{
				TenancyID:     defaultTenancyID,
				CompartmentID: defaultCompartmentID,
				InstanceID:    defaultInstanceID,
			},
		},
		{
			name: "block redirect",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.NotEqual(t, "/dontgohere", r.RequestURI, "redirect was incorrectly performed")
				http.Redirect(w, r, "/dontgohere", http.StatusFound)
			}),
			assert: assert.Error,
		},
		{
			name:    "http error",
			handler: defaultHandle(http.StatusNotFound, AuthenticateClientResult{}),
			assert:  assert.Error,
		},
		{
			name: "api error",
			handler: defaultHandle(http.StatusOK, AuthenticateClientResult{
				ErrorMessage: "it didn't work",
			}),
			assert: assert.Error,
		},
		{
			name: "invalid response type",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("some junk"))
			}),
			assert: assert.Error,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			t.Cleanup(srv.Close)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			req, err := http.NewRequest("", srv.URL, nil)
			require.NoError(t, err)
			claims, err := FetchOraclePrincipalClaims(ctx, req)
			tc.assert(t, err)
			assert.Equal(t, tc.expectedClaims, claims)
		})
	}
}
