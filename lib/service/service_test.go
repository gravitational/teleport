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
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/athena"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestServiceSelfSignedHTTPS(t *testing.T) {
	cfg := &servicecfg.Config{
		DataDir:  t.TempDir(),
		Hostname: "example.com",
		Log:      utils.WrapLogger(logrus.New().WithField("test", "TestServiceSelfSignedHTTPS")),
	}
	require.NoError(t, initSelfSignedHTTPSCert(cfg))
	require.Len(t, cfg.Proxy.KeyPairs, 1)
	require.FileExists(t, cfg.Proxy.KeyPairs[0].Certificate)
	require.FileExists(t, cfg.Proxy.KeyPairs[0].PrivateKey)
}

func TestAdditionalExpectedRoles(t *testing.T) {
	tests := []struct {
		name          string
		cfg           func() *servicecfg.Config
		expectedRoles map[types.SystemRole]string
	}{
		{
			name: "everything enabled",
			cfg: func() *servicecfg.Config {
				cfg := servicecfg.MakeDefaultConfig()
				cfg.DataDir = t.TempDir()
				cfg.SetAuthServerAddress(utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"})
				cfg.Auth.StorageConfig.Params["path"] = t.TempDir()
				cfg.DiagnosticAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}
				cfg.Auth.ListenAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}

				cfg.Auth.Enabled = true
				cfg.SSH.Enabled = true
				cfg.Proxy.Enabled = true
				cfg.Kube.Enabled = true
				cfg.Apps.Enabled = true
				cfg.Databases.Enabled = true
				cfg.WindowsDesktop.Enabled = true
				cfg.Discovery.Enabled = true
				return cfg
			},
			expectedRoles: map[types.SystemRole]string{
				types.RoleAuth:           AuthIdentityEvent,
				types.RoleNode:           SSHIdentityEvent,
				types.RoleKube:           KubeIdentityEvent,
				types.RoleApp:            AppsIdentityEvent,
				types.RoleDatabase:       DatabasesIdentityEvent,
				types.RoleWindowsDesktop: WindowsDesktopIdentityEvent,
				types.RoleDiscovery:      DiscoveryIdentityEvent,
				types.RoleProxy:          ProxyIdentityEvent,
			},
		},
		{
			name: "everything enabled with additional roles",
			cfg: func() *servicecfg.Config {
				cfg := servicecfg.MakeDefaultConfig()
				cfg.DataDir = t.TempDir()
				cfg.SetAuthServerAddress(utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"})
				cfg.Auth.StorageConfig.Params["path"] = t.TempDir()
				cfg.DiagnosticAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}
				cfg.Auth.ListenAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}

				cfg.Auth.Enabled = true
				cfg.SSH.Enabled = true
				cfg.Proxy.Enabled = true
				cfg.Kube.Enabled = true
				cfg.Apps.Enabled = true
				cfg.Databases.Enabled = true
				cfg.WindowsDesktop.Enabled = true
				cfg.Discovery.Enabled = true

				cfg.AdditionalExpectedRoles = []servicecfg.RoleAndIdentityEvent{
					{
						Role:          types.RoleOkta,
						IdentityEvent: "some-identity-event",
					},
					{
						Role:          types.RoleBot,
						IdentityEvent: "some-other-identity-event",
					},
				}

				return cfg
			},
			expectedRoles: map[types.SystemRole]string{
				types.RoleAuth:           AuthIdentityEvent,
				types.RoleNode:           SSHIdentityEvent,
				types.RoleKube:           KubeIdentityEvent,
				types.RoleApp:            AppsIdentityEvent,
				types.RoleDatabase:       DatabasesIdentityEvent,
				types.RoleWindowsDesktop: WindowsDesktopIdentityEvent,
				types.RoleDiscovery:      DiscoveryIdentityEvent,
				types.RoleProxy:          ProxyIdentityEvent,
				types.RoleOkta:           "some-identity-event",
				types.RoleBot:            "some-other-identity-event",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process, err := NewTeleport(test.cfg())
			require.NoError(t, err)
			require.Equal(t, test.expectedRoles, process.instanceRoles)
		})
	}
}

