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
	"os"
	"path"

	"gopkg.in/check.v1"
)

type ProfileTestSuite struct {
}

var _ = check.Suite(&ProfileTestSuite{})

func (s *ProfileTestSuite) TestEverything(c *check.C) {
	p := &ClientProfile{
		WebProxyAddr:          "proxy:3088",
		SSHProxyAddr:          "proxy:3023",
		Username:              "testuser",
		ForwardedPorts:        []string{"8000:example.com:8000"},
		DynamicForwardedPorts: []string{"localhost:8080"},
	}

	home := c.MkDir()
	pfile := path.Join(home, "test.yaml")

	// save to a file:
	err := p.SaveTo(ProfileLocation{Path: pfile})
	c.Assert(err, check.IsNil)

	// try to save to non-existent dir, should get an error
	err = p.SaveTo(ProfileLocation{Path: "/bad/directory/profile.yaml"})
	c.Assert(err, check.NotNil)

	// make sure there is no symlink:
	symlink := path.Join(home, CurrentProfileSymlink)
	_, err = os.Stat(symlink)
	c.Assert(os.IsNotExist(err), check.Equals, true)

	// save again, this time with a symlink:
	p.SaveTo(ProfileLocation{Path: pfile, Options: ProfileMakeCurrent})
	stat, err := os.Stat(symlink)
	c.Assert(err, check.IsNil)
	c.Assert(stat.Size() > 10, check.Equals, true)

	// load and verify from symlink
	clone, err := ProfileFromDir(home, "")
	c.Assert(err, check.IsNil)
	c.Assert(*clone, check.DeepEquals, *p)

	// load and verify directly
	clone, err = ProfileFromDir(home, "test")
	c.Assert(err, check.IsNil)
	c.Assert(*clone, check.DeepEquals, *p)

	// Save with alias
	aliasPath := path.Join(home, "alias.yaml")
	err = p.SaveTo(ProfileLocation{AliasPath: aliasPath, Path: pfile, Options: ProfileMakeCurrent})
	c.Assert(err, check.IsNil)

	// Load from alias works
	clone, err = ProfileFromDir(home, "alias")
	c.Assert(err, check.IsNil)
	c.Assert(*clone, check.DeepEquals, *p)
}
