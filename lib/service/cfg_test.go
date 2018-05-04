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
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/lib/backend/dir"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

func TestConfig(t *testing.T) { check.TestingT(t) }

type ConfigSuite struct {
}

var _ = check.Suite(&ConfigSuite{})

func (s *ConfigSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *ConfigSuite) TestDefaultConfig(c *check.C) {
	config := MakeDefaultConfig()
	c.Assert(config, check.NotNil)

	// all 3 services should be enabled by default
	c.Assert(config.Auth.Enabled, check.Equals, true)
	c.Assert(config.SSH.Enabled, check.Equals, true)
	c.Assert(config.Proxy.Enabled, check.Equals, true)

	localAuthAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: "0.0.0.0:3025"}
	localProxyAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: "0.0.0.0:3023"}
	localSSHAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: "0.0.0.0:3022"}

	// data dir, hostname and auth server
	c.Assert(config.DataDir, check.Equals, defaults.DataDir)
	if len(config.Hostname) < 2 {
		c.Error("default hostname wasn't properly set")
	}

	// auth section
	auth := config.Auth
	c.Assert(auth.SSHAddr, check.DeepEquals, localAuthAddr)
	c.Assert(auth.Limiter.MaxConnections, check.Equals, int64(defaults.LimiterMaxConnections))
	c.Assert(auth.Limiter.MaxNumberOfUsers, check.Equals, defaults.LimiterMaxConcurrentUsers)
	c.Assert(config.Auth.StorageConfig.Type, check.Equals, dir.GetName())
	c.Assert(auth.StorageConfig.Params[defaults.BackendPath], check.Equals, filepath.Join(config.DataDir, defaults.BackendDir))

	// SSH section
	ssh := config.SSH
	c.Assert(ssh.Addr, check.DeepEquals, localSSHAddr)
	c.Assert(ssh.Limiter.MaxConnections, check.Equals, int64(defaults.LimiterMaxConnections))
	c.Assert(ssh.Limiter.MaxNumberOfUsers, check.Equals, defaults.LimiterMaxConcurrentUsers)

	// proxy section
	proxy := config.Proxy
	c.Assert(proxy.SSHAddr, check.DeepEquals, localProxyAddr)
	c.Assert(proxy.Limiter.MaxConnections, check.Equals, int64(defaults.LimiterMaxConnections))
	c.Assert(proxy.Limiter.MaxNumberOfUsers, check.Equals, defaults.LimiterMaxConcurrentUsers)
}
