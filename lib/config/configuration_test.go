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

package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
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
var _ = fmt.Printf

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

func (s *ConfigTestSuite) TearDownTest(c *check.C) {
	utils.InitLoggerForTests()

}

func (s *ConfigTestSuite) SetUpTest(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *ConfigTestSuite) TestSampleConfig(c *check.C) {
	// generate sample config and write it into a temp file:
	sfc, err := MakeSampleFileConfig()
	c.Assert(err, check.IsNil)
	c.Assert(sfc, check.NotNil)
	fn := filepath.Join(c.MkDir(), "default-config.yaml")
	err = ioutil.WriteFile(fn, []byte(sfc.DebugDumpToYAML()), 0660)
	c.Assert(err, check.IsNil)

	// make sure it could be parsed:
	fc, err := ReadFromFile(fn)
	c.Assert(err, check.IsNil)

	// validate a couple of values:
	c.Assert(fc.AuthServers, check.DeepEquals, []string{fmt.Sprintf("%s:%d", defaults.Localhost, defaults.AuthListenPort)})
	c.Assert(fc.Global.DataDir, check.Equals, defaults.DataDir)
	c.Assert(fc.Logger.Severity, check.Equals, "INFO")

	c.Assert(lib.IsInsecureDevMode(), check.Equals, false)
}

// TestBooleanParsing tests that boolean options
// are parsed properly
func (s *ConfigTestSuite) TestBooleanParsing(c *check.C) {
	testCases := []struct {
		s string
		b bool
	}{
		{s: "true", b: true},
		{s: "'true'", b: true},
		{s: "yes", b: true},
		{s: "'yes'", b: true},
		{s: "'1'", b: true},
		{s: "1", b: true},
		{s: "no", b: false},
		{s: "0", b: false},
	}
	for i, tc := range testCases {
		comment := check.Commentf("test case %v", i)
		conf, err := ReadFromString(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`
teleport:
  advertise_ip: 10.10.10.1
auth_service:
  enabled: yes
  disconnect_expired_cert: %v
`, tc.s))))
		c.Assert(err, check.IsNil)
		c.Assert(conf.Auth.DisconnectExpiredCert.Value(), check.Equals, tc.b, comment)
	}
}

// TestDurationParsing tests that duration options
// are parsed properly
func (s *ConfigTestSuite) TestDuration(c *check.C) {
	testCases := []struct {
		s string
		d time.Duration
	}{
		{s: "1s", d: time.Second},
		{s: "never", d: 0},
		{s: "'1m'", d: time.Minute},
	}
	for i, tc := range testCases {
		comment := check.Commentf("test case %v", i)
		conf, err := ReadFromString(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`
teleport:
  advertise_ip: 10.10.10.1
auth_service:
  enabled: yes
  client_idle_timeout: %v
`, tc.s))))
		c.Assert(err, check.IsNil)
		c.Assert(conf.Auth.ClientIdleTimeout.Value(), check.Equals, tc.d, comment)
	}
}