func TestMonitor(t *testing.T) {
	t.Parallel()
	fakeClock := clockwork.NewFakeClock()

	cfg := servicecfg.MakeDefaultConfig()
	cfg.Clock = fakeClock
	var err error
	cfg.DataDir = t.TempDir()
	cfg.DiagnosticAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}
	cfg.SetAuthServerAddress(utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"})
	cfg.Auth.Enabled = true
	cfg.Auth.StorageConfig.Params["path"] = t.TempDir()
	cfg.Auth.ListenAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}
	cfg.Proxy.Enabled = false
	cfg.SSH.Enabled = false
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	process, err := NewTeleport(cfg)
	require.NoError(t, err)

	// this simulates events that happened to be broadcast before the
	// readyz.monitor started listening for events
	process.BroadcastEvent(Event{Name: TeleportOKEvent, Payload: teleport.ComponentAuth})

	require.NoError(t, process.Start())
	t.Cleanup(func() { require.NoError(t, process.Close()) })

	diagAddr, err := process.DiagnosticAddr()
	require.NoError(t, err)
	require.NotNil(t, diagAddr)

	endpoint := fmt.Sprintf("http://%v/readyz", diagAddr.String())
	waitForStatus := func(statusCodes ...int) func() bool {
		return func() bool {
			resp, err := http.Get(endpoint)
			require.NoError(t, err)
			resp.Body.Close()
			for _, c := range statusCodes {
				if resp.StatusCode == c {
					return true
				}
			}
			return false
		}
	}

	require.Eventually(t, waitForStatus(http.StatusOK), 5*time.Second, 100*time.Millisecond)

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
			require.Eventually(t, waitForStatus(tt.wantStatus...), 5*time.Second, 100*time.Millisecond)
		})
	}
}

// TestServiceCheckPrincipals checks certificates regeneration only requests
// regeneration when the principals change.
func TestServiceCheckPrincipals(t *testing.T) {
	// Create a test auth server to extract the server identity (SSH and TLS
	// certificates).
	testAuthServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)
	tlsServer, err := testAuthServer.NewTestTLSServer()
	require.NoError(t, err)
	defer tlsServer.Close()

	testConnector := &Connector{
		ServerIdentity: tlsServer.Identity,
	}

	tests := []struct {
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
	for i, tt := range tests {
		ok := checkServerIdentity(testConnector, tt.inPrincipals, tt.inDNS, logrus.New().WithField("test", "TestServiceCheckPrincipals"))
		require.Equal(t, tt.outRegenerate, ok, "test %d", i)
	}
}

// TestServiceInitExternalLog verifies that external logging can be used both as a means of
// overriding the local audit event target.  Ideally, this test would also verify
// setup of true external loggers, but at the time of writing there isn't good
// support for setting up fake external logging endpoints.
func TestServiceInitExternalLog(t *testing.T) {
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

	for _, tt := range tts {
		backend, err := memory.New(memory.Config{})
		require.NoError(t, err)

		t.Run(strings.Join(tt.events, ","), func(t *testing.T) {
			// isErr implies isNil.
			if tt.isErr {
				tt.isNil = true
			}

			auditConfig, err := types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
				AuditEventsURI: tt.events,
			})
			require.NoError(t, err)
			loggers, err := initAuthExternalAuditLog(context.Background(), auditConfig, backend, nil /* tracingProvider */)
			if tt.isErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.isNil {
				require.Nil(t, loggers)
			} else {
				require.NotNil(t, loggers)
			}
		})
	}
}

