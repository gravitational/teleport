/*
Copyright 2019 Gravitational, Inc.

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