func (s *ConfigTestSuite) TestConfigReading(c *check.C) {
	// non-existing file:
	conf, err := ReadFromFile("/heaven/trees/apple.ymL")
	c.Assert(conf, check.IsNil)
	c.Assert(err, check.NotNil)
	c.Assert(err, check.ErrorMatches, ".*failed to open file.*")
	// bad content:
	_, err = ReadFromFile(s.configFileBadContent)
	c.Assert(err, check.NotNil)
	// mpty config (must not fail)
	conf, err = ReadFromFile(s.configFileNoContent)
	c.Assert(err, check.IsNil)
	c.Assert(conf, check.NotNil)

	// static config
	conf, err = ReadFromFile(s.configFile)
	c.Assert(err, check.IsNil)
	c.Assert(conf, check.NotNil)
	c.Assert(conf.NodeName, check.Equals, NodeName)
	c.Assert(conf.AuthServers, check.DeepEquals, []string{"auth0.server.example.org:3024", "auth1.server.example.org:3024"})
	c.Assert(conf.Limits.MaxConnections, check.Equals, int64(100))
	c.Assert(conf.Limits.MaxUsers, check.Equals, 5)
	c.Assert(conf.Limits.Rates, check.DeepEquals, ConnectionRates)
	c.Assert(conf.Logger.Output, check.Equals, "stderr")
	c.Assert(conf.Logger.Severity, check.Equals, "INFO")
	c.Assert(conf.Storage.Type, check.Equals, "bolt")
	c.Assert(conf.DataDir, check.Equals, "/path/to/data")
	c.Assert(conf.Auth.Enabled(), check.Equals, true)
	c.Assert(conf.Auth.ListenAddress, check.Equals, "tcp://auth")
	c.Assert(conf.Auth.LicenseFile, check.Equals, "lic.pem")
	c.Assert(conf.Auth.DisconnectExpiredCert.Value(), check.Equals, true)
	c.Assert(conf.Auth.ClientIdleTimeout.Value(), check.Equals, 17*time.Second)
	c.Assert(conf.SSH.Configured(), check.Equals, true)
	c.Assert(conf.SSH.Enabled(), check.Equals, true)
	c.Assert(conf.SSH.ListenAddress, check.Equals, "tcp://ssh")
	c.Assert(conf.SSH.Labels, check.DeepEquals, Labels)
	c.Assert(conf.SSH.Commands, check.DeepEquals, CommandLabels)
	c.Assert(conf.Proxy.Configured(), check.Equals, true)
	c.Assert(conf.Proxy.Enabled(), check.Equals, true)
	c.Assert(conf.Proxy.KeyFile, check.Equals, "/etc/teleport/proxy.key")
	c.Assert(conf.Proxy.CertFile, check.Equals, "/etc/teleport/proxy.crt")
	c.Assert(conf.Proxy.ListenAddress, check.Equals, "tcp://proxy_ssh_addr")
	c.Assert(conf.Proxy.WebAddr, check.Equals, "tcp://web_addr")
	c.Assert(conf.Proxy.TunAddr, check.Equals, "reverse_tunnel_address:3311")

	// good config from file
	conf, err = ReadFromFile(s.configFileStatic)
	c.Assert(err, check.IsNil)
	c.Assert(conf, check.NotNil)
	checkStaticConfig(c, conf)

	// good config from base64 encoded string
	conf, err = ReadFromString(base64.StdEncoding.EncodeToString([]byte(StaticConfigString)))
	c.Assert(err, check.IsNil)
	c.Assert(conf, check.NotNil)
	checkStaticConfig(c, conf)
}

func (s *ConfigTestSuite) TestLabelParsing(c *check.C) {
	var conf service.SSHConfig
	var err error
	// empty spec. no errors, no labels
	err = parseLabels("", &conf)
	c.Assert(err, check.IsNil)
	c.Assert(conf.CmdLabels, check.IsNil)
	c.Assert(conf.Labels, check.IsNil)

	// simple static labels
	err = parseLabels(`key=value,more="much better"`, &conf)
	c.Assert(err, check.IsNil)
	c.Assert(conf.CmdLabels, check.NotNil)
	c.Assert(conf.CmdLabels, check.HasLen, 0)
	c.Assert(conf.Labels, check.DeepEquals, map[string]string{
		"key":  "value",
		"more": "much better",
	})

	// static labels + command labels
	err = parseLabels(`key=value,more="much better",arch=[5m2s:/bin/uname -m "p1 p2"]`, &conf)
	c.Assert(err, check.IsNil)
	c.Assert(conf.Labels, check.DeepEquals, map[string]string{
		"key":  "value",
		"more": "much better",
	})
	c.Assert(conf.CmdLabels, check.DeepEquals, services.CommandLabels{
		"arch": &services.CommandLabelV2{
			Period:  services.NewDuration(time.Minute*5 + time.Second*2),
			Command: []string{"/bin/uname", "-m", `"p1 p2"`},
		},
	})
}

