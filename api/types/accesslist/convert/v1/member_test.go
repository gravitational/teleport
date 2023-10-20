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
	// Spec is nil
	member := ToMemberProto(newAccessListMember(t, "access-list-member"))
	member.Spec = nil

	_, err := FromMemberProto(member)
	require.Error(t, err)

	// AccessList is empty
	member = ToMemberProto(newAccessListMember(t, "access-list-member"))
	member.Spec.AccessList = ""

	_, err = FromMemberProto(member)
	require.Error(t, err)

	// Name is empty
	member = ToMemberProto(newAccessListMember(t, "access-list-member"))
	member.Spec.Name = ""

	_, err = FromMemberProto(member)
	require.Error(t, err)

	// Joined is nil
	member = ToMemberProto(newAccessListMember(t, "access-list-member"))
	member.Spec.Joined = nil

	_, err = FromMemberProto(member)
	require.Error(t, err)

	// Expires is nil
	member = ToMemberProto(newAccessListMember(t, "access-list-member"))
	member.Spec.Expires = nil

	_, err = FromMemberProto(member)
	require.NoError(t, err)

	// Reason is empty
	member = ToMemberProto(newAccessListMember(t, "access-list-member"))
	member.Spec.Reason = ""

	_, err = FromMemberProto(member)
	require.NoError(t, err)

	// AddedBy is empty
	member = ToMemberProto(newAccessListMember(t, "access-list-member"))
	member.Spec.AddedBy = ""

	_, err = FromMemberProto(member)
	require.Error(t, err)
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
