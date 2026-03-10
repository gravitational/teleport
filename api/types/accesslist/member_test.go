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

package accesslist

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/testutils/structfill"
)

func TestAccessListMemberDefaults(t *testing.T) {
	newValidAccessListMember := func() *AccessListMember {
		accesslist := uuid.New().String()
		user := "some-user"

		return &AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Kind:    types.KindAccessListMember,
				Version: types.V1,
				Metadata: header.Metadata{
					Name: fmt.Sprintf("%s/%s", accesslist, user),
				},
			},
			Spec: AccessListMemberSpec{
				AccessList: accesslist,
				Name:       user,
				Joined:     time.Date(1969, time.July, 20, 20, 17, 40, 0, time.UTC),
				AddedBy:    "some-other-user",
			},
		}
	}

	t.Run("membership kind defaults to user", func(t *testing.T) {
		uut := newValidAccessListMember()
		uut.Spec.MembershipKind = ""

		err := uut.CheckAndSetDefaults()
		require.NoError(t, err)
		require.Equal(t, MembershipKindUser, uut.Spec.MembershipKind)
	})
}

func TestAccessListMemberClone(t *testing.T) {
	item := &AccessListMember{}
	err := structfill.Fill(item)
	require.NoError(t, err)
	cpy := item.Clone()
	require.Empty(t, cmp.Diff(item, cpy))
	require.NotSame(t, item, cpy)
}

func TestEqualAccessListMembers(t *testing.T) {
	t.Parallel()
	now := time.Now()

	tests := []struct {
		name   string
		a, b   *AccessListMember
		expect bool
	}{
		{
			name: "both equal",
			a: &AccessListMember{
				ResourceHeader: header.ResourceHeader{
					Metadata: header.Metadata{Name: "member"},
				},
				Spec: AccessListMemberSpec{
					AccessList:       "list-1",
					Name:             "alice",
					Joined:           now,
					Expires:          now.Add(time.Hour),
					Reason:           "added",
					AddedBy:          "bob",
					IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_UNSPECIFIED.String(),
					MembershipKind:   accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(),
				},
			},
			b: &AccessListMember{
				ResourceHeader: header.ResourceHeader{
					Metadata: header.Metadata{Name: "member"},
				},
				Spec: AccessListMemberSpec{
					AccessList:       "list-1",
					Name:             "alice",
					Joined:           now,
					Expires:          now.Add(time.Hour),
					Reason:           "added",
					AddedBy:          "bob",
					IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_UNSPECIFIED.String(),
					MembershipKind:   accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(),
				},
			},
			expect: true,
		},
		{
			name: "with differing ephemeral fields",
			a: &AccessListMember{
				ResourceHeader: header.ResourceHeader{
					Metadata: header.Metadata{
						Name:     "member",
						Revision: "abc",
					},
				},
				Spec: AccessListMemberSpec{
					AccessList:       "list-1",
					Name:             "alice",
					Title:            "abc",
					Joined:           now,
					Expires:          now.Add(time.Hour),
					Reason:           "added",
					AddedBy:          "bob",
					IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_UNSPECIFIED.String(),
					MembershipKind:   accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(),
				},
			},
			b: &AccessListMember{
				ResourceHeader: header.ResourceHeader{
					Metadata: header.Metadata{
						Name:     "member",
						Revision: "def",
					},
				},
				Spec: AccessListMemberSpec{
					AccessList:       "list-1",
					Name:             "alice",
					Title:            "def",
					Joined:           now,
					Expires:          now.Add(time.Hour),
					Reason:           "added",
					AddedBy:          "bob",
					IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
					MembershipKind:   accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(),
				},
			},
			expect: true,
		},
		{
			name:   "with differing non-ephemeral fields",
			a:      &AccessListMember{Spec: AccessListMemberSpec{AccessList: "a"}},
			b:      &AccessListMember{Spec: AccessListMemberSpec{AccessList: "b"}},
			expect: false,
		},
		{
			name:   "both nil",
			a:      nil,
			b:      nil,
			expect: true,
		},
		{
			name:   "non-nil and nil",
			a:      &AccessListMember{},
			b:      nil,
			expect: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expect, EqualAccessListMembers(tc.a, tc.b))
		})
	}

	t.Run("original objects are not mutated", func(t *testing.T) {
		t.Parallel()

		a := &AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name:     "member",
					Revision: "original-revision-a",
				},
			},
			Spec: AccessListMemberSpec{
				AccessList:       "list-1",
				Name:             "alice",
				Title:            "original-title-a",
				IneligibleStatus: "original-status-a",
			},
		}
		b := &AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name:     "member",
					Revision: "original-revision-b",
				},
			},
			Spec: AccessListMemberSpec{
				AccessList:       "list-1",
				Name:             "alice",
				Title:            "original-title-b",
				IneligibleStatus: "original-status-b",
			},
		}

		require.True(t, EqualAccessListMembers(a, b))
		require.Equal(t, "original-revision-a", a.Metadata.Revision)
		require.Equal(t, "original-title-a", a.Spec.Title)
		require.Equal(t, "original-status-a", a.Spec.IneligibleStatus)
		require.Equal(t, "original-revision-b", b.Metadata.Revision)
		require.Equal(t, "original-title-b", b.Spec.Title)
		require.Equal(t, "original-status-b", b.Spec.IneligibleStatus)
	})
}
