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

package utils

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestHMACAnonymizer(t *testing.T) {
	t.Parallel()

	a, err := NewHMACAnonymizer(" ")
	require.IsType(t, err, trace.BadParameter(""))
	require.Nil(t, a)

	a, err = NewHMACAnonymizer("key")
	require.NoError(t, err)
	require.NotNil(t, a)

	data := "secret"
	result := a.Anonymize([]byte(data))
	require.NotEmpty(t, result)
	require.NotEqual(t, result, data)
}
