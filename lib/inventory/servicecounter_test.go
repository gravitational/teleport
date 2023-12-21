/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package inventory

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestServiceCounter(t *testing.T) {
	sc := serviceCounter{}

	require.Equal(t, uint64(0), sc.get(types.RoleAuth))

	sc.increment(types.RoleApp)
	require.Equal(t, uint64(1), sc.get(types.RoleApp))
	sc.increment(types.RoleApp)
	require.Equal(t, uint64(2), sc.get(types.RoleApp))

	require.Equal(t, map[types.SystemRole]uint64{
		types.RoleApp: 2,
	}, sc.counts())

	sc.decrement(types.RoleApp)
	require.Equal(t, uint64(1), sc.get(types.RoleApp))
	sc.decrement(types.RoleApp)
	require.Equal(t, uint64(0), sc.get(types.RoleApp))
}
