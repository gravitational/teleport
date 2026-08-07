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

package msgraphtest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/msgraph/models"
	"github.com/gravitational/teleport/lib/msgraph/msgraphtest"
	"github.com/gravitational/teleport/lib/utils"
)

func TestSetUsersDelta(t *testing.T) {
	defaultStorage := msgraphtest.NewDefaultStorage()

	// seed clean storage.
	storage := msgraphtest.NewStorage()
	fakeServer := msgraphtest.NewServer(msgraphtest.WithStorage(storage))

	ctx := t.Context()
	deltaLink := setupLatestUserDelta(t, ctx, fakeServer)

	// add new user alice.
	alice := defaultStorage.Users[msgraphtest.AliceID]
	fakeServer.SetUsers([]*models.User{alice})

	deltaLink, deltas := roundTrip[models.ListUsersDeltaResponse](t, ctx, deltaLink, fakeServer.TLSServer.Client())
	require.NotEmpty(t, deltaLink)

	require.Len(t, deltas, 1)
	require.Equal(t, msgraphtest.AliceID, *deltas[0].GetID())
}

func TestDeleteUsersDelta(t *testing.T) {
	defaultStorage := msgraphtest.NewDefaultStorage()

	// seed storage with alice user, group1 and add membership and ownership to group1.
	storage := msgraphtest.NewStorage()
	alice := defaultStorage.Users[msgraphtest.AliceID]
	storage.Users[msgraphtest.AliceID] = alice
	storage.Groups[msgraphtest.Group1ID] = defaultStorage.Groups[msgraphtest.Group1ID]
	storage.GroupMembers[msgraphtest.Group1ID] = []models.GroupMember{alice}
	storage.GroupOwners[msgraphtest.Group1ID] = []*models.User{alice}

	fakeServer := msgraphtest.NewServer(msgraphtest.WithStorage(storage))

	ctx := t.Context()
	userDeltaLink := setupLatestUserDelta(t, ctx, fakeServer)
	groupDeltaLink := setupLatestGroupsDelta(t, ctx, fakeServer)

	// delete user
	fakeServer.DeleteUsers([]string{*alice.GetID()})

	// check user is deleted.
	userDeltaLink, userDeltas := roundTrip[models.ListUsersDeltaResponse](t, ctx, userDeltaLink, fakeServer.TLSServer.Client())
	require.NotEmpty(t, userDeltaLink)

	require.Len(t, userDeltas, 1)
	require.Equal(t, msgraphtest.AliceID, *userDeltas[0].GetID())

	// check deleted user is removed from group.
	groupDeltaLink, groupDeltas := roundTrip[models.ListGroupsDeltaResponse](t, ctx, groupDeltaLink, fakeServer.TLSServer.Client())
	require.NotEmpty(t, groupDeltaLink)

	require.Len(t, groupDeltas, 1)
	require.Equal(t, msgraphtest.Group1ID, *groupDeltas[0].ID)
	require.Len(t, groupDeltas[0].Members, 1)
	require.Equal(t, msgraphtest.AliceID, *groupDeltas[0].Members[0].ID)
	require.Equal(t, "deleted", *groupDeltas[0].Members[0].Removed.Reason)
	require.Len(t, groupDeltas[0].Owners, 1)
	require.Equal(t, msgraphtest.AliceID, *groupDeltas[0].Owners[0].ID)
	require.Equal(t, "deleted", *groupDeltas[0].Owners[0].Removed.Reason)
}

func TestSetGroup(t *testing.T) {
	defaultStorage := msgraphtest.NewDefaultStorage()

	// seed empty storage.
	storage := msgraphtest.NewStorage()
	fakeServer := msgraphtest.NewServer(msgraphtest.WithStorage(storage))

	ctx := t.Context()
	deltaLink := setupLatestGroupsDelta(t, ctx, fakeServer)

	// add new group
	group1 := defaultStorage.Groups[msgraphtest.Group1ID]
	fakeServer.SetGroups([]*models.Group{group1})

	deltaLink, deltas := roundTrip[models.ListGroupsDeltaResponse](t, ctx, deltaLink, fakeServer.TLSServer.Client())
	require.NotEmpty(t, deltaLink)

	require.Len(t, deltas, 1)
	require.Equal(t, msgraphtest.Group1ID, *deltas[0].GetID())
}

