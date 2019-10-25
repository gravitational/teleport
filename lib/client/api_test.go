/*
Copyright 2016-2019 Gravitational, Inc.

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
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/lib/client/extensions"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
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
	err := conf.ParseProxyHost("example.org")
	c.Assert(err, check.IsNil)
	c.Assert(conf.ProxySpecified(), check.Equals, true)
	c.Assert(conf.SSHProxyAddr, check.Equals, "example.org:3023")
	c.Assert(conf.WebProxyAddr, check.Equals, "example.org:3080")

	conf.WebProxyAddr = "example.org:100"
	conf.SSHProxyAddr = "example.org:200"
	c.Assert(conf.WebProxyAddr, check.Equals, "example.org:100")
	c.Assert(conf.SSHProxyAddr, check.Equals, "example.org:200")

	err = conf.ParseProxyHost("example.org:200")
	c.Assert(err, check.IsNil)
	c.Assert(conf.WebProxyAddr, check.Equals, "example.org:200")
	c.Assert(conf.SSHProxyAddr, check.Equals, "example.org:3023")

	err = conf.ParseProxyHost("example.org:,200")
	c.Assert(err, check.IsNil)
	c.Assert(conf.SSHProxyAddr, check.Equals, "example.org:200")
	c.Assert(conf.WebProxyAddr, check.Equals, "example.org:3080")

	conf.WebProxyAddr = "example.org:100"
	conf.SSHProxyAddr = "example.org:200"
	c.Assert(conf.WebProxyAddr, check.Equals, "example.org:100")
	c.Assert(conf.SSHProxyAddr, check.Equals, "example.org:200")
}

func (s *APITestSuite) TestNew(c *check.C) {
	conf := Config{
		Host:      "localhost",
		HostLogin: "vincent",
		HostPort:  22,
		KeysDir:   "/tmp",
		Username:  "localuser",
		SiteName:  "site",
	}
	err := conf.ParseProxyHost("proxy")
	c.Assert(err, check.IsNil)

	tc, err := NewClient(&conf)
	c.Assert(err, check.IsNil)
	c.Assert(tc, check.NotNil)

	la := tc.LocalAgent()
	c.Assert(la, check.NotNil)
}

// TestConfigureFeatures verifies that Teleport client invokes appropriate
// configurators if server supports additional features.
func (s *APITestSuite) TestConfigureFeatures(c *check.C) {
	tmpDir := c.MkDir()
	docker := &testConfigurator{}
	helm := &testConfigurator{}
	config := &Config{
		KeysDir: tmpDir,
		Configurators: map[string]extensions.Configurator{
			FeatureDocker: docker,
			FeatureHelm:   helm,
		},
	}
	err := config.ParseProxyHost("proxy")
	c.Assert(err, check.IsNil)

	profile := ClientProfile{
		WebProxyAddr: "example.com:3080",
		Username:     "alice@example.com",
	}
	err = profile.SaveTo("", filepath.Join(tmpDir, profile.Name()), ProfileMakeCurrent)
	c.Assert(err, check.IsNil)

	tc, err := NewClient(config)
	c.Assert(err, check.IsNil)
	c.Assert(tc, check.NotNil)

	// Server does not provide Docker/Helm so nothing should be configured.
	err = tc.ConfigureFeatures()
	c.Assert(err, check.IsNil)
	c.Assert(docker.configured, check.Equals, false)
	c.Assert(helm.configured, check.Equals, false)

	// Docker/Helm registries should be configured.
	tc.ServerFeatures = []string{FeatureDocker, FeatureHelm}
	err = tc.ConfigureFeatures()
	c.Assert(err, check.IsNil)
	c.Assert(docker.configured, check.Equals, true)
	c.Assert(helm.configured, check.Equals, true)
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
	clone := ports.String()
	c.Assert(spec[0], check.Equals, clone[0])
	c.Assert(spec[1], check.Equals, clone[1])

	// parse invalid spec:
	spec = []string{"foo", "bar"}
	ports, err = ParsePortForwardSpec(spec)
	c.Assert(ports, check.IsNil)
	c.Assert(err, check.ErrorMatches, "^Invalid port forwarding spec: .foo.*")
}

func (s *APITestSuite) TestDynamicPortsParsing(c *check.C) {

	tests := []struct {
		spec    []string
		isError bool
		output  DynamicForwardedPorts
	}{
		{
			spec:    nil,
			isError: false,
			output:  DynamicForwardedPorts{},
		},
		{
			spec:    []string{},
			isError: false,
			output:  DynamicForwardedPorts{},
		},
		{
			spec:    []string{"8080"},
			isError: true,
			output:  DynamicForwardedPorts{},
		},
		{
			spec:    []string{":8080"},
			isError: false,
			output: DynamicForwardedPorts{
				DynamicForwardedPort{
					SrcIP:   "127.0.0.1",
					SrcPort: 8080,
				},
			},
		},
		{
			spec:    []string{"10.0.0.1:8080"},
			isError: false,
			output: DynamicForwardedPorts{
				DynamicForwardedPort{
					SrcIP:   "10.0.0.1",
					SrcPort: 8080,
				},
			},
		},
		{
			spec:    []string{":8080", "10.0.0.1:8080"},
			isError: false,
			output: DynamicForwardedPorts{
				DynamicForwardedPort{
					SrcIP:   "127.0.0.1",
					SrcPort: 8080,
				},
				DynamicForwardedPort{
					SrcIP:   "10.0.0.1",
					SrcPort: 8080,
				},
			},
		},
	}

	for _, tt := range tests {
		specs, err := ParseDynamicPortForwardSpec(tt.spec)
		if tt.isError {
			c.Assert(err, check.NotNil)
			continue
		} else {
			c.Assert(err, check.IsNil)
		}

		c.Assert(specs, check.DeepEquals, tt.output)
	}
}

// testConfigurator is used in tests to test additional features configuration.
type testConfigurator struct {
	configured bool
}

// Configure marks the feature as configured.
func (c *testConfigurator) Configure(_ extensions.Config) error {
	c.configured = true
	return nil
}

// Deconfigure marks the feature as not configured.
func (c *testConfigurator) Deconfigure(_ extensions.Config) error {
	c.configured = false
	return nil
}

// String returns test configurator name.
func (c *testConfigurator) String() string {
	return "test"
}
