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

package regular

import (
	"github.com/gravitational/teleport/lib/srv"

	"gopkg.in/check.v1"
)

type ProxyTestSuite struct {
	srv *Server
}

var _ = check.Suite(&ProxyTestSuite{})

func (s *ProxyTestSuite) SetUpSuite(c *check.C) {
	s.srv = &Server{}
	s.srv.hostname = "redhorse"
	s.srv.proxyMode = true
}

func (s *ProxyTestSuite) TestParseProxyRequest(c *check.C) {
	ctx := &srv.ServerContext{}

	// proxy request for a host:port
	subsys, err := parseProxySubsys("proxy:host:22", s.srv, ctx)
	c.Assert(err, check.IsNil)
	c.Assert(subsys, check.NotNil)
	c.Assert(subsys.srv, check.Equals, s.srv)
	c.Assert(subsys.host, check.Equals, "host")
	c.Assert(subsys.port, check.Equals, "22")
	c.Assert(subsys.clusterName, check.Equals, "")

	// similar request, just with '@' at the end (missing site)
	subsys, err = parseProxySubsys("proxy:host:22@", s.srv, ctx)
	c.Assert(err, check.IsNil)
	c.Assert(subsys.srv, check.Equals, s.srv)
	c.Assert(subsys.host, check.Equals, "host")
	c.Assert(subsys.port, check.Equals, "22")
	c.Assert(subsys.clusterName, check.Equals, "")

	// proxy request for just the sitename
	subsys, err = parseProxySubsys("proxy:@moon", s.srv, ctx)
	c.Assert(err, check.IsNil)
	c.Assert(subsys, check.NotNil)
	c.Assert(subsys.srv, check.Equals, s.srv)
	c.Assert(subsys.host, check.Equals, "")
	c.Assert(subsys.port, check.Equals, "")
	c.Assert(subsys.clusterName, check.Equals, "moon")

	// proxy request for the host:port@sitename
	subsys, err = parseProxySubsys("proxy:station:100@moon", s.srv, ctx)
	c.Assert(err, check.IsNil)
	c.Assert(subsys, check.NotNil)
	c.Assert(subsys.srv, check.Equals, s.srv)
	c.Assert(subsys.host, check.Equals, "station")
	c.Assert(subsys.port, check.Equals, "100")
	c.Assert(subsys.clusterName, check.Equals, "moon")

	// proxy request for the host:port@namespace@cluster
	subsys, err = parseProxySubsys("proxy:station:100@system@moon", s.srv, ctx)
	c.Assert(err, check.IsNil)
	c.Assert(subsys, check.NotNil)
	c.Assert(subsys.srv, check.Equals, s.srv)
	c.Assert(subsys.host, check.Equals, "station")
	c.Assert(subsys.port, check.Equals, "100")
	c.Assert(subsys.clusterName, check.Equals, "moon")
	c.Assert(subsys.namespace, check.Equals, "system")
}

func (s *ProxyTestSuite) TestParseBadRequests(c *check.C) {
	ctx := &srv.ServerContext{}

	testCases := []string{
		// empty request
		"proxy:",
		// missing hostname
		"proxy::80",
		// missing hostname and missing cluster name
		"proxy:@",
		// just random string
		"this is bad string",
	}
	for _, input := range testCases {
		comment := check.Commentf("test case: %q", input)
		_, err := parseProxySubsys(input, s.srv, ctx)
		c.Assert(err, check.NotNil, comment)
	}
}
