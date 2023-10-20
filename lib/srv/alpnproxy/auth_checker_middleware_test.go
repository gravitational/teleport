// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alpnproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthorizationCheckerMiddleware_HandleRequest(t *testing.T) {
	middleware := &AuthorizationCheckerMiddleware{Secret: "secret"}
	require.NoError(t, middleware.CheckAndSetDefaults())

	tests := []struct {
		name    string
		headers map[string]string
		want    bool
		code    int
	}{
		{
			name:    "ignore requests without Authorization",
			headers: nil,
			want:    false,
			code:    200,
		},
		{
			name:    "pass requests with proper secret",
			headers: map[string]string{"Authorization": "Bearer secret"},
			want:    false,
			code:    200,
		},
		{
			name:    "block requests with invalid secret",
			headers: map[string]string{"Authorization": "Bearer invalid secret"},
			want:    true,
			code:    403,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "https://compute.googleapis.com:443", nil)
			require.NoError(t, err)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			rw := httptest.NewRecorder()
			ret := middleware.HandleRequest(rw, req)
			require.Equal(t, tt.want, ret)
			require.Equal(t, tt.code, rw.Code)
		})
	}
}
