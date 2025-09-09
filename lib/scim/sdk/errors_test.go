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

package scimsdk

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/go-jose/go-jose/v3/json"
	"github.com/stretchr/testify/require"
)

func requireErrorContaining(text string) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, args ...any) {
		require.Error(t, err, args...)
		require.Contains(t, err.Error(), text, args...)
	}
}

func TestDecodeError(t *testing.T) {
	testCases := []struct {
		name        string
		status      int
		body        *ErrorResponse
		expectation require.ErrorAssertionFunc
	}{
		{
			name:   "not found",
			status: http.StatusNotFound,
			body: &ErrorResponse{
				SCIMType: "noTarget",
				Detail:   "No such resource",
			},
			expectation: requireErrorContaining("No such resource"),
		},
		{
			name:        "internal server error",
			status:      http.StatusInternalServerError,
			expectation: require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			resp := &http.Response{
				Status:     http.StatusText(test.status),
				StatusCode: test.status,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}

			if test.body != nil {
				if test.body.Schemas == nil {
					test.body.Schemas = []string{errorSchema}
				}
				if test.body.Status == "" {
					test.body.Status = http.StatusText(test.status)
				}

				bodyBytes, err := json.Marshal(test.body)
				require.NoError(t, err)

				resp.Header.Set(ContentTypeHeader, ContentType)
				resp.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))
				resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}

			test.expectation(t, decodeError(resp))
		})
	}
}
