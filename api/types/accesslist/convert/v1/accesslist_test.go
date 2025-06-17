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

func TestWithOwnersIneligibleStatusField(t *testing.T) {
	proto := []*accesslistv1.AccessListOwner{
		{
			Name:             "expired",
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_EXPIRED,
		},
		{
			Name:             "missing",
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS,
		},
		{
			Name:             "dne",
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_USER_NOT_EXIST,
		},
		{
			Name:             "unspecified",
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_UNSPECIFIED,
		},
	}

	owners := []accesslist.Owner{
		{Name: "expired"},
		{Name: "missing"},
		{Name: "dne"},
		{Name: "unspecified"},
	}
	al := &accesslist.AccessList{
		Spec: accesslist.Spec{
			Owners: owners,
		},
	}
	require.Empty(t, cmp.Diff(al.Spec.Owners, owners))

	fn := WithOwnersIneligibleStatusField(proto)
	fn(al)

	require.Empty(t, cmp.Diff(al.Spec.Owners, []accesslist.Owner{
		{
			Name:             "expired",
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_EXPIRED.String(),
		},
		{
			Name:             "missing",
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_MISSING_REQUIREMENTS.String(),
		},
		{
			Name:             "dne",
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_USER_NOT_EXIST.String(),
		},
		{
			Name:             "unspecified",
			IneligibleStatus: "",
		},
	}))
}

func TestRoundtrip(t *testing.T) {
	accessList := newAccessList(t, "access-list")
	accessList.ResourceHeader.SetSubKind("access-list-subkind")

	converted, err := FromProto(ToProto(accessList))
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(accessList, converted))
}

// Make sure that we don't panic if any of the message fields are missing.
func TestFromProtoNils(t *testing.T) {
	t.Run("spec", func(t *testing.T) {
		accessList := ToProto(newAccessList(t, "access-list"))
		accessList.Spec = nil

		_, err := FromProto(accessList)
		require.Error(t, err)
	})

	t.Run("owners", func(t *testing.T) {
		accessList := ToProto(newAccessList(t, "access-list"))
		accessList.Spec.Owners = nil

		_, err := FromProto(accessList)
		require.Error(t, err)
	})

	t.Run("audit", func(t *testing.T) {
		accessList := ToProto(newAccessList(t, "access-list"))
		accessList.Spec.Audit = nil

		_, err := FromProto(accessList)
		require.Error(t, err)
	})

	t.Run("recurrence", func(t *testing.T) {
		accessList := ToProto(newAccessList(t, "access-list"))
		accessList.Spec.Audit.Recurrence = nil

		_, err := FromProto(accessList)
		require.NoError(t, err)
	})

	t.Run("notifications", func(t *testing.T) {
		accessList := ToProto(newAccessList(t, "access-list"))
		accessList.Spec.Audit.Notifications = nil

		_, err := FromProto(accessList)
		require.NoError(t, err)
	})

	t.Run("membership-requires", func(t *testing.T) {
		accessList := ToProto(newAccessList(t, "access-list"))
		accessList.Spec.MembershipRequires = nil

		_, err := FromProto(accessList)
		require.Error(t, err)
	})

	t.Run("ownership-requires", func(t *testing.T) {
		accessList := ToProto(newAccessList(t, "access-list"))
		accessList.Spec.OwnershipRequires = nil

		_, err := FromProto(accessList)
		require.Error(t, err)
	})

	t.Run("grants", func(t *testing.T) {
		accessList := ToProto(newAccessList(t, "access-list"))
		accessList.Spec.Grants = nil

		_, err := FromProto(accessList)
		require.Error(t, err)
	})

	t.Run("owner_grants", func(t *testing.T) {
		msg := ToProto(newAccessList(t, "access-list"))
		msg.Spec.OwnerGrants = nil

		_, err := FromProto(msg)
		require.NoError(t, err)
	})

	t.Run("status", func(t *testing.T) {
		msg := ToProto(newAccessList(t, "access-list"))
		msg.Status = nil

		_, err := FromProto(msg)
		require.NoError(t, err)
	})

	t.Run("member_count", func(t *testing.T) {
		msg := ToProto(newAccessList(t, "access-list"))
		msg.Status.MemberCount = nil

		_, err := FromProto(msg)
		require.NoError(t, err)
	})
}

func newAccessList(t *testing.T, name string) *accesslist.AccessList {
	t.Helper()

	memberCount := uint32(10)
	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: name,
		},
		accesslist.Spec{
			Title:       "title",
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
				NextAuditDate: time.Now(),
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
			OwnerGrants: accesslist.Grants{
				Roles: []string{"ogrole1", "ogrole2"},
				Traits: map[string][]string{
					"ogtrait1": {"ogvalue1", "ogvalue2"},
					"ogtrait2": {"ogvalue3", "ogvalue4"},
				},
			},
		},
	)
	require.NoError(t, err)

	accessList.Status = accesslist.Status{
		MemberCount: &memberCount,
	}

	return accessList
}

func TestNextAuditDateZeroTime(t *testing.T) {
	// When a proto message without expiration is converted to an AL
	// we expect next audit date to be mapped to golang's zero time. Then
	// AccessList.CheckAndSetDefaults will set a time in the future based on
	// the recurrence rules.
	accessList := ToProto(newAccessList(t, "access-list"))
	accessList.Spec.Audit.NextAuditDate = nil
	converted, err := FromProto(accessList)

	require.NoError(t, err)
	require.NotZero(
		t,
		converted.Spec.Audit.NextAuditDate.Unix(),
		"next audit date should not be epoch",
	)

	converted.Spec.Audit.NextAuditDate = time.Time{}
	// When an Access List without next audit date is converted to protobuf
	// it should be nil.
	convertedTwice := ToProto(converted)

	require.Nil(t, convertedTwice.Spec.Audit.NextAuditDate)
}
