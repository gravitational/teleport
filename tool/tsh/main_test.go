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

package main

import (
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"gopkg.in/check.v1"
)

// bootstrap check
func TestTshMain(t *testing.T) {
	utils.InitLoggerForTests()
	check.TestingT(t)
}

// register test suite
type MainTestSuite struct {
}

var _ = check.Suite(&MainTestSuite{})

func (s *MainTestSuite) SetUpSuite(c *check.C) {
	dir := client.FullProfilePath("")
	os.RemoveAll(dir)
}

func (s *MainTestSuite) TestMakeClient(c *check.C) {
	var conf CLIConf

	// empty config won't work:
	tc, err := makeClient(&conf)
	c.Assert(tc, check.IsNil)
	c.Assert(err, check.NotNil)

	// minimal configuration (with defaults)
	conf.Proxy = "proxy"
	conf.UserHost = "localhost"
	tc, err = makeClient(&conf)
	c.Assert(err, check.IsNil)
	c.Assert(tc, check.NotNil)
	c.Assert(tc.Config.NodeHostPort(), check.Equals, "localhost:3022")
	c.Assert(tc.Config.ProxySSHHostPort(), check.Equals, "proxy:3023")
	c.Assert(tc.Config.ProxyWebHostPort(), check.Equals, "proxy:3080")
	c.Assert(tc.Config.HostLogin, check.Equals, client.Username())
	c.Assert(tc.Config.KeyTTL, check.Equals, defaults.CertDuration)

	// specific configuration
	conf.MinsToLive = 5
	conf.UserHost = "root@localhost"
	conf.LocalForwardPorts = []string{"80:remote:180"}
	tc, err = makeClient(&conf)
	c.Assert(tc.Config.KeyTTL, check.Equals, time.Minute*time.Duration(conf.MinsToLive))
	c.Assert(tc.Config.HostLogin, check.Equals, "root")
	c.Assert(tc.Config.LocalForwardPorts, check.DeepEquals, client.ForwardedPorts{
		{
			SrcIP:    "127.0.0.1",
			SrcPort:  80,
			DestHost: "remote",
			DestPort: 180,
		},
	})
}
