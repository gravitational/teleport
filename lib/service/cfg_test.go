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
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/configure"
	log "github.com/Sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestConfig(t *testing.T) { TestingT(t) }

type ConfigSuite struct {
}

var _ = Suite(&ConfigSuite{})

func (s *ConfigSuite) SetUpSuite(c *C) {
	log.Initialize("console", "INFO")
}

func (s *ConfigSuite) TestParseYAML(c *C) {
	var cfg Config
	err := configure.ParseYAML([]byte(configYAML), &cfg)
	c.Assert(err, IsNil)
	s.checkVariables(c, &cfg)
}

func (s *ConfigSuite) TestParseEnv(c *C) {

	vars := map[string]string{
		"TELEPORT_LOG_OUTPUT":                        "console",
		"TELEPORT_LOG_SEVERITY":                      "INFO",
		"TELEPORT_AUTH_SERVERS":                      `["tcp://localhost:5000", "unix:///var/run/auth.sock"]`,
		"TELEPORT_DATA_DIR":                          "/tmp/data_dir",
		"TELEPORT_HOSTNAME":                          "domain.example.com",
		"TELEPORT_AUTH_ENABLED":                      "true",
		"TELEPORT_AUTH_SSH_ADDR":                     "tcp://localhost:5555",
		"TELEPORT_AUTH_HOST_AUTHORITY_DOMAIN":        "a.domain.example.com",
		"TELEPORT_AUTH_TOKEN":                        "authtoken",
		"TELEPORT_AUTH_SECRET_KEY":                   "authsecret",
		"TELEPORT_AUTH_ALLOWED_TOKENS":               `{"ntoken1": "node1.a.domain.example.com", "atoken2": "node2.a.domain.example.com"}`,
		"TELEPORT_AUTH_TRUSTED_AUTHORITIES":          `[{"type": "user", "domain_name":"a.example.com", "id":"user.a.example.com", "public_key": "user value a"},{"type": "host", "domain_name":"b.example.com", "id":"host.b.example.com", "public_key": "host value b"}]`,
		"TELEPORT_AUTH_KEYS_BACKEND_TYPE":            "bolt",
		"TELEPORT_AUTH_KEYS_BACKEND_PARAMS":          `{"path":"/keys"}`,
		"TELEPORT_AUTH_KEYS_BACKEND_ENCRYPTION_KEYS": `["somekey1", "key2"]`,
		"TELEPORT_AUTH_EVENTS_BACKEND_TYPE":          "bolt",
		"TELEPORT_AUTH_EVENTS_BACKEND_PARAMS":        `{"path":"/events"}`,
		"TELEPORT_AUTH_RECORDS_BACKEND_TYPE":         "bolt",
		"TELEPORT_AUTH_RECORDS_BACKEND_PARAMS":       `{"path":"/records"}`,
		"TELEPORT_AUTH_USER_CA_KEYPAIR":              `{"public": {"id":"1", "domain_name":"localhost", "public_key":"user ca public key"}, "private_key": "dXNlciBjYSBwcml2YXRlIGtleQ=="}`,
		"TELEPORT_AUTH_HOST_CA_KEYPAIR":              `{"public": {"id":"2", "domain_name":"localhost", "public_key":"host ca public key"}, "private_key": "aG9zdCBjYSBwcml2YXRlIGtleQ=="}`,
		"TELEPORT_SSH_ENABLED":                       "true",
		"TELEPORT_SSH_TOKEN":                         "sshtoken",
		"TELEPORT_SSH_ADDR":                          "tcp://localhost:1234",
		"TELEPORT_SSH_SHELL":                         "/bin/bash",
		"TELEPORT_SSH_LABELS":                        `{"label1":"value1", "label2":"value2"}`,
		"TELEPORT_SSH_LABEL_COMMANDS":                `{"cmd1": {"period": "1s", "command": ["c1", "arg1", "arg2"]}, "cmd2":{"period": "3s", "command": ["c2", "arg3"]}}`,
		"TELEPORT_SSH_LIMITER":                       `{"max_connections": 2, "rates":[{"period": "20m", "average": 3, "burst": 7}, {"period": "1s", "average": 5, "burst": 3}]}`,
		"TELEPORT_REVERSE_TUNNEL_ENABLED":            "true",
		"TELEPORT_REVERSE_TUNNEL_TOKEN":              "tuntoken",
		"TELEPORT_REVERSE_TUNNEL_DIAL_ADDR":          "tcp://telescope.example.com",
		"TELEPORT_PROXY_ENABLED":                     "true",
		"TELEPORT_PROXY_TOKEN":                       "proxytoken",
		"TELEPORT_PROXY_REVERSE_TUNNEL_LISTEN_ADDR":  "tcp://proxy.vendor.io:33006",
		"TELEPORT_PROXY_WEB_ADDR":                    "tcp://proxy.vendor.io:33007",
		"TELEPORT_PROXY_ASSETS_DIR":                  "web/assets",
		"TELEPORT_PROXY_TLS_KEY":                     "base64key",
		"TELEPORT_PROXY_TLS_CERT":                    "base64cert",
	}
	for k, v := range vars {
		c.Assert(os.Setenv(k, v), IsNil)
	}
	var cfg Config
	err := configure.ParseEnv(&cfg)
	c.Assert(err, IsNil)
	s.checkVariables(c, &cfg)
}

