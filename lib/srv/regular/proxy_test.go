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
	"github.com/gravitational/teleport/lib/defaults"
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

	tt := []struct {
		req, host, port, cluster, namespace string
	}{
		{ // proxy request for a host:port
			req:  "proxy:host:22",
			host: "host",
			port: "22",
		},
		{ // similar request, just with '@' at the end (missing site)
			req:  "proxy:host:22@",
			host: "host",
			port: "22",
		},
		{ // proxy request for just the sitename
			req:     "proxy:@moon",
			cluster: "moon",
		},
		{ // proxy request for the host:port@sitename
			req:     "proxy:station:100@moon",
			host:    "station",
			port:    "100",
			cluster: "moon",
		},
		{ // proxy request for the host:port@namespace@cluster
			req:       "proxy:station:100@system@moon",
			host:      "station",
			port:      "100",
			cluster:   "moon",
			namespace: "system",
		},
	}

	for i, t := range tt {
		if t.namespace == "" {
			// test cases without a defined namespace are testing for
			// the presence of the default namespace; namespace should
			// never actually be empty.
			t.namespace = defaults.Namespace
		}
		cmt := check.Commentf("Test case %d: %+v", i, t)
		req, err := parseProxySubsysRequest(t.req)
		c.Assert(err, check.IsNil, cmt)
		c.Assert(req.host, check.Equals, t.host, cmt)
		c.Assert(req.port, check.Equals, t.port, cmt)
		c.Assert(req.clusterName, check.Equals, t.cluster, cmt)
		c.Assert(req.namespace, check.Equals, t.namespace, cmt)
	}
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
