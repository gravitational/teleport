// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package web

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuthExport(t *testing.T) {
	env := newWebPack(t, 1)
	clusterName := env.server.ClusterName()

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test-user@example.com", nil)

	validateTLSCertificateDERFunc := func(t *testing.T, b []byte) {
		cert, err := x509.ParseCertificate(b)
		require.NoError(t, err)
		require.NotNil(t, cert, "ParseCertificate failed")
		require.Equal(t, "localhost", cert.Subject.CommonName, "unexpected certificate subject CN")
	}

	validateTLSCertificatePEMFunc := func(t *testing.T, b []byte) {
		pemBlock, _ := pem.Decode(b)
		require.NotNil(t, pemBlock, "pem.Decode failed")

		validateTLSCertificateDERFunc(t, pemBlock.Bytes)
	}

	for _, tt := range []struct {
		name           string
		authType       string
		expectedStatus int
		assertBody     func(t *testing.T, bs []byte)
	}{
		{
			name:           "all",
			authType:       "",
			expectedStatus: http.StatusOK,
			assertBody: func(t *testing.T, b []byte) {
				require.Contains(t, string(b), "@cert-authority localhost,*.localhost ecdsa-sha2-nistp256 ")
				require.Contains(t, string(b), "cert-authority ecdsa-sha2-nistp256")
			},
		},
		{
			name:           "host",
			authType:       "host",
			expectedStatus: http.StatusOK,
			assertBody: func(t *testing.T, b []byte) {
				require.Contains(t, string(b), "@cert-authority localhost,*.localhost ecdsa-sha2-nistp256 ")
			},
		},
		{
			name:           "user",
			authType:       "user",
			expectedStatus: http.StatusOK,
			assertBody: func(t *testing.T, b []byte) {
				require.Contains(t, string(b), "cert-authority ecdsa-sha2-nistp256")
			},
		},
		{
			name:           "windows",
			authType:       "windows",
			expectedStatus: http.StatusOK,
			assertBody:     validateTLSCertificateDERFunc,
		},
		{
			name:           "db",
			authType:       "db",
			expectedStatus: http.StatusOK,
			assertBody:     validateTLSCertificatePEMFunc,
		},
		{
			name:           "db-der",
			authType:       "db-der",
			expectedStatus: http.StatusOK,
			assertBody:     validateTLSCertificateDERFunc,
		},
		{
			name:           "db-client",
			authType:       "db-client",
			expectedStatus: http.StatusOK,
			assertBody:     validateTLSCertificatePEMFunc,
		},
		{
			name:           "db-client-der",
			authType:       "db-client-der",
			expectedStatus: http.StatusOK,
			assertBody:     validateTLSCertificateDERFunc,
		},
		{
			name:           "tls",
			authType:       "tls",
			expectedStatus: http.StatusOK,
			assertBody:     validateTLSCertificatePEMFunc,
		},
		{
			name:           "invalid",
			authType:       "invalid",
			expectedStatus: http.StatusBadRequest,
			assertBody: func(t *testing.T, b []byte) {
				require.Contains(t, string(b), `"invalid" authority type is not supported`)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// export host certificate
			t.Run("deprecated endpoint", func(t *testing.T) {
				endpointExport := pack.clt.Endpoint("webapi", "sites", clusterName, "auth", "export")
				authExportTestByEndpoint(t, endpointExport, tt.authType, tt.expectedStatus, tt.assertBody)
			})
			t.Run("new endpoint", func(t *testing.T) {
				endpointExport := pack.clt.Endpoint("webapi", "auth", "export")
				authExportTestByEndpoint(t, endpointExport, tt.authType, tt.expectedStatus, tt.assertBody)
			})
		})
	}
}

func authExportTestByEndpoint(t *testing.T, endpointExport, authType string, expectedStatus int, assertBody func(t *testing.T, bs []byte)) {
	ctx := context.Background()

	if authType != "" {
		endpointExport = fmt.Sprintf("%s?type=%s", endpointExport, authType)
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpointExport, nil)
	require.NoError(t, err)

	anonHTTPClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := anonHTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	bs, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, expectedStatus, resp.StatusCode, "invalid status code with body %s", string(bs))

	require.NotEmpty(t, bs, "unexpected empty body from http response")
	if assertBody != nil {
		assertBody(t, bs)
	}
}
