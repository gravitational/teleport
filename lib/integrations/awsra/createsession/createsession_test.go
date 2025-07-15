/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package createsession

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestCreateSession(t *testing.T) {
	oneHourInSeconds := 60 * 60

	baseReq := func() CreateSessionRequest {
		return CreateSessionRequest{
			TrustAnchorARN:  "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
			ProfileARN:      "arn:aws:rolesanywhere:us-east-1:123456789012:profile/12345678-1234-1234-1234-123456789012",
			RoleARN:         "arn:aws:iam::123456789012:role/teleport-role",
			RoleSessionName: "teleport-session",
			DurationSeconds: &oneHourInSeconds,
			Certificate:     &x509.Certificate{},
			PrivateKey:      &ecdsa.PrivateKey{},
		}
	}

	t.Run("check defaults", func(t *testing.T) {
		for _, tt := range []struct {
			name     string
			req      func() *CreateSessionRequest
			errCheck require.ErrorAssertionFunc
		}{
			{
				name: "valid",
				req: func() *CreateSessionRequest {
					req := baseReq()
					return &req
				},
				errCheck: require.NoError,
			},
			{
				name: "invalid trust anchor",
				req: func() *CreateSessionRequest {
					req := baseReq()
					req.TrustAnchorARN = "invalid"
					return &req
				},
				errCheck: require.Error,
			},
			{
				name: "invalid profile",
				req: func() *CreateSessionRequest {
					req := baseReq()
					req.ProfileARN = "invalid"
					return &req
				},
				errCheck: require.Error,
			},
			{
				name: "invalid role",
				req: func() *CreateSessionRequest {
					req := baseReq()
					req.RoleARN = "invalid"
					return &req
				},
				errCheck: require.Error,
			},
			{
				name: "duration too long",
				req: func() *CreateSessionRequest {
					req := baseReq()
					oneDayInSeconds := 24 * 60 * 60
					req.DurationSeconds = &oneDayInSeconds
					return &req
				},
				errCheck: require.Error,
			},
			{
				name: "duration too short",
				req: func() *CreateSessionRequest {
					req := baseReq()
					oneMinute := 1 * 60
					req.DurationSeconds = &oneMinute
					return &req
				},
				errCheck: require.Error,
			},
			{
				name: "missing certificate",
				req: func() *CreateSessionRequest {
					req := baseReq()
					req.Certificate = nil
					return &req
				},
				errCheck: require.Error,
			},
			{
				name: "missing private key",
				req: func() *CreateSessionRequest {
					req := baseReq()
					req.PrivateKey = nil
					return &req
				},
				errCheck: require.Error,
			},
			{
				name: "invalid region in trust anchor",
				req: func() *CreateSessionRequest {
					req := baseReq()
					req.TrustAnchorARN = "arn:aws:rolesanywhere:invalid-region:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012"
					return &req
				},
				errCheck: require.Error,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				tt.errCheck(t, tt.req().checkAndSetDefaults())
			})
		}
	})

	t.Run("golden request", func(t *testing.T) {
		ctx := context.Background()
		clock := clockwork.NewFakeClockAt(time.Date(1984, time.April, 4, 0, 0, 0, 0, time.UTC))

		privateKey, x509Cert := newHardcodedPrivateKeyAndCert(t)

		httpClient := &fakeHTTPClient{
			resp: []byte(`{
  "credentialSet": [
    {
      "credentials": {
        "accessKeyId": "access key id",
        "expiration": "1984-04-04T01:00:00Z",
        "secretAccessKey": "access key",
        "sessionToken": "session token"
      }
    }
  ]
}`),
			statusCode: http.StatusCreated,
			requestChecker: func(r *http.Request) {
				expectedAuthorizationHeaderPrefix := "AWS4-X509-ECDSA-SHA256 Credential=12345/19840404/us-east-1/rolesanywhere/aws4_request, SignedHeaders=content-type;host;x-amz-date;x-amz-x509, Signature="
				expectedX509Header := "MIIBZzCCAQygAwIBAgICMDkwCgYIKoZIzj0EAwIwGTEXMBUGA1UEAxMObXktY29tbW9uLW5hbWUwIBgPMDAwMTAxMDEwMDAwMDBaFw04NDA0MDQwMDAxMDBaMBkxFzAVBgNVBAMTDm15LWNvbW1vbi1uYW1lMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEQ1RGW1ejV5FfT4Vi3TLdw9fp6OuWpxJTf5Su/PE23rq406tzEF172VOlIklAHv5CFactwvWPcQfx5FTN8/wFGaNCMEAwDgYDVR0PAQH/BAQDAgeAMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFLVszSqeFZx+PqHJrb6JvqEcGl0eMAoGCCqGSM49BAMCA0kAMEYCIQDFa3U+zI6l/V8b3IukoxZd5N+UN4k/ZohChsM7srfk+QIhAMDxA2u8I09u6qsVyHB0T47Bx56X4suEZR5+qTgR1JWu"
				expectedBody := `{"profileArn":"arn:aws:rolesanywhere:us-east-1:123456789012:profile/12345678-1234-1234-1234-123456789012","roleArn":"arn:aws:iam::123456789012:role/teleport-role","trustAnchorArn":"arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012","roleSessionName":"teleport-session","durationSeconds":3600}`

				bodyBytes, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				defer r.Body.Close()

				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/sessions", r.URL.Path)
				require.Equal(t, "rolesanywhere.us-east-1.amazonaws.com", r.Host)
				require.Equal(t, "application/json", r.Header.Get("Content-Type"))
				require.Equal(t, "19840404T000000Z", r.Header.Get("X-Amz-Date"))
				require.Equal(t, expectedX509Header, r.Header.Get("X-Amz-X509"))
				require.Contains(t, r.Header.Get("Authorization"), expectedAuthorizationHeaderPrefix)
				require.Equal(t, expectedBody, string(bodyBytes))

			},
		}

		req := baseReq()
		req.httpClient = httpClient
		req.clock = clock
		req.Certificate = x509Cert
		req.PrivateKey = privateKey

		resp, err := CreateSession(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, 1, resp.Version)
		require.Equal(t, "access key id", resp.AccessKeyID)
		require.Equal(t, "access key", resp.SecretAccessKey)
		require.Equal(t, "session token", resp.SessionToken)
	})
}

