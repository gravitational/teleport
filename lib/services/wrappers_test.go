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

package services

import (
	"encoding/hex"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/wrappers"
)

func TestUnmarshalBackwards(t *testing.T) {
	var traits wrappers.Traits

	// Attempt to unmarshal protobuf encoded data.
	protoBytes := "0a120a066c6f67696e7312080a06666f6f6261720a150a116b756265726e657465735f67726f7570731200"
	data, err := hex.DecodeString(protoBytes)
	require.NoError(t, err)
	err = wrappers.UnmarshalTraits(data, &traits)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(traits["logins"], []string{"foobar"}))

	// Attempt to unmarshal JSON encoded data.
	jsonBytes := `{"logins": ["foobar"]}`
	err = wrappers.UnmarshalTraits([]byte(jsonBytes), &traits)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(traits["logins"], []string{"foobar"}))
}
