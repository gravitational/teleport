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
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testlog"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

type ServiceTestSuite struct {
}

var _ = check.Suite(&ServiceTestSuite{})

func (s *ServiceTestSuite) TestDebugModeEnv(c *check.C) {
	c.Assert(isDebugMode(), check.Equals, false)
	os.Setenv(teleport.DebugEnvVar, "no")
	c.Assert(isDebugMode(), check.Equals, false)
	os.Setenv(teleport.DebugEnvVar, "0")
	c.Assert(isDebugMode(), check.Equals, false)
	os.Setenv(teleport.DebugEnvVar, "1")
	c.Assert(isDebugMode(), check.Equals, true)
	os.Setenv(teleport.DebugEnvVar, "true")
	c.Assert(isDebugMode(), check.Equals, true)
}

func (s *ServiceTestSuite) TestSelfSignedHTTPS(c *check.C) {
	cfg := &Config{
		DataDir:  c.MkDir(),
		Hostname: "example.com",
		Log:      utils.WrapLogger(logrus.New().WithField("test", c.TestName())),
	}
	err := initSelfSignedHTTPSCert(cfg)
	c.Assert(err, check.IsNil)
	c.Assert(cfg.Proxy.KeyPairs, check.HasLen, 1)
	c.Assert(utils.FileExists(cfg.Proxy.KeyPairs[0].Certificate), check.Equals, true)
	c.Assert(utils.FileExists(cfg.Proxy.KeyPairs[0].PrivateKey), check.Equals, true)
}

func TestMonitor(t *testing.T) {
	t.Parallel()
	fakeClock := clockwork.NewFakeClock()

	cfg := MakeDefaultConfig()
	cfg.Clock = fakeClock
	var err error
	cfg.DataDir, err = ioutil.TempDir("", "teleport")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.DataDir)
	cfg.DiagnosticAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}
	cfg.AuthServers = []utils.NetAddr{{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}}
	cfg.Auth.Enabled = true
	cfg.Auth.StorageConfig.Params["path"], err = ioutil.TempDir("", "teleport")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.DataDir)
	cfg.Auth.SSHAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}
	cfg.Proxy.Enabled = false
	cfg.SSH.Enabled = false

	process, err := NewTeleport(cfg)
	require.NoError(t, err)

	diagAddr, err := process.DiagnosticAddr()
	require.NoError(t, err)
	require.NotNil(t, diagAddr)
	endpoint := fmt.Sprintf("http://%v/readyz", diagAddr.String())

	// Start Teleport and make sure the status is OK.
	go func() {
		require.NoError(t, process.Run())
	}()
	err = waitForStatus(endpoint, http.StatusOK)
	require.NoError(t, err)

	tests := []struct {
		desc         string
		event        Event
		advanceClock time.Duration
		wantStatus   []int
	}{
		{
			desc:       "degraded event causes degraded state",
			event:      Event{Name: TeleportDegradedEvent, Payload: teleport.ComponentAuth},
			wantStatus: []int{http.StatusServiceUnavailable, http.StatusBadRequest},
		},
		{
			desc:       "ok event causes recovering state",
			event:      Event{Name: TeleportOKEvent, Payload: teleport.ComponentAuth},
			wantStatus: []int{http.StatusBadRequest},
		},
		{
			desc:       "ok event remains in recovering state because not enough time passed",
			event:      Event{Name: TeleportOKEvent, Payload: teleport.ComponentAuth},
			wantStatus: []int{http.StatusBadRequest},
		},
		{
			desc:         "ok event after enough time causes OK state",
			event:        Event{Name: TeleportOKEvent, Payload: teleport.ComponentAuth},
			advanceClock: defaults.HeartbeatCheckPeriod*2 + 1,
			wantStatus:   []int{http.StatusOK},
		},
		{
			desc:       "degraded event in a new component causes degraded state",
			event:      Event{Name: TeleportDegradedEvent, Payload: teleport.ComponentNode},
			wantStatus: []int{http.StatusServiceUnavailable, http.StatusBadRequest},
		},
		{
			desc:         "ok event in one component keeps overall status degraded due to other component",
			advanceClock: defaults.HeartbeatCheckPeriod*2 + 1,
			event:        Event{Name: TeleportOKEvent, Payload: teleport.ComponentAuth},
			wantStatus:   []int{http.StatusServiceUnavailable, http.StatusBadRequest},
		},
		{
			desc:         "ok event in new component causes overall recovering state",
			advanceClock: defaults.HeartbeatCheckPeriod*2 + 1,
			event:        Event{Name: TeleportOKEvent, Payload: teleport.ComponentNode},
			wantStatus:   []int{http.StatusBadRequest},
		},
		{
			desc:         "ok event in new component causes overall OK state",
			advanceClock: defaults.HeartbeatCheckPeriod*2 + 1,
			event:        Event{Name: TeleportOKEvent, Payload: teleport.ComponentNode},
			wantStatus:   []int{http.StatusOK},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			fakeClock.Advance(tt.advanceClock)
			process.BroadcastEvent(tt.event)
			err = waitForStatus(endpoint, tt.wantStatus...)
			require.NoError(t, err)
		})
	}
}

