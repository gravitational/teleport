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

package msgraph

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const msGraphErrorPayload = `{
  "error": {
    "code": "Error_BadRequest",
    "message": "Uploaded fragment overlaps with existing data.",
    "innerError": {
      "code": "invalidRange",
      "request-id": "request-id",
      "date": "date-time"
    }
  }
}`

func TestUnmarshalGraphError(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		graphError, err := readError([]byte(msGraphErrorPayload), 400)
		require.NoError(t, err)
		require.NotNil(t, graphError)
		expected := &GraphError{
			Code:    "Error_BadRequest",
			Message: "Uploaded fragment overlaps with existing data.",
			InnerError: &GraphError{
				Code: "invalidRange",
			},
			StatusCode: 400,
		}
		require.Equal(t, expected, graphError)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := readError([]byte("invalid json"), 400)
		require.Error(t, err)
	})

	t.Run("empty", func(t *testing.T) {
		graphError, err := readError([]byte("{}"), 400)
		require.NoError(t, err)
		require.Nil(t, graphError)
	})
}

const authErrorPayload = `{
  "error": "invalid_scope",
  "error_description": "AADSTS70011: The provided value for the input parameter 'scope' isn't valid. The scope https://example.contoso.com/activity.read isn't valid.\r\nTrace ID: 0000aaaa-11bb-cccc-dd22-eeeeee333333\r\nCorrelation ID: aaaa0000-bb11-2222-33cc-444444dddddd\r\nTimestamp: 2016-01-09 02:02:12Z",
  "error_codes": [
    70011
  ],
  "timestamp": "2016-01-09 02:02:12Z",
  "trace_id": "0000aaaa-11bb-cccc-dd22-eeeeee333333",
  "correlation_id": "aaaa0000-bb11-2222-33cc-444444dddddd",
  "error_uri":"https://login.microsoftonline.com/error?code=70011"
}`

func TestUnmarshalAuthError(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		authError, err := readAuthError(strings.NewReader(authErrorPayload), 400)
		require.NoError(t, err)
		require.NotNil(t, authError)
		expected := &AuthError{
			ErrorCode:        "invalid_scope",
			ErrorDescription: "AADSTS70011: The provided value for the input parameter 'scope' isn't valid. The scope https://example.contoso.com/activity.read isn't valid.\r\nTrace ID: 0000aaaa-11bb-cccc-dd22-eeeeee333333\r\nCorrelation ID: aaaa0000-bb11-2222-33cc-444444dddddd\r\nTimestamp: 2016-01-09 02:02:12Z",
			DiagCodes:        []int{70011},
			StatusCode:       400,
		}
		require.Equal(t, expected, authError)
		expectedMessage := "AADSTS70011: The provided value for the input parameter 'scope' isn't valid. The scope https://example.contoso.com/activity.read isn't valid.\r\nTrace ID: 0000aaaa-11bb-cccc-dd22-eeeeee333333\r\nCorrelation ID: aaaa0000-bb11-2222-33cc-444444dddddd\r\nTimestamp: 2016-01-09 02:02:12Z " +
			"(invalid_scope, diag code 70011)"
		require.Equal(t, expectedMessage, authError.Error())
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := readAuthError(strings.NewReader("invalid json"), 400)
		require.Error(t, err)
	})

	t.Run("message", func(t *testing.T) {
		tests := []struct {
			name      string
			diagCodes []int
			want      string
		}{
			{
				name: "no diag codes",
				want: "foo bar (invalid_client)",
			},
			{
				name:      "one diag code",
				diagCodes: []int{42},
				want:      "foo bar (invalid_client, diag code 42)",
			},
			{
				name:      "multiple diag codes",
				diagCodes: []int{42, 37},
				want:      "foo bar (invalid_client, diag codes 42, 37)",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				authError := &AuthError{
					ErrorCode:        "invalid_client",
					ErrorDescription: "foo bar",
					DiagCodes:        tt.diagCodes,
					StatusCode:       400,
				}
				require.Equal(t, tt.want, authError.Error())
			})
		}
	})
}
