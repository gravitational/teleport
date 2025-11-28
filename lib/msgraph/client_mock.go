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

package msgraph

import (
	"context"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// NewClientMock creates and returns a new instance of ClientMock.
func NewClientMock(customMockData *MockedMSGraphStateType) *ClientMock {
	client := &ClientMock{
		MockedMSGraphStateType: NewMockedMSGraphState(),
	}
	if customMockData != nil {
		client.MockedMSGraphStateType = *customMockData
	}
	return client
}

// ClientMock is a mock implementation of the Client interface for testing purposes.
type ClientMock struct {
	Mu sync.Mutex
	MockedMSGraphStateType

	// MonkeyPatch allows tests to override the default behavior of a mock
	// instance.
	MonkeyPatch struct {
		IterateUsers        func(ctx context.Context, f func(u *User) bool, opts ...IterateOpt) error
		IterateGroups       func(ctx context.Context, f func(*Group) bool, opts ...IterateOpt) error
		IterateGroupMembers func(ctx context.Context, groupID string, f func(GroupMember) bool, opts ...IterateOpt) error
		IterateApplications func(ctx context.Context, f func(*Application) bool, opts ...IterateOpt) error
		GetApplication      func(ctx context.Context, applicationID string) (*Application, error)
	}
}

// MockStateOption describes an option application function for constructing
// custom mocked Entra ID states
type MockStateOption func(*MockedMSGraphStateType)

// NewMockedMSGraphState returns a default mock state.
func NewMockedMSGraphState(options ...MockStateOption) MockedMSGraphStateType {
	// Using user email for id, upn makes it easy to compare values in tests.
	alice := &User{
		DirectoryObject: DirectoryObject{
			ID:          to.Ptr("alice@example.com"),
			DisplayName: to.Ptr("Alice Alison"),
		},
		GivenName:         to.Ptr("Alice"),
		Surname:           to.Ptr("Alison"),
		Mail:              to.Ptr("alice@example.com"),
		UserPrincipalName: to.Ptr("alice@example.com"),
	}
	bob := &User{
		DirectoryObject: DirectoryObject{
			ID:          to.Ptr("bob@example.com"),
			DisplayName: to.Ptr("Bob Bobert"),
		},
		GivenName:         to.Ptr("Bob"),
		Surname:           to.Ptr("Bobert"),
		Mail:              to.Ptr("bob@example.com"),
		UserPrincipalName: to.Ptr("bob@example.com"),
	}

	carol := &User{
		DirectoryObject: DirectoryObject{
			ID:          to.Ptr("carol@example.com"),
			DisplayName: to.Ptr("Carol C"),
		},
		GivenName:         to.Ptr("Carol"),
		Surname:           to.Ptr("C"),
		Mail:              to.Ptr("carol@example.com"),
		UserPrincipalName: to.Ptr("carol@example.com"),
	}

	state := MockedMSGraphStateType{
		Users: []*User{alice, bob, carol},
		Groups: []*Group{
			{
				DirectoryObject: DirectoryObject{
					ID:          to.Ptr("g1"),
					DisplayName: to.Ptr("g1"),
				},
				GroupTypes: []string{types.EntraIDSecurityGroups},
			},
			{
				DirectoryObject: DirectoryObject{
					ID:          to.Ptr("g2"),
					DisplayName: to.Ptr("g2"),
				},
				GroupTypes: []string{types.EntraIDSecurityGroups},
			},
			{
				DirectoryObject: DirectoryObject{
					ID:          to.Ptr("g3"),
					DisplayName: to.Ptr("g3"),
				},
				GroupTypes: []string{types.EntraIDSecurityGroups},
			},
		},
		GroupMembers: map[string][]GroupMember{
			"g1": {alice, bob, carol},
			"g2": {alice, bob, carol},
			"g3": {alice, bob, carol},
		},
		Applications: []*Application{
			{
				DirectoryObject: DirectoryObject{
					ID:          to.Ptr("app1"),
					DisplayName: to.Ptr("test SAML App"),
				},
				AppID: to.Ptr("app1"),
			},
		},
	}

	for _, applyOption := range options {
		applyOption(&state)
	}

	return state
}

// MockedMSGraphStateType is a struct that holds the mocked Entra ID state.
type MockedMSGraphStateType struct {
	// Entra ID users.
	Users []*User
	// Entra ID groups.
	Groups []*Group
	// Entra ID group members.
	// Member can be of user or group type.
	GroupMembers map[string][]GroupMember
	// Entra ID enterprise applications.
	Applications []*Application
}

// IterateUsers returns mocked users.
func (c *ClientMock) IterateUsers(ctx context.Context, f func(*User) bool, opts ...IterateOpt) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.IterateUsers != nil {
		return c.MonkeyPatch.IterateUsers(ctx, f)
	}

	for _, u := range c.Users {
		if !f(u) {
			return nil
		}
	}
	return nil
}

// IterateGroups returns mocked groups.
func (c *ClientMock) IterateGroups(ctx context.Context, f func(*Group) bool, opts ...IterateOpt) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.IterateGroups != nil {
		return c.MonkeyPatch.IterateGroups(ctx, f)
	}

	for _, g := range c.Groups {
		if !f(g) {
			return nil
		}
	}
	return nil
}

// IterateGroupMembers returns mocked group members.
func (c *ClientMock) IterateGroupMembers(ctx context.Context, groupID string, f func(GroupMember) bool, opts ...IterateOpt) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.IterateGroupMembers != nil {
		return c.MonkeyPatch.IterateGroupMembers(ctx, groupID, f)
	}

	for _, m := range c.GroupMembers[groupID] {
		if !f(m) {
			return nil
		}
	}
	return nil
}

// IterateApplications returns mocked applications
func (c *ClientMock) IterateApplications(ctx context.Context, f func(*Application) bool, opts ...IterateOpt) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.IterateApplications != nil {
		return c.MonkeyPatch.IterateApplications(ctx, f)
	}

	for _, a := range c.Applications {
		if !f(a) {
			return nil
		}
	}
	return nil
}

// GetApplication returns specific application.
func (c *ClientMock) GetApplication(ctx context.Context, applicationID string) (*Application, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.GetApplication != nil {
		return c.MonkeyPatch.GetApplication(ctx, applicationID)
	}

	for _, app := range c.Applications {
		if *app.AppID == applicationID {
			return app, nil
		}
	}

	return nil, trace.NotFound("application %q not found", applicationID)
}
