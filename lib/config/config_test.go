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
package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/check.v1"
)

// bootstrap check
func TestConfig(t *testing.T) { check.TestingT(t) }

// register test suite
type ConfigTestSuite struct {
	tempDir              string
	configFile           string // good
	configFileNoContent  string // empty file
	configFileBadContent string // garbage inside
	configFileStatic     string // file from a static YAML fixture
}

var _ = check.Suite(&ConfigTestSuite{})

func (s *ConfigTestSuite) SetUpSuite(c *check.C) {
	var err error
	s.tempDir, err = ioutil.TempDir("", "teleport-config")
	if err != nil {
		c.FailNow()
	}
	// create a good config file fixture
	s.configFile = filepath.Join(s.tempDir, "good-config.yaml")
	if err = ioutil.WriteFile(s.configFile, []byte(makeConfigFixture()), 0660); err != nil {
		c.FailNow()
	}
	// create a static config file fixture
	s.configFileStatic = filepath.Join(s.tempDir, "static-config.yaml")
	if err = ioutil.WriteFile(s.configFileStatic, []byte(StaticConfigString), 0660); err != nil {
		c.FailNow()
	}
	// create an empty config file
	s.configFileNoContent = filepath.Join(s.tempDir, "empty-config.yaml")
	if err = ioutil.WriteFile(s.configFileNoContent, []byte(""), 0660); err != nil {
		c.FailNow()
	}
	// create a bad config file fixture
	s.configFileBadContent = filepath.Join(s.tempDir, "bad-config.yaml")
	if err = ioutil.WriteFile(s.configFileBadContent, []byte("bad-data!"), 0660); err != nil {
		c.FailNow()
	}
}

func (s *ConfigTestSuite) TearDownSuite(c *check.C) {
	os.RemoveAll(s.tempDir)
}

func (s *ConfigTestSuite) TestConfigReading(c *check.C) {
	// invalid config file type:
	conf, err := ReadFromFile("/bin/true")
	c.Assert(conf, check.IsNil)
	c.Assert(err, check.NotNil)
	c.Assert(err, check.ErrorMatches, ".*invalid configuration file type.*")
	// non-existing file:
	conf, err = ReadFromFile("/heaven/trees/apple.ymL")
	c.Assert(conf, check.IsNil)
	c.Assert(err, check.NotNil)
	c.Assert(err, check.ErrorMatches, ".*no such file.*")
	// bad content:
	conf, err = ReadFromFile(s.configFileBadContent)
	c.Assert(err, check.NotNil)
	// mpty config (must not fail)
	conf, err = ReadFromFile(s.configFileNoContent)
	c.Assert(err, check.IsNil)
	c.Assert(conf, check.NotNil)

	// good config
	conf, err = ReadFromFile(s.configFile)
	c.Assert(err, check.IsNil)
	c.Assert(conf, check.NotNil)
	c.Assert(conf.NodeName, check.Equals, NodeName)
	c.Assert(conf.AuthServers, check.DeepEquals, AuthServers)
	c.Assert(conf.Limits.MaxConnections, check.Equals, 100)
	c.Assert(conf.Limits.MaxUsers, check.Equals, 5)
	c.Assert(conf.Limits.Rates, check.DeepEquals, ConnectionRates)
	c.Assert(conf.Logger.Output, check.Equals, "stderr")
	c.Assert(conf.Logger.Severity, check.Equals, "INFO")
	c.Assert(conf.Storage.Type, check.Equals, "bolt")
	c.Assert(conf.Storage.Param, check.Equals, `{ "path": /var/lib/teleport }`)
	c.Assert(conf.Auth.Enabled(), check.Equals, true)
	c.Assert(conf.Auth.ListenAddress, check.Equals, "tcp://auth")
	c.Assert(conf.SSH.Configured(), check.Equals, true)
	c.Assert(conf.SSH.Enabled(), check.Equals, true)
	c.Assert(conf.SSH.ListenAddress, check.Equals, "tcp://ssh")
	c.Assert(conf.SSH.Labels, check.DeepEquals, Labels)
	c.Assert(conf.SSH.Commands, check.DeepEquals, CommandLabels)
	c.Assert(conf.Proxy.Configured(), check.Equals, true)
	c.Assert(conf.Proxy.Enabled(), check.Equals, true)
	c.Assert(conf.Proxy.ListenAddress, check.Equals, "tcp://proxy")
	c.Assert(conf.Proxy.KeyFile, check.Equals, "/etc/teleport/proxy.key")
	c.Assert(conf.Proxy.CertFile, check.Equals, "/etc/teleport/proxy.crt")
	c.Assert(conf.Proxy.SSHAddr, check.Equals, "tcp://proxy_ssh_addr")
	c.Assert(conf.Proxy.WebAddr, check.Equals, "tcp://web_addr")

	// static config:
	conf, err = ReadFromFile(s.configFileStatic)
	c.Assert(err, check.IsNil)
	c.Assert(conf, check.NotNil)
	c.Assert(conf.SSH.Enabled(), check.Equals, false)      // YAML treats 'no' as False
	c.Assert(conf.Proxy.Configured(), check.Equals, false) // Missing "proxy_service" section must lead to 'not configured'
	c.Assert(conf.Proxy.Enabled(), check.Equals, true)     // Missing "proxy_service" section must lead to 'true'
}

