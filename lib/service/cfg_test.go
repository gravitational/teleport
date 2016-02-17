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
	"time"

	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

func TestConfig(t *testing.T) { TestingT(t) }

type ConfigSuite struct {
}

var _ = Suite(&ConfigSuite{})

func (s *ConfigSuite) SetUpSuite(c *C) {
	utils.InitLoggerCLI()
}

func (s *ConfigSuite) checkVariables(c *C, cfg *Config) {
	// check common section
	c.Assert(cfg.DataDir, Equals, "/tmp/data_dir")
	c.Assert(cfg.Hostname, Equals, "domain.example.com")
	c.Assert(cfg.AuthServers, DeepEquals, NetAddrSlice{
		{AddrNetwork: "tcp", Addr: "localhost:5000"},
		{AddrNetwork: "unix", Addr: "/var/run/auth.sock"},
	})

	// auth section
	c.Assert(cfg.Auth.Enabled, Equals, true)
	c.Assert(cfg.Auth.SSHAddr, Equals,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:5555"})
	c.Assert(cfg.Auth.HostAuthorityDomain, Equals, "a.domain.example.com")
	c.Assert(cfg.Auth.Token, Equals, "authtoken")
	c.Assert(cfg.Auth.SecretKey, Equals, "authsecret")

	c.Assert(cfg.Auth.AllowedTokens, DeepEquals,
		KeyVal{
			"ntoken1": "node1.a.domain.example.com",
			"atoken2": "node2.a.domain.example.com",
		})

	c.Assert(cfg.Auth.TrustedAuthorities, DeepEquals,
		CertificateAuthorities{
			{
				Type:       "user",
				DomainName: "a.example.com",
				ID:         "user.a.example.com",
				PublicKey:  "user value a"},
			{
				Type:       "host",
				DomainName: "b.example.com",
				ID:         "host.b.example.com",
				PublicKey:  "host value b"},
		})

	c.Assert(cfg.Auth.KeysBackend.Type, Equals, "bolt")
	c.Assert(cfg.Auth.KeysBackend.Params,
		DeepEquals, `{"path":"/keys"}`)
	c.Assert(cfg.Auth.KeysBackend.EncryptionKeys, DeepEquals, StringArray{"somekey1", "key2"})

	c.Assert(cfg.Auth.EventsBackend.Type, Equals, "bolt")
	c.Assert(cfg.Auth.EventsBackend.Params,
		Equals, `{"path":"/events"}`)

	c.Assert(cfg.Auth.RecordsBackend.Type, Equals, "bolt")
	c.Assert(cfg.Auth.RecordsBackend.Params,
		Equals, `{"path":"/records"}`)

	/*
	    TODO(klizhentas) fix this once the new config is done
			c.Assert(cfg.Auth.UserCA.PublicKey, Equals, "user ca public key")
			userCA, err := cfg.Auth.UserCA.CA()
			c.Assert(err, IsNil)
			c.Assert(string(userCA.PrivateKey), Equals, "user ca private key")
	*/

	// SSH section
	c.Assert(cfg.SSH.Enabled, Equals, true)
	c.Assert(cfg.SSH.Addr, Equals,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:1234"})
	c.Assert(cfg.SSH.Token, Equals, "sshtoken")
	c.Assert(cfg.SSH.Shell, Equals, "/bin/bash")
	c.Assert(cfg.SSH.Labels, DeepEquals, map[string]string{
		"label1": "value1",
		"label2": "value2",
	})
	c.Assert(cfg.SSH.CmdLabels, DeepEquals, services.CommandLabels{
		"cmd1": services.CommandLabel{
			Period:  time.Second,
			Command: []string{"c1", "arg1", "arg2"},
		},
		"cmd2": services.CommandLabel{
			Period:  3 * time.Second,
			Command: []string{"c2", "arg3"},
		},
	})
	c.Assert(cfg.SSH.Limiter, DeepEquals, limiter.LimiterConfig{
		MaxConnections: 2,
		Rates: []limiter.Rate{
			limiter.Rate{
				Period:  20 * time.Minute,
				Average: 3,
				Burst:   7,
			},
			limiter.Rate{
				Period:  1 * time.Second,
				Average: 5,
				Burst:   3,
			},
		},
	})

	// ReverseTunnel section
	c.Assert(cfg.ReverseTunnel.Enabled, Equals, true)
	c.Assert(cfg.ReverseTunnel.DialAddr, Equals,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "telescope.example.com"})
	c.Assert(cfg.ReverseTunnel.Token, Equals, "tuntoken")

	c.Assert(cfg.Proxy.Enabled, Equals, true)
	c.Assert(cfg.Proxy.ReverseTunnelListenAddr, Equals,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "proxy.vendor.io:33006"})
	c.Assert(cfg.Proxy.Token, Equals, "proxytoken")

}