func TestDeleteGroup(t *testing.T) {
	defaultStorage := msgraphtest.NewDefaultStorage()

	// seed storage with group1 and group2 and add group1 as member of group2.
	storage := msgraphtest.NewStorage()
	group1 := defaultStorage.Groups[msgraphtest.Group1ID]
	storage.Groups[msgraphtest.Group1ID] = group1

	storage.Groups[msgraphtest.Group2ID] = defaultStorage.Groups[msgraphtest.Group2ID]
	storage.GroupMembers[msgraphtest.Group2ID] = []models.GroupMember{group1}

	fakeServer := msgraphtest.NewServer(msgraphtest.WithStorage(storage))

	ctx := t.Context()
	deltaLink := setupLatestGroupsDelta(t, ctx, fakeServer)

	// delete group
	fakeServer.DeleteGroups([]string{msgraphtest.Group1ID})

	deltaLink, deltas := roundTrip[models.ListGroupsDeltaResponse](t, ctx, deltaLink, fakeServer.TLSServer.Client())
	require.NotEmpty(t, deltaLink)

	// expected group is deleted and removed from existing group membership.
	expected := []models.ListGroupsDeltaResponse{
		{
			Group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(msgraphtest.Group1ID),
				},
			},
			Removed: &models.RemovedReason{
				Reason: to.Ptr("deleted"),
			},
		},
		{
			Group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID:          to.Ptr(string(msgraphtest.Group2ID)),
					DisplayName: to.Ptr("group2"),
				},
			},
			Members: []models.MembersDelta{
				{
					DirectoryObject: &models.DirectoryObject{
						ID: to.Ptr(msgraphtest.Group1ID),
					},
					Type: models.ODataGroup,
					Removed: &models.RemovedReason{
						Reason: to.Ptr("deleted"),
					},
				},
			},
		},
	}

	require.ElementsMatch(t, expected, deltas)
}

func TestSetGroupMember(t *testing.T) {
	defaultStorage := msgraphtest.NewDefaultStorage()

	// seed the storage with group1.
	storage := msgraphtest.NewStorage()
	storage.Groups[msgraphtest.Group1ID] = defaultStorage.Groups[msgraphtest.Group1ID]
	fakeServer := msgraphtest.NewServer(msgraphtest.WithStorage(storage))

	ctx := t.Context()
	deltaLink := setupLatestGroupsDelta(t, ctx, fakeServer)

	// add alice as a member of group1.
	alice := defaultStorage.Users[msgraphtest.AliceID]
	fakeServer.SetGroupMembers(msgraphtest.Group1ID, []models.GroupMember{alice})

	deltaLink, deltas := roundTrip[models.ListGroupsDeltaResponse](t, ctx, deltaLink, fakeServer.TLSServer.Client())
	require.NotEmpty(t, deltaLink)

	require.Len(t, deltas, 1)
	require.Equal(t, msgraphtest.Group1ID, *deltas[0].GetID())
	require.Len(t, deltas[0].Members, 1)
	require.Equal(t, *alice.ID, *deltas[0].Members[0].ID)
}

func TestDeleteGroupMember(t *testing.T) {
	defaultStorage := msgraphtest.NewDefaultStorage()

	// seed storage with group1 and user alice as its member.
	storage := msgraphtest.NewStorage()
	storage.Groups[msgraphtest.Group1ID] = defaultStorage.Groups[msgraphtest.Group1ID]

	alice := defaultStorage.Users[msgraphtest.AliceID]
	storage.GroupMembers[msgraphtest.Group1ID] = []models.GroupMember{alice}
	fakeServer := msgraphtest.NewServer(msgraphtest.WithStorage(storage))

	ctx := t.Context()
	deltaLink := setupLatestGroupsDelta(t, ctx, fakeServer)

	// delete alice from group membership.
	fakeServer.DeleteGroupMembers(msgraphtest.Group1ID, []string{*alice.ID})

	deltaLink, deltas := roundTrip[models.ListGroupsDeltaResponse](t, ctx, deltaLink, fakeServer.TLSServer.Client())
	require.NotEmpty(t, deltaLink)

	require.Len(t, deltas, 1)
	require.Equal(t, msgraphtest.Group1ID, *deltas[0].GetID())
	require.Len(t, deltas[0].Members, 1)
	require.Equal(t, *alice.ID, *deltas[0].Members[0].ID)
	require.Equal(t, "deleted", *deltas[0].Members[0].Removed.Reason)
}

