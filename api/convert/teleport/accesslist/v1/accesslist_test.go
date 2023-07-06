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

package accesslistv1

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/stretchr/testify/require"
)

func TestRoundtrip(t *testing.T) {
	accessList, err := types.NewAccessList(
		header.Metadata{
			Name: "access-list",
		},
		types.AccessListSpec{
			Description: "test access list",
			Owners: []types.AccessListOwner{
				{
					Name:        "test-user1",
					Description: "test user 1",
				},
				{
					Name:        "test-user2",
					Description: "test user 2",
				},
			},
			Audit: types.AccessListAudit{
				Frequency: time.Hour,
			},
			MembershipRequires: types.AccessListRequires{
				Roles: []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			OwnershipRequires: types.AccessListRequires{
				Roles: []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			Grants: types.AccessListGrants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
			Members: []types.AccessListMember{
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

	converted, err := FromV1(ToV1(accessList))
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(accessList, converted))
}
