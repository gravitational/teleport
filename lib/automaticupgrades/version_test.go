/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package automaticupgrades

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	ctx := context.Background()

	isBadParameterErr := func(tt require.TestingT, err error, i ...interface{}) {
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
				assert.Equal(t, r.URL.Path, "/v1/stable/cloud/version")
				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponseString))
			}))
			defer httpTestServer.Close()

			v, err := Version(ctx, httpTestServer.URL)
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			require.Equal(t, v, tt.expectedVersion)
		})
	}
}