func newHardcodedPrivateKeyAndCert(t *testing.T) (*ecdsa.PrivateKey, *x509.Certificate) {
	privateKeyBytes, _ := pem.Decode([]byte(`-----BEGIN PRIVATE KEY-----
MHcCAQEEIKanMDk2rw+0MTczJZA+uUJBPqUDPjpmpdcvFR3ZE3SboAoGCCqGSM49
AwEHoUQDQgAEQ1RGW1ejV5FfT4Vi3TLdw9fp6OuWpxJTf5Su/PE23rq406tzEF17
2VOlIklAHv5CFactwvWPcQfx5FTN8/wFGQ==
-----END PRIVATE KEY-----`))
	privateKey, err := x509.ParseECPrivateKey(privateKeyBytes.Bytes)
	require.NoError(t, err)

	certBase64EncodedBytes := "MIIBZzCCAQygAwIBAgICMDkwCgYIKoZIzj0EAwIwGTEXMBUGA1UEAxMObXktY29tbW9uLW5hbWUwIBgPMDAwMTAxMDEwMDAwMDBaFw04NDA0MDQwMDAxMDBaMBkxFzAVBgNVBAMTDm15LWNvbW1vbi1uYW1lMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEQ1RGW1ejV5FfT4Vi3TLdw9fp6OuWpxJTf5Su/PE23rq406tzEF172VOlIklAHv5CFactwvWPcQfx5FTN8/wFGaNCMEAwDgYDVR0PAQH/BAQDAgeAMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFLVszSqeFZx+PqHJrb6JvqEcGl0eMAoGCCqGSM49BAMCA0kAMEYCIQDFa3U+zI6l/V8b3IukoxZd5N+UN4k/ZohChsM7srfk+QIhAMDxA2u8I09u6qsVyHB0T47Bx56X4suEZR5+qTgR1JWu"
	certBytes, err := base64.StdEncoding.DecodeString(certBase64EncodedBytes)
	require.NoError(t, err)

	x509Cert, err := x509.ParseCertificate(certBytes)
	require.NoError(t, err)

	return privateKey, x509Cert
}

type fakeHTTPClient struct {
	requestChecker func(*http.Request)
	resp           []byte
	statusCode     int
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// Simulate a successful response from the STS service
	resp := &http.Response{
		StatusCode: f.statusCode,
		Body:       io.NopCloser(bytes.NewReader(f.resp)),
	}

	if f.requestChecker != nil {
		f.requestChecker(req)
	}
	return resp, nil
}
