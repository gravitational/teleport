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

package aws_sync

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestMergeResources(t *testing.T) {
	oldUsers, newUsers := generateUsers()
	oldRoles, newRoles := generateRoles()
	oldEC2, newEC2 := generateEC2()

	oldResults := Resources{
		Users:     oldUsers,
		Roles:     oldRoles,
		Instances: oldEC2,
	}
	newResults := Resources{
		Users:     newUsers,
		Roles:     newRoles,
		Instances: newEC2,
	}

	result := MergeResources(&oldResults, &newResults)
	expected := Resources{
		Users:     deduplicateSlice(append(oldUsers, newUsers...), usersKey),
		Roles:     deduplicateSlice(append(oldRoles, newRoles...), roleKey),
		Instances: deduplicateSlice(append(oldEC2, newEC2...), instanceKey),
	}
	require.Empty(t, cmp.Diff(&expected, result, protocmp.Transform(), cmpopts.EquateEmpty()))
}

func TestCount(t *testing.T) {
	_, users := generateUsers()
	_, roles := generateRoles()
	_, instances := generateEC2()
	resources := Resources{
		Users:     users,
		Roles:     roles,
		Instances: instances,
	}

	count := resources.count()
	require.Equal(t, len(users)+len(roles)+len(instances), count)
}
