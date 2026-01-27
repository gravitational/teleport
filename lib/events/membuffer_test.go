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

package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemBuffer(t *testing.T) {
	t.Parallel()

	buf := &MemBuffer{}

	n, err := buf.WriteAt([]byte(" likes "), 5)
	require.NoError(t, err)
	assert.Equal(t, 7, n)

	n, err = buf.WriteAt([]byte("Alice"), 0)
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	n, err = buf.Write([]byte("Bob"))
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	assert.Equal(t, "Alice likes Bob", string(buf.Bytes()))
}