var (
	NodeName    = "edsger.example.com"
	AuthServers = []AuthServer{
		{
			Address: "tcp://auth.server.example.org:3024",
			Token:   "xxx",
		},
		{
			Address: "tcp://auth.server.example.com:3024",
			Token:   "yyy",
		},
	}
	ConnectionRates = []ConnectionRate{
		{
			Period:  time.Minute,
			Average: 5,
			Burst:   10,
		},
		{
			Period:  time.Minute * 10,
			Average: 10,
			Burst:   100,
		},
	}
	Labels = map[string]string{
		"name": "mondoserver",
		"role": "slave",
	}
	CommandLabels = []CommandLabel{
		{
			Name:    "os",
			Command: "uname -o",
			Period:  time.Minute * 15,
		},
		{
			Name:    "hostname",
			Command: "/bin/hostname",
			Period:  time.Millisecond * 10,
		},
	}
)

// makeConfigFixture returns a valid content for teleport.yaml file
func makeConfigFixture() string {
	conf := FileConfig{}

	// common config:
	conf.NodeName = NodeName
	conf.AuthServers = AuthServers
	conf.Limits.MaxConnections = 100
	conf.Limits.MaxUsers = 5
	conf.Limits.Rates = ConnectionRates
	conf.Logger.Output = "stderr"
	conf.Logger.Severity = "INFO"
	conf.Storage.Type = "bolt"
	conf.Storage.Param = `{ "path": /var/lib/teleport }`

	// auth service:
	conf.Auth.EnabledFlag = "Yeah"
	conf.Auth.ListenAddress = "tcp://auth"

	// ssh service:
	conf.SSH.EnabledFlag = "true"
	conf.SSH.ListenAddress = "tcp://ssh"
	conf.SSH.Labels = Labels
	conf.SSH.Commands = CommandLabels

	// proxy-service:
	conf.Proxy.EnabledFlag = "yes"
	conf.Proxy.ListenAddress = "tcp://proxy"
	conf.Proxy.KeyFile = "/etc/teleport/proxy.key"
	conf.Proxy.CertFile = "/etc/teleport/proxy.crt"
	conf.Proxy.SSHAddr = "tcp://proxy_ssh_addr"
	conf.Proxy.WebAddr = "tcp://web_addr"

	return conf.DebugDumpToYAML()
}

const (
	StaticConfigString = `
#
# Some comments
#
teleport:
  nodename: edsger.example.com
  auth_servers:
  - address: tcp://auth.server.example.org:3024
    token: xxx
  log:
    output: stderr
    severity: INFO
  storage:
    type: bolt
    param: '{ "path": /var/lib/teleport }'

auth_service:
  enabled: yes
  listen_addr: tcp://auth

ssh_service:
  enabled: no
  listen_addr: tcp://ssh
  labels:
    name: mondoserver
    role: slave
  commands:
  - name: hostname
    command: /bin/hostname
    period: 10ms
`
)
