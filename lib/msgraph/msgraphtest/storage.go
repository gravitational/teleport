package msgraphtest

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/msgraph"
)

type Storage struct {
	Users        map[string]*msgraph.User
	Groups       map[string]*msgraph.Group
	GroupMembers map[string][]msgraph.GroupMember
	Applications map[string]*msgraph.Application
}

func NewStorage() *Storage {
	return &Storage{
		Users:        make(map[string]*msgraph.User),
		Groups:       make(map[string]*msgraph.Group),
		GroupMembers: make(map[string][]msgraph.GroupMember),
		Applications: make(map[string]*msgraph.Application),
	}
}

func NewDefaultStorage() *Storage {
	alice := &msgraph.User{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("alice@example.com"),
			DisplayName: to.Ptr("Alice Alison"),
		},
		GivenName:         to.Ptr("Alice"),
		Surname:           to.Ptr("Alison"),
		Mail:              to.Ptr("alice@example.com"),
		UserPrincipalName: to.Ptr("alice@example.com"),
	}
	bob := &msgraph.User{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("bob@example.com"),
			DisplayName: to.Ptr("Bob Bobert"),
		},
		GivenName:         to.Ptr("Bob"),
		Surname:           to.Ptr("Bobert"),
		Mail:              to.Ptr("bob@example.com"),
		UserPrincipalName: to.Ptr("bob@example.com"),
	}
	carol := &msgraph.User{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("carol@example.com"),
			DisplayName: to.Ptr("Carol C"),
		},
		GivenName:         to.Ptr("Carol"),
		Surname:           to.Ptr("C"),
		Mail:              to.Ptr("carol@example.com"),
		UserPrincipalName: to.Ptr("carol@example.com"),
	}

	g1 := &msgraph.Group{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("g1"),
			DisplayName: to.Ptr("g1"),
		},
		GroupTypes: []string{types.EntraIDSecurityGroups},
	}
	g2 := &msgraph.Group{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("g2"),
			DisplayName: to.Ptr("g2"),
		},
		GroupTypes: []string{types.EntraIDSecurityGroups},
	}
	g3 := &msgraph.Group{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("g3"),
			DisplayName: to.Ptr("g3"),
		},
		GroupTypes: []string{types.EntraIDSecurityGroups},
	}

	app1 := &msgraph.Application{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("app1"),
			DisplayName: to.Ptr("test SAML App"),
		},
		AppID: to.Ptr("app1"),
	}

	storage := NewStorage()

	storage.Users[*alice.ID] = alice
	storage.Users[*bob.ID] = bob
	storage.Users[*carol.ID] = carol

	storage.Groups[*g1.ID] = g1
	storage.Groups[*g2.ID] = g2
	storage.Groups[*g3.ID] = g3

	storage.GroupMembers["g1"] = []msgraph.GroupMember{alice, bob, carol}
	storage.GroupMembers["g2"] = []msgraph.GroupMember{alice, bob, carol}
	storage.GroupMembers["g3"] = []msgraph.GroupMember{alice, bob, carol}

	storage.Applications[*app1.AppID] = app1

	return storage
}
