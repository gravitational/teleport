/*
Copyright 2022 Gravitational, Inc.

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
package utils

import (
	"bytes"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

// TestMarshalMapConsistency ensures serialized byte comparisons succeed
// after multiple serialize/deserialize round trips. Some JSON marshaling
// backends don't sort map keys for performance reasons, which can make
// operations that depend on the byte ordering fail (e.g. CompareAndSwap).
func TestMarshalMapConsistency(t *testing.T) {
	value := map[string]string{
		types.TeleportNamespace + "/foo": "1234",
		types.TeleportNamespace + "/bar": "5678",
	}

	compareTo, err := FastMarshal(value)
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		roundTrip := make(map[string]string)
		err := FastUnmarshal(compareTo, &roundTrip)
		require.NoError(t, err)

		val, err := FastMarshal(roundTrip)
		require.NoError(t, err)

		require.Truef(t, bytes.Equal(val, compareTo), "maps must serialize consistently (attempt %d)", i)
	}
}