func (s *ConfigTestSuite) TestTrustedClusters(c *check.C) {
	err := readTrustedClusters(nil, nil)
	c.Assert(err, check.IsNil)

	var conf service.Config
	err = readTrustedClusters([]TrustedCluster{
		{
			AllowedLogins: "vagrant, root",
			KeyFile:       "../../fixtures/trusted_clusters/cluster-a",
			TunnelAddr:    "one,two",
		},
	}, &conf)
	c.Assert(err, check.IsNil)
	authorities := conf.Auth.Authorities
	c.Assert(len(authorities), check.Equals, 2)
	c.Assert(authorities[0].GetClusterName(), check.Equals, "cluster-a")
	c.Assert(authorities[0].GetType(), check.Equals, services.HostCA)
	c.Assert(len(authorities[0].GetCheckingKeys()), check.Equals, 1)
	c.Assert(authorities[1].GetClusterName(), check.Equals, "cluster-a")
	c.Assert(authorities[1].GetType(), check.Equals, services.UserCA)
	c.Assert(len(authorities[1].GetCheckingKeys()), check.Equals, 1)
	_, _, _, _, err = ssh.ParseAuthorizedKey(authorities[1].GetCheckingKeys()[0])
	c.Assert(err, check.IsNil)

	tunnels := conf.ReverseTunnels
	c.Assert(len(tunnels), check.Equals, 1)
	c.Assert(tunnels[0].GetClusterName(), check.Equals, "cluster-a")
	c.Assert(len(tunnels[0].GetDialAddrs()), check.Equals, 2)
	c.Assert(tunnels[0].GetDialAddrs()[0], check.Equals, "tcp://one:3024")
	c.Assert(tunnels[0].GetDialAddrs()[1], check.Equals, "tcp://two:3024")

	// invalid data:
	err = readTrustedClusters([]TrustedCluster{
		{
			AllowedLogins: "vagrant, root",
			KeyFile:       "non-existing",
			TunnelAddr:    "one,two",
		},
	}, &conf)
	c.Assert(err, check.NotNil)
	c.Assert(err, check.ErrorMatches, "^.*reading trusted cluster keys.*$")
	err = readTrustedClusters([]TrustedCluster{
		{
			KeyFile:    "../../fixtures/trusted_clusters/cluster-a",
			TunnelAddr: "one,two",
		},
	}, &conf)
	c.Assert(err, check.ErrorMatches, ".*needs allow_logins parameter")
	conf.ReverseTunnels = nil
	err = readTrustedClusters([]TrustedCluster{
		{
			KeyFile:       "../../fixtures/trusted_clusters/cluster-a",
			AllowedLogins: "vagrant",
			TunnelAddr:    "",
		},
	}, &conf)
	c.Assert(err, check.IsNil)
	c.Assert(len(conf.ReverseTunnels), check.Equals, 0)
}

// TestFileConfigCheck makes sure we don't start with invalid settings.
func (s *ConfigTestSuite) TestFileConfigCheck(c *check.C) {
	tests := []struct {
		desc     string
		inConfig string
		outError bool
	}{
		{
			desc: "all defaults, valid",
			inConfig: `
teleport:
`,
		},
		{
			desc: "invalid cipher, not valid",
			inConfig: `
teleport:
  ciphers:
    - aes256-ctr
    - fake-cipher
  kex_algos:
    - kexAlgoCurve25519SHA256
  mac_algos:
    - hmac-sha2-256-etm@openssh.com
`,
			outError: true,
		},
		{
			desc: "change CA signature alg, valid",
			inConfig: `
teleport:
  ca_signature_algo: ssh-rsa
`,
		},
		{
			desc: "invalid CA signature alg, not valid",
			inConfig: `
teleport:
  ca_signature_algo: foobar
`,
			outError: true,
		},
	}

	for _, tt := range tests {
		comment := check.Commentf(tt.desc)

		_, err := ReadConfig(bytes.NewBufferString(tt.inConfig))
		if tt.outError {
			c.Assert(err, check.NotNil, comment)
		} else {
			c.Assert(err, check.IsNil, comment)
		}
	}
}

