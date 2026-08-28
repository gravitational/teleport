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
