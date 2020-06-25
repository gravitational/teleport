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

package common

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	log "github.com/sirupsen/logrus"

	"gopkg.in/check.v1"
)

// bootstrap check
func TestTeleportMain(t *testing.T) { check.TestingT(t) }

// register test suite
type MainTestSuite struct {
	hostname   string
	configFile string
}

var _ = check.Suite(&MainTestSuite{})

func (s *MainTestSuite) SetUpTest(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *MainTestSuite) SetUpSuite(c *check.C) {
	var err error
	// get the hostname
	s.hostname, err = os.Hostname()
	if err != nil {
		panic(err)
	}
	// generate the fixture config file
	dirname, err := ioutil.TempDir("", "teleport")
	if err != nil {
		panic(err)
	}
	s.configFile = filepath.Join(dirname, "teleport.yaml")
	err = ioutil.WriteFile(s.configFile, []byte(YAMLConfig), 0770)
	c.Assert(err, check.IsNil)

	// set imprtant defaults to test-mode (non-existing files&locations)
	defaults.ConfigFilePath = "/tmp/teleport/etc/teleport.yaml"
	defaults.DataDir = "/tmp/teleport/var/lib/teleport"
}

func (s *MainTestSuite) TestDefault(c *check.C) {
	cmd, conf := Run(Options{
		Args:     []string{"start"},
		InitOnly: true,
	})
	c.Assert(cmd, check.Equals, "start")
	c.Assert(conf.Hostname, check.Equals, s.hostname)
	c.Assert(conf.DataDir, check.Equals, "/tmp/teleport/var/lib/teleport")
	c.Assert(conf.Auth.Enabled, check.Equals, true)
	c.Assert(conf.SSH.Enabled, check.Equals, true)
	c.Assert(conf.Proxy.Enabled, check.Equals, true)
	c.Assert(conf.Console, check.Equals, os.Stdout)
	c.Assert(log.GetLevel(), check.Equals, log.ErrorLevel)
}

func (s *MainTestSuite) TestRolesFlag(c *check.C) {
	cmd, conf := Run(Options{
		Args:     []string{"start", "--roles=node"},
		InitOnly: true,
	})
	c.Assert(conf.SSH.Enabled, check.Equals, true)
	c.Assert(conf.Auth.Enabled, check.Equals, false)
	c.Assert(conf.Proxy.Enabled, check.Equals, false)
	c.Assert(cmd, check.Equals, "start")

	cmd, conf = Run(Options{
		Args:     []string{"start", "--roles=proxy"},
		InitOnly: true,
	})
	c.Assert(conf.SSH.Enabled, check.Equals, false)
	c.Assert(conf.Auth.Enabled, check.Equals, false)
	c.Assert(conf.Proxy.Enabled, check.Equals, true)
	c.Assert(cmd, check.Equals, "start")

	cmd, conf = Run(Options{
		Args:     []string{"start", "--roles=auth"},
		InitOnly: true,
	})
	c.Assert(conf.SSH.Enabled, check.Equals, false)
	c.Assert(conf.Auth.Enabled, check.Equals, true)
	c.Assert(conf.Proxy.Enabled, check.Equals, false)
	c.Assert(cmd, check.Equals, "start")
}

func (s *MainTestSuite) TestConfigFile(c *check.C) {
	cmd, conf := Run(Options{
		Args:     []string{"start", "--roles=node", "--labels=a=a1,b=b1", "--config=" + s.configFile},
		InitOnly: true,
	})
	c.Assert(cmd, check.Equals, "start")
	c.Assert(conf.SSH.Enabled, check.Equals, true)
	c.Assert(conf.Auth.Enabled, check.Equals, false)
	c.Assert(conf.Proxy.Enabled, check.Equals, false)
	c.Assert(log.GetLevel(), check.Equals, log.DebugLevel)
	c.Assert(conf.Hostname, check.Equals, "hvostongo.example.org")
	c.Assert(conf.Token, check.Equals, "xxxyyy")
	c.Assert(conf.AdvertiseIP, check.DeepEquals, "10.5.5.5")
	c.Assert(conf.SSH.Labels, check.DeepEquals, map[string]string{"a": "a1", "b": "b1"})
}

const YAMLConfig = `
teleport:
  advertise_ip: 10.5.5.5
  nodename: hvostongo.example.org
  auth_servers:
    - auth.server.example.org:3024
  auth_token: xxxyyy
  log:
    output: stderr
    severity: DEBUG
  connection_limits:
    max_connections: 90
    max_users: 91
    rates:
    - period: 1m1s
      average: 70
      burst: 71
    - period: 10m10s
      average: 170
      burst: 171

auth_service:
  enabled: yes
  listen_addr: tcp://auth

ssh_service:
  enabled: no
  listen_addr: tcp://ssh
  labels:
    name: mondoserver
    role: follower
  commands:
  - name: hostname
    command: [/bin/hostname]
    period: 10ms
  - name: date
    command: [/bin/date]
    period: 20ms
`