func TestSetGroupOwners(t *testing.T) {
	defaultStorage := msgraphtest.NewDefaultStorage()

	// seed storage with group1 and alice.
	storage := msgraphtest.NewStorage()
	alice := defaultStorage.Users[msgraphtest.AliceID]
	storage.Users[*alice.GetID()] = alice
	storage.Groups[msgraphtest.Group1ID] = defaultStorage.Groups[msgraphtest.Group1ID]
	fakeServer := msgraphtest.NewServer(msgraphtest.WithStorage(storage))

	ctx := t.Context()
	deltaLink := setupLatestGroupsDelta(t, ctx, fakeServer)

	// add user alice as owner of group1.
	fakeServer.SetGroupOwners(msgraphtest.Group1ID, []*models.User{alice})

	deltaLink, deltas := roundTrip[models.ListGroupsDeltaResponse](t, ctx, deltaLink, fakeServer.TLSServer.Client())
	require.NotEmpty(t, deltaLink)

	require.Len(t, deltas, 1)
	require.Equal(t, msgraphtest.Group1ID, *deltas[0].GetID())
	require.Empty(t, deltas[0].Members)
	require.Len(t, deltas[0].Owners, 1)
	require.Equal(t, *alice.ID, *deltas[0].Owners[0].ID)
}

func TestDeleteGroupOwners(t *testing.T) {
	defaultStorage := msgraphtest.NewDefaultStorage()

	// seed storage with group1 and alice and alice as owner of group1.
	storage := msgraphtest.NewStorage()
	storage.Groups[msgraphtest.Group1ID] = defaultStorage.Groups[msgraphtest.Group1ID]

	alice := defaultStorage.Users[msgraphtest.AliceID]
	storage.Users[*alice.GetID()] = alice
	storage.GroupOwners[msgraphtest.Group1ID] = []*models.User{alice}
	fakeServer := msgraphtest.NewServer(msgraphtest.WithStorage(storage))

	ctx := t.Context()
	deltaLink := setupLatestGroupsDelta(t, ctx, fakeServer)

	// remove alice from group ownership.
	fakeServer.DeleteGroupOwners(msgraphtest.Group1ID, []string{*alice.ID})

	deltaLink, deltas := roundTrip[models.ListGroupsDeltaResponse](t, ctx, deltaLink, fakeServer.TLSServer.Client())
	require.NotEmpty(t, deltaLink)

	require.Len(t, deltas, 1)
	require.Equal(t, msgraphtest.Group1ID, *deltas[0].GetID())
	require.Empty(t, deltas[0].Members)
	require.Len(t, deltas[0].Owners, 1)
	require.Equal(t, *alice.ID, *deltas[0].Owners[0].ID)
	require.Equal(t, "deleted", *deltas[0].Owners[0].Removed.Reason)
}

func roundTrip[T any](t *testing.T, ctx context.Context, url string, client *http.Client) (string, []T) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode, "expected response status to match")

	var odata models.ODataPage
	err = json.Unmarshal(respBody, &odata)
	require.NoError(t, err)

	var out []T
	err = json.Unmarshal(odata.Value, &out)
	require.NoError(t, err, "expected valid delta response object")

	return odata.DeltaLink, out
}

func setupLatestUserDelta(t *testing.T, ctx context.Context, s *msgraphtest.Server) string {
	t.Helper()

	url := fmt.Sprintf("%s/v1.0/users/delta?$deltatoken=latest", s.TLSServer.URL)
	userDeltaLink, userDeltas := roundTrip[models.ListUsersDeltaResponse](t, ctx, url, s.TLSServer.Client())
	require.Empty(t, userDeltas)

	return userDeltaLink
}

func setupLatestGroupsDelta(t *testing.T, ctx context.Context, s *msgraphtest.Server) string {
	t.Helper()

	url := fmt.Sprintf("%s/v1.0/groups/delta?$deltatoken=latest", s.TLSServer.URL)
	groupDeltaLink, groupDeltas := roundTrip[models.ListGroupsDeltaResponse](t, ctx, url, s.TLSServer.Client())
	require.Empty(t, groupDeltas)

	return groupDeltaLink
}
