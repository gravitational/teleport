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

package automaticupgrades

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	ctx := context.Background()

	isBadParameterErr := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	for _, tt := range []struct {
		name               string
		mockStatusCode     int
		mockResponseString string
		errCheck           require.ErrorAssertionFunc
		expectedVersion    string
	}{
		{
			name:               "real response",
			mockStatusCode:     http.StatusOK,
			mockResponseString: "v13.1.1\n",
			errCheck:           require.NoError,
			expectedVersion:    "v13.1.1",
		},
		{
			name:           "invalid status code (500)",
			mockStatusCode: http.StatusInternalServerError,
			errCheck:       isBadParameterErr,
		},
		{
			name:           "invalid status code (403)",
			mockStatusCode: http.StatusForbidden,
			errCheck:       isBadParameterErr,
		},
		{
			name:               "valid but has spaces",
			mockStatusCode:     http.StatusOK,
			mockResponseString: " v13.1.1 \n \r\n",
			errCheck:           require.NoError,
			expectedVersion:    "v13.1.1",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			httpTestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1/stable/cloud/version", r.URL.Path)
				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponseString))
			}))
			defer httpTestServer.Close()

			versionURL, err := url.JoinPath(httpTestServer.URL, "/v1/stable/cloud/version")
			require.NoError(t, err)

			v, err := Version(ctx, versionURL)
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tt.expectedVersion, v)
		})
	}
}

func TestCritical(t *testing.T) {
	ctx := context.Background()

	isBadParameterErr := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	for _, tt := range []struct {
		name               string
		mockStatusCode     int
		mockResponseString string
		errCheck           require.ErrorAssertionFunc
		expectedCritical   bool
	}{
		{
			name:               "critical available",
			mockStatusCode:     http.StatusOK,
			mockResponseString: "yes\n",
			errCheck:           require.NoError,
			expectedCritical:   true,
		},
		{
			name:               "critical is not available",
			mockStatusCode:     http.StatusOK,
			mockResponseString: "no\n",
			errCheck:           require.NoError,
			expectedCritical:   false,
		},
		{
			name:           "invalid status code (500)",
			mockStatusCode: http.StatusInternalServerError,
			errCheck:       isBadParameterErr,
		},
		{
			name:           "invalid status code (403)",
			mockStatusCode: http.StatusForbidden,
			errCheck:       isBadParameterErr,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			httpTestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1/stable/cloud/critical", r.URL.Path)
				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponseString))
			}))
			defer httpTestServer.Close()

			criticalURL, err := url.JoinPath(httpTestServer.URL, "/v1/stable/cloud/critical")
			require.NoError(t, err)

			v, err := Critical(ctx, criticalURL)
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tt.expectedCritical, v)
		})
	}
}

func TestGetVersionURL(t *testing.T) {
	for _, tt := range []struct {
		name        string
		versionURL  string
		expectedURL string
	}{
		{
			name:        "default stable/cloud version url",
			versionURL:  "",
			expectedURL: "https://updates.releases.teleport.dev/v1/stable/cloud/version",
		},
		{
			name:        "custom version url",
			versionURL:  "https://custom.dev/version",
			expectedURL: "https://custom.dev/version",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v, err := getVersionURL(tt.versionURL)
			require.NoError(t, err)
			require.Equal(t, tt.expectedURL, v)
		})
	}
}

func TestGetCriticalURL(t *testing.T) {
	for _, tt := range []struct {
		name        string
		criticalURL string
		expectedURL string
	}{
		{
			name:        "default stable/cloud critical url",
			criticalURL: "",
			expectedURL: "https://updates.releases.teleport.dev/v1/stable/cloud/critical",
		},
		{
			name:        "custom critical url",
			criticalURL: "https://custom.dev/critical",
			expectedURL: "https://custom.dev/critical",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v, err := getCriticalURL(tt.criticalURL)
			require.NoError(t, err)
			require.Equal(t, tt.expectedURL, v)
		})
	}
}