func (s *ConfigSuite) checkVariables(c *C, cfg *Config) {

	// check logs section
	c.Assert(cfg.Log.Output, Equals, "console")
	c.Assert(cfg.Log.Severity, Equals, "INFO")

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

	c.Assert(cfg.Auth.UserCA.PublicKey, Equals, "user ca public key")
	userCA, err := cfg.Auth.UserCA.CA()
	c.Assert(err, IsNil)
	c.Assert(string(userCA.PrivateKey), Equals, "user ca private key")

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

const configYAML = `
log:
  output: console
  severity: INFO

data_dir: /tmp/data_dir
hostname: domain.example.com
auth_servers: ['tcp://localhost:5000', 'unix:///var/run/auth.sock']

auth:
  enabled: true
  ssh_addr: 'tcp://localhost:5555'
  host_authority_domain: a.domain.example.com
  token: authtoken
  secret_key: authsecret
  allowed_tokens: 
    ntoken1: node1.a.domain.example.com
    atoken2: node2.a.domain.example.com

  user_ca_keypair:
    public:
      id: 1
      domain_name: localhost
      public_key: user ca public key
    private_key: dXNlciBjYSBwcml2YXRlIGtleQ==

  host_ca_keypair:
    public:
      id: 1
      domain_name: localhost
      public_key: host ca public key
    private_key: aG9zdCBjYSBwcml2YXRlIGtleQ==

  trusted_authorities: 

    - type: user
      domain_name: a.example.com
      id: user.a.example.com
      public_key: user value a

    - type: host
      domain_name: b.example.com
      id:  host.b.example.com
      public_key: host value b

  keys_backend:
    type: bolt
    params: '{"path":"/keys"}'
    encryption_keys: 
      - somekey1
      - key2

  events_backend:
    type: bolt
    params: '{"path":"/events"}'

  records_backend:
    type: bolt
    params: '{"path":"/records"}'

ssh:
  enabled: true
  token: sshtoken
  addr: 'tcp://localhost:1234'
  shell: /bin/bash
  limiter:
    max_connections: 2
    rates:
      - period: 20m
        average: 3
        burst: 7
      - period: 1s
        average: 5
        burst: 3
  labels:
    label1: value1
    label2: value2
  label-commands:
    cmd1: 
      period: 1s
      command: ["c1", "arg1", "arg2"]
    cmd2:
      period: 3s
      command: ["c2", "arg3"]

reverse_tunnel:
  enabled: true
  token: tuntoken
  dial_addr: 'tcp://telescope.example.com'

proxy:
  enabled: true
  assets_dir: assets/web # directory with javascript, html and css for web
  token: proxytoken
  reverse_tunnel_listen_addr: tcp://proxy.vendor.io:33006
  web_addr: tcp://proxy.vendor.io:33007
  tls_key: base64key
  tls_cert: base64cert
`