func (s *ConfigTestSuite) TestApplyConfig(c *check.C) {
	tokenPath := filepath.Join(s.tempDir, "small-config-token")
	err := ioutil.WriteFile(tokenPath, []byte("join-token"), 0644)
	c.Assert(err, check.IsNil)

	conf, err := ReadConfig(bytes.NewBufferString(fmt.Sprintf(SmallConfigString, tokenPath)))
	c.Assert(err, check.IsNil)
	c.Assert(conf, check.NotNil)
	c.Assert(conf.Proxy.PublicAddr, check.DeepEquals, utils.Strings{"web3:443"})

	cfg := service.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	c.Assert(err, check.IsNil)

	c.Assert(cfg.Token, check.Equals, "join-token")
	c.Assert(cfg.Auth.StaticTokens.GetStaticTokens(), check.DeepEquals, services.ProvisionTokensFromV1([]services.ProvisionTokenV1{
		{
			Token:   "xxx",
			Roles:   teleport.Roles([]teleport.Role{"Proxy", "Node"}),
			Expires: time.Unix(0, 0).UTC(),
		},
		{
			Token:   "yyy",
			Roles:   teleport.Roles([]teleport.Role{"Auth"}),
			Expires: time.Unix(0, 0).UTC(),
		},
	}))
	c.Assert(cfg.Auth.ClusterName.GetClusterName(), check.Equals, "magadan")
	c.Assert(cfg.Auth.ClusterConfig.GetLocalAuth(), check.Equals, true)
	c.Assert(cfg.AdvertiseIP, check.Equals, "10.10.10.1")

	c.Assert(cfg.Proxy.Enabled, check.Equals, true)
	c.Assert(cfg.Proxy.WebAddr.FullAddress(), check.Equals, "tcp://webhost:3080")
	c.Assert(cfg.Proxy.ReverseTunnelListenAddr.FullAddress(), check.Equals, "tcp://tunnelhost:1001")
}

// TestApplyConfigNoneEnabled makes sure that if a section is not enabled,
// it's fields are not read in.
func (s *ConfigTestSuite) TestApplyConfigNoneEnabled(c *check.C) {
	conf, err := ReadConfig(bytes.NewBufferString(NoServicesConfigString))
	c.Assert(err, check.IsNil)
	c.Assert(conf, check.NotNil)

	cfg := service.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	c.Assert(err, check.IsNil)

	c.Assert(cfg.Auth.Enabled, check.Equals, false)
	c.Assert(cfg.Auth.PublicAddrs, check.HasLen, 0)
	c.Assert(cfg.Proxy.Enabled, check.Equals, false)
	c.Assert(cfg.Proxy.PublicAddrs, check.HasLen, 0)
	c.Assert(cfg.SSH.Enabled, check.Equals, false)
	c.Assert(cfg.SSH.PublicAddrs, check.HasLen, 0)
}

func (s *ConfigTestSuite) TestBackendDefaults(c *check.C) {
	read := func(val string) *service.Config {
		// Default value is lite backend.
		conf, err := ReadConfig(bytes.NewBufferString(val))
		c.Assert(err, check.IsNil)
		c.Assert(conf, check.NotNil)

		cfg := service.MakeDefaultConfig()
		err = ApplyFileConfig(conf, cfg)
		c.Assert(err, check.IsNil)
		return cfg
	}

	// Default value is lite backend.
	cfg := read(`teleport:
  data_dir: /var/lib/teleport
`)
	c.Assert(cfg.Auth.StorageConfig.Type, check.Equals, lite.GetName())
	c.Assert(cfg.Auth.StorageConfig.Params[defaults.BackendPath], check.Equals, filepath.Join("/var/lib/teleport", defaults.BackendDir))

	// If no path is specified, the default is picked. In addition, internally
	// dir gets converted into lite.
	cfg = read(`teleport:
     data_dir: /var/lib/teleport
     storage:
       type: dir
`)
	c.Assert(cfg.Auth.StorageConfig.Type, check.Equals, lite.GetName())
	c.Assert(cfg.Auth.StorageConfig.Params[defaults.BackendPath], check.Equals, filepath.Join("/var/lib/teleport", defaults.BackendDir))

	// Support custom paths for dir/lite backends.
	cfg = read(`teleport:
     data_dir: /var/lib/teleport
     storage:
       type: dir
       path: /var/lib/teleport/mybackend
`)
	c.Assert(cfg.Auth.StorageConfig.Type, check.Equals, lite.GetName())
	c.Assert(cfg.Auth.StorageConfig.Params[defaults.BackendPath], check.Equals, "/var/lib/teleport/mybackend")

	// Kubernetes proxy is disabled by default.
	cfg = read(`teleport:
     data_dir: /var/lib/teleport
`)
	c.Assert(cfg.Proxy.Kube.Enabled, check.Equals, false)
}

