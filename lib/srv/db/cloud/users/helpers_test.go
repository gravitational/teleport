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

package users

import (
	"context"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestLookupMap(t *testing.T) {
	lookup := newLookupMap()
	db1 := mustCreateElastiCacheDatabase(t, "db1")
	db2 := mustCreateElastiCacheDatabase(t, "db2")
	db3 := mustCreateElastiCacheDatabase(t, "db3")
	user1 := newMockUser("userID1", "user1")
	user2 := newMockUser("userID2", "user2")
	user3 := newMockUser("userID3", "user3")

	t.Run("setDatabaseUsers", func(t *testing.T) {
		lookup.setDatabaseUsers(db1, []User{user1, user2})
		lookup.setDatabaseUsers(db2, []User{})
		lookup.setDatabaseUsers(db3, []User{user3})

		require.Equal(t, []string{"user1", "user2"}, db1.GetManagedUsers())
		require.Empty(t, db2.GetManagedUsers())
		require.Equal(t, []string{"user3"}, db3.GetManagedUsers())
	})

	t.Run("getDatabaseUser", func(t *testing.T) {
		for _, db := range []types.Database{db1, db2, db3} {
			for _, user := range []User{user1, user2, user3} {
				userGet, found := lookup.getDatabaseUser(db, user.GetDatabaseUsername())

				if slices.Contains(db.GetManagedUsers(), user.GetDatabaseUsername()) {
					require.True(t, found)
					require.Equal(t, user, userGet)
				} else {
					require.False(t, found)
				}
			}
		}
	})

	t.Run("removeUnusedDatabases", func(t *testing.T) {
		// Initially have three users.
		require.Equal(t, map[string]User{
			"userID1": user1,
			"userID2": user2,
			"userID3": user3,
		}, lookup.usersByID())

		// Removes db1 -> only one user left.
		activeDatabases := types.Databases{db2, db3}
		lookup.removeUnusedDatabases(activeDatabases)

		require.Equal(t, map[string]User{
			"userID3": user3,
		}, lookup.usersByID())
	})

	t.Run("removeIfURIChanged", func(t *testing.T) {
		// URI does not change. No users should be removed.
		lookup.removeIfURIChanged(db3)
		require.Equal(t, map[string]User{
			"userID3": user3,
		}, lookup.usersByID())

		// Now replace with a RDS.
		lookup.removeIfURIChanged(mustCreateRDSDatabase(t, "db3"))
		require.Empty(t, lookup.usersByID())
	})
}

func TestGenRandomPassword(t *testing.T) {
	for _, test := range []struct {
		name        string
		inputLength int
		expectError bool
	}{
		{
			name:        "even",
			inputLength: 50,
		},
		{
			name:        "odd",
			inputLength: 51,
		},
		{
			name:        "invalid",
			inputLength: 0,
			expectError: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			generated, err := genRandomPassword(test.inputLength)
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, generated, test.inputLength)
			}
		})
	}
}

func TestSecretKeyFromAWSARN(t *testing.T) {
	_, err := secretKeyFromAWSARN("invalid:arn")
	require.True(t, trace.IsBadParameter(err))

	key, err := secretKeyFromAWSARN("arn:aws-cn:elasticache:cn-north-1:123456789012:user:alice")
	require.NoError(t, err)
	require.Equal(t, "elasticache/cn-north-1/123456789012/user/alice", key)
}

type mockUser struct {
	id               string
	databaseUsername string
}

func newMockUser(id, databaseUsername string) *mockUser {
	return &mockUser{
		id:               id,
		databaseUsername: databaseUsername,
	}
}

func (m *mockUser) GetID() string                                   { return m.id }
func (m *mockUser) GetDatabaseUsername() string                     { return m.databaseUsername }
func (m *mockUser) Setup(ctx context.Context) error                 { return nil }
func (m *mockUser) Teardown(ctx context.Context) error              { return nil }
func (m *mockUser) GetPassword(ctx context.Context) (string, error) { return "password", nil }
func (m *mockUser) RotatePassword(ctx context.Context) error        { return nil }
