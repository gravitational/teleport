/*
Copyright 2016-2021 Gravitational, Inc.

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

package client

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"

	"github.com/stretchr/testify/require"
)

// TestProfileBasics verifies basic profile operations such as
// load/store and setting current.
func TestProfileBasics(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "teleport")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	p := &Profile{
		WebProxyAddr:          "proxy:3088",
		SSHProxyAddr:          "proxy:3023",
		Username:              "testuser",
		ForwardedPorts:        []string{"8000:example.com:8000"},
		DynamicForwardedPorts: []string{"localhost:8080"},
	}

	// verify that profile name is proxy host component
	require.Equal(t, "proxy", p.Name())

	// save to a file:
	err = p.SaveToDir(dir, false)
	require.NoError(t, err)

	// verify that the resulting file exists and is of the form `<profile-dir>/<profile-name>.yaml`.
	_, err = os.Stat(filepath.Join(dir, p.Name()+".yaml"))
	require.NoError(t, err)

	// try to save to non-existent dir, should get an error
	err = p.SaveToDir("/bad/directory/", false)
	require.Error(t, err)

	// make sure current profile was not set
	_, err = GetCurrentProfileName(dir)
	require.True(t, trace.IsNotFound(err))

	// save again, this time also making current
	err = p.SaveToDir(dir, true)
	require.NoError(t, err)

	// verify that current profile is set and matches this profile
	name, err := GetCurrentProfileName(dir)
	require.NoError(t, err)
	require.Equal(t, p.Name(), name)

	// load and verify current profile
	clone, err := ProfileFromDir(dir, "")
	require.NoError(t, err)
	require.Equal(t, *p, *clone)

	// load and verify directly
	clone, err = ProfileFromDir(dir, p.Name())
	require.NoError(t, err)
	require.Equal(t, *p, *clone)
}
