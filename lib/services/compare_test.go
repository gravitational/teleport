/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
)

func TestCompareResources(t *testing.T) {
	compareTestCase(t, "cmp equal", compareResource{true}, compareResource{true}, Equal)
	compareTestCase(t, "cmp not equal", compareResource{true}, compareResource{false}, Different)

	// These results should be forced since we're going through a custom compare function.
	compareTestCase(t, "IsEqual equal", &compareResourceWithEqual{true}, &compareResourceWithEqual{false}, Equal)
	compareTestCase(t, "IsEqual not equal", &compareResourceWithEqual{false}, &compareResourceWithEqual{false}, Different)

	// These results compare AccessListMemberSpec, which should ignore the IneligibleStatus field.
	newAccessListMemberSpec := func(ineligibleStatus, accessList string) accesslist.AccessListMemberSpec {
		return accesslist.AccessListMemberSpec{
			AccessList:       accessList,
			IneligibleStatus: ineligibleStatus,
		}
	}
	compareTestCase(t, "cmp equal with equal IneligibleStatus", newAccessListMemberSpec("status1", "accessList1"), newAccessListMemberSpec("status1", "accessList1"), Equal)
	compareTestCase(t, "cmp equal with different IneligibleStatus", newAccessListMemberSpec("status1", "accessList1"), newAccessListMemberSpec("status2", "accessList1"), Equal)
	compareTestCase(t, "cmp not equal", newAccessListMemberSpec("status1", "accessList1"), newAccessListMemberSpec("status1", "accessList2"), Different)

	// These results compare the IneligibleStatus field in accesslist.Owner, which should be ignored.
	newAccessListOwner := func(ineligibleStatus, name string) accesslist.Owner {
		return accesslist.Owner{
			Name:             name,
			IneligibleStatus: ineligibleStatus,
		}
	}
	compareTestCase(t, "cmp equal with equal IneligibleStatus", newAccessListOwner("status1", "alice"), newAccessListOwner("status1", "alice"), Equal)
	compareTestCase(t, "cmp equal with different IneligibleStatus", newAccessListOwner("status1", "alice"), newAccessListOwner("status2", "alice"), Equal)
	compareTestCase(t, "cmp different when name differs", newAccessListOwner("status1", "alice"), newAccessListOwner("status1", "bob"), Different)
}

func compareTestCase[T any](t *testing.T, name string, resA, resB T, expected int) {
	t.Run(name, func(t *testing.T) {
		require.Equal(t, expected, CompareResources(resA, resB))
	})
}

type compareResource struct {
	Field bool
}

type compareResourceWithEqual struct {
	ForceCompare bool
}

func (r *compareResourceWithEqual) IsEqual(_ *compareResourceWithEqual) bool {
	return r.ForceCompare
}