// TestCheckPrincipals checks certificates regeneration only requests
// regeneration when the principals change.
func (s *ServiceTestSuite) TestCheckPrincipals(c *check.C) {
	dataDir := c.MkDir()

	t := testlog.NewCheckTestWrapper(c)
	defer t.Close()

	// Create a test auth server to extract the server identity (SSH and TLS
	// certificates).
	testAuthServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir: dataDir,
	})
	c.Assert(err, check.IsNil)
	tlsServer, err := testAuthServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)
	defer tlsServer.Close()

	testConnector := &Connector{
		ServerIdentity: tlsServer.Identity,
	}

	var tests = []struct {
		inPrincipals  []string
		inDNS         []string
		outRegenerate bool
	}{
		// If nothing has been updated, don't regenerate certificate.
		{
			inPrincipals:  []string{},
			inDNS:         []string{},
			outRegenerate: false,
		},
		// Don't regenerate certificate if the node does not know it's own address.
		{
			inPrincipals:  []string{"0.0.0.0"},
			inDNS:         []string{},
			outRegenerate: false,
		},
		// If a new SSH principal is found, regenerate certificate.
		{
			inPrincipals:  []string{"1.1.1.1"},
			inDNS:         []string{},
			outRegenerate: true,
		},
		// If a new TLS DNS name is found, regenerate certificate.
		{
			inPrincipals:  []string{},
			inDNS:         []string{"server.example.com"},
			outRegenerate: true,
		},
		// Don't regenerate certificate if additional principals is already on the
		// certificate.
		{
			inPrincipals:  []string{"test-tls-server"},
			inDNS:         []string{},
			outRegenerate: false,
		},
	}
	for _, tt := range tests {
		ok := checkServerIdentity(testConnector, tt.inPrincipals, tt.inDNS, t.Log)
		c.Assert(ok, check.Equals, tt.outRegenerate)
	}
}

