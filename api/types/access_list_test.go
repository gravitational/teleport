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

package types

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestBuilder(t *testing.T) {
	clock := clockwork.NewFakeClockAt(time.Now())

	accessList, err := NewAccessListBuilder().
		Name("accesslist1").
		Labels(map[string]string{"label1": "value1", "label2": "value2"}).
		Owners([]*AccessListOwner{
			{
				Name:        "owner1",
				Description: "description1",
			},
			{
				Name:        "owner2",
				Description: "description2",
			},
		}).
		Audit(&AccessListAudit{
			Frequency: time.Duration(time.Minute),
		}).
		MembershipRequires(&AccessListRequires{
			Roles:  []string{"role1", "role2"},
			Traits: map[string]string{"trait1": "value1", "trait2": "value2"},
		}).
		OwnershipRequires(&AccessListRequires{
			Roles:  []string{"owner-role1", "owner-role2"},
			Traits: map[string]string{"owner-trait1": "owner-value1", "owner-trait2": "owner-value2"},
		}).
		Grants(&AccessListGrants{
			Roles:  []string{"grant-role1", "grant-role2"},
			Traits: map[string]string{"grant-trait1": "grant-value1", "grant-trait2": "grant-value2"},
		}).
		Members([]*AccessListMember{
			{
				Name:    "member1",
				Joined:  clock.Now(),
				Expires: clock.Now().Add(time.Hour),
				Reason:  "reason1",
				AddedBy: "added-by1",
			},
			{
				Name:    "member2",
				Joined:  clock.Now(),
				Expires: clock.Now().Add(time.Hour),
				Reason:  "reason2",
				AddedBy: "added-by2",
			},
		}).
		Build()
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(accessList, &AccessList{
		ResourceHeader: &common.ResourceHeader{
			Kind:    KindAccessList,
			Version: V1,
			Metadata: &common.Metadata{
				Name:   "accesslist1",
				Labels: map[string]string{"label1": "value1", "label2": "value2"},
			},
		},
		Spec: &AccessListSpec{
			Owners: []*AccessListOwner{
				{
					Name:        "owner1",
					Description: "description1",
				},
				{
					Name:        "owner2",
					Description: "description2",
				},
			},
			Audit: &AccessListAudit{
				Frequency: time.Duration(time.Minute),
			},
			MembershipRequires: &AccessListRequires{
				Roles:  []string{"role1", "role2"},
				Traits: map[string]string{"trait1": "value1", "trait2": "value2"},
			},
			OwnershipRequires: &AccessListRequires{
				Roles:  []string{"owner-role1", "owner-role2"},
				Traits: map[string]string{"owner-trait1": "owner-value1", "owner-trait2": "owner-value2"},
			},
			Grants: &AccessListGrants{
				Roles:  []string{"grant-role1", "grant-role2"},
				Traits: map[string]string{"grant-trait1": "grant-value1", "grant-trait2": "grant-value2"},
			},
			Members: []*AccessListMember{
				{
					Name:    "member1",
					Joined:  clock.Now(),
					Expires: clock.Now().Add(time.Hour),
					Reason:  "reason1",
					AddedBy: "added-by1",
				},
				{
					Name:    "member2",
					Joined:  clock.Now(),
					Expires: clock.Now().Add(time.Hour),
					Reason:  "reason2",
					AddedBy: "added-by2",
				},
			},
		},
	}))
}
