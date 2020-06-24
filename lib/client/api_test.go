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
	"context"
	"testing"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
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

	// multiple and unicode:
	m, err = ParseLabelSpec(`服务器环境=测试,操作系统类别=Linux,机房=华北`)
	c.Assert(err, check.IsNil)
	c.Assert(m, check.NotNil)
	c.Assert(m, check.HasLen, 3)
	c.Assert(m["服务器环境"], check.Equals, "测试")
	c.Assert(m["操作系统类别"], check.Equals, "Linux")
	c.Assert(m["机房"], check.Equals, "华北")

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
			spec:    []string{"localhost"},
			isError: true,
			output:  DynamicForwardedPorts{},
		},
		{
			spec:    []string{"localhost:123:456"},
			isError: true,
			output:  DynamicForwardedPorts{},
		},
		{
			spec:    []string{"8080"},
			isError: false,
			output: DynamicForwardedPorts{
				DynamicForwardedPort{
					SrcIP:   "127.0.0.1",
					SrcPort: 8080,
				},
			},
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
			spec:    []string{":8080:8081"},
			isError: true,
			output:  DynamicForwardedPorts{},
		},
		{
			spec:    []string{"[::1]:8080"},
			isError: false,
			output: DynamicForwardedPorts{
				DynamicForwardedPort{
					SrcIP:   "::1",
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

// TestLoginCluster makes sure the cluster name is correctly returned. This is
// to make sure "tsh login <clusterName>" correctly updates the profile.
func (s *APITestSuite) TestLoginCluster(c *check.C) {
	tests := []struct {
		inClusterName  string
		inCertGetter   *testCertGetter
		inCertificates []auth.TrustedCerts
		outClusterName string
		outError       bool
	}{
		// "tsh login", root cluster: example.com, leaf clusters: none.
		{
			inClusterName: "",
			inCertGetter:  &testCertGetter{},
			inCertificates: []auth.TrustedCerts{
				auth.TrustedCerts{
					ClusterName: "example.com",
				},
			},
			outClusterName: "example.com",
			outError:       false,
		},
		// "tsh login example.com", root cluster: example.com, leafClusters: none.
		{
			inClusterName: "example.com",
			inCertGetter:  &testCertGetter{},
			inCertificates: []auth.TrustedCerts{
				auth.TrustedCerts{
					ClusterName: "example.com",
				},
			},
			outClusterName: "example.com",
			outError:       false,
		},
		// "tsh login leaf.example.com", root cluster: example.com, leafClusters: [leaf.example.com].
		{
			inClusterName: "leaf.example.com",
			inCertGetter: &testCertGetter{
				clusterNames: []string{"leaf.example.com"},
			},
			inCertificates: []auth.TrustedCerts{
				auth.TrustedCerts{
					ClusterName: "example.com",
				},
			},
			outClusterName: "leaf.example.com",
			outError:       false,
		},
		// "tsh login invalid.example.com", root cluster: example.com, leafClusters: [leaf.example.com].
		{
			inClusterName: "invalid.example.com",
			inCertGetter: &testCertGetter{
				clusterNames: []string{"leaf.example.com"},
			},
			inCertificates: []auth.TrustedCerts{
				auth.TrustedCerts{
					ClusterName: "example.com",
				},
			},
			outClusterName: "",
			outError:       true,
		},
	}

	for _, tt := range tests {
		clusterName, err := updateClusterName(context.Background(), tt.inCertGetter, tt.inClusterName, tt.inCertificates)
		c.Assert(clusterName, check.Equals, tt.outClusterName)
		c.Assert(err != nil, check.Equals, tt.outError)
	}
}

// testCertGetter implies the certGetter interface allowing tests to simulate
// response from auth server.
type testCertGetter struct {
	clusterNames []string
}

// GetTrustedCA returns a list of trusted clusters.
func (t *testCertGetter) GetTrustedCA(ctx context.Context, clusterName string) ([]services.CertAuthority, error) {
	var cas []services.CertAuthority

	for _, clusterName := range t.clusterNames {
		// Only the cluster name is checked in tests, pass in nil for the keys.
		cas = append(cas, services.NewCertAuthority(services.HostCA, clusterName, nil, nil, nil, services.CertAuthoritySpecV2_UNKNOWN))
	}

	return cas, nil
}
