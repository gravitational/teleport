/*
Copyright 2015-2017 Gravitational, Inc.

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
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"

	"golang.org/x/crypto/ssh"
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
	tc, err := makeClient(&conf, true)
	c.Assert(tc, check.IsNil)
	c.Assert(err, check.NotNil)

	// minimal configuration (with defaults)
	conf.Proxy = "proxy"
	conf.UserHost = "localhost"
	tc, err = makeClient(&conf, true)
	c.Assert(err, check.IsNil)
	c.Assert(tc, check.NotNil)
	c.Assert(tc.Config.SSHProxyAddr, check.Equals, "proxy:3023")
	c.Assert(tc.Config.WebProxyAddr, check.Equals, "proxy:3080")
	localUser, err := client.Username()
	c.Assert(err, check.IsNil)
	c.Assert(tc.Config.HostLogin, check.Equals, localUser)
	c.Assert(tc.Config.KeyTTL, check.Equals, defaults.CertDuration)

	// specific configuration
	conf.MinsToLive = 5
	conf.UserHost = "root@localhost"
	conf.NodePort = 46528
	conf.LocalForwardPorts = []string{"80:remote:180"}
	conf.DynamicForwardedPorts = []string{"8080"}
	tc, err = makeClient(&conf, true)
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
	c.Assert(tc.Config.DynamicForwardedPorts, check.DeepEquals, client.DynamicForwardedPorts{
		{
			SrcIP:   "127.0.0.1",
			SrcPort: 8080,
		},
	})
}

func (s *MainTestSuite) TestIdentityRead(c *check.C) {
	// 3 different types of identities
	ids := []string{
		"cert-key.pem", // cert + key concatenated togther, cert first
		"key-cert.pem", // cert + key concatenated togther, key first
		"key",          // two separate files: key and key-cert.pub
	}
	for _, id := range ids {
		// test reading:
		k, cb, err := loadIdentity(fmt.Sprintf("../../fixtures/certs/identities/%s", id))
		c.Assert(err, check.IsNil)
		c.Assert(k, check.NotNil)
		c.Assert(cb, check.IsNil)

		// test creating an auth method from the key:
		am, err := authFromIdentity(k)
		c.Assert(err, check.IsNil)
		c.Assert(am, check.NotNil)
	}
	k, _, err := loadIdentity("../../fixtures/certs/identities/lonekey")
	c.Assert(k, check.IsNil)
	c.Assert(err, check.NotNil)

	// lets read an indentity which includes a CA cert
	k, hostAuthCallback, err := loadIdentity("../../fixtures/certs/identities/key-cert-ca.pem")
	c.Assert(err, check.IsNil)
	c.Assert(k, check.NotNil)
	c.Assert(hostAuthCallback, check.NotNil)
	// prepare the cluster CA separately
	certBytes, err := ioutil.ReadFile("../../fixtures/certs/identities/ca.pem")
	c.Assert(err, check.IsNil)
	_, hosts, cert, _, _, err := ssh.ParseKnownHosts(certBytes)
	c.Assert(err, check.IsNil)
	var a net.Addr
	// host auth callback must succeed
	err = hostAuthCallback(hosts[0], a, cert)
	c.Assert(err, check.IsNil)
}
