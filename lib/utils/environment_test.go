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
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/check.v1"
)

type EnvironmentSuite struct{}

var _ = check.Suite(&EnvironmentSuite{})
var _ = fmt.Printf

func (s *EnvironmentSuite) SetUpSuite(c *check.C) {
	InitLoggerForTests()
}
func (s *EnvironmentSuite) TearDownSuite(c *check.C) {}
func (s *EnvironmentSuite) SetUpTest(c *check.C)     {}
func (s *EnvironmentSuite) TearDownTest(c *check.C)  {}

func (s *EnvironmentSuite) TestReadEnvironmentFile(c *check.C) {
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
	f, err := ioutil.TempFile("", "teleport-environment-")
	c.Assert(err, check.IsNil)
	defer os.Remove(f.Name())
	_, err = f.Write(rawenv)
	c.Assert(err, check.IsNil)
	err = f.Close()
	c.Assert(err, check.IsNil)

	// read in the temp file
	env, err := ReadEnvironmentFile(f.Name())
	c.Assert(err, check.IsNil)

	// check we parsed it correctly
	c.Assert(env, check.DeepEquals, []string{"foo=bar", "foo=bar=baz", "foo="})
}