// TestInitExternalLog verifies that external logging can be used both as a means of
// overriding the local audit event target.  Ideally, this test would also verify
// setup of true external loggers, but at the time of writing there isn't good
// support for setting up fake external logging endpoints.
func (s *ServiceTestSuite) TestInitExternalLog(c *check.C) {
	t := testlog.NewCheckTestWrapper(c)
	defer t.Close()

	tts := []struct {
		events []string
		isNil  bool
		isErr  bool
	}{
		// no URIs => no external logger
		{isNil: true},
		// local-only event uri w/o hostname => ok
		{events: []string{"file:///tmp/teleport-test/events"}},
		// local-only event uri w/ localhost => ok
		{events: []string{"file://localhost/tmp/teleport-test/events"}},
		// invalid host parameter => rejected
		{events: []string{"file://example.com/should/fail"}, isErr: true},
		// missing path specifier => rejected
		{events: []string{"file://localhost"}, isErr: true},
	}

	backend, err := memory.New(memory.Config{})
	c.Assert(err, check.IsNil)

	for i, tt := range tts {
		// isErr implies isNil.
		if tt.isErr {
			tt.isNil = true
		}

		cmt := check.Commentf("tt[%v]: %+v", i, tt)

		auditConfig, err := types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
			AuditEventsURI: tt.events,
		})
		c.Assert(err, check.IsNil, cmt)
		loggers, err := initExternalLog(context.Background(), auditConfig, t.Log, backend)

		if tt.isErr {
			c.Assert(err, check.NotNil, cmt)
		} else {
			c.Assert(err, check.IsNil, cmt)
		}

		if tt.isNil {
			c.Assert(loggers, check.IsNil, cmt)
		} else {
			c.Assert(loggers, check.NotNil, cmt)
		}
	}
}

func TestGetAdditionalPrincipals(t *testing.T) {
	p := &TeleportProcess{
		Config: &Config{
			Hostname:    "global-hostname",
			HostUUID:    "global-uuid",
			AdvertiseIP: "1.2.3.4",
			Proxy: ProxyConfig{
				PublicAddrs:         utils.MustParseAddrList("proxy-public-1", "proxy-public-2"),
				SSHPublicAddrs:      utils.MustParseAddrList("proxy-ssh-public-1", "proxy-ssh-public-2"),
				TunnelPublicAddrs:   utils.MustParseAddrList("proxy-tunnel-public-1", "proxy-tunnel-public-2"),
				PostgresPublicAddrs: utils.MustParseAddrList("proxy-postgres-public-1", "proxy-postgres-public-2"),
				MySQLPublicAddrs:    utils.MustParseAddrList("proxy-mysql-public-1", "proxy-mysql-public-2"),
				Kube: KubeProxyConfig{
					Enabled:     true,
					PublicAddrs: utils.MustParseAddrList("proxy-kube-public-1", "proxy-kube-public-2"),
				},
				WebAddr: *utils.MustParseAddr(":443"),
			},
			Auth: AuthConfig{
				PublicAddrs: utils.MustParseAddrList("auth-public-1", "auth-public-2"),
			},
			SSH: SSHConfig{
				PublicAddrs: utils.MustParseAddrList("node-public-1", "node-public-2"),
			},
			Kube: KubeConfig{
				PublicAddrs: utils.MustParseAddrList("kube-public-1", "kube-public-2"),
			},
		},
	}
	tests := []struct {
		role           types.SystemRole
		wantPrincipals []string
		wantDNS        []string
	}{
		{
			role: types.RoleProxy,
			wantPrincipals: []string{
				"global-hostname",
				"proxy-public-1",
				"proxy-public-2",
				defaults.BindIP,
				string(teleport.PrincipalLocalhost),
				string(teleport.PrincipalLoopbackV4),
				string(teleport.PrincipalLoopbackV6),
				reversetunnel.LocalKubernetes,
				"proxy-ssh-public-1",
				"proxy-ssh-public-2",
				"proxy-tunnel-public-1",
				"proxy-tunnel-public-2",
				"proxy-postgres-public-1",
				"proxy-postgres-public-2",
				"proxy-mysql-public-1",
				"proxy-mysql-public-2",
				"proxy-kube-public-1",
				"proxy-kube-public-2",
			},
			wantDNS: []string{
				"*.teleport.cluster.local",
				"teleport.cluster.local",
				"*.proxy-public-1",
				"*.proxy-public-2",
				"*.proxy-kube-public-1",
				"*.proxy-kube-public-2",
			},
		},
		{
			role: types.RoleAuth,
			wantPrincipals: []string{
				"global-hostname",
				"auth-public-1",
				"auth-public-2",
			},
			wantDNS: []string{
				"*.teleport.cluster.local",
				"teleport.cluster.local",
			},
		},
		{
			role: types.RoleAdmin,
			wantPrincipals: []string{
				"global-hostname",
				"auth-public-1",
				"auth-public-2",
			},
			wantDNS: []string{
				"*.teleport.cluster.local",
				"teleport.cluster.local",
			},
		},
		{
			role: types.RoleNode,
			wantPrincipals: []string{
				"global-hostname",
				"global-uuid",
				"node-public-1",
				"node-public-2",
				"1.2.3.4",
			},
			wantDNS: []string{},
		},
		{
			role: types.RoleKube,
			wantPrincipals: []string{
				"global-hostname",
				string(teleport.PrincipalLocalhost),
				string(teleport.PrincipalLoopbackV4),
				string(teleport.PrincipalLoopbackV6),
				reversetunnel.LocalKubernetes,
				"kube-public-1",
				"kube-public-2",
			},
			wantDNS: []string{
				"*.teleport.cluster.local",
				"teleport.cluster.local",
			},
		},
		{
			role: types.RoleApp,
			wantPrincipals: []string{
				"global-hostname",
				"global-uuid",
			},
			wantDNS: []string{
				"*.teleport.cluster.local",
				"teleport.cluster.local",
			},
		},
		{
			role: types.SystemRole("unknown"),
			wantPrincipals: []string{
				"global-hostname",
			},
			wantDNS: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.role.String(), func(t *testing.T) {
			principals, dns, err := p.getAdditionalPrincipals(tt.role)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(principals, tt.wantPrincipals))
			require.Empty(t, cmp.Diff(dns, tt.wantDNS, cmpopts.EquateEmpty()))
		})
	}
}