// TestParseKey ensures that keys are parsed correctly if they are in
// authorized_keys format or known_hosts format.
func (s *ConfigTestSuite) TestParseKey(c *check.C) {
	tests := []struct {
		inCABytes      []byte
		outType        services.CertAuthType
		outClusterName string
	}{
		// 0 - host ca in known_hosts format
		{
			[]byte(`@cert-authority *.foo ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCz+PzY6z2Xa1cMeiJqOH5BRpwY+PlS3Q6C4e3Yj8xjLW1zD3Cehm71zjsYmrpuFTmdylbcKB6CcM6Ft4YbKLG3PTLSKvCPTgfSBk8RCYX02PtOV5ixwa7xl5Gfhc1GRIheXgFO9IT+W9w9ube9r002AGpkMnRRtWAWiZHMGeJoaUoCsjDLDbWsQHj06pr7fD98c7PVcVzCKPTQpadXEP6sF8w417DvypHY1bYsvhRqHw9Njx6T3b9BM3bJ4QXgy18XuO5fCpLjKLsngLwSbqe/1IP4Q0zlUaNOTph3WnjeKJZO9yQeVX1cWDwY4Iz5lSHhsJnQD99hBDdw2RklHU0j type=host`),
			services.HostCA,
			"foo",
		},
		// 1 - user ca in known_hosts format (legacy)
		{
			[]byte(`@cert-authority *.bar ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCfhrvzbHAHrukeDhLSzoXtpctiumao1MQElwhOeuzFRYwrGV/1L2gsx4OJk4ztXKOCpon1FB+dy2aJN0WIr/9qXg37D6K/XJhgDaSfW8cjpl72Lw8kknDpmgSSA3cTvzFNmXfw4DNT/klRwEw6MMrDmfT9QvaV2d35lSoMMeTZ1ilFeJqXdUkY+bgijLBQU5MUjZUfQfS3jpSxVD0DD9D1VbAE1nGSNyFqf34JxJmqJ3R5hfZqNfb9CWouv+uFF99tzOr7tnKM/sQMPGmJ5G+zjTaErNSSLiIU1iCwVKUpNFcGiR1lpOEET+neJVnEeqEqKv2ookkXaIdKjk1UKZEn type=user`),
			services.UserCA,
			"bar",
		},
		// 2 - user ca in authorized_keys format
		{
			[]byte(`cert-authority ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCiIxyz0ctsyQbKLpWVYNF+ZIOrF150Wma2GqkrOWZaOzu5NSnt9Hmp7DaIa2Gn8fh8+8vjP02qp3i43SDOlLyYSn05nJjEXaz7QGysgeppN8ayojl5dkOhA00ROpCl5HhS9cmga7fy1Uwy4jhxenNpfQ5ap0COQi3UrXPepaq8z+I4XQK//qFWnkgyD1VXCnRKXXiajOf3dShYJqLCgwYiViuFmzi2p3lysoYS5eRwTCKiyyBtlkUtpTAse455yGf3QCpe+UOBiJ/4AElxacDndtMkjjctHSPCiztnph1xej64vSy8C2nGsnPIK7RfiOzSEdd5hwva+wPLgNTcKXZz type=user&clustername=baz`),
			services.UserCA,
			"baz",
		},
	}

	// run tests
	for i, tt := range tests {
		comment := check.Commentf("Test %v", i)

		ca, _, err := parseCAKey(tt.inCABytes, []string{"foo"})
		c.Assert(err, check.IsNil, comment)
		c.Assert(ca.GetType(), check.Equals, tt.outType)
		c.Assert(ca.GetClusterName(), check.Equals, tt.outClusterName)
	}
}

