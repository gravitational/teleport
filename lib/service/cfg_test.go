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

package service

import (
	"testing"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

func TestConfig(t *testing.T) { TestingT(t) }

type ConfigSuite struct {
}

var _ = Suite(&ConfigSuite{})

func (s *ConfigSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *ConfigSuite) TestDefaultConfig(c *C) {
	config := MakeDefaultConfig()
	c.Assert(config, NotNil)

	// all 3 services should be enabled by default
	c.Assert(config.Auth.Enabled, Equals, true)
	c.Assert(config.SSH.Enabled, Equals, true)
	c.Assert(config.Proxy.Enabled, Equals, true)

	localAuthAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: "0.0.0.0:3025"}
	localProxyAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: "0.0.0.0:3023"}
	localSSHAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: "0.0.0.0:3022"}

	// data dir, hostname and auth server
	c.Assert(config.DataDir, Equals, defaults.DataDir)
	if len(config.Hostname) < 2 {
		c.Error("default hostname wasn't properly set")
	}

	// auth section
	auth := config.Auth
	c.Assert(auth.SSHAddr, DeepEquals, localAuthAddr)
	c.Assert(auth.Limiter.MaxConnections, Equals, int64(defaults.LimiterMaxConnections))
	c.Assert(auth.Limiter.MaxNumberOfUsers, Equals, defaults.LimiterMaxConcurrentUsers)
	c.Assert(auth.KeysBackend.Type, Equals, "bolt")

	// SSH section
	ssh := config.SSH
	c.Assert(ssh.Addr, DeepEquals, localSSHAddr)
	c.Assert(ssh.Limiter.MaxConnections, Equals, int64(defaults.LimiterMaxConnections))
	c.Assert(ssh.Limiter.MaxNumberOfUsers, Equals, defaults.LimiterMaxConcurrentUsers)

	// proxy section
	proxy := config.Proxy
	c.Assert(proxy.SSHAddr, DeepEquals, localProxyAddr)
	c.Assert(proxy.Limiter.MaxConnections, Equals, int64(defaults.LimiterMaxConnections))
	c.Assert(proxy.Limiter.MaxNumberOfUsers, Equals, defaults.LimiterMaxConcurrentUsers)
}
