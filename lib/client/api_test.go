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
	"github.com/gravitational/teleport/lib/utils"
	"gopkg.in/check.v1"
	"testing"
)

// register test suite
type APITestSuite struct {
}

// bootstrap check
func TestClientAPI(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&APITestSuite{})

func (s *APITestSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *APITestSuite) TestConfig(c *check.C) {
	var conf Config
	c.Assert(conf.ProxySpecified(), check.Equals, false)
	conf.ProxyHostPort = "example.org"
	c.Assert(conf.ProxySpecified(), check.Equals, true)
	c.Assert(conf.ProxySSHHostPort(), check.Equals, "example.org:3023")
	c.Assert(conf.ProxyWebHostPort(), check.Equals, "example.org:3080")

	conf.SetProxy("example.org", 100, 200)
	c.Assert(conf.ProxyWebHostPort(), check.Equals, "example.org:100")
	c.Assert(conf.ProxySSHHostPort(), check.Equals, "example.org:200")

	conf.ProxyHostPort = "example.org:200"
	c.Assert(conf.ProxyWebHostPort(), check.Equals, "example.org:200")
	c.Assert(conf.ProxySSHHostPort(), check.Equals, "example.org:3023")

	conf.ProxyHostPort = "example.org:,200"
	c.Assert(conf.ProxySSHHostPort(), check.Equals, "example.org:200")
	c.Assert(conf.ProxyWebHostPort(), check.Equals, "example.org:3080")
}

func (s *APITestSuite) TestNew(c *check.C) {
	conf := Config{
		Host:          "localhost",
		HostLogin:     "vincent",
		HostPort:      22,
		KeysDir:       "/tmp",
		Username:      "localuser",
		ProxyHostPort: "proxy",
		SiteName:      "site",
	}
	tc, err := NewClient(&conf)
	c.Assert(err, check.IsNil)
	c.Assert(tc, check.NotNil)

	la := tc.LocalAgent()
	c.Assert(la, check.NotNil)
}

func (s *APITestSuite) TestParseLabels(c *check.C) {
	// simplest case:
	m, err := ParseLabelSpec("key=value")
	c.Assert(m, check.NotNil)
	c.Assert(err, check.IsNil)
	c.Assert(m, check.DeepEquals, map[string]string{
		"key": "value",
	})
	// multiple values:
	m, err = ParseLabelSpec(`type="database";" role"=master,ver="mongoDB v1,2"`)
	c.Assert(m, check.NotNil)
	c.Assert(err, check.IsNil)
	c.Assert(m, check.HasLen, 3)
	c.Assert(m["role"], check.Equals, "master")
	c.Assert(m["type"], check.Equals, "database")
	c.Assert(m["ver"], check.Equals, "mongoDB v1,2")
	// invalid specs
	m, err = ParseLabelSpec(`type="database,"role"=master,ver="mongoDB v1,2"`)
	c.Assert(m, check.IsNil)
	c.Assert(err, check.NotNil)
	m, err = ParseLabelSpec(`type="database",role,master`)
	c.Assert(m, check.IsNil)
	c.Assert(err, check.NotNil)
}

func (s *APITestSuite) TestSCPParsing(c *check.C) {
	user, host, dest := parseSCPDestination("root@remote.host:/etc/nginx.conf")
	c.Assert(user, check.Equals, "root")
	c.Assert(host, check.Equals, "remote.host")
	c.Assert(dest, check.Equals, "/etc/nginx.conf")

	user, host, dest = parseSCPDestination("remote.host:/etc/nginx.conf")
	c.Assert(user, check.Equals, "")
	c.Assert(host, check.Equals, "remote.host")
	c.Assert(dest, check.Equals, "/etc/nginx.conf")
}

func (s *APITestSuite) TestPortsParsing(c *check.C) {
	// empty:
	ports, err := ParsePortForwardSpec(nil)
	c.Assert(ports, check.IsNil)
	c.Assert(err, check.IsNil)
	ports, err = ParsePortForwardSpec([]string{})
	c.Assert(ports, check.IsNil)
	c.Assert(err, check.IsNil)
	// not empty (but valid)
	spec := []string{
		"80:remote.host:180",
		"10.0.10.1:443:deep.host:1443",
	}
	ports, err = ParsePortForwardSpec(spec)
	c.Assert(err, check.IsNil)
	c.Assert(ports, check.HasLen, 2)
	c.Assert(ports, check.DeepEquals, ForwardedPorts{
		{
			SrcIP:    "127.0.0.1",
			SrcPort:  80,
			DestHost: "remote.host",
			DestPort: 180,
		},
		{
			SrcIP:    "10.0.10.1",
			SrcPort:  443,
			DestHost: "deep.host",
			DestPort: 1443,
		},
	})
	// back to strings:
	clone := ports.ToStringSpec()
	c.Assert(spec[0], check.Equals, clone[0])
	c.Assert(spec[1], check.Equals, clone[1])

	// parse invalid spec:
	spec = []string{"foo", "bar"}
	ports, err = ParsePortForwardSpec(spec)
	c.Assert(ports, check.IsNil)
	c.Assert(err, check.ErrorMatches, "^Invalid port forwarding spec: .foo.*")
}