func (s *ConfigTestSuite) TestParseCachePolicy(c *check.C) {
	tcs := []struct {
		in  *CachePolicy
		out *service.CachePolicy
		err error
	}{
		{in: &CachePolicy{EnabledFlag: "yes", TTL: "never"}, out: &service.CachePolicy{Enabled: true, NeverExpires: true, Type: lite.GetName()}},
		{in: &CachePolicy{EnabledFlag: "yes", TTL: "10h"}, out: &service.CachePolicy{Enabled: true, NeverExpires: false, TTL: 10 * time.Hour, Type: lite.GetName()}},
		{in: &CachePolicy{Type: memory.GetName(), EnabledFlag: "false", TTL: "10h"}, out: &service.CachePolicy{Enabled: false, NeverExpires: false, TTL: 10 * time.Hour, Type: memory.GetName()}},
		{in: &CachePolicy{Type: memory.GetName(), EnabledFlag: "yes", TTL: "never"}, out: &service.CachePolicy{Enabled: true, NeverExpires: true, Type: memory.GetName()}},
		{in: &CachePolicy{EnabledFlag: "no"}, out: &service.CachePolicy{Type: lite.GetName(), Enabled: false}},
		{in: &CachePolicy{EnabledFlag: "false", TTL: "zap"}, err: trace.BadParameter("bad format")},
		{in: &CachePolicy{Type: "memsql"}, err: trace.BadParameter("unsupported backend")},
	}
	for i, tc := range tcs {
		comment := check.Commentf("test case #%v", i)
		out, err := tc.in.Parse()
		if tc.err != nil {
			c.Assert(err, check.FitsTypeOf, err, comment)
		} else {
			c.Assert(err, check.IsNil, comment)
			fixtures.DeepCompare(c, out, tc.out)
		}
	}
}

