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

import (
	"os"

	"github.com/gravitational/teleport/lib/sshutils"

	"gopkg.in/check.v1"
)

type ClientTestSuite struct {
}

var _ = check.Suite(&ClientTestSuite{})

func (s *ClientTestSuite) TestHelperFunctions(c *check.C) {
	c.Assert(nodeName("one"), check.Equals, "one")
	c.Assert(nodeName("one:22"), check.Equals, "one")
}

func (s *ClientTestSuite) TestNewSession(c *check.C) {
	nc := &NodeClient{
		Namespace: "blue",
	}

	// defaults:
	ses, err := newSession(nc, nil, nil, nil, nil, nil)
	c.Assert(err, check.IsNil)
	c.Assert(ses, check.NotNil)
	c.Assert(ses.NodeClient(), check.Equals, nc)
	c.Assert(ses.namespace, check.Equals, nc.Namespace)
	c.Assert(ses.env, check.NotNil)
	c.Assert(ses.stderr, check.Equals, os.Stderr)
	c.Assert(ses.stdout, check.Equals, os.Stdout)
	c.Assert(ses.stdin, check.Equals, os.Stdin)

	// pass environ map
	env := map[string]string{
		sshutils.SessionEnvVar: "session-id",
	}
	ses, err = newSession(nc, nil, env, nil, nil, nil)
	c.Assert(err, check.IsNil)
	c.Assert(ses, check.NotNil)
	c.Assert(ses.env, check.DeepEquals, env)
	// the session ID must be taken from tne environ map, if passed:
	c.Assert(string(ses.id), check.Equals, "session-id")
}
