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

package pagination

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPageRequestToken(t *testing.T) {
	t.Run("zero value is valid", func(t *testing.T) {
		var token PageRequestToken
		value, err := token.Consume()
		require.NoError(t, err)
		require.Empty(t, value)
	})

	t.Run("recycling a value is an error", func(t *testing.T) {
		var token PageRequestToken
		_, _ = token.Consume()
		_, err := token.Consume()
		require.Error(t, err)
	})

	t.Run("updating token resets stale state", func(t *testing.T) {
		var token PageRequestToken
		_, _ = token.Consume()
		token.Update(NextPageToken("banana"))
		value, err := token.Consume()
		require.NoError(t, err)
		require.Equal(t, "banana", value)
	})
}
