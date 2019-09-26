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

package configurator

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type ConfiguratorSuite struct{}

var _ = check.Suite(&ConfiguratorSuite{})

func (s *ConfiguratorSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *ConfiguratorSuite) TestSymlinks(c *check.C) {
	// Create two temporary directories and two files in the first directory.
	tmpDir1 := c.MkDir()
	tmpDir2 := c.MkDir()
	file1 := filepath.Join(tmpDir1, "file1")
	file2 := filepath.Join(tmpDir1, "file2")
	c.Assert(ioutil.WriteFile(file1, nil, 0644), check.IsNil)
	c.Assert(ioutil.WriteFile(file2, nil, 0644), check.IsNil)

	// Request and verify two symlinks:
	//  - /tmpdir2/file1 -> /tmpdir1/file1
	//  - /tmpdir2/file2 -> /tmpdir2/file2
	symlinks := map[string]string{
		file1: filepath.Join(tmpDir2, "file1"),
		file2: filepath.Join(tmpDir2, "file2"),
	}

	ok, err := verifySymlinks(symlinks)
	c.Assert(err, check.IsNil)
	c.Assert(ok, check.Equals, false)

	err = ensureSymlinks(symlinks)
	c.Assert(err, check.IsNil)

	ok, err = verifySymlinks(symlinks)
	c.Assert(err, check.IsNil)
	c.Assert(ok, check.Equals, true)

	// Now shuffle symlinks and verify again:
	//  - /tmpdir2/file1 -> /tmpdir1/file2
	//  - /tmpdir2/file2 -> /tmpdir2/file1
	symlinks = map[string]string{
		file1: filepath.Join(tmpDir2, "file2"),
		file2: filepath.Join(tmpDir2, "file1"),
	}

	ok, err = verifySymlinks(symlinks)
	c.Assert(err, check.IsNil)
	c.Assert(ok, check.Equals, false)

	err = ensureSymlinks(symlinks)
	c.Assert(err, check.IsNil)

	ok, err = verifySymlinks(symlinks)
	c.Assert(err, check.IsNil)
	c.Assert(ok, check.Equals, true)
}
