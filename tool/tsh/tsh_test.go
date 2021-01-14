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
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tsh/common"
	"github.com/stretchr/testify/require"

	"gopkg.in/check.v1"
)

// bootstrap check
func TestTshMain(t *testing.T) {
	check.TestingT(t)
}

// register test suite
type MainTestSuite struct{}

var _ = check.Suite(&MainTestSuite{})

func (s *MainTestSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests(testing.Verbose())
	os.RemoveAll(client.FullProfilePath(""))
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
	conf.DynamicForwardedPorts = []string{":8080"}
	tc, err = makeClient(&conf, true)
	c.Assert(err, check.IsNil)
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

	// specific configuration with email like user
	conf.MinsToLive = 5
	conf.UserHost = "root@example.com@localhost"
	conf.NodePort = 46528
	conf.LocalForwardPorts = []string{"80:remote:180"}
	conf.DynamicForwardedPorts = []string{":8080"}
	tc, err = makeClient(&conf, true)
	c.Assert(err, check.IsNil)
	c.Assert(tc.Config.KeyTTL, check.Equals, time.Minute*time.Duration(conf.MinsToLive))
	c.Assert(tc.Config.HostLogin, check.Equals, "root@example.com")
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

	randomLocalAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}
	const staticToken = "test-static-token"

	// Set up a test auth server.
	//
	// We need this to get a random port assigned to it and allow parallel
	// execution of this test.
	cfg := service.MakeDefaultConfig()
	cfg.DataDir = c.MkDir()
	cfg.AuthServers = []utils.NetAddr{randomLocalAddr}
	cfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(cfg.DataDir, defaults.BackendDir)}
	cfg.Auth.StaticTokens, err = services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionTokenV1{{
			Roles:   []teleport.Role{teleport.RoleProxy},
			Expires: time.Now().Add(time.Minute),
			Token:   staticToken,
		}},
	})
	c.Assert(err, check.IsNil)
	cfg.SSH.Enabled = false
	cfg.Auth.Enabled = true
	cfg.Auth.SSHAddr = randomLocalAddr
	cfg.Proxy.Enabled = false

	auth, err := service.NewTeleport(cfg)
	c.Assert(err, check.IsNil)
	c.Assert(auth.Start(), check.IsNil)
	defer auth.Close()

	// Wait for proxy to become ready.
	eventCh := make(chan service.Event, 1)
	auth.WaitForEvent(auth.ExitContext(), service.AuthTLSReady, eventCh)
	select {
	case <-eventCh:
	case <-time.After(10 * time.Second):
		c.Fatal("auth server didn't start after 10s")
	}

	authAddr, err := auth.AuthSSHAddr()
	c.Assert(err, check.IsNil)

	// Set up a test proxy service.
	proxyPublicSSHAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: "proxy.example.com:22"}
	cfg = service.MakeDefaultConfig()
	cfg.DataDir = c.MkDir()
	cfg.AuthServers = []utils.NetAddr{*authAddr}
	cfg.Token = staticToken
	cfg.SSH.Enabled = false
	cfg.Auth.Enabled = false
	cfg.Proxy.Enabled = true
	cfg.Proxy.WebAddr = randomLocalAddr
	cfg.Proxy.SSHPublicAddrs = []utils.NetAddr{proxyPublicSSHAddr}
	cfg.Proxy.DisableReverseTunnel = true
	cfg.Proxy.DisableWebInterface = true

	proxy, err := service.NewTeleport(cfg)
	c.Assert(err, check.IsNil)
	c.Assert(proxy.Start(), check.IsNil)
	defer proxy.Close()

	// Wait for proxy to become ready.
	proxy.WaitForEvent(proxy.ExitContext(), service.ProxyWebServerReady, eventCh)
	select {
	case <-eventCh:
	case <-time.After(10 * time.Second):
		c.Fatal("proxy web server didn't start after 10s")
	}

	proxyWebAddr, err := proxy.ProxyWebAddr()
	c.Assert(err, check.IsNil)

	// With provided identity file.
	//
	// makeClient should call Ping on the proxy to fetch SSHProxyAddr, which is
	// different from the default.
	conf = CLIConf{
		Proxy:              proxyWebAddr.String(),
		IdentityFileIn:     "../../fixtures/certs/identities/key-cert-ca.pem",
		Context:            context.Background(),
		InsecureSkipVerify: true,
	}
	tc, err = makeClient(&conf, true)
	c.Assert(err, check.IsNil)
	c.Assert(tc, check.NotNil)
	c.Assert(tc.Config.WebProxyAddr, check.Equals, proxyWebAddr.String())
	c.Assert(tc.Config.SSHProxyAddr, check.Equals, proxyPublicSSHAddr.String())
	c.Assert(tc.LocalAgent().Agent, check.NotNil)
	// Client should have an in-memory agent with keys loaded, in case agent
	// forwarding is required for proxy recording mode.
	agentKeys, err := tc.LocalAgent().Agent.List()
	c.Assert(err, check.IsNil)
	c.Assert(len(agentKeys), check.Not(check.Equals), 0)
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
		k, err := common.LoadIdentity(fmt.Sprintf("../../fixtures/certs/identities/%s", id))
		c.Assert(err, check.IsNil)
		c.Assert(k, check.NotNil)
		cb, err := k.HostKeyCallback()
		c.Assert(err, check.IsNil)
		c.Assert(cb, check.IsNil)

		// test creating an auth method from the key:
		am, err := authFromIdentity(k)
		c.Assert(err, check.IsNil)
		c.Assert(am, check.NotNil)
	}
	k, err := common.LoadIdentity("../../fixtures/certs/identities/lonekey")
	c.Assert(k, check.IsNil)
	c.Assert(err, check.NotNil)

	// lets read an indentity which includes a CA cert
	k, err = common.LoadIdentity("../../fixtures/certs/identities/key-cert-ca.pem")
	c.Assert(err, check.IsNil)
	c.Assert(k, check.NotNil)
	cb, err := k.HostKeyCallback()
	c.Assert(err, check.IsNil)
	c.Assert(cb, check.NotNil)
	// prepare the cluster CA separately
	certBytes, err := ioutil.ReadFile("../../fixtures/certs/identities/ca.pem")
	c.Assert(err, check.IsNil)
	_, hosts, cert, _, _, err := ssh.ParseKnownHosts(certBytes)
	c.Assert(err, check.IsNil)
	var a net.Addr
	// host auth callback must succeed
	err = cb(hosts[0], a, cert)
	c.Assert(err, check.IsNil)

	// load an identity which include TLS certificates
	k, err = common.LoadIdentity("../../fixtures/certs/identities/tls.pem")
	c.Assert(err, check.IsNil)
	c.Assert(k, check.NotNil)
	c.Assert(k.TLSCert, check.NotNil)
	// generate a TLS client config
	conf, err := k.TeleportClientTLSConfig(nil)
	c.Assert(err, check.IsNil)
	c.Assert(conf, check.NotNil)
	// ensure that at least root CA was successfully loaded
	if len(conf.RootCAs.Subjects()) < 1 {
		c.Errorf("Failed to load TLS CAs from identity file")
	}
}

