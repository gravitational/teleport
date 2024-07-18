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
		graphError, err := readError(strings.NewReader(msGraphErrorPayload))
		require.NoError(t, err)
		require.NotNil(t, graphError)
		expected := &GraphError{
			Code:    "Error_BadRequest",
			Message: "Uploaded fragment overlaps with existing data.",
			InnerError: &GraphError{
				Code: "invalidRange",
			},
		}
		require.Equal(t, expected, graphError)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := readError(strings.NewReader("invalid json"))
		require.Error(t, err)
	})

	t.Run("empty", func(t *testing.T) {
		graphError, err := readError(strings.NewReader("{}"))
		require.NoError(t, err)
		require.Nil(t, graphError)
	})
}
