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

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
)

func TestMemberRoundtrip(t *testing.T) {
	member := newAccessListMember(t, "access-list-member")

	converted, err := FromMemberProto(ToMemberProto(member))
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(member, converted))
}

func TestWithMemberIneligibleStatusField(t *testing.T) {
	proto := &accesslistv1.Member{
		Spec: &accesslistv1.MemberSpec{
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_EXPIRED,
		},
	}

	alMember := &accesslist.AccessListMember{
		Spec: accesslist.AccessListMemberSpec{},
	}
	require.Empty(t, alMember.Spec.IneligibleStatus)

	fn := WithMemberIneligibleStatusField(proto)
	fn(alMember)

	require.Equal(t, accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_EXPIRED.Enum().String(), alMember.Spec.IneligibleStatus)
}

// Make sure that we don't panic if any of the message fields are missing.
func TestMemberFromProtoNils(t *testing.T) {
	testCases := []struct {
		name     string
		mutate   func(*accesslistv1.Member)
		checkErr require.ErrorAssertionFunc
		checkVal func(*testing.T, *accesslist.AccessListMember)
	}{
		{
			name:     "spec",
			mutate:   func(m *accesslistv1.Member) { m.Spec = nil },
			checkErr: require.Error,
		},
		{
			name:     "accesslist",
			mutate:   func(m *accesslistv1.Member) { m.Spec.AccessList = "" },
			checkErr: require.NoError,
		},
		{
			name:     "joined",
			mutate:   func(m *accesslistv1.Member) { m.Spec.Joined = nil },
			checkErr: require.NoError,
		},
		{
			name:     "expires",
			mutate:   func(m *accesslistv1.Member) { m.Spec.Expires = nil },
			checkErr: require.NoError,
		},
		{
			name:     "reason",
			mutate:   func(m *accesslistv1.Member) { m.Spec.Reason = "" },
			checkErr: require.NoError,
		},
		{
			name:     "added by",
			mutate:   func(m *accesslistv1.Member) { m.Spec.AddedBy = "" },
			checkErr: require.NoError,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			msg := ToMemberProto(newAccessListMember(t, "access-list-member"))
			tt.mutate(msg)

			member, err := FromMemberProto(msg)
			tt.checkErr(t, err)
			if tt.checkVal != nil {
				tt.checkVal(t, member)
			}
		})
	}
}

func TestMemberTimeConversion(t *testing.T) {
	t.Run("when zero converts to proto nil", func(t *testing.T) {
		member := newAccessListMember(t, "test_member")
		member.Spec.Expires = time.Time{}
		member.Spec.Joined = time.Time{}

		proto := ToMemberProto(member)
		require.Nil(t, proto.Spec.Expires)
		require.Nil(t, proto.Spec.Joined)
	})
	t.Run("when non-zero converts to proto time", func(t *testing.T) {
		member := newAccessListMember(t, "test_member")
		expires, err := time.Parse(time.RFC3339, "2025-10-09T15:00:00Z")
		require.NoError(t, err)
		joined, err := time.Parse(time.RFC3339, "2025-08-07T15:00:00Z")
		require.NoError(t, err)
		member.Spec.Expires = expires
		member.Spec.Joined = joined

		proto := ToMemberProto(member)
		require.NotNil(t, proto.Spec.Expires)
		require.NotNil(t, proto.Spec.Joined)
		require.True(t, proto.Spec.Expires.AsTime().Equal(expires))
		require.True(t, proto.Spec.Joined.AsTime().Equal(joined))
	})
	t.Run("proto nil converts to zero time", func(t *testing.T) {
		proto := ToMemberProto(newAccessListMember(t, "test_member"))
		proto.Spec.Expires = nil
		proto.Spec.Joined = nil

		member, err := FromMemberProto(proto)
		require.NoError(t, err)
		require.True(t, member.Spec.Expires.IsZero())
		require.True(t, member.Spec.Joined.IsZero())
	})
}

func newAccessListMember(t *testing.T, name string) *accesslist.AccessListMember {
	t.Helper()

	accessList, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: name,
		},
		accesslist.AccessListMemberSpec{
			AccessList: "access-list",
			Name:       "username",
			Joined:     time.Now(),
			Expires:    time.Now(),
			Reason:     "reason",
			AddedBy:    "some-user",
		},
	)
	require.NoError(t, err)
	return accessList
}