func checkStaticConfig(c *check.C, conf *FileConfig) {
	c.Assert(conf.AuthToken, check.Equals, "xxxyyy")
	c.Assert(conf.SSH.Enabled(), check.Equals, false)      // YAML treats 'no' as False
	c.Assert(conf.Proxy.Configured(), check.Equals, false) // Missing "proxy_service" section must lead to 'not configured'
	c.Assert(conf.Proxy.Enabled(), check.Equals, true)     // Missing "proxy_service" section must lead to 'true'
	c.Assert(conf.Proxy.Disabled(), check.Equals, false)   // Missing "proxy_service" does NOT mean it's been disabled
	c.Assert(conf.AdvertiseIP, check.Equals, "10.10.10.1:3022")
	c.Assert(conf.PIDFile, check.Equals, "/var/run/teleport.pid")

	c.Assert(conf.Limits.MaxConnections, check.Equals, int64(90))
	c.Assert(conf.Limits.MaxUsers, check.Equals, 91)
	c.Assert(conf.Limits.Rates, check.HasLen, 2)
	c.Assert(conf.Limits.Rates[0].Average, check.Equals, int64(70))
	c.Assert(conf.Limits.Rates[0].Burst, check.Equals, int64(71))
	c.Assert(conf.Limits.Rates[0].Period.String(), check.Equals, "1m1s")
	c.Assert(conf.Limits.Rates[1].Average, check.Equals, int64(170))
	c.Assert(conf.Limits.Rates[1].Burst, check.Equals, int64(171))
	c.Assert(conf.Limits.Rates[1].Period.String(), check.Equals, "10m10s")

	c.Assert(conf.SSH.Disabled(), check.Equals, true) // "ssh_service" has been explicitly set to "no"
	c.Assert(conf.SSH.Commands, check.HasLen, 2)
	c.Assert(conf.SSH.Commands[0].Name, check.Equals, "hostname")
	c.Assert(conf.SSH.Commands[0].Command, check.DeepEquals, []string{"/bin/hostname"})
	c.Assert(conf.SSH.Commands[0].Period.Nanoseconds(), check.Equals, int64(10000000))
	c.Assert(conf.SSH.Commands[1].Name, check.Equals, "date")
	c.Assert(conf.SSH.Commands[1].Command, check.DeepEquals, []string{"/bin/date"})
	c.Assert(conf.SSH.Commands[1].Period.Nanoseconds(), check.Equals, int64(20000000))
	c.Assert(conf.SSH.PublicAddr, check.DeepEquals, utils.Strings{
		"luna3:22",
	})

	c.Assert(conf.Auth.Authorities[0].Type, check.Equals, services.HostCA)
	c.Assert(conf.Auth.Authorities[0].DomainName, check.Equals, "example.com")
	c.Assert(conf.Auth.Authorities[0].CheckingKeys[0], check.Equals, "checking key 1")
	c.Assert(conf.Auth.Authorities[0].CheckingKeyFiles[0], check.Equals, "/ca.checking.key")
	c.Assert(conf.Auth.Authorities[0].SigningKeys[0], check.Equals, "signing key 1")
	c.Assert(conf.Auth.Authorities[0].SigningKeyFiles[0], check.Equals, "/ca.signing.key")
	c.Assert(conf.Auth.ReverseTunnels, check.DeepEquals, []ReverseTunnel{
		{
			DomainName: "tunnel.example.com",
			Addresses:  []string{"com-1", "com-2"},
		},
		{
			DomainName: "tunnel.example.org",
			Addresses:  []string{"org-1"},
		},
	})
	c.Assert(conf.Auth.StaticTokens, check.DeepEquals,
		StaticTokens{"proxy,node:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", "auth:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})

	c.Assert(conf.Auth.PublicAddr, check.DeepEquals, utils.Strings{
		"auth.default.svc.cluster.local:3080",
	})

	policy, err := conf.CachePolicy.Parse()
	c.Assert(err, check.IsNil)
	c.Assert(policy.Enabled, check.Equals, true)
	c.Assert(policy.NeverExpires, check.Equals, false)
	c.Assert(policy.TTL, check.Equals, 20*time.Hour)
}

var (
	NodeName        = "edsger.example.com"
	AuthServers     = []string{"auth0.server.example.org:3024", "auth1.server.example.org:3024"}
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
		"name": "mongoserver",
		"role": "follower",
	}
	CommandLabels = []CommandLabel{
		{
			Name:    "os",
			Command: []string{"uname", "-o"},
			Period:  time.Minute * 15,
		},
		{
			Name:    "hostname",
			Command: []string{"/bin/hostname"},
			Period:  time.Millisecond * 10,
		},
	}
)

// makeConfigFixture returns a valid content for teleport.yaml file
func makeConfigFixture() string {
	conf := FileConfig{}

	// common config:
	conf.NodeName = NodeName
	conf.DataDir = "/path/to/data"
	conf.AuthServers = AuthServers
	conf.Limits.MaxConnections = 100
	conf.Limits.MaxUsers = 5
	conf.Limits.Rates = ConnectionRates
	conf.Logger.Output = "stderr"
	conf.Logger.Severity = "INFO"
	conf.Storage.Type = "bolt"

	// auth service:
	conf.Auth.EnabledFlag = "Yeah"
	conf.Auth.ListenAddress = "tcp://auth"
	conf.Auth.LicenseFile = "lic.pem"
	conf.Auth.ClientIdleTimeout = services.NewDuration(17 * time.Second)
	conf.Auth.DisconnectExpiredCert = services.NewBool(true)

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
	conf.Proxy.ListenAddress = "tcp://proxy_ssh_addr"
	conf.Proxy.WebAddr = "tcp://web_addr"
	conf.Proxy.TunAddr = "reverse_tunnel_address:3311"

	return conf.DebugDumpToYAML()
}

func (s *ConfigTestSuite) TestPermitUserEnvironment(c *check.C) {
	tests := []struct {
		inConfigString           string
		inPermitUserEnvironment  bool
		outPermitUserEnvironment bool
	}{
		// 0 - set on the command line, expect PermitUserEnvironment to be true
		{
			``,
			true,
			true,
		},
		// 1 - set in config file, expect PermitUserEnvironment to be true
		{
			`
ssh_service:
  permit_user_env: true
`,
			false,
			true,
		},
		// 2 - not set anywhere, expect PermitUserEnvironment to be false
		{
			``,
			false,
			false,
		},
	}

	// run tests
	for i, tt := range tests {
		comment := check.Commentf("Test %v", i)

		clf := CommandLineFlags{
			ConfigString:          base64.StdEncoding.EncodeToString([]byte(tt.inConfigString)),
			PermitUserEnvironment: tt.inPermitUserEnvironment,
		}
		cfg := service.MakeDefaultConfig()

		err := Configure(&clf, cfg)
		c.Assert(err, check.IsNil, comment)

		c.Assert(cfg.SSH.PermitUserEnvironment, check.Equals, tt.outPermitUserEnvironment, comment)
	}
}

// TestDebugFlag ensures that the debug command-line flag is correctly set in the config.
func (s *ConfigTestSuite) TestDebugFlag(c *check.C) {
	clf := CommandLineFlags{
		Debug: true,
	}
	cfg := service.MakeDefaultConfig()
	c.Assert(cfg.Debug, check.Equals, false)
	err := Configure(&clf, cfg)
	c.Assert(err, check.IsNil)
	c.Assert(cfg.Debug, check.Equals, true)
}

func (s *ConfigTestSuite) TestLicenseFile(c *check.C) {
	testCases := []struct {
		path   string
		result string
	}{
		// 0 - no license
		{
			path:   "",
			result: filepath.Join(defaults.DataDir, defaults.LicenseFile),
		},
		// 1 - relative path
		{
			path:   "lic.pem",
			result: filepath.Join(defaults.DataDir, "lic.pem"),
		},
		// 2 - absolute path
		{
			path:   "/etc/teleport/license",
			result: "/etc/teleport/license",
		},
	}

	cfg := service.MakeDefaultConfig()
	c.Assert(cfg.Auth.LicenseFile, check.Equals,
		filepath.Join(defaults.DataDir, defaults.LicenseFile))

	for _, tc := range testCases {
		err := ApplyFileConfig(&FileConfig{
			Auth: Auth{
				LicenseFile: tc.path,
			},
		}, cfg)
		c.Assert(err, check.IsNil)
		c.Assert(cfg.Auth.LicenseFile, check.Equals, tc.result)
	}
}

// TestFIPS makes sure configuration is correctly updated/enforced when in
// FedRAMP/FIPS 140-2 mode.
func (s *ConfigTestSuite) TestFIPS(c *check.C) {
	tests := []struct {
		inConfigString string
		inFIPSMode     bool
		outError       bool
	}{
		{
			inConfigString: configWithoutFIPSKex,
			inFIPSMode:     true,
			outError:       true,
		},
		{
			inConfigString: configWithoutFIPSKex,
			inFIPSMode:     false,
			outError:       false,
		},
		{
			inConfigString: configWithFIPSKex,
			inFIPSMode:     true,
			outError:       false,
		},
		{
			inConfigString: configWithFIPSKex,
			inFIPSMode:     false,
			outError:       false,
		},
	}

	for i, tt := range tests {
		comment := check.Commentf("Test %v", i)

		clf := CommandLineFlags{
			ConfigString: base64.StdEncoding.EncodeToString([]byte(tt.inConfigString)),
			FIPS:         tt.inFIPSMode,
		}

		cfg := service.MakeDefaultConfig()
		service.ApplyDefaults(cfg)
		service.ApplyFIPSDefaults(cfg)

		err := Configure(&clf, cfg)
		if tt.outError {
			c.Assert(err, check.NotNil, comment)
		} else {
			c.Assert(err, check.IsNil, comment)
		}
	}
}