func TestAthenaAuditLogSetup(t *testing.T) {
	sampleValidConfig := "athena://db.table?topicArn=arn:aws:sns:eu-central-1:accnr:topicName&queryResultsS3=s3://testbucket/query-result/&workgroup=workgroup&locationS3=s3://testbucket/events-location&queueURL=https://sqs.eu-central-1.amazonaws.com/accnr/sqsname&largeEventsS3=s3://testbucket/largeevents"
	tests := []struct {
		name   string
		uri    string
		wantFn func(*testing.T, events.AuditLogger, error)
	}{
		{
			name: "valid athena config",
			uri:  sampleValidConfig,
			wantFn: func(t *testing.T, alog events.AuditLogger, err error) {
				require.NoError(t, err)
				v, ok := alog.(*athena.Log)
				require.True(t, ok, "invalid logger type, got %T", v)
			},
		},
		{
			name: "config with rate limit - should use events.SearchEventsLimiter",
			uri:  sampleValidConfig + "&limiterRefillAmount=3&limiterBurst=2",
			wantFn: func(t *testing.T, alog events.AuditLogger, err error) {
				require.NoError(t, err)
				_, ok := alog.(*events.SearchEventsLimiter)
				require.True(t, ok, "invalid logger type, got %T", alog)
			},
		},
	}
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auditConfig, err := types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
				AuditEventsURI:   []string{tt.uri},
				AuditSessionsURI: "s3://testbucket/sessions-rec",
			})
			require.NoError(t, err)
			log, err := initAuthExternalAuditLog(context.Background(), auditConfig, backend, nil /* tracingProvider */)
			tt.wantFn(t, log, err)
		})
	}
}

