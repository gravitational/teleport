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

package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangedMethods(t *testing.T) {
	head, err := parseMethodMap(filepath.Join("testdata", "ast", "head", "a_simple_test.go"), nil, nil)
	require.NoError(t, err)

	forkPoint, err := parseMethodMap(filepath.Join("testdata", "ast", "fork-point", "a_simple_test.go"), nil, nil)
	require.NoError(t, err)

	r := compare(forkPoint, head)

	assert.True(t, r.HasNew())
	assert.True(t, r.HasChanged())

	assert.Equal(t, []Method{{Name: "TestFourth", SHA1: "035a07a1e38e5387cd682b2c6b37114d187fa3d2", RefName: "TestFourth"}}, r.New)
	assert.Equal(t, []Method{{Name: "TestFirst", SHA1: "f045d205e581369b1c7c4148086c838c710f97c8", RefName: "TestFirst"}}, r.Changed)
}
