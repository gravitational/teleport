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
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const (
	awsScimTokenEnv    = "AWS_SCIM_TOKEN_TEST_INTEGRATION"
	awsScimEndpointEnv = "AWS_SCIM_ENDPOINT_TEST_INTEGRATION"
)

func TestSCIMSDKClient(t *testing.T) {
	if os.Getenv(awsScimTokenEnv) == "" || os.Getenv(awsScimEndpointEnv) == "" {
		t.Skipf("Skipping integration test, set %s and %s", awsScimTokenEnv, awsScimEndpointEnv)
	}
	ctx := context.Background()
	cfg := &Config{
		Endpoint: os.Getenv(awsScimEndpointEnv),
		Token:    os.Getenv(awsScimTokenEnv),
	}

	cli, err := New(cfg)
	require.NoError(t, err)

	testSCIMIntegration(t, ctx, cli)
}

func testSCIMIntegration(t *testing.T, ctx context.Context, cli Client) {
	require.NoError(t, cli.Ping(ctx))
	usersToCreate := []*User{
		{
			UserName:    "richard",
			Name:        &Name{FamilyName: "-", GivenName: "-"},
			DisplayName: "Richard Test User",
			Active:      true,
		},
		{

			UserName:    "alice",
			Name:        &Name{FamilyName: "-", GivenName: "-"},
			DisplayName: "Alice Test User",
			Active:      false,
		},
	}

	genUserFunc := mkUserGenerator()
	for range 101 {
		usersToCreate = append(usersToCreate, genUserFunc())
	}

	var createdUsers []*User

	for _, user := range usersToCreate {
		newUser, err := cli.CreateUser(ctx, user)
		require.NoError(t, err)
		createdUsers = append(createdUsers, newUser)

		t.Cleanup(func() {
			err = cli.DeleteUser(ctx, newUser.ID)
			require.NoError(t, err)
		})
	}

	richardUser, err := cli.GetUserByUserName(ctx, "richard")
	require.NoError(t, err)
	require.Equal(t, "richard", richardUser.UserName)
	richardUser.Active = false

	u, err := cli.UpdateUser(ctx, richardUser)
	require.NoError(t, err)
	require.False(t, u.Active)

	aliceUser, err := cli.GetUserByUserName(ctx, "alice")
	require.NoError(t, err)
	require.Equal(t, "alice", aliceUser.UserName)

	groupsToCreate := []*Group{
		{
			DisplayName: "TestGroup",
			Members: []*GroupMember{
				{
					ExternalID: richardUser.ID,
				},
			},
		},
		{
			DisplayName: "TestGroup2",
			Members: []*GroupMember{
				{
					ExternalID: aliceUser.ID,
				},
			},
		},
	}
	for _, group := range groupsToCreate {
		n, err := cli.CreateGroup(ctx, group)
		require.NoError(t, err)
		t.Cleanup(func() {
			err = cli.DeleteGroup(ctx, n.ID)
			require.NoError(t, err)
		})
	}
	testGroup, err := cli.GetGroupByDisplayName(ctx, "TestGroup")
	require.NoError(t, err)
	require.Equal(t, "TestGroup", testGroup.DisplayName)

	err = cli.ReplaceGroupMembers(ctx, testGroup.ID, []*GroupMember{
		{ExternalID: aliceUser.ID},
		{ExternalID: richardUser.ID},
	})
	require.NoError(t, err)

	var members []*GroupMember

	for _, v := range createdUsers {
		members = append(members, &GroupMember{ExternalID: v.ID})
	}

	err = cli.ReplaceGroupMembers(ctx, testGroup.ID, members)
	require.NoError(t, err)

	g, err := cli.GetGroup(ctx, testGroup.ID)
	require.NoError(t, err)
	require.Equal(t, testGroup.ID, g.ID)

	g.DisplayName = "TestGroupUpdated"
	_, err = cli.UpdateGroup(ctx, g)
	require.NoError(t, err)

	u, err = cli.GetUser(ctx, richardUser.ID)
	require.NoError(t, err)
	require.Equal(t, richardUser.ID, u.ID)

}

func mkUserGenerator() func() *User {
	var counter int32
	return func() *User {
		atomic.AddInt32(&counter, 1)
		id := uuid.New()
		return &User{
			UserName:    fmt.Sprintf("%d-test-username-%s", counter, id),
			Name:        &Name{FamilyName: "-", GivenName: "-"},
			DisplayName: fmt.Sprintf("%d-test-display-name-%s", counter, id),
			Active:      true,
		}
	}
}
