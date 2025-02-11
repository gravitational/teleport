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

package services

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestValidateWorkloadIdentityX509Revocation(t *testing.T) {
	t.Parallel()

	var errContains = func(contains string) require.ErrorAssertionFunc {
		return func(t require.TestingT, err error, msgAndArgs ...interface{}) {
			require.ErrorContains(t, err, contains, msgAndArgs...)
		}
	}

	testCases := []struct {
		name       string
		in         *workloadidentityv1pb.WorkloadIdentityX509Revocation
		requireErr require.ErrorAssertionFunc
	}{
		{
			name: "success - full",
			in: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
				Kind:    types.KindWorkloadIdentityX509Revocation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:    "aabbccddeeff",
					Expires: timestamppb.New(time.Now().Add(time.Hour)),
				},
				Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
					Reason:    "compromised",
					RevokedAt: timestamppb.Now(),
				},
			},
			requireErr: require.NoError,
		},
		{
			name: "missing name",
			in: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
				Kind:    types.KindWorkloadIdentityX509Revocation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Expires: timestamppb.New(time.Now().Add(time.Hour)),
				},
				Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
					Reason:    "compromised",
					RevokedAt: timestamppb.Now(),
				},
			},
			requireErr: errContains("metadata.name: is required"),
		},
		{
			name: "missing reason",
			in: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
				Kind:    types.KindWorkloadIdentityX509Revocation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:    "aabbccddeeff",
					Expires: timestamppb.New(time.Now().Add(time.Hour)),
				},
				Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
					RevokedAt: timestamppb.Now(),
				},
			},
			requireErr: errContains("spec.reason: is required"),
		},
		{
			name: "invalid name: colons",
			in: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
				Kind:    types.KindWorkloadIdentityX509Revocation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:    "aa:bb:cc:dd:ee:ff",
					Expires: timestamppb.New(time.Now().Add(time.Hour)),
				},
				Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
					Reason:    "compromised",
					RevokedAt: timestamppb.Now(),
				},
			},
			requireErr: errContains("metadata.name: must be a hex encoded integer without colons"),
		},
		{
			name: "invalid name: not lowercase",
			in: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
				Kind:    types.KindWorkloadIdentityX509Revocation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:    "AAbbCCddEE",
					Expires: timestamppb.New(time.Now().Add(time.Hour)),
				},
				Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
					Reason:    "compromised",
					RevokedAt: timestamppb.Now(),
				},
			},
			requireErr: errContains("metadata.name: must be a lower-case encoded hex string"),
		},
		{
			name: "invalid name: not base 16",
			in: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
				Kind:    types.KindWorkloadIdentityX509Revocation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:    "aabbxx",
					Expires: timestamppb.New(time.Now().Add(time.Hour)),
				},
				Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
					Reason:    "compromised",
					RevokedAt: timestamppb.Now(),
				},
			},
			requireErr: errContains("metadata.name: must be a hex encoded integer without colons"),
		},
		{
			name: "missing expiry",
			in: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
				Kind:    types.KindWorkloadIdentityX509Revocation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "aabbccddeeff",
				},
				Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
					Reason:    "compromised",
					RevokedAt: timestamppb.Now(),
				},
			},
			requireErr: errContains("metadata.expires: is required"),
		},
		{
			name: "missing revoked at",
			in: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
				Kind:    types.KindWorkloadIdentityX509Revocation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:    "aabbccddeeff",
					Expires: timestamppb.New(time.Now().Add(time.Hour)),
				},
				Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
					Reason: "compromised",
				},
			},
			requireErr: errContains("spec.revoked_at: is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateWorkloadIdentityX509Revocation(tc.in)
			tc.requireErr(t, err)
		})
	}
}

func TestWorkloadIdentityX509RevocationMarshaling(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		in   *workloadidentityv1pb.WorkloadIdentityX509Revocation
	}{
		{
			name: "normal",
			in: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
				Kind:    types.KindWorkloadIdentityX509Revocation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:    "aabbccddeeff",
					Expires: timestamppb.New(time.Now().Add(time.Hour)),
				},
				Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
					Reason:    "compromised",
					RevokedAt: timestamppb.Now(),
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotBytes, err := MarshalWorkloadIdentityX509Revocation(tc.in)
			require.NoError(t, err)
			// Test that unmarshaling gives us the same object
			got, err := UnmarshalWorkloadIdentityX509Revocation(gotBytes)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tc.in, got, protocmp.Transform()))
		})
	}
}
