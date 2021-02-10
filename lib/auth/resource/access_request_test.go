/*
Copyright 2021 Gravitational, Inc.

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

package resource

import (
	"testing"

	"github.com/gravitational/teleport/lib/auth"

	"github.com/stretchr/testify/require"
)

// TestAccessRequestMarshaling verifies that marshaling/unmarshaling access requests
// works as expected (failures likely indicate a problem with json schema).
func TestAccessRequestMarshaling(t *testing.T) {
	req1, err := auth.NewAccessRequest("some-user", "role-1", "role-2")
	require.NoError(t, err)

	marshaled, err := MarshalAccessRequest(req1)
	require.NoError(t, err)

	req2, err := UnmarshalAccessRequest(marshaled)
	require.NoError(t, err)

	require.True(t, req1.Equals(req2))
}
