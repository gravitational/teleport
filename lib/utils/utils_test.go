/*
Copyright 2015 Gravitational, Inc.

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
	"gopkg.in/check.v1"
)

type UtilsSuite struct {
}

var _ = check.Suite(&UtilsSuite{})

func (s *UtilsSuite) TestHostUUID(c *check.C) {
	// call twice, get same result
	dir := c.MkDir()
	uuid, err := ReadOrMakeHostUUID(dir)
	c.Assert(uuid, check.HasLen, 36)
	c.Assert(err, check.IsNil)
	uuidCopy, err := ReadOrMakeHostUUID(dir)
	c.Assert(err, check.IsNil)
	c.Assert(uuid, check.Equals, uuidCopy)

	// call with a read-only dir, make sure to get an error
	uuid, err = ReadOrMakeHostUUID("/")
	c.Assert(err, check.NotNil)
	c.Assert(uuid, check.Equals, "")
	c.Assert(err.Error(), check.Matches, "^.*permission error.*$")
}

func (s *UtilsSuite) TestSelfSignedCert(c *check.C) {
	creds, err := GenerateSelfSignedCert(
		[]string{"example.com"},
		[]string{"127.0.0.1"})
	c.Assert(err, check.IsNil)
	c.Assert(creds, check.NotNil)
	c.Assert(len(creds.PublicKey)/100, check.Equals, 4)
	c.Assert(len(creds.PrivateKey)/100, check.Equals, 16)
	c.Assert(len(creds.Cert)/100, check.Equals, 11)
}
