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

package msgraphtest

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/msgraph/models"
)

// Storage to be used by the Server
type Storage struct {
	Users        map[string]*models.User
	UsersDelta   map[int][]models.ListUsersDeltaResponse
	Groups       map[string]*models.Group
	GroupsDelta  map[int][]models.ListGroupsDeltaResponse
	GroupMembers map[string][]models.GroupMember
	GroupOwners  map[string][]*models.User
	Applications map[string]*models.Application
}

// NewStorage creates a new empty Storage.
func NewStorage() *Storage {
	return &Storage{
		Users:        make(map[string]*models.User),
		UsersDelta:   make(map[int][]models.ListUsersDeltaResponse),
		Groups:       make(map[string]*models.Group),
		GroupsDelta:  make(map[int][]models.ListGroupsDeltaResponse),
		GroupMembers: make(map[string][]models.GroupMember),
		GroupOwners:  make(map[string][]*models.User),
		Applications: make(map[string]*models.Application),
	}
}

// NewEntraGroup returns a new Entra ID group.
func NewEntraGroup(id, name string) *models.Group {
	return &models.Group{
		DirectoryObject: models.DirectoryObject{
			ID:          to.Ptr(id),
			DisplayName: to.Ptr(name),
		},
		GroupTypes: []string{types.EntraIDSecurityGroups},
	}
}

// NewEntraUser returns a new Entra ID user.
func NewEntraUser(id, name string) *models.User {
	return &models.User{
		DirectoryObject: models.DirectoryObject{
			ID: to.Ptr(id),
		},
		UserPrincipalName: to.Ptr(name),
		Mail:              to.Ptr(name),
	}
}

const (
	AliceID = "2765d9b2-a70c-4d30-a1ec-f02c40fcf4ad"
	BobID   = "aace3f26-9f57-4519-b5fb-0d38fe93d3c2"
	CarolID = "1c5f5517-27dc-415f-9793-c9531cd17d48"

	Group1ID = "fdfc6317-cc24-4c9c-b32a-143b0fbf3cd0"
	Group2ID = "7b1e66cc-3768-4281-bc4d-b84720654842"
	Group3ID = "4698ee2a-bf74-467e-8bde-63db8f323a44"

	App1ID = "0e0038e9-6653-4701-8c44-826afbbc39f6"
)

// NewDefaultStorage creates a new msgraphtest.Storage with hardcoded test data.
func NewDefaultStorage() *Storage {
	storage := NewStorage()

	alice := NewEntraUser(AliceID, "alice@example.com")
	storage.Users[AliceID] = alice
	bob := NewEntraUser(BobID, "bob@example.com")
	storage.Users[BobID] = bob
	carol := NewEntraUser(CarolID, "carol@example.com")
	storage.Users[CarolID] = carol

	group1 := NewEntraGroup(Group1ID, "group1")
	storage.Groups[Group1ID] = group1
	group2 := NewEntraGroup(Group2ID, "group2")
	storage.Groups[Group2ID] = group2
	group3 := NewEntraGroup(Group3ID, "group3")
	storage.Groups[Group3ID] = group3

	storage.GroupMembers[Group1ID] = []models.GroupMember{alice, group2}
	storage.GroupMembers[Group2ID] = []models.GroupMember{alice, bob, carol}
	storage.GroupMembers[Group3ID] = []models.GroupMember{alice, bob, carol}

	storage.GroupOwners[Group1ID] = []*models.User{alice, bob}
	storage.GroupOwners[Group3ID] = []*models.User{bob, carol}

	app1 := &models.Application{
		DirectoryObject: models.DirectoryObject{
			ID:          to.Ptr("ddca8610-0fa7-4acf-a80a-4d3b9c8346b9"),
			DisplayName: to.Ptr("test SAML App"),
		},
		AppID: to.Ptr(App1ID),
	}
	storage.Applications[App1ID] = app1

	return storage
}
