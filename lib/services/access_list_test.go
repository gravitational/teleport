/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

const (
	ownerUser = "owner-user"
	member1   = "member1"
	member2   = "member2"
	member3   = "member3"
)

// TestAccessListUnmarshal verifies an access list resource can be unmarshaled.
func TestAccessListUnmarshal(t *testing.T) {
	expected, err := accesslist.NewAccessList(
		header.Metadata{
			Name: "test-access-list",
		},
		accesslist.Spec{
			Description: "test access list",
			Owners: []accesslist.Owner{
				{
					Name:        "test-user1",
					Description: "test user 1",
				},
				{
					Name:        "test-user2",
					Description: "test user 2",
				},
			},
			Audit: accesslist.Audit{
				Frequency: time.Hour,
			},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			OwnershipRequires: accesslist.Requires{
				Roles: []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			Grants: accesslist.Grants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
			Members: []accesslist.Member{
				{
					Name:    "member1",
					Joined:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
					Expires: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Reason:  "because",
					AddedBy: "test-user1",
				},
				{
					Name:    "member2",
					Joined:  time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					Expires: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
					Reason:  "because again",
					AddedBy: "test-user2",
				},
			},
		},
	)
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(accessListYAML))
	require.NoError(t, err)
	actual, err := UnmarshalAccessList(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestAccessListMarshal verifies a marshaled access list resource can be unmarshaled back.
func TestAccessListMarshal(t *testing.T) {
	expected, err := accesslist.NewAccessList(
		header.Metadata{
			Name: "test-rule",
		},
		accesslist.Spec{
			Description: "test access list",
			Owners: []accesslist.Owner{
				{
					Name:        "test-user1",
					Description: "test user 1",
				},
				{
					Name:        "test-user2",
					Description: "test user 2",
				},
			},
			Audit: accesslist.Audit{
				Frequency: time.Hour,
			},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			OwnershipRequires: accesslist.Requires{
				Roles: []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			Grants: accesslist.Grants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
			Members: []accesslist.Member{
				{
					Name:    "member1",
					Joined:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
					Expires: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Reason:  "because",
					AddedBy: "test-user1",
				},
				{
					Name:    "member2",
					Joined:  time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					Expires: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
					Reason:  "because again",
					AddedBy: "test-user2",
				},
			},
		},
	)
	require.NoError(t, err)
	data, err := MarshalAccessList(expected)
	require.NoError(t, err)
	actual, err := UnmarshalAccessList(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestIsOwner(t *testing.T) {
	tests := []struct {
		name             string
		identity         tlsca.Identity
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			name: "is owner",
			identity: tlsca.Identity{
				Username: ownerUser,
				Groups:   []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "is not an owner",
			identity: tlsca.Identity{
				Username: "not-owner",
				Groups:   []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "is owner with missing roles",
			identity: tlsca.Identity{
				Username: "not-owner",
				Groups:   []string{"orole1"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "is owner with missing traits",
			identity: tlsca.Identity{
				Username: "not-owner",
				Groups:   []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1"},
					"otrait2": {"ovalue3"},
				},
			},
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			accessList := newAccessList(t)
			test.errAssertionFunc(t, IsOwner(test.identity, accessList))
		})
	}
}

func TestIsMember(t *testing.T) {
	tests := []struct {
		name             string
		identity         tlsca.Identity
		memberCtx        context.Context
		currentTime      time.Time
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			name: "is member",
			identity: tlsca.Identity{
				Username: member1,
				Groups:   []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			currentTime:      time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: require.NoError,
		},
		{
			name: "is not a member",
			identity: tlsca.Identity{
				Username: member3,
				Groups:   []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			currentTime: time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name: "is expired member",
			identity: tlsca.Identity{
				Username: member1,
				Groups:   []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			currentTime: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name: "is member with missing roles",
			identity: tlsca.Identity{
				Username: member1,
				Groups:   []string{"mrole1"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			currentTime: time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "is member with missing traits",
			identity: tlsca.Identity{
				Username: member1,
				Groups:   []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1"},
					"mtrait2": {"mvalue3"},
				},
			},
			currentTime: time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			accessList := newAccessList(t)

			test.errAssertionFunc(t, IsMember(test.identity, clockwork.NewFakeClockAt(test.currentTime), accessList))
		})
	}
}

func newAccessList(t *testing.T) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: "test",
		},
		accesslist.Spec{
			Description: "test access list",
			Owners: []accesslist.Owner{
				{
					Name:        ownerUser,
					Description: "owner user",
				},
				{
					Name:        "test-user2",
					Description: "test user 2",
				},
			},
			Audit: accesslist.Audit{
				Frequency: time.Hour,
			},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			OwnershipRequires: accesslist.Requires{
				Roles: []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			Grants: accesslist.Grants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
			Members: []accesslist.Member{
				{
					Name:    member1,
					Joined:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
					Expires: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Reason:  "because",
					AddedBy: ownerUser,
				},
				{
					Name:    member2,
					Joined:  time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					Expires: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
					Reason:  "because again",
					AddedBy: ownerUser,
				},
			},
		},
	)
	require.NoError(t, err)

	return accessList
}

var accessListYAML = `---
kind: access_list
version: v1
metadata:
  name: test-access-list
spec:
  description: "test access list"  
  owners:
  - name: test-user1
    description: "test user 1"
  - name: test-user2
    description: "test user 2"
  audit:
    frequency: "1h"
  membership_requires:
    roles:
    - mrole1
    - mrole2
    traits:
      mtrait1:
      - mvalue1
      - mvalue2
      mtrait2:
      - mvalue3
      - mvalue4
  ownership_requires:
    roles:
    - orole1
    - orole2
    traits:
      otrait1:
      - ovalue1
      - ovalue2
      otrait2:
      - ovalue3
      - ovalue4
  grants:
    roles:
    - grole1
    - grole2
    traits:
      gtrait1:
      - gvalue1
      - gvalue2
      gtrait2:
      - gvalue3
      - gvalue4
  members:
  - name: member1
    joined: 2023-01-01T00:00:00Z
    expires: 2024-01-01T00:00:00Z
    reason: "because"
    added_by: "test-user1"
  - name: member2
    joined: 2022-01-01T00:00:00Z
    expires: 2025-01-01T00:00:00Z
    reason: "because again"
    added_by: "test-user2"
`
