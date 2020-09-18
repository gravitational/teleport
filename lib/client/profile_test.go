/*
Copyright 2016-2019 Gravitational, Inc.

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

	"github.com/stretchr/testify/assert"
)

// TestProfileBasics verifies basic profile operations such as
// load/store and setting current.
func TestProfileBasics(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "teleport")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	p := &ClientProfile{
		WebProxyAddr:          "proxy:3088",
		SSHProxyAddr:          "proxy:3023",
		Username:              "testuser",
		ForwardedPorts:        []string{"8000:example.com:8000"},
		DynamicForwardedPorts: []string{"localhost:8080"},
	}

	// verify that profile name is proxy host component
	assert.Equal(t, "proxy", p.Name())

	// save to a file:
	err = p.SaveToDir(dir, false)
	assert.NoError(t, err)

	// verify that the resulting file exists and is of the form `<profile-dir>/<profile-name>.yaml`.
	_, err = os.Stat(filepath.Join(dir, p.Name()+".yaml"))
	assert.NoError(t, err)

	// try to save to non-existent dir, should get an error
	err = p.SaveToDir("/bad/directory/", false)
	assert.Error(t, err)

	// make sure current profile was not set
	_, err = GetCurrentProfileName(dir)
	assert.True(t, trace.IsNotFound(err))

	// save again, this time also making current
	err = p.SaveToDir(dir, true)
	assert.NoError(t, err)

	// verify that current profile is set and matches this profile
	name, err := GetCurrentProfileName(dir)
	assert.NoError(t, err)
	assert.Equal(t, p.Name(), name)

	// load and verify current profile
	clone, err := ProfileFromDir(dir, "")
	assert.NoError(t, err)
	assert.Equal(t, *p, *clone)

	// load and verify directly
	clone, err = ProfileFromDir(dir, p.Name())
	assert.NoError(t, err)
	assert.Equal(t, *p, *clone)
}

// TestProfileSymlinkMigration verifies that the old `profile` symlink
// is correctly migrated to the new `current-profile` file.
//
// DELETE IN: 6.0
func TestProfileSymlinkMigration(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "teleport")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	name := "some-profile"
	file := filepath.Join(dir, name+".yaml")
	link := filepath.Join(dir, CurrentProfileSymlink)

	// note that we don't bother to create the actual profile; this
	// migration deals solely with converting the `profile` symlink
	// to a `current-profile` file.

	// create old style symlink
	assert.NoError(t, os.Symlink(file, link))

	// ensure that link exists
	_, err = os.Lstat(link)
	assert.NoError(t, err)

	// load current profile name; this should automatically
	// trigger the migration and return the correct name.
	cn, err := GetCurrentProfileName(dir)
	assert.NoError(t, err)
	assert.Equal(t, name, cn)

	// verify that current-profile file now exists
	_, err = os.Stat(filepath.Join(dir, CurrentProfileFilename))
	assert.NoError(t, err)

	// forcibly remove the symlink
	assert.NoError(t, os.Remove(link))

	// loading current profile should still succeed.
	cn, err = GetCurrentProfileName(dir)
	assert.NoError(t, err)
	assert.Equal(t, name, cn)
}
