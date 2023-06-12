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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserGroupMatchSearch(t *testing.T) {
	ug, err := NewUserGroup(Metadata{
		Name:        "test",
		Description: "description",
	})
	require.NoError(t, err)

	// Match against the name
	require.True(t, ug.MatchSearch([]string{"test"}))

	// Match against the description
	require.True(t, ug.MatchSearch([]string{"description"}))

	// No match
	require.False(t, ug.MatchSearch([]string{"nothing"}))
}
