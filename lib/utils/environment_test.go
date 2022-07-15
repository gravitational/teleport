/*
Copyright 2017 Gravitational, Inc.

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
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestReadEnvironmentFile(t *testing.T) {
	t.Parallel()

	// contents of environment file
	rawenv := []byte(`
foo=bar
# comment
foo=bar=baz
    # comment 2
=
foo=

=bar
`)

	// create a temp file with an environment in it
	f, err := os.CreateTemp("", "teleport-environment-")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.Write(rawenv)
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)

	// read in the temp file
	env, err := ReadEnvironmentFile(f.Name())
	require.NoError(t, err)

	// check we parsed it correctly
	require.Empty(t, cmp.Diff(env, []string{"foo=bar", "foo=bar=baz", "foo="}))
}
