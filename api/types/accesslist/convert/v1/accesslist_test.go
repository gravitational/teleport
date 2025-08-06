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
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
)

func TestWithOwnersIneligibleStatusField(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	type testCase struct {
		name           string
		modificationFn func(*accesslist.AccessList)
	}

	for _, tc := range []testCase{
		{
			name:           "no-modifications",
			modificationFn: func(accessList *accesslist.AccessList) {},
		},
		{
			name: "with-subkind",
			modificationFn: func(accessList *accesslist.AccessList) {
				accessList.ResourceHeader.SetSubKind("access-list-subkind")
			},
		},
		{
			name: "deprecated-dynamic-type",
			modificationFn: func(accessList *accesslist.AccessList) {
				accessList.Spec.Type = accesslist.DeprecatedDynamic
			},
		},
		{
			name: "default-type",
			modificationFn: func(accessList *accesslist.AccessList) {
				accessList.Spec.Type = accesslist.Default
			},
		},
		{
			name: "implicit-dynamic-type",
			modificationFn: func(accessList *accesslist.AccessList) {
				accessList.Spec.Type = ""
			},
		},
		{
			name: "static-type",
			modificationFn: func(accessList *accesslist.AccessList) {
				accessList.Spec.Type = accesslist.Static
				accessList.Spec.Audit = accesslist.Audit{}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			accessList := newAccessList(t, "access-list")
			tc.modificationFn(accessList)

			converted, err := FromProto(ToProto(accessList))
			require.NoError(t, err)

			if accessList.Spec.Type == accesslist.DeprecatedDynamic {
				accessList.Spec.Type = accesslist.Default
			}

			require.Empty(t, cmp.Diff(accessList, converted))
		})
	}
}