func waitForStatus(diagAddr string, statusCodes ...int) error {
	tickCh := time.Tick(100 * time.Millisecond)
	timeoutCh := time.After(10 * time.Second)
	var lastStatus int
	for {
		select {
		case <-tickCh:
			resp, err := http.Get(diagAddr)
			if err != nil {
				return trace.Wrap(err)
			}
			resp.Body.Close()
			lastStatus = resp.StatusCode
			for _, statusCode := range statusCodes {
				if resp.StatusCode == statusCode {
					return nil
				}
			}
		case <-timeoutCh:
			return trace.BadParameter("timeout waiting for status: %v; last status: %v", statusCodes, lastStatus)
		}
	}
}

func TestTeleportProcess_reconnectToAuth(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	// Create and configure a default Teleport configuration.
	cfg := MakeDefaultConfig()
	cfg.AuthServers = []utils.NetAddr{{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}}
	cfg.Clock = clock
	cfg.DataDir = t.TempDir()
	cfg.Auth.Enabled = false
	cfg.Proxy.Enabled = false
	cfg.SSH.Enabled = true
	cfg.MaxRetryPeriod = defaults.MaxWatcherBackoff
	cfg.ConnectFailureC = make(chan time.Duration, 5)
	process, err := NewTeleport(cfg)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c, err := process.reconnectToAuthService(types.RoleAdmin)
		require.Equal(t, ErrTeleportExited, err)
		require.Nil(t, c)
	}()

	step := cfg.MaxRetryPeriod / 5.0
	for i := 0; i < 5; i++ {
		// wait for connection to fail
		select {
		case duration := <-process.Config.ConnectFailureC:
			stepMin := step * time.Duration(i) / 2
			stepMax := step * time.Duration(i+1)

			require.GreaterOrEqual(t, duration, stepMin)
			require.LessOrEqual(t, duration, stepMax)

			// wait for connection to get to retry.After
			clock.BlockUntil(1)

			// add some extra to the duration to ensure the retry occurs
			clock.Advance(cfg.MaxRetryPeriod)
		case <-time.After(time.Minute):
			t.Fatalf("timeout waiting for failure")
		}
	}

	supervisor, ok := process.Supervisor.(*LocalSupervisor)
	require.True(t, ok)
	supervisor.signalExit()
	wg.Wait()
}