func TestGetAdditionalPrincipals(t *testing.T) {
	p := &TeleportProcess{
		Config: &servicecfg.Config{
			Hostname:    "global-hostname",
			HostUUID:    "global-uuid",
			AdvertiseIP: "1.2.3.4",
			Proxy: servicecfg.ProxyConfig{
				PublicAddrs:         utils.MustParseAddrList("proxy-public-1", "proxy-public-2"),
				SSHPublicAddrs:      utils.MustParseAddrList("proxy-ssh-public-1", "proxy-ssh-public-2"),
				TunnelPublicAddrs:   utils.MustParseAddrList("proxy-tunnel-public-1", "proxy-tunnel-public-2"),
				PostgresPublicAddrs: utils.MustParseAddrList("proxy-postgres-public-1", "proxy-postgres-public-2"),
				MySQLPublicAddrs:    utils.MustParseAddrList("proxy-mysql-public-1", "proxy-mysql-public-2"),
				Kube: servicecfg.KubeProxyConfig{
					Enabled:     true,
					PublicAddrs: utils.MustParseAddrList("proxy-kube-public-1", "proxy-kube-public-2"),
				},
				WebAddr: *utils.MustParseAddr(":443"),
			},
			Auth: servicecfg.AuthConfig{
				PublicAddrs: utils.MustParseAddrList("auth-public-1", "auth-public-2"),
			},
			SSH: servicecfg.SSHConfig{
				PublicAddrs: utils.MustParseAddrList("node-public-1", "node-public-2"),
			},
			Kube: servicecfg.KubeConfig{
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
			role: types.RoleOkta,
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

// TestDesktopAccessFIPS makes sure that Desktop Access can not be started in
// FIPS mode. Remove this test once Rust code has been updated to use
// BoringCrypto instead of OpenSSL.
func TestDesktopAccessFIPS(t *testing.T) {
	t.Parallel()

	// Create and configure a default Teleport configuration.
	cfg := servicecfg.MakeDefaultConfig()
	cfg.SetAuthServerAddress(utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"})
	cfg.Clock = clockwork.NewFakeClock()
	cfg.DataDir = t.TempDir()
	cfg.Auth.Enabled = false
	cfg.Proxy.Enabled = false
	cfg.SSH.Enabled = false

	// Enable FIPS mode and Desktop Access, this should fail.
	cfg.FIPS = true
	cfg.WindowsDesktop.Enabled = true
	_, err := NewTeleport(cfg)
	require.Error(t, err)
}

type mockAccessPoint struct {
	auth.ProxyAccessPoint
}

type mockReverseTunnelServer struct {
	reversetunnel.Server
}

func TestSetupProxyTLSConfig(t *testing.T) {
	testCases := []struct {
		name           string
		acmeEnabled    bool
		wantNextProtos []string
	}{
		{
			name:        "ACME enabled, teleport ALPN protocols should be appended",
			acmeEnabled: true,
			wantNextProtos: []string{
				// Ensure http/1.1 has precedence over http2.
				"http/1.1",
				"h2",
				"acme-tls/1",
				"teleport-tcp-ping",
				"teleport-postgres-ping",
				"teleport-mysql-ping",
				"teleport-mongodb-ping",
				"teleport-oracle-ping",
				"teleport-redis-ping",
				"teleport-sqlserver-ping",
				"teleport-snowflake-ping",
				"teleport-cassandra-ping",
				"teleport-elasticsearch-ping",
				"teleport-opensearch-ping",
				"teleport-dynamodb-ping",
				"teleport-proxy-ssh",
				"teleport-reversetunnel",
				"teleport-auth@",
				"teleport-tcp",
				"teleport-proxy-ssh-grpc",
				"teleport-proxy-grpc",
				"teleport-proxy-grpc-mtls",
				"teleport-postgres",
				"teleport-mysql",
				"teleport-mongodb",
				"teleport-oracle",
				"teleport-redis",
				"teleport-sqlserver",
				"teleport-snowflake",
				"teleport-cassandra",
				"teleport-elasticsearch",
				"teleport-opensearch",
				"teleport-dynamodb",
			},
		},
		{
			name:        "ACME disabled",
			acmeEnabled: false,
			wantNextProtos: []string{
				"teleport-tcp-ping",
				"teleport-postgres-ping",
				"teleport-mysql-ping",
				"teleport-mongodb-ping",
				"teleport-oracle-ping",
				"teleport-redis-ping",
				"teleport-sqlserver-ping",
				"teleport-snowflake-ping",
				"teleport-cassandra-ping",
				"teleport-elasticsearch-ping",
				"teleport-opensearch-ping",
				"teleport-dynamodb-ping",
				// Ensure http/1.1 has precedence over http2.
				"http/1.1",
				"h2",
				"teleport-proxy-ssh",
				"teleport-reversetunnel",
				"teleport-auth@",
				"teleport-tcp",
				"teleport-proxy-ssh-grpc",
				"teleport-proxy-grpc",
				"teleport-proxy-grpc-mtls",
				"teleport-postgres",
				"teleport-mysql",
				"teleport-mongodb",
				"teleport-oracle",
				"teleport-redis",
				"teleport-sqlserver",
				"teleport-snowflake",
				"teleport-cassandra",
				"teleport-elasticsearch",
				"teleport-opensearch",
				"teleport-dynamodb",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := servicecfg.MakeDefaultConfig()
			cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
			cfg.Proxy.ACME.Enabled = tc.acmeEnabled
			cfg.DataDir = t.TempDir()
			cfg.Proxy.PublicAddrs = utils.MustParseAddrList("localhost")
			process := TeleportProcess{
				Config: cfg,
				// Setting Supervisor so that `ExitContext` can be called.
				Supervisor: NewSupervisor("process-id", cfg.Log),
			}
			conn := &Connector{
				ServerIdentity: &auth.Identity{
					Cert: &ssh.Certificate{
						Permissions: ssh.Permissions{
							Extensions: map[string]string{},
						},
					},
				},
			}
			tls, err := process.setupProxyTLSConfig(
				conn,
				&mockReverseTunnelServer{},
				&mockAccessPoint{},
				"cluster",
			)
			require.NoError(t, err)
			require.Equal(t, tc.wantNextProtos, tls.NextProtos)
		})
	}
}

func TestTeleportProcess_reconnectToAuth(t *testing.T) {
	t.Parallel()
	// Create and configure a default Teleport configuration.
	cfg := servicecfg.MakeDefaultConfig()
	cfg.SetAuthServerAddress(utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"})
	cfg.Clock = clockwork.NewRealClock()
	cfg.DataDir = t.TempDir()
	cfg.Auth.Enabled = false
	cfg.Proxy.Enabled = false
	cfg.SSH.Enabled = true
	cfg.MaxRetryPeriod = 5 * time.Millisecond
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.ConnectFailureC = make(chan time.Duration, 5)
	cfg.ClientTimeout = time.Millisecond
	cfg.InstanceMetadataClient = cloud.NewDisabledIMDSClient()
	cfg.Log = utils.NewLoggerForTests()
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

	timeout := time.After(10 * time.Second)
	step := cfg.MaxRetryPeriod / 5.0
	for i := 0; i < 5; i++ {
		// wait for connection to fail
		select {
		case duration := <-process.Config.ConnectFailureC:
			stepMin := step * time.Duration(i) / 2
			stepMax := step * time.Duration(i+1)

			require.GreaterOrEqual(t, duration, stepMin)
			require.LessOrEqual(t, duration, stepMax)
		case <-timeout:
			t.Fatalf("timeout waiting for failure %d", i)
		}
	}

	supervisor, ok := process.Supervisor.(*LocalSupervisor)
	require.True(t, ok)
	supervisor.signalExit()
	wg.Wait()
}

func TestTeleportProcessAuthVersionCheck(t *testing.T) {
	t.Parallel()

	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	authAddr, err := getFreePort()
	require.NoError(t, err)
	listenAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: authAddr}
	token := "join-token"

	// Create Node process.
	nodeCfg := servicecfg.MakeDefaultConfig()
	nodeCfg.SetAuthServerAddress(listenAddr)
	nodeCfg.DataDir = t.TempDir()
	nodeCfg.SetToken(token)
	nodeCfg.Auth.Enabled = false
	nodeCfg.Proxy.Enabled = false
	nodeCfg.SSH.Enabled = true

	// Set the Node's major version to be greater than the Auth Service's,
	// which should make the version check fail.
	currentVersion, err := semver.NewVersion(teleport.Version)
	require.NoError(t, err)
	currentVersion.Major++
	nodeCfg.TeleportVersion = currentVersion.String()

	// Create Auth Service process.
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Roles: []types.SystemRole{
					types.RoleNode,
				},
				Token: token,
			},
		},
	})
	require.NoError(t, err)

	authCfg := servicecfg.MakeDefaultConfig()
	authCfg.SetAuthServerAddress(listenAddr)
	authCfg.DataDir = t.TempDir()
	authCfg.Auth.Enabled = true
	authCfg.Auth.StaticTokens = staticTokens
	authCfg.Auth.StorageConfig.Type = lite.GetName()
	authCfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(authCfg.DataDir, defaults.BackendDir)}
	authCfg.Auth.ListenAddr = listenAddr
	authCfg.Proxy.Enabled = false
	authCfg.SSH.Enabled = false

	authProc, err := NewTeleport(authCfg)
	require.NoError(t, err)

	err = authProc.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		authProc.Close()
	})

	t.Run("with version check", func(t *testing.T) {
		testVersionCheck(t, nodeCfg, false)
	})

	t.Run("without version check", func(t *testing.T) {
		testVersionCheck(t, nodeCfg, true)
	})
}