// Make sure that we don't panic if any of the message fields are missing.
func TestFromProtoNils(t *testing.T) {
	t.Parallel()

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
		require.NoError(t, err)
	})

	t.Run("audit", func(t *testing.T) {
		accessList := ToProto(newAccessList(t, "access-list"))
		accessList.Spec.Audit = nil

		_, err := FromProto(accessList)
		require.NoError(t, err)
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

	t.Run("requires is not set to nil", func(t *testing.T) {
		acl := newAccessList(t, "access-list")
		acl.Spec.MembershipRequires = accesslist.Requires{}
		acl.Spec.OwnershipRequires = accesslist.Requires{}
		msg := ToProto(acl)
		// Ensure Requires fields are not set to nil for backward compatibility.
		// Older implementations of FromProto (e.g., in Teleport v16) may incorrectly set these fields to nil:
		// https://github.com/gravitational/teleport/blob/branch/v16/api/types/accesslist/convert/v1/accesslist.go#L46
		// Since FromProto is invoked client-side (e.g., by the Terraform provider),
		// setting Requires to nil could introduce breaking changes for existing clients.
		require.NotNil(t, msg.Spec.MembershipRequires)
		require.NotNil(t, msg.Spec.OwnershipRequires)
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

func TestConvAccessList(t *testing.T) {
	t.Parallel()

	newAccessList := func(modifyFn func(*accesslistv1.AccessList)) *accesslistv1.AccessList {
		al := &accesslistv1.AccessList{
			Header: &v1.ResourceHeader{
				Version: "v1",
				Kind:    types.KindAccessList,
				Metadata: &v1.Metadata{
					Name: "access-list",
				},
			},
			Spec: &accesslistv1.AccessListSpec{
				Title:              "test access list",
				Description:        "test description",
				OwnershipRequires:  &accesslistv1.AccessListRequires{},
				MembershipRequires: &accesslistv1.AccessListRequires{},
				Owners: []*accesslistv1.AccessListOwner{
					{
						Name: "test-user1",
					},
				},
				Audit: &accesslistv1.AccessListAudit{
					Recurrence: &accesslistv1.Recurrence{
						Frequency:  1,
						DayOfMonth: 1,
					},
					NextAuditDate: &timestamppb.Timestamp{
						Seconds: 6,
						Nanos:   1,
					},
					Notifications: &accesslistv1.Notifications{
						Start: &durationpb.Duration{
							Seconds: 1209600,
						},
					},
				},
				Grants: &accesslistv1.AccessListGrants{
					Roles: []string{"role1"},
				},
			},
			Status: &accesslistv1.AccessListStatus{},
		}
		if modifyFn != nil {
			modifyFn(al)
		}
		return al
	}

	tests := []struct {
		name  string
		input *accesslistv1.AccessList
	}{
		{
			name:  "basic conversion",
			input: newAccessList(nil),
		},
		{
			name: "nil grants",
			input: newAccessList(func(al *accesslistv1.AccessList) {
				al.Spec.Grants = nil
			}),
		},
		{
			name: "SCIM, Static access list allows for empty owners",
			input: newAccessList(func(al *accesslistv1.AccessList) {
				al.Spec.Type = string(accesslist.SCIM)
				al.Spec.Owners = []*accesslistv1.AccessListOwner{}
			}),
		},
		{
			name: "audit with only Recurrence.DayOfMonth set",
			input: newAccessList(func(al *accesslistv1.AccessList) {
				al.Spec.Type = string(accesslist.SCIM)
				al.Spec.Audit = &accesslistv1.AccessListAudit{
					Recurrence: &accesslistv1.Recurrence{
						DayOfMonth: accesslistv1.ReviewDayOfMonth_REVIEW_DAY_OF_MONTH_LAST,
					},
					Notifications: &accesslistv1.Notifications{
						Start: &durationpb.Duration{
							Seconds: 12345,
						},
					},
				}
			}),
		},
		{
			name: "audit with only Recurrence.Frequency and Notifications.Start set",
			input: newAccessList(func(al *accesslistv1.AccessList) {
				al.Spec.Type = string(accesslist.SCIM)
				al.Spec.Audit = &accesslistv1.AccessListAudit{
					Recurrence: &accesslistv1.Recurrence{
						Frequency: accesslistv1.ReviewFrequency_REVIEW_FREQUENCY_ONE_YEAR,
					},
					Notifications: &accesslistv1.Notifications{
						Start: &durationpb.Duration{},
					},
				}
			}),
		},
		{
			name: "scim-type",
			input: newAccessList(func(al *accesslistv1.AccessList) {
				al.Spec.Type = string(accesslist.SCIM)
			}),
		},
		{
			name: "static-type",
			input: newAccessList(func(al *accesslistv1.AccessList) {
				al.Spec.Type = string(accesslist.SCIM)
			}),
		},
		{
			name: "scim-type and zero audit",
			input: newAccessList(func(al *accesslistv1.AccessList) {
				al.Spec.Type = string(accesslist.SCIM)
				al.Spec.Audit = &accesslistv1.AccessListAudit{
					NextAuditDate: &timestamppb.Timestamp{},
					Recurrence: &accesslistv1.Recurrence{
						Frequency:  0,
						DayOfMonth: 0,
					},
					Notifications: &accesslistv1.Notifications{
						Start: &durationpb.Duration{},
					},
				}
			}),
		},
		{
			name: "static-type and partial audit",
			input: newAccessList(func(al *accesslistv1.AccessList) {
				al.Spec.Type = string(accesslist.Static)
				al.Spec.Audit = &accesslistv1.AccessListAudit{
					NextAuditDate: &timestamppb.Timestamp{},
					Recurrence: &accesslistv1.Recurrence{
						Frequency:  0,
						DayOfMonth: 4,
					},
					Notifications: &accesslistv1.Notifications{
						Start: &durationpb.Duration{},
					},
				}
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acl, err := FromProto(tt.input)
			require.NoError(t, err)

			got := ToProto(acl)
			require.NoError(t, err)

			require.Equal(t, tt.input, got)
		})
	}
}
