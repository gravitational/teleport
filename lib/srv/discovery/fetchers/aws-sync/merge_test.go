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

	"github.com/stretchr/testify/require"
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
		Users:     append(oldUsers, newUsers...),
		Roles:     append(oldRoles, newRoles...),
		Instances: append(oldEC2, newEC2...),
	}
	require.Equal(t, &expected, result)
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
