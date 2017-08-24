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
	"time"
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
	uuid, err = ReadOrMakeHostUUID("/bad-location")
	c.Assert(err, check.NotNil)
	c.Assert(uuid, check.Equals, "")
	c.Assert(err.Error(), check.Matches, "^.*no such file or directory.*$")
}

func (s *UtilsSuite) TestSelfSignedCert(c *check.C) {
	creds, err := GenerateSelfSignedCert([]string{"example.com"})
	c.Assert(err, check.IsNil)
	c.Assert(creds, check.NotNil)
	c.Assert(len(creds.PublicKey)/100, check.Equals, 4)
	c.Assert(len(creds.PrivateKey)/100, check.Equals, 16)
}

func (s *UtilsSuite) TestRandomDuration(c *check.C) {
	expectedMin := time.Duration(0)
	expectedMax := time.Second * 10
	for i := 0; i < 50; i++ {
		dur := RandomDuration(expectedMax)
		c.Assert(dur >= expectedMin, check.Equals, true)
		c.Assert(dur < expectedMax, check.Equals, true)
	}
}

func (s *UtilsSuite) TestMiscFunctions(c *check.C) {
	// SliceContainsStr
	c.Assert(SliceContainsStr([]string{"two", "one"}, "one"), check.Equals, true)
	c.Assert(SliceContainsStr([]string{"two", "one"}, "five"), check.Equals, false)
	c.Assert(SliceContainsStr([]string(nil), "one"), check.Equals, false)

	// Deduplicate
	c.Assert(Deduplicate([]string{}), check.DeepEquals, []string{})
	c.Assert(Deduplicate([]string{"a", "b"}), check.DeepEquals, []string{"a", "b"})
	c.Assert(Deduplicate([]string{"a", "b", "b", "a", "c"}), check.DeepEquals, []string{"a", "b", "c"})
}
