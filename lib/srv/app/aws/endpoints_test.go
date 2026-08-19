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

package aws

import (
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/stretchr/testify/require"

	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

func TestResolveEndpoints(t *testing.T) {
	creds := aws.Credentials{AccessKeyID: "fakeClientKeyID", SecretAccessKey: "fakeClientSecret"}
	signer := v4.NewSigner()
	region := "us-east-1"
	now := time.Now()

	t.Run("unsupported SDK resolver", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost", nil)
		require.NoError(t, err)

		err = signer.SignHTTP(t.Context(), creds, req, awsutils.EmptyPayloadHash, "ecr", "us-east-1", now)
		require.NoError(t, err)

		_, err = resolveEndpoint(req, awsutils.AuthorizationHeader)
		require.Error(t, err)
	})

	t.Run("X-Forwarded-Host", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost", nil)
		require.NoError(t, err)
		req.Header.Set("X-Forwarded-Host", "some-service.us-east-1.amazonaws.com")

		err = signer.SignHTTP(t.Context(), creds, req, awsutils.EmptyPayloadHash, "some-service", region, now)
		require.NoError(t, err)

		endpoint, err := resolveEndpoint(req, awsutils.AuthorizationHeader)
		require.NoError(t, err)
		require.Equal(t, "some-service", endpoint.SigningName)
		require.Equal(t, "https://some-service.us-east-1.amazonaws.com", endpoint.URL)
	})

	// Reject X-Forwarded-Host values that try to exploit a parser differential
	// between endpoint validation and the URL used to dial the upstream. These
	// must not resolve to an attacker-controlled host. See the AWS app signer
	// SSRF finding (X-Forwarded-Host validated as *.amazonaws.com but dialed as
	// an attacker host via a url.Parse scheme differential).
	t.Run("rejects host/validation parser differentials", func(t *testing.T) {
		for _, forwardedHost := range []string{
			// scheme differential: parses as scheme="attacker.example.com",
			// host="s3.amazonaws.com" for the validator but dials attacker host.
			"attacker.example.com://s3.amazonaws.com",
			// same trick with an explicit port on the attacker host.
			"attacker.example.com:8080://s3.amazonaws.com",
			// userinfo spoof: real host is attacker.example.com.
			"s3.amazonaws.com@attacker.example.com",
			// trailing path / query / fragment after a valid-looking host.
			"s3.amazonaws.com/../@attacker.example.com",
			"s3.amazonaws.com#@attacker.example.com",
			// disallowed port.
			"s3.amazonaws.com:8080",
			// not an AWS endpoint at all.
			"attacker.example.com",
		} {
			t.Run(forwardedHost, func(t *testing.T) {
				req, err := http.NewRequest("GET", "http://localhost", nil)
				require.NoError(t, err)
				req.Header.Set("X-Forwarded-Host", forwardedHost)

				err = signer.SignHTTP(t.Context(), creds, req, awsutils.EmptyPayloadHash, "s3", region, now)
				require.NoError(t, err)

				_, err = resolveEndpoint(req, awsutils.AuthorizationHeader)
				require.Error(t, err)
			})
		}
	})

	// For every X-Forwarded-Host that resolveEndpoint accepts, the host that
	// urlForResolvedEndpoint subsequently dials must be the same validated
	// *.amazonaws.com host. The validate path and the forward path can never
	// diverge.
	t.Run("accepted endpoint dials the validated host", func(t *testing.T) {
		for _, tt := range []struct {
			forwardedHost string
			wantHost      string
		}{
			{
				forwardedHost: "s3.amazonaws.com",
				wantHost:      "s3.amazonaws.com",
			},
			{
				forwardedHost: "some-service.us-east-1.amazonaws.com",
				wantHost:      "some-service.us-east-1.amazonaws.com",
			},
			{
				forwardedHost: "some-service.us-east-1.amazonaws.com:443",
				wantHost:      "some-service.us-east-1.amazonaws.com:443",
			},
			{
				forwardedHost: "example.amazonaws.com.cn",
				wantHost:      "example.amazonaws.com.cn",
			},
			{
				forwardedHost: "aws-mcp.us-east-1.api.aws",
				wantHost:      "aws-mcp.us-east-1.api.aws",
			},
		} {
			t.Run(tt.forwardedHost, func(t *testing.T) {
				req, err := http.NewRequest("GET", "http://localhost/some/path?q=1", nil)
				require.NoError(t, err)
				req.Header.Set("X-Forwarded-Host", tt.forwardedHost)

				err = signer.SignHTTP(t.Context(), creds, req, awsutils.EmptyPayloadHash, "s3", region, now)
				require.NoError(t, err)

				endpoint, err := resolveEndpoint(req, awsutils.AuthorizationHeader)
				require.NoError(t, err)

				dialURL, err := urlForResolvedEndpoint(req, endpoint)
				require.NoError(t, err)
				require.Equal(t, tt.wantHost, dialURL.Host)
				require.Equal(t, "https", dialURL.Scheme)
			})
		}
	})
}
