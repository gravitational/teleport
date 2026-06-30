/*
Copyright 2026 Gravitational, Inc.

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

package provider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatSQN(t *testing.T) {
	require.Equal(t, "/staging/west::myrole", formatSQN("/staging/west", "myrole"))
}

func TestParseID(t *testing.T) {
	prefix, name, err := parseID("prefix/name")
	require.NoError(t, err)
	require.Equal(t, "prefix", prefix)
	require.Equal(t, "name", name)

	for _, id := range []string{
		"",
		"prefix",
		"prefix/name/extra",
		"/name",
		"prefix/",
	} {
		t.Run(id, func(t *testing.T) {
			_, _, err := parseID(id)
			require.Error(t, err)
		})
	}
}
