// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestIdentityCenterAccountClone(t *testing.T) {
	// GIVEN an Account Record
	src := IdentityCenterAccount{
		Account: &identitycenterv1.Account{
			Kind:     types.KindIdentityCenterAccount,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{Name: "some-account"},
			Spec: &identitycenterv1.AccountSpec{
				Id:          "aws-account-id",
				Arn:         "arn:aws:sso::account-id:",
				Description: "Test account",
				PermissionSetInfo: []*identitycenterv1.PermissionSetInfo{
					{
						Name: "original value",
						Arn:  "arn:aws:sso:::permissionSet/ic-instance/ps-instance",
					},
				},
			},
		},
	}

	// WHEN I clone the resource
	dst := src.CloneResource().(IdentityCenterAccount)

	// EXPECT that the resulting clone compares equally
	require.Equal(t, src, dst)

	// WHEN I modify the source object in a way that would be shared with a
	// shallow copy
	src.Spec.PermissionSetInfo[0].Name = "some new value"

	// EXPECT that the cloned object DOES NOT inherit the update
	require.NotEqual(t, src, dst)
	require.Equal(t, "original value", dst.Spec.PermissionSetInfo[0].Name)
}

func TestIdentityCenterAccountAssignmentClone(t *testing.T) {
	// GIVEN an Account Assignment Record
	src := IdentityCenterAccountAssignment{
		AccountAssignment: &identitycenterv1.AccountAssignment{
			Kind:     types.KindIdentityCenterAccountAssignment,
			Version:  types.V1,
			Metadata: &headerv1.Metadata{Name: "u-test@example.com"},
			Spec: &identitycenterv1.AccountAssignmentSpec{
				Display: "Some-Permission-set on Some-AWS-account",
				PermissionSet: &identitycenterv1.PermissionSetInfo{
					Arn:  "arn:aws:sso:::permissionSet/ic-instance/ps-instance",
					Name: "original name",
				},
				AccountName: "Some Account Name",
				AccountId:   "some account id",
			},
		},
	}

	// WHEN I clone the resource
	dst := src.CloneResource().(IdentityCenterAccountAssignment)

	// EXPECT that the resulting clone compares equally
	require.Equal(t, src, dst)

	// WHEN I modify the source object in a way that would be shared with a
	// shallow copy
	src.Spec.PermissionSet.Name = "some new name"

	// EXPECT that the cloned object DOES NOT inherit the update
	require.NotEqual(t, src, dst)
	require.Equal(t, "original name", dst.Spec.PermissionSet.Name)
}
