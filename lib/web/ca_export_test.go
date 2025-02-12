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
	"archive/zip"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthExport(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	clusterName := env.server.ClusterName()

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test-user@example.com", nil)

	validateTLSCertificateDERFunc := func(t *testing.T, b []byte) {
		cert, err := x509.ParseCertificate(b)
		require.NoError(t, err)
		require.Equal(t, "localhost", cert.Subject.CommonName, "unexpected certificate subject CN")
	}

	validateTLSCertificatePEMFunc := func(t *testing.T, b []byte) {
		pemBlock, _ := pem.Decode(b)
		require.NotNil(t, pemBlock, "pem.Decode failed")

		validateTLSCertificateDERFunc(t, pemBlock.Bytes)
	}

	validateFormatZip := func(
		t *testing.T,
		body []byte,
		wantCAFiles int,
		validateCAFile func(t *testing.T, contents []byte),
	) {
		r, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		require.NoError(t, err, "zip.NewReader")

		files := r.File
		assert.Len(t, files, wantCAFiles, "mismatched number of CA files inside zip")

		// Traverse files in order. We want them to be named "ca0.cer, "ca1.cer",
		// etc.
		sort.Slice(files, func(i, j int) bool {
			return files[i].Name < files[j].Name
		})
		for i, f := range files {
			wantName := fmt.Sprintf("ca%d.cer", i)
			assert.Equal(t, wantName, f.Name, "mismatched name of CA file inside zip")

			fileReader, err := f.Open()
			require.NoError(t, err, "open CA file inside zip")
			fileBytes, err := io.ReadAll(fileReader)
			require.NoError(t, err, "read CA file contents inside zip")

			validateCAFile(t, fileBytes)
		}
	}
	validateFormatZipPEM := func(t *testing.T, body []byte, wantCAFiles int) {
		validateFormatZip(t, body, wantCAFiles, validateTLSCertificatePEMFunc)
	}

	ctx := context.Background()

	for _, tt := range []struct {
		name           string
		params         url.Values
		expectedStatus int
		assertBody     func(t *testing.T, bs []byte)
	}{
		{
			name:           "all",
			expectedStatus: http.StatusOK,
			assertBody: func(t *testing.T, b []byte) {
				require.Contains(t, string(b), "@cert-authority localhost,*.localhost ecdsa-sha2-nistp256 ")
				require.Contains(t, string(b), "cert-authority ecdsa-sha2-nistp256")
			},
		},
		{
			name: "host",
			params: url.Values{
				"type": []string{"host"},
			},
			expectedStatus: http.StatusOK,
			assertBody: func(t *testing.T, b []byte) {
				require.Contains(t, string(b), "@cert-authority localhost,*.localhost ecdsa-sha2-nistp256 ")
			},
		},
		{
			name: "user",
			params: url.Values{
				"type": []string{"user"},
			},
			expectedStatus: http.StatusOK,
			assertBody: func(t *testing.T, b []byte) {
				require.Contains(t, string(b), "cert-authority ecdsa-sha2-nistp256")
			},
		},
		{
			name: "windows",
			params: url.Values{
				"type": []string{"windows"},
			},
			expectedStatus: http.StatusOK,
			assertBody:     validateTLSCertificateDERFunc,
		},
		{
			name: "db",
			params: url.Values{
				"type": []string{"db"},
			},
			expectedStatus: http.StatusOK,
			assertBody:     validateTLSCertificatePEMFunc,
		},
		{
			name: "db-der",
			params: url.Values{
				"type": []string{"db-der"},
			},
			expectedStatus: http.StatusOK,
			assertBody:     validateTLSCertificateDERFunc,
		},
		{
			name: "db-client",
			params: url.Values{
				"type": []string{"db-client"},
			},
			expectedStatus: http.StatusOK,
			assertBody:     validateTLSCertificatePEMFunc,
		},
		{
			name: "db-client-der",
			params: url.Values{
				"type": []string{"db-client-der"},
			},
			expectedStatus: http.StatusOK,
			assertBody:     validateTLSCertificateDERFunc,
		},
		{
			name: "tls",
			params: url.Values{
				"type": []string{"tls"},
			},
			expectedStatus: http.StatusOK,
			assertBody:     validateTLSCertificatePEMFunc,
		},
		{
			name: "invalid",
			params: url.Values{
				"type": []string{"invalid"},
			},
			expectedStatus: http.StatusBadRequest,
			assertBody: func(t *testing.T, b []byte) {
				require.Contains(t, string(b), `"invalid" authority type is not supported`)
			},
		},
		{
			name: "format empty",
			params: url.Values{
				"type":   []string{"tls-user"},
				"format": []string{""},
			},
			expectedStatus: http.StatusOK,
			assertBody:     validateTLSCertificatePEMFunc,
		},
		{
			name: "format invalid",
			params: url.Values{
				"type":   []string{"tls-user"},
				"format": []string{"invalid"},
			},
			expectedStatus: http.StatusBadRequest,
			assertBody: func(t *testing.T, b []byte) {
				assert.Contains(t, string(b), "unsupported format")
			},
		},
		{
			name: "format=zip",
			params: url.Values{
				"type":   []string{"tls-user"},
				"format": []string{"zip"},
			},
			expectedStatus: http.StatusOK,
			assertBody: func(t *testing.T, b []byte) {
				validateFormatZipPEM(t, b, 1 /* wantCAFiles */)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runTest := func(t *testing.T, endpoint string) {
				authExportTestByEndpoint(ctx, t, endpoint, tt.params, tt.expectedStatus, tt.assertBody)
			}

			t.Run("deprecated endpoint", func(t *testing.T) {
				runTest(t, pack.clt.Endpoint("webapi", "sites", clusterName, "auth", "export"))
			})
			t.Run("new endpoint", func(t *testing.T) {
				runTest(t, pack.clt.Endpoint("webapi", "auth", "export"))
			})
		})
	}
}

func authExportTestByEndpoint(
	ctx context.Context,
	t *testing.T,
	exportEndpoint string,
	params url.Values,
	expectedStatus int,
	assertBody func(t *testing.T, bs []byte),
) {
	reqCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	encodedParams := params.Encode()
	if encodedParams != "" {
		exportEndpoint = exportEndpoint + "?" + encodedParams
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, exportEndpoint, nil)
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

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)

	require.Equal(t, expectedStatus, resp.StatusCode, "invalid status code with body %s", string(body))

	require.NotEmpty(t, body, "unexpected empty body from http response")
	if assertBody != nil {
		assertBody(t, body)
	}
}