func testVersionCheck(t *testing.T, nodeCfg *servicecfg.Config, skipVersionCheck bool) {
	nodeCfg.SkipVersionCheck = skipVersionCheck

	nodeProc, err := NewTeleport(nodeCfg)
	require.NoError(t, err)

	c, err := nodeProc.reconnectToAuthService(types.RoleNode)
	if skipVersionCheck {
		require.NoError(t, err)
		require.NotNil(t, c)
	} else {
		require.True(t, trace.IsNotImplemented(err))
		require.Nil(t, c)
	}

	supervisor, ok := nodeProc.Supervisor.(*LocalSupervisor)
	require.True(t, ok)
	supervisor.signalExit()
}

func getFreePort() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "", err
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).String(), nil
}

func Test_readOrGenerateHostID(t *testing.T) {
	var (
		id          = uuid.New().String()
		hostUUIDKey = "/host_uuid"
	)
	type args struct {
		kubeBackend   *fakeKubeBackend
		hostIDContent string
		identity      []*auth.Identity
	}
	tests := []struct {
		name             string
		args             args
		wantFunc         func(string) bool
		wantKubeItemFunc func(*backend.Item) bool
	}{
		{
			name: "load from storage without kube backend",
			args: args{
				kubeBackend:   nil,
				hostIDContent: id,
			},
			wantFunc: func(receivedID string) bool {
				return receivedID == id
			},
		},
		{
			name: "Kube Backend is available but key is missing. Load from local storage and store in lube",
			args: args{
				kubeBackend: &fakeKubeBackend{
					getData: nil,
					getErr:  fmt.Errorf("key not found"),
				},
				hostIDContent: id,
			},
			wantFunc: func(receivedID string) bool {
				return receivedID == id
			},
			wantKubeItemFunc: func(i *backend.Item) bool {
				return cmp.Diff(&backend.Item{
					Key:   []byte(hostUUIDKey),
					Value: []byte(id),
				}, i) == ""
			},
		},
		{
			name: "Kube Backend is available with key. Load from kube storage",
			args: args{
				kubeBackend: &fakeKubeBackend{
					getData: &backend.Item{
						Key:   []byte(hostUUIDKey),
						Value: []byte(id),
					},
					getErr: nil,
				},
			},
			wantFunc: func(receivedID string) bool {
				return receivedID == id
			},
			wantKubeItemFunc: func(i *backend.Item) bool {
				return i == nil
			},
		},
		{
			name: "No hostID available. Generate one and store it into Kube and Local Storage",
			args: args{
				kubeBackend: &fakeKubeBackend{
					getData: nil,
					getErr:  fmt.Errorf("key not found"),
				},
			},
			wantFunc: func(receivedID string) bool {
				_, err := uuid.Parse(receivedID)
				return err == nil
			},
			wantKubeItemFunc: func(i *backend.Item) bool {
				_, err := uuid.Parse(string(i.Value))
				return err == nil && string(i.Key) == hostUUIDKey
			},
		},
		{
			name: "No hostID available. Generate one and store it into Local Storage",
			args: args{
				kubeBackend: nil,
			},
			wantFunc: func(receivedID string) bool {
				_, err := uuid.Parse(receivedID)
				return err == nil
			},
			wantKubeItemFunc: nil,
		},
		{
			name: "No hostID available. Grab from provided static identity",
			args: args{
				kubeBackend: &fakeKubeBackend{
					getData: nil,
					getErr:  fmt.Errorf("key not found"),
				},

				identity: []*auth.Identity{
					{
						ID: auth.IdentityID{
							HostUUID: id,
						},
					},
				},
			},
			wantFunc: func(receivedID string) bool {
				return receivedID == id
			},
			wantKubeItemFunc: func(i *backend.Item) bool {
				_, err := uuid.Parse(string(i.Value))
				return err == nil && string(i.Key) == hostUUIDKey
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			// write host_uuid file to temp dir.
			if len(tt.args.hostIDContent) > 0 {
				err := utils.WriteHostUUID(dataDir, tt.args.hostIDContent)
				require.NoError(t, err)
			}

			cfg := &servicecfg.Config{
				DataDir:    dataDir,
				Log:        logrus.New(),
				JoinMethod: types.JoinMethodToken,
				Identities: tt.args.identity,
			}

			var kubeBackend kubernetesBackend
			if tt.args.kubeBackend != nil {
				kubeBackend = tt.args.kubeBackend
			}

			err := readOrGenerateHostID(context.Background(), cfg, kubeBackend)
			require.NoError(t, err)

			require.True(t, tt.wantFunc(cfg.HostUUID))

			if tt.args.kubeBackend != nil {
				require.True(t, tt.wantKubeItemFunc(tt.args.kubeBackend.putData))
			}
		})
	}
}

