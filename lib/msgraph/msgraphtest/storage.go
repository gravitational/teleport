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
	"github.com/gravitational/teleport/lib/msgraph"
)

// Storage to be used by the Server
type Storage struct {
	Users        map[string]*msgraph.User
	UsersDelta   map[int][]msgraph.ListUsersDeltaResponse
	Groups       map[string]*msgraph.Group
	GroupsDelta  map[int][]msgraph.ListGroupsDeltaResponse
	GroupMembers map[string][]msgraph.GroupMember
	GroupOwners  map[string][]*msgraph.User
	Applications map[string]*msgraph.Application
}

// NewStorage creates a new empty Storage.
func NewStorage() *Storage {
	return &Storage{
		Users:        make(map[string]*msgraph.User),
		UsersDelta:   make(map[int][]msgraph.ListUsersDeltaResponse),
		Groups:       make(map[string]*msgraph.Group),
		GroupsDelta:  make(map[int][]msgraph.ListGroupsDeltaResponse),
		GroupMembers: make(map[string][]msgraph.GroupMember),
		GroupOwners:  make(map[string][]*msgraph.User),
		Applications: make(map[string]*msgraph.Application),
	}
}

const (
	Group1 = "fdfc6317-cc24-4c9c-b32a-143b0fbf3cd0"
	Group2 = "7b1e66cc-3768-4281-bc4d-b84720654842"
	Group3 = "4698ee2a-bf74-467e-8bde-63db8f323a44"
)

const (
	Carol = "1c5f5517-27dc-415f-9793-c9531cd17d48"
)

// NewDefaultStorage creates a new Storage with hardcoded test data.
func NewDefaultStorage() *Storage {
	alice := &msgraph.User{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("2765d9b2-a70c-4d30-a1ec-f02c40fcf4ad"),
			DisplayName: to.Ptr("Alice Alison"),
		},
		GivenName:         to.Ptr("Alice"),
		Surname:           to.Ptr("Alison"),
		Mail:              to.Ptr("alice@example.com"),
		UserPrincipalName: to.Ptr("alice@example.com"),
	}
	bob := &msgraph.User{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("aace3f26-9f57-4519-b5fb-0d38fe93d3c2"),
			DisplayName: to.Ptr("Bob Bobert"),
		},
		GivenName:         to.Ptr("Bob"),
		Surname:           to.Ptr("Bobert"),
		Mail:              to.Ptr("bob@example.com"),
		UserPrincipalName: to.Ptr("bob@example.com"),
	}
	carol := &msgraph.User{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr(Carol),
			DisplayName: to.Ptr("Carol C"),
		},
		GivenName:         to.Ptr("Carol"),
		Surname:           to.Ptr("C"),
		Mail:              to.Ptr("carol@example.com"),
		UserPrincipalName: to.Ptr("carol@example.com"),
	}

	group1 := &msgraph.Group{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr(Group1),
			DisplayName: to.Ptr("group1"),
		},
		GroupTypes: []string{types.EntraIDSecurityGroups},
	}
	group2 := &msgraph.Group{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr(Group2),
			DisplayName: to.Ptr("group2"),
		},
		GroupTypes: []string{types.EntraIDSecurityGroups},
	}
	group3 := &msgraph.Group{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr(Group3),
			DisplayName: to.Ptr("group3"),
		},
		GroupTypes: []string{types.EntraIDSecurityGroups},
	}

	app1 := &msgraph.Application{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("0e0038e9-6653-4701-8c44-826afbbc39f6"),
			DisplayName: to.Ptr("test SAML App"),
		},
		AppID: to.Ptr("app1"),
	}

	storage := NewStorage()

	storage.Users[*alice.ID] = alice
	storage.Users[*bob.ID] = bob
	storage.Users[*carol.ID] = carol

	storage.Groups[*group1.ID] = group1
	storage.Groups[*group2.ID] = group2
	storage.Groups[*group3.ID] = group3

	storage.GroupMembers[Group1] = []msgraph.GroupMember{alice, group2}
	storage.GroupMembers[Group2] = []msgraph.GroupMember{alice, bob, carol}
	storage.GroupMembers[Group3] = []msgraph.GroupMember{alice, bob, carol}

	storage.GroupOwners[Group1] = []*msgraph.User{alice, bob}
	// no owners for group2
	storage.GroupOwners[Group3] = []*msgraph.User{bob, carol}

	storage.Applications[*app1.AppID] = app1

	return storage
}
