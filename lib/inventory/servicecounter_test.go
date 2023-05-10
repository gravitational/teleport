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
