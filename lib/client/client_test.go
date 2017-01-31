/*
Copyright 2016 Gravitational, Inc.

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

import "gopkg.in/check.v1"

type ClientTestSuite struct {
}

var _ = check.Suite(&ClientTestSuite{})

func (s *ClientTestSuite) TestHelperFunctions(c *check.C) {
	c.Assert(nodeName("one"), check.Equals, "one")
	c.Assert(nodeName("one:22"), check.Equals, "one")
}