type fakeKubeBackend struct {
	putData *backend.Item
	getData *backend.Item
	getErr  error
}

// Put puts value into backend (creates if it does not
// exists, updates it otherwise)
func (f *fakeKubeBackend) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	f.putData = &i
	return &backend.Lease{}, nil
}

// Get returns a single item or not found error
func (f *fakeKubeBackend) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	return f.getData, f.getErr
}

func TestProxyGRPCServers(t *testing.T) {
	hostID := uuid.NewString()
	// Create a test auth server to extract the server identity (SSH and TLS
	// certificates).
	testAuthServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now()),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, testAuthServer.Close())
	})

	tlsServer, err := testAuthServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, tlsServer.Close())
	})
	// Create a new client using the server identity.
	client, err := tlsServer.NewClient(auth.TestServerID(types.RoleProxy, hostID))
	require.NoError(t, err)
	// TLS config for proxy service.
	serverIdentity, err := auth.NewServerIdentity(testAuthServer.AuthServer, hostID, types.RoleProxy)
	require.NoError(t, err)

	testConnector := &Connector{
		ServerIdentity: serverIdentity,
		Client:         client,
	}

	// Create a listener for the insecure gRPC server.
	insecureListener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := insecureListener.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	// Create a listener for the secure gRPC server.
	secureListener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := secureListener.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	// Create a new Teleport process to initialize the gRPC servers with KubeProxy
	// enabled.
	log := logrus.New()
	process := &TeleportProcess{
		Supervisor: NewSupervisor(hostID, log),
		Config: &servicecfg.Config{
			Proxy: servicecfg.ProxyConfig{
				Kube: servicecfg.KubeProxyConfig{
					Enabled: true,
				},
			},
		},
		log: log,
	}

	// Create a limiter with no limits.
	limiter, err := limiter.NewLimiter(limiter.Config{})
	require.NoError(t, err)

	// Create a error channel to collect the errors from the gRPC servers.
	errC := make(chan error, 2)
	t.Cleanup(func() {
		for i := 0; i < 2; i++ {
			err := <-errC
			if errors.Is(err, net.ErrClosed) {
				continue
			}
			require.NoError(t, err)
		}
	})

	// Insecure gRPC server.
	insecureGPRC := process.initPublicGRPCServer(limiter, testConnector, insecureListener)
	t.Cleanup(insecureGPRC.GracefulStop)

	proxyLockWatcher, err := services.NewLockWatcher(context.Background(), services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    testConnector.Client,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		proxyLockWatcher.Close()
	})
	// Secure gRPC server.
	secureGRPC, err := process.initSecureGRPCServer(initSecureGRPCServerCfg{
		limiter:     limiter,
		conn:        testConnector,
		listener:    secureListener,
		accessPoint: testConnector.Client,
		lockWatcher: proxyLockWatcher,
		emitter:     testConnector.Client,
	})
	require.NoError(t, err)
	t.Cleanup(secureGRPC.GracefulStop)

	// Start the gRPC servers.
	go func() {
		errC <- trace.Wrap(insecureGPRC.Serve(insecureListener))
	}()
	go func() {
		errC <- secureGRPC.Serve(secureListener)
	}()

	tests := []struct {
		name         string
		credentials  credentials.TransportCredentials
		listenerAddr string
		assertErr    require.ErrorAssertionFunc
	}{
		{
			name:         "insecure client to insecure server",
			credentials:  insecure.NewCredentials(),
			listenerAddr: insecureListener.Addr().String(),
			assertErr:    require.NoError,
		},
		{
			name:         "insecure client to secure server with insecure skip verify",
			credentials:  credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}),
			listenerAddr: secureListener.Addr().String(),
			assertErr:    require.Error,
		},
		{
			name:         "insecure client to secure server",
			credentials:  credentials.NewTLS(&tls.Config{}),
			listenerAddr: secureListener.Addr().String(),
			assertErr:    require.Error,
		},
		{
			name: "secure client to secure server",
			credentials: func() credentials.TransportCredentials {
				// Create a new client using the server identity.
				creds, err := testConnector.ServerIdentity.TLSConfig(nil)
				require.NoError(t, err)
				return credentials.NewTLS(creds)
			}(),
			listenerAddr: secureListener.Addr().String(),
			assertErr:    require.NoError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			t.Cleanup(cancel)
			_, err := grpc.DialContext(
				ctx,
				tt.listenerAddr,
				grpc.WithTransportCredentials(tt.credentials),
				// This setting is required to return the connection error instead of
				// wrapping it in a "context deadline exceeded" error.
				// It also enforces the grpc.WithBlock() option.
				grpc.WithReturnConnectionError(),
				grpc.WithDisableRetry(),
				grpc.FailOnNonTempDialError(true),
			)
			tt.assertErr(t, err)
		})
	}
}

