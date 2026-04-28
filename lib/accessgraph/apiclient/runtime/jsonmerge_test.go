/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package runtime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONMerge(t *testing.T) {
	t.Parallel()

	got, err := JSONMerge(
		json.RawMessage(`{"a":1,"b":{"c":2},"d":[{"x":1},2]}`),
		json.RawMessage(`{"b":{"e":3},"d":{"0":{"y":2},"1":3},"f":4}`),
	)
	require.NoError(t, err)
	require.JSONEq(t, `{"a":1,"b":{"c":2,"e":3},"d":[{"x":1,"y":2},3],"f":4}`, string(got))
}