func (s *MainTestSuite) TestOptions(c *check.C) {
	tests := []struct {
		inOptions  []string
		outError   bool
		outOptions Options
	}{
		// Valid
		{
			inOptions: []string{
				"AddKeysToAgent yes",
			},
			outError: false,
			outOptions: Options{
				AddKeysToAgent:        true,
				ForwardAgent:          false,
				RequestTTY:            false,
				StrictHostKeyChecking: true,
			},
		},
		// Valid
		{
			inOptions: []string{
				"AddKeysToAgent=yes",
			},
			outError: false,
			outOptions: Options{
				AddKeysToAgent:        true,
				ForwardAgent:          false,
				RequestTTY:            false,
				StrictHostKeyChecking: true,
			},
		},
		// Invalid value.
		{
			inOptions: []string{
				"AddKeysToAgent foo",
			},
			outError:   true,
			outOptions: Options{},
		},
		// Invalid key.
		{
			inOptions: []string{
				"foo foo",
			},
			outError:   true,
			outOptions: Options{},
		},
		// Incomplete option.
		{
			inOptions: []string{
				"AddKeysToAgent",
			},
			outError:   true,
			outOptions: Options{},
		},
	}

	for _, tt := range tests {
		options, err := parseOptions(tt.inOptions)
		if tt.outError {
			c.Assert(err, check.NotNil)
			continue
		} else {
			c.Assert(err, check.IsNil)
		}

		c.Assert(options.AddKeysToAgent, check.Equals, tt.outOptions.AddKeysToAgent)
		c.Assert(options.ForwardAgent, check.Equals, tt.outOptions.ForwardAgent)
		c.Assert(options.RequestTTY, check.Equals, tt.outOptions.RequestTTY)
		c.Assert(options.StrictHostKeyChecking, check.Equals, tt.outOptions.StrictHostKeyChecking)
	}
}

func TestFormatConnectCommand(t *testing.T) {
	cluster := "root"
	tests := []struct {
		comment string
		db      tlsca.RouteToDatabase
		command string
	}{
		{
			comment: "no default user/database are specified",
			db: tlsca.RouteToDatabase{
				ServiceName: "test",
				Protocol:    defaults.ProtocolPostgres,
			},
			command: `psql "service=root-test user= dbname="`,
		},
		{
			comment: "default user is specified",
			db: tlsca.RouteToDatabase{
				ServiceName: "test",
				Protocol:    defaults.ProtocolPostgres,
				Username:    "postgres",
			},
			command: `psql "service=root-test dbname="`,
		},
		{
			comment: "default database is specified",
			db: tlsca.RouteToDatabase{
				ServiceName: "test",
				Protocol:    defaults.ProtocolPostgres,
				Database:    "postgres",
			},
			command: `psql "service=root-test user="`,
		},
		{
			comment: "default user/database are specified",
			db: tlsca.RouteToDatabase{
				ServiceName: "test",
				Protocol:    defaults.ProtocolPostgres,
				Username:    "postgres",
				Database:    "postgres",
			},
			command: `psql "service=root-test"`,
		},
		{
			comment: "unsupported database protocol",
			db: tlsca.RouteToDatabase{
				ServiceName: "test",
				Protocol:    "mongodb",
			},
			command: "",
		},
	}
	for _, test := range tests {
		t.Run(test.comment, func(t *testing.T) {
			require.Equal(t, test.command, formatConnectCommand(cluster, test.db))
		})
	}
}