func TestEnterpriseServicesEnabled(t *testing.T) {
	tests := []struct {
		name       string
		enterprise bool
		config     *servicecfg.Config
		expected   bool
	}{
		{
			name:       "enterprise enabled, okta enabled",
			enterprise: true,
			config: &servicecfg.Config{
				Okta: servicecfg.OktaConfig{
					Enabled: true,
				},
			},
			expected: true,
		},
		{
			name:       "enterprise disabled, okta enabled",
			enterprise: false,
			config: &servicecfg.Config{
				Okta: servicecfg.OktaConfig{
					Enabled: true,
				},
			},
			expected: false,
		},
		{
			name:       "enterprise enabled, okta disabled",
			enterprise: true,
			config: &servicecfg.Config{
				Okta: servicecfg.OktaConfig{
					Enabled: false,
				},
			},
			expected: false,
		},
		{
			name:       "jamf enabled",
			enterprise: true,
			config: &servicecfg.Config{
				Jamf: servicecfg.JamfConfig{
					Spec: &types.JamfSpecV1{
						Enabled:     true,
						ApiEndpoint: "https://example.jamfcloud.com",
						Username:    "llama",
						Password:    "supersecret!!1!ONE",
					},
				},
			},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildType := modules.BuildOSS
			if tt.enterprise {
				buildType = modules.BuildEnterprise
			}
			modules.SetTestModules(t, &modules.TestModules{
				TestBuildType: buildType,
			})

			process := &TeleportProcess{
				Config: tt.config,
			}

			require.Equal(t, tt.expected, process.enterpriseServicesEnabled())
		})
	}
}

func TestSingleProcessModeResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mode      types.ProxyListenerMode
		config    servicecfg.Config
		wantError bool
		wantAddr  string
	}{
		{
			name: "not single process mode",
			mode: types.ProxyListenerMode_Separate,
			config: servicecfg.Config{
				Proxy: servicecfg.ProxyConfig{
					Enabled: true,
				},
				Auth: servicecfg.AuthConfig{
					Enabled: false,
				},
			},
			wantError: true,
		},
		{
			name: "reverse tunnel disabled",
			mode: types.ProxyListenerMode_Separate,
			config: servicecfg.Config{
				Proxy: servicecfg.ProxyConfig{
					Enabled:              true,
					DisableReverseTunnel: true,
				},
				Auth: servicecfg.AuthConfig{
					Enabled: true,
				},
			},
			wantError: true,
		},
		{
			name: "separate port localhost",
			mode: types.ProxyListenerMode_Separate,
			config: servicecfg.Config{
				Proxy: servicecfg.ProxyConfig{
					Enabled: true,
				},
				Auth: servicecfg.AuthConfig{
					Enabled: true,
				},
			},
			wantAddr: "tcp://localhost:3024",
		},
		{
			name: "separate port tunnel addr",
			mode: types.ProxyListenerMode_Separate,
			config: servicecfg.Config{
				Proxy: servicecfg.ProxyConfig{
					Enabled: true,
					TunnelPublicAddrs: []utils.NetAddr{
						*utils.MustParseAddr("example.com:12345"),
						*utils.MustParseAddr("example.org:12345"),
					},
				},
				Auth: servicecfg.AuthConfig{
					Enabled: true,
				},
			},
			wantAddr: "tcp://example.com:12345",
		},
		{
			name: "multiplex public addr",
			mode: types.ProxyListenerMode_Multiplex,
			config: servicecfg.Config{
				Proxy: servicecfg.ProxyConfig{
					Enabled: true,
					PublicAddrs: []utils.NetAddr{
						*utils.MustParseAddr("example.com:12345"),
						*utils.MustParseAddr("example.org:12345"),
					},
				},
				Auth: servicecfg.AuthConfig{
					Enabled: true,
				},
			},
			wantAddr: "tcp://example.com:12345",
		},
		{
			name: "multiplex web addr with https scheme",
			mode: types.ProxyListenerMode_Multiplex,
			config: servicecfg.Config{
				Proxy: servicecfg.ProxyConfig{
					Enabled: true,
					WebAddr: *utils.MustParseAddr("https://example.com:12345"),
				},
				Auth: servicecfg.AuthConfig{
					Enabled: true,
				},
			},
			wantAddr: "tcp://example.com:12345",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process := TeleportProcess{Config: &test.config}
			resolver := process.SingleProcessModeResolver(test.mode)
			require.NotNil(t, resolver)
			addr, mode, err := resolver(context.Background())
			if test.wantError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, mode, test.mode)
			require.Equal(t, addr.FullAddress(), test.wantAddr)
		})
	}
}
