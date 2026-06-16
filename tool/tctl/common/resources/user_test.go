// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package resources

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

const (
	testDisplayNameTrait = "displayName"
	testEmailTrait       = "email"
)

func TestUserCollectionWriteTextDisplaysUserIdentity(t *testing.T) {
	users := []types.User{
		newUserWithDisplayTraits(t, "display-both", "Alice Adams", "alice@example.com"),
		newUserWithDisplayTraits(t, "display-primary", "Bob Baker", ""),
		newUserWithDisplayTraits(t, "display-secondary", "", "secondary@example.com"),
		newUserWithDisplayTraits(t, "display-none", "", ""),
		newUserWithDisplayTraits(t, "alice@example.com", "alice@example.com", "alice@example.com"),
		newUserWithDisplayTraits(t, "display-sanitized", "Eve\x1b[31m\nAdmin", "eve@example.com\r\n"),
	}
	table := asciitable.MakeTable(
		[]string{"User"},
		[]string{"display-both (Alice Adams) <alice@example.com>"},
		[]string{"display-primary (Bob Baker)"},
		[]string{"display-secondary <secondary@example.com>"},
		[]string{"display-none"},
		[]string{"alice@example.com"},
		[]string{"display-sanitized (Eve Admin) <eve@example.com>"},
	)
	formatted := table.AsBuffer().String()

	collectionFormatTest(t, &userCollection{users: users}, formatted, formatted)
}

func newUserWithDisplayTraits(t *testing.T, username, primary, secondary string) types.User {
	t.Helper()

	user, err := types.NewUser(username)
	require.NoError(t, err)

	traits := make(map[string][]string)
	if primary != "" {
		traits[testDisplayNameTrait] = []string{primary}
	}
	if secondary != "" {
		traits[testEmailTrait] = []string{secondary}
	}
	user.SetTraits(traits)
	return user
}
