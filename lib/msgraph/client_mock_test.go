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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientMock(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	var c ClientI = NewClientMock(nil /*custom mock data*/)
	t.Run("should list users", func(t *testing.T) {
		out := []string{}
		err := c.IterateUsers(ctx, func(u *User) bool {
			out = append(out, *u.Mail)
			return true
		})
		require.NoError(t, err)

		require.ElementsMatch(t, out, []string{"alice@example.com", "bob@example.com", "carol@example.com"})
	})
}
