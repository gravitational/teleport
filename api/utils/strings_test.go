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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapToStrings(t *testing.T) {
	t.Parallel()

	s := MapToStrings(map[string]string{"env": "prod", "Os": "Mac"})
	require.ElementsMatch(t, []string{"env", "prod", "Os", "Mac"}, s)
}

func TestToLowerStrings(t *testing.T) {
	t.Parallel()

	s := ToLowerStrings([]string{"FOO", "bAr", "baz"})
	require.ElementsMatch(t, []string{"foo", "bar", "baz"}, s)
}
