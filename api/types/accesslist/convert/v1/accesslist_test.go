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

package v1

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/types/accesslist"
	"github.com/gravitational/teleport/lib/types/header"
)

func TestRoundtrip(t *testing.T) {
	accessList := newAccessList(t, "access-list")

	converted, err := FromProto(ToProto(accessList))
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(accessList, converted))
}

// Make sure that we don't panic if any of the message fields are missing.
func TestFromProtoNils(t *testing.T) {
	// Spec is nil
	accessList := ToProto(newAccessList(t, "access-list"))
	accessList.Spec = nil

	_, err := FromProto(accessList)
	require.Error(t, err)

	// Owners is nil
	accessList = ToProto(newAccessList(t, "access-list"))
	accessList.Spec.Owners = nil

	_, err = FromProto(accessList)
	require.Error(t, err)

	// Audit is nil
	accessList = ToProto(newAccessList(t, "access-list"))
	accessList.Spec.Audit = nil

	_, err = FromProto(accessList)
	require.Error(t, err)

	// MembershipRequires is nil
	accessList = ToProto(newAccessList(t, "access-list"))
	accessList.Spec.MembershipRequires = nil

	_, err = FromProto(accessList)
	require.Error(t, err)

	// OwnershipRequires is nil
	accessList = ToProto(newAccessList(t, "access-list"))
	accessList.Spec.OwnershipRequires = nil

	_, err = FromProto(accessList)
	require.Error(t, err)

	// Grants is nil
	accessList = ToProto(newAccessList(t, "access-list"))
	accessList.Spec.Grants = nil

	_, err = FromProto(accessList)
	require.Error(t, err)

	// Members is nil
	accessList = ToProto(newAccessList(t, "access-list"))
	accessList.Spec.Members = nil

	_, err = FromProto(accessList)
	require.NoError(t, err)
}

func newAccessList(t *testing.T, name string) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: name,
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
	return accessList
}
