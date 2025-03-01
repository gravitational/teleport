/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package web

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	authproto "github.com/gravitational/teleport/api/client/proto"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	kubeproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	transportpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/transport/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	tlsutils "github.com/gravitational/teleport/lib/auth/keygen"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/conntest"
	dbrepl "github.com/gravitational/teleport/lib/client/db/repl"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/inventory"
	kubeproxy "github.com/gravitational/teleport/lib/kube/proxy"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/secret"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/desktop"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/regular"
	"github.com/gravitational/teleport/lib/srv/transport/transportv1"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/ui"
	"github.com/gravitational/teleport/lib/utils"
	utilsaws "github.com/gravitational/teleport/lib/utils/aws"
	"github.com/gravitational/teleport/lib/web/app"
	websession "github.com/gravitational/teleport/lib/web/session"
	"github.com/gravitational/teleport/lib/web/terminal"
	webui "github.com/gravitational/teleport/lib/web/ui"
)

const hostID = "00000000-0000-0000-0000-000000000000"

type WebSuite struct {
	ctx    context.Context
	cancel context.CancelFunc

	node        *regular.Server
	proxy       *regular.Server
	proxyTunnel reversetunnelclient.Server
	srvID       string

	user       string
	webServer  *httptest.Server
	webHandler *APIHandler

	mockU2F     *mocku2f.Key
	server      *auth.TestServer
	proxyClient *authclient.Client
	clock       *clockwork.FakeClock
}

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise, it will run tests as normal.
func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	modules.SetInsecureTestMode(true)
	// If the test is re-executing itself, execute the command that comes over
	// the pipe.
	if srv.IsReexec() {
		srv.RunAndExit(os.Args[1])
		return
	}
	cryptosuites.PrecomputeRSATestKeys(m)

	// Otherwise run tests as normal.
	code := m.Run()
	os.Exit(code)
}

func newWebSuite(t *testing.T) *WebSuite {
	return newWebSuiteWithConfig(t, webSuiteConfig{})
}

type webSuiteConfig struct {
	// AuthPreferenceSpec is custom initial AuthPreference spec for the test.
	authPreferenceSpec *types.AuthPreferenceSpecV2

	disableDiskBasedRecording bool

	uiConfig webclient.UIConfig

	// Custom "HealthCheckAppServer" function. Can be used to avoid dialing app
	// services.
	HealthCheckAppServer healthCheckAppServerFunc

	// ClusterFeatures allows overriding default auth server features
	ClusterFeatures *authproto.Features

	// presenceChecker executes presence prompts for tests
	presenceChecker PresenceChecker

	// clock to use for all server components
	clock *clockwork.FakeClock

	// databaseREPLGetter allows setting custom database REPLs.
	databaseREPLGetter dbrepl.REPLRegistry
}

func newWebSuiteWithConfig(t *testing.T, cfg webSuiteConfig) *WebSuite {
	mockU2F, err := mocku2f.Create()
	require.NoError(t, err)
	require.NotNil(t, mockU2F)

	u, err := user.Current()
	require.NoError(t, err)

	if cfg.clock == nil {
		cfg.clock = clockwork.NewFakeClock()
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &WebSuite{
		mockU2F: mockU2F,
		clock:   cfg.clock,
		user:    u.Username,
		ctx:     ctx,
		cancel:  cancel,
	}

	networkingConfig, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
		KeepAliveInterval: types.Duration(10 * time.Second),
	})
	require.NoError(t, err)

	authCfg := auth.TestServerConfig{
		Auth: auth.TestAuthServerConfig{
			ClusterName:             "localhost",
			Dir:                     t.TempDir(),
			Clock:                   s.clock,
			ClusterNetworkingConfig: networkingConfig,
			AuthPreferenceSpec:      cfg.authPreferenceSpec,
		},
	}

	if cfg.disableDiskBasedRecording {
		authCfg.Auth.AuditLog = events.NewDiscardAuditLog()
	}

	s.server, err = auth.NewTestServer(authCfg)
	require.NoError(t, err)

	if cfg.disableDiskBasedRecording {
		// use a sync recording mode because the disk-based uploader
		// that runs in the background introduces races with test cleanup
		recConfig := types.DefaultSessionRecordingConfig()
		recConfig.SetMode(types.RecordAtNodeSync)
		_, err := s.server.AuthServer.AuthServer.UpsertSessionRecordingConfig(context.Background(), recConfig)
		require.NoError(t, err)
	}

	// Register the auth server, since test auth server doesn't start its own
	// heartbeat.
	err = s.server.Auth().UpsertAuthServer(ctx, &types.ServerV2{
		Kind:    types.KindAuthServer,
		Version: types.V2,
		Metadata: types.Metadata{
			Namespace: apidefaults.Namespace,
			Name:      "auth",
		},
		Spec: types.ServerSpecV2{
			Addr:     s.server.TLS.Listener.Addr().String(),
			Hostname: "localhost",
			Version:  teleport.Version,
		},
	})
	require.NoError(t, err)

	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPub, err := auth.PrivateKeyToPublicKeyTLS(priv)
	require.NoError(t, err)

	nodeID := "node"

	// start node
	certs, err := s.server.Auth().GenerateHostCerts(s.ctx,
		&authproto.HostCertsRequest{
			HostID:       hostID,
			NodeName:     nodeID,
			Role:         types.RoleNode,
			PublicSSHKey: pub,
			PublicTLSKey: tlsPub,
		})
	require.NoError(t, err)

	signer, err := sshutils.NewSigner(priv, certs.SSH)
	require.NoError(t, err)

	nodeClient, err := s.server.NewClient(auth.TestIdentity{
		I: authz.BuiltinRole{
			Role:     types.RoleNode,
			Username: nodeID,
		},
	})
	require.NoError(t, err)

	nodeLockWatcher, err := services.NewLockWatcher(s.ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentNode,
			Client:    nodeClient,
		},
	})
	require.NoError(t, err)

	nodeSessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   nodeClient,
		AccessPoint:  nodeClient,
		LockEnforcer: nodeLockWatcher,
		Emitter:      nodeClient,
		Component:    teleport.ComponentNode,
		ServerID:     nodeID,
	})
	require.NoError(t, err)

	// create SSH service:
	nodeDataDir := t.TempDir()
	node, err := regular.New(
		ctx,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		nodeID,
		sshutils.StaticHostSigners(signer),
		nodeClient,
		nodeDataDir,
		"",
		utils.NetAddr{},
		nodeClient,
		regular.SetUUID(nodeID),
		regular.SetNamespace(apidefaults.Namespace),
		regular.SetEmitter(nodeClient),
		regular.SetPAMConfig(&servicecfg.PAMConfig{Enabled: false}),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetClock(s.clock),
		regular.SetLockWatcher(nodeLockWatcher),
		regular.SetSessionController(nodeSessionController),
	)
	require.NoError(t, err)
	s.node = node
	s.srvID = node.ID()
	require.NoError(t, s.node.Start())

	// create reverse tunnel service:
	proxyID := "proxy"
	s.proxyClient, err = s.server.NewClient(auth.TestIdentity{
		I: authz.BuiltinRole{
			Role:     types.RoleProxy,
			Username: proxyID,
		},
	})
	require.NoError(t, err)

	revTunListener, err := net.Listen("tcp", fmt.Sprintf("%v:0", s.server.ClusterName()))
	require.NoError(t, err)

	proxyLockWatcher, err := services.NewLockWatcher(s.ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    s.proxyClient,
		},
	})
	require.NoError(t, err)

	proxyNodeWatcher, err := services.NewNodeWatcher(s.ctx, services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    s.proxyClient,
		},
		NodesGetter: s.proxyClient,
	})
	require.NoError(t, err)

	caWatcher, err := services.NewCertAuthorityWatcher(s.ctx, services.CertAuthorityWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    s.proxyClient,
		},
		Types: []types.CertAuthType{types.HostCA, types.UserCA},
	})
	require.NoError(t, err)
	defer caWatcher.Close()

	proxyGitServerWatcher, err := services.NewGitServerWatcher(ctx, services.GitServerWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    s.proxyClient,
		},
		GitServerGetter: s.proxyClient.GitServerReadOnlyClient(),
	})
	require.NoError(t, err)
	t.Cleanup(proxyGitServerWatcher.Close)

	revTunServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ID:       node.ID(),
		Listener: revTunListener,
		GetClientTLSCertificate: func() (*tls.Certificate, error) {
			return &s.proxyClient.TLSConfig().Certificates[0], nil
		},
		ClusterName:           s.server.ClusterName(),
		GetHostSigners:        sshutils.StaticHostSigners(signer),
		LocalAuthClient:       s.proxyClient,
		LocalAccessPoint:      s.proxyClient,
		Emitter:               s.proxyClient,
		NewCachingAccessPoint: noCache,
		DataDir:               t.TempDir(),
		LockWatcher:           proxyLockWatcher,
		NodeWatcher:           proxyNodeWatcher,
		GitServerWatcher:      proxyGitServerWatcher,
		CertAuthorityWatcher:  caWatcher,
		CircuitBreakerConfig:  breaker.NoopBreakerConfig(),
		LocalAuthAddresses:    []string{s.server.TLS.Listener.Addr().String()},
		Clock:                 s.clock,
	})
	require.NoError(t, err)
	s.proxyTunnel = revTunServer

	router, err := proxy.NewRouter(proxy.RouterConfig{
		ClusterName:      s.server.ClusterName(),
		LocalAccessPoint: s.proxyClient,
		SiteGetter:       revTunServer,
		TracerProvider:   tracing.NoopProvider(),
	})
	require.NoError(t, err)

	proxySessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   s.proxyClient,
		AccessPoint:  s.proxyClient,
		LockEnforcer: proxyLockWatcher,
		Emitter:      s.proxyClient,
		Component:    teleport.ComponentProxy,
		ServerID:     proxyID,
	})
	require.NoError(t, err)

	// proxy server:
	s.proxy, err = regular.New(
		ctx,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		s.server.ClusterName(),
		sshutils.StaticHostSigners(signer),
		s.proxyClient,
		t.TempDir(),
		"",
		utils.NetAddr{},
		s.proxyClient,
		regular.SetUUID(proxyID),
		regular.SetProxyMode("", revTunServer, s.proxyClient, router),
		regular.SetEmitter(s.proxyClient),
		regular.SetNamespace(apidefaults.Namespace),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetClock(s.clock),
		regular.SetLockWatcher(proxyLockWatcher),
		regular.SetSessionController(proxySessionController),
	)
	require.NoError(t, err)

	// Expired sessions are purged immediately
	var sessionLingeringThreshold time.Duration
	fs, err := NewDebugFileSystem(false)
	require.NoError(t, err)

	features := *modules.GetModules().Features().ToProto() // safe to dereference because ToProto creates a struct and return a pointer to it
	if cfg.ClusterFeatures != nil {
		features = *cfg.ClusterFeatures
	}
	authID := state.IdentityID{
		Role:     types.RoleProxy,
		HostUUID: proxyID,
	}
	dns := []string{"localhost", "127.0.0.1"}
	proxyIdentity, err := auth.LocalRegister(authID, s.server.Auth(), nil, dns, "", nil)
	require.NoError(t, err)
	proxyClientCert, err := keys.X509KeyPair(proxyIdentity.TLSCertBytes, proxyIdentity.KeyBytes)
	require.NoError(t, err)

	handlerConfig := Config{
		ClusterFeatures:                 features,
		Proxy:                           revTunServer,
		AuthServers:                     utils.FromAddr(s.server.TLS.Addr()),
		ProxyClient:                     s.proxyClient,
		CipherSuites:                    utils.DefaultCipherSuites(),
		AccessPoint:                     s.proxyClient,
		Context:                         s.ctx,
		HostUUID:                        proxyID,
		Emitter:                         s.proxyClient,
		StaticFS:                        fs,
		CachedSessionLingeringThreshold: &sessionLingeringThreshold,
		ProxySettings: &ProxySettings{
			ServiceConfig: servicecfg.MakeDefaultConfig(),
			ProxySSHAddr:  "127.0.0.1",
			AccessPoint:   s.server.Auth(),
		},
		SessionControl: SessionControllerFunc(func(ctx context.Context, sctx *SessionContext, login, localAddr, remoteAddr string) (context.Context, error) {
			controller := srv.WebSessionController(proxySessionController)
			ctx, err := controller(ctx, sctx, login, localAddr, remoteAddr)
			return ctx, trace.Wrap(err)
		}),
		Router:               router,
		HealthCheckAppServer: cfg.HealthCheckAppServer,
		UI:                   cfg.uiConfig,
		PresenceChecker:      cfg.presenceChecker,
		GetProxyClientCertificate: func() (*tls.Certificate, error) {
			return &proxyClientCert, nil
		},
		IntegrationAppHandler: &mockIntegrationAppHandler{},
		DatabaseREPLRegistry:  cfg.databaseREPLGetter,
	}

	if handlerConfig.HealthCheckAppServer == nil {
		handlerConfig.HealthCheckAppServer = func(context.Context, string, string) error { return nil }
	}

	handler, err := NewHandler(handlerConfig, SetClock(s.clock))
	require.NoError(t, err)

	s.webServer = httptest.NewUnstartedServer(handler)
	s.webHandler = handler
	s.webServer.StartTLS()
	err = s.proxy.Start()
	require.NoError(t, err)

	// Wait for proxy to fully register before starting the test.
	for start := time.Now(); ; {
		proxies, err := s.proxyClient.GetProxies()
		require.NoError(t, err)
		if len(proxies) != 0 {
			break
		}
		if time.Since(start) > 5*time.Second {
			t.Fatal("proxy didn't register within 5s after startup")
		}
	}

	proxyAddr := utils.MustParseAddr(s.proxy.Addr())

	addr := utils.MustParseAddr(s.webServer.Listener.Addr().String())
	handler.handler.cfg.ProxyWebAddr = *addr
	handler.handler.cfg.ProxySSHAddr = *proxyAddr
	_, sshPort, err := net.SplitHostPort(proxyAddr.String())
	require.NoError(t, err)
	handler.handler.sshPort = sshPort

	t.Cleanup(func() {
		// In particular close the lock watchers by canceling the context.
		s.cancel()

		s.webServer.Close()

		var errors []error
		if err := s.proxyTunnel.Close(); err != nil {
			errors = append(errors, err)
		}
		if err := s.node.Close(); err != nil {
			errors = append(errors, err)
		}
		s.webServer.Close()
		if err := s.proxy.Close(); err != nil {
			errors = append(errors, err)
		}
		if err := s.server.Shutdown(context.Background()); err != nil {
			errors = append(errors, err)
		}
		require.Empty(t, errors)
	})

	return s
}

func (s *WebSuite) addNode(t *testing.T, uuid string, hostname string, address string) *regular.Server {
	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPub, err := auth.PrivateKeyToPublicKeyTLS(priv)
	require.NoError(t, err)

	// start node
	certs, err := s.server.Auth().GenerateHostCerts(s.ctx,
		&authproto.HostCertsRequest{
			HostID:       uuid,
			NodeName:     hostname,
			Role:         types.RoleNode,
			PublicSSHKey: pub,
			PublicTLSKey: tlsPub,
		})
	require.NoError(t, err)

	signer, err := sshutils.NewSigner(priv, certs.SSH)
	require.NoError(t, err)

	nodeClient, err := s.server.NewClient(auth.TestIdentity{
		I: authz.BuiltinRole{
			Role:     types.RoleNode,
			Username: uuid,
		},
	})
	require.NoError(t, err)

	nodeLockWatcher, err := services.NewLockWatcher(s.ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentNode,
			Client:    nodeClient,
		},
	})
	require.NoError(t, err)

	nodeSessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   nodeClient,
		AccessPoint:  nodeClient,
		LockEnforcer: nodeLockWatcher,
		Emitter:      nodeClient,
		Component:    teleport.ComponentNode,
		ServerID:     uuid,
	})
	require.NoError(t, err)

	// create SSH service:
	node, err := regular.New(
		context.Background(),
		utils.NetAddr{AddrNetwork: "tcp", Addr: address},
		hostname,
		sshutils.StaticHostSigners(signer),
		nodeClient,
		t.TempDir(),
		"",
		utils.NetAddr{},
		nodeClient,
		regular.SetUUID(uuid),
		regular.SetNamespace(apidefaults.Namespace),
		regular.SetEmitter(nodeClient),
		regular.SetPAMConfig(&servicecfg.PAMConfig{Enabled: false}),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetClock(s.clock),
		regular.SetLockWatcher(nodeLockWatcher),
		regular.SetSessionController(nodeSessionController),
	)
	require.NoError(t, err)
	require.NoError(t, node.Start())

	t.Cleanup(func() {
		require.NoError(t, node.Close())
		node.Wait()
	})

	return node
}

func noCache(clt authclient.ClientI, cacheName []string) (authclient.RemoteProxyAccessPoint, error) {
	return clt, nil
}

func (r *authPack) renewSession(ctx context.Context, t *testing.T) *roundtrip.Response {
	resp, err := r.clt.PostJSON(ctx, r.clt.Endpoint("webapi", "sessions", "web", "renew"), nil)
	require.NoError(t, err)
	return resp
}

func (r *authPack) validateAPI(ctx context.Context, t *testing.T) {
	_, err := r.clt.Get(ctx, r.clt.Endpoint("webapi", "sites"), url.Values{})
	require.NoError(t, err)
}

type authPack struct {
	otpSecret string
	user      string
	login     string
	password  string
	session   *CreateSessionResponse
	clt       *TestWebClient
	cookies   []*http.Cookie
	device    *auth.TestDevice
}

// authPack returns new authenticated package consisting of created valid
// user, otp token, created web session and authenticated client.
func (s *WebSuite) authPack(t *testing.T, user string, roles ...string) *authPack {
	login := s.user
	pass := "abcdef123456"
	otpSecret := newOTPSharedSecret()

	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	require.NoError(t, err)
	_, err = s.server.Auth().UpsertAuthPreference(s.ctx, ap)
	require.NoError(t, err)

	s.createUser(t, user, login, pass, otpSecret, roles...)

	ctx := context.Background()
	sessionResp, httpResp := loginWebOTP(t, ctx, loginWebOTPParams{
		webClient: s.client(t),
		clock:     s.clock,
		user:      user,
		password:  pass,
		otpSecret: otpSecret,
	})

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	clt := s.client(t, roundtrip.BearerAuth(sessionResp.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(s.url(), httpResp.Cookies())

	return &authPack{
		otpSecret: otpSecret,
		user:      user,
		login:     login,
		session:   sessionResp,
		clt:       clt,
		cookies:   httpResp.Cookies(),
	}
}

func (s *WebSuite) authPackWithMFA(t *testing.T, name string, roles ...types.Role) *authPack {
	const password = "testing12345"
	user, err := types.NewUser(name)
	require.NoError(t, err)

	userRole := services.RoleForUser(user)
	userRole.SetLogins(types.Allow, []string{s.user})
	userRole, err = s.server.Auth().UpsertRole(s.ctx, userRole)
	require.NoError(t, err)

	for _, role := range roles {
		role, err = s.server.Auth().UpsertRole(s.ctx, role)
		require.NoError(t, err)
		user.AddRole(role.GetName())
	}

	user.AddRole(userRole.GetName())
	_, err = s.server.Auth().CreateUser(s.ctx, user)
	require.NoError(t, err)

	clt := s.client(t)

	// create register challenge
	token, err := s.server.Auth().CreateResetPasswordToken(s.ctx, authclient.CreateUserTokenRequest{
		Name: name,
	})
	require.NoError(t, err)

	res, err := s.server.Auth().CreateRegisterChallenge(s.ctx, &authproto.CreateRegisterChallengeRequest{
		TokenID:     token.GetName(),
		DeviceType:  authproto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		DeviceUsage: authproto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
	})
	require.NoError(t, err)

	cc := wantypes.CredentialCreationFromProto(res.GetWebauthn())

	// use passwordless as auth method
	device, err := mocku2f.Create()
	require.NoError(t, err)

	device.SetPasswordless()

	const rpID = "localhost"
	ccr, err := device.SignCredentialCreation("https://"+rpID, cc)
	require.NoError(t, err)

	_, err = s.server.Auth().ChangeUserAuthentication(s.ctx, &authproto.ChangeUserAuthenticationRequest{
		TokenID:     token.GetName(),
		NewPassword: []byte(password),
		NewMFARegisterResponse: &authproto.MFARegisterResponse{
			Response: &authproto.MFARegisterResponse_Webauthn{
				Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
			},
		},
	})
	require.NoError(t, err)

	ctx := context.Background()
	sessionResp, httpResp := loginWebMFA(ctx, t, loginWebMFAParams{
		webClient:     clt,
		rpID:          rpID,
		user:          name,
		password:      password,
		authenticator: device,
	})

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	clt = s.client(t, roundtrip.BearerAuth(sessionResp.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(s.url(), httpResp.Cookies())

	return &authPack{
		user:    name,
		login:   s.user,
		session: sessionResp,
		clt:     clt,
		cookies: httpResp.Cookies(),
		device:  &auth.TestDevice{Key: device},
	}
}

func (s *WebSuite) createUser(t *testing.T, user string, login string, pass string, otpSecret string, roles ...string) {
	teleUser, err := types.NewUser(user)
	require.NoError(t, err)
	role := services.RoleForUser(teleUser)
	role.SetLogins(types.Allow, []string{login})
	options := role.GetOptions()
	options.ForwardAgent = types.NewBool(true)
	role.SetOptions(options)
	role, err = s.server.Auth().UpsertRole(s.ctx, role)
	require.NoError(t, err)
	teleUser.AddRole(role.GetName())

	for _, r := range roles {
		teleUser.AddRole(r)
	}

	teleUser.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: "some-auth-user"},
	})
	_, err = s.server.Auth().CreateUser(s.ctx, teleUser)
	require.NoError(t, err)

	err = s.server.Auth().UpsertPassword(user, []byte(pass))
	require.NoError(t, err)

	if otpSecret != "" {
		dev, err := services.NewTOTPDevice("otp", otpSecret, s.clock.Now())
		require.NoError(t, err)
		err = s.server.Auth().UpsertMFADevice(context.Background(), user, dev)
		require.NoError(t, err)
	}
}

func verifySecurityResponseHeaders(t *testing.T, h http.Header) {
	t.Helper()
	cases := []struct {
		header        string
		expectedValue string
	}{
		{
			header:        "X-Content-Type-Options",
			expectedValue: "nosniff",
		},
		{
			header:        "Referrer-Policy",
			expectedValue: "strict-origin",
		},
		{
			header:        "X-Frame-Options",
			expectedValue: "SAMEORIGIN",
		},
		{
			header:        "Strict-Transport-Security",
			expectedValue: "max-age=31536000; includeSubDomains",
		},
	}

	for _, tc := range cases {
		require.Contains(t, h, tc.header)
		require.Equal(t, tc.expectedValue, h.Get(tc.header))
	}
}

func TestValidRedirectURL(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		desc, url string
		valid     bool
	}{
		{"valid absolute https url", "https://example.com?a=1", true},
		{"valid absolute http url", "http://example.com?a=1", true},
		{"valid relative url", "/path/to/something", true},
		{"garbage", "fjoiewjwpods302j09", false},
		{"empty string", "", false},
		{"block bad protocol", "javascript:alert('xss')", false},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.valid, isValidRedirectURL(tt.url))
		})
	}
}

func TestMetaRedirect(t *testing.T) {
	t.Parallel()
	h := &Handler{}
	redirectHandler := h.WithMetaRedirect(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) string {
		return "https://example.com"
	})
	req := httptest.NewRequest(http.MethodPost, "/some/route", nil)
	resp := httptest.NewRecorder()
	redirectHandler(resp, req, nil)
	targetElement := `<meta http-equiv="refresh" content="0;URL='https://example.com'" />`
	require.Equal(t, http.StatusOK, resp.Code)
	body := resp.Body.String()
	require.Contains(t, body, targetElement)
}

func Test_clientMetaFromReq(t *testing.T) {
	ua := "foobar"
	r := httptest.NewRequest(
		http.MethodGet, "https://example.com/webapi/foo", nil,
	)
	r.Header.Set("User-Agent", ua)

	got := clientMetaFromReq(r)
	require.Equal(t, &authclient.ForwardedClientMetadata{
		UserAgent:  ua,
		RemoteAddr: "192.0.2.1:1234",
	}, got)
}

func TestWebSessionsCRUD(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	pack := s.authPack(t, "foo")

	// make sure we can use client to make authenticated requests
	re, err := pack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites"), url.Values{})
	require.NoError(t, err)

	var clusters []webui.Cluster
	require.NoError(t, json.Unmarshal(re.Bytes(), &clusters))

	// now delete session
	_, err = pack.clt.Delete(
		context.Background(),
		pack.clt.Endpoint("webapi", "sessions", "web"))
	require.NoError(t, err)

	// subsequent requests trying to use this session will fail
	_, err = pack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites"), url.Values{})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestCSRF(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)

	// create a valid user
	user := "csrfuser"
	pass := "abcdef123456"
	otpSecret := newOTPSharedSecret()
	s.createUser(t, user, user, pass, otpSecret)

	clt := s.client(t)
	ctx := context.Background()

	// valid
	validReq := loginWebOTPParams{
		webClient: clt,
		clock:     s.clock,
		user:      user,
		password:  pass,
		otpSecret: otpSecret,
	}
	loginWebOTP(t, ctx, validReq)

	// invalid - wrong content-type header
	invalidReq := validReq
	invalidReq.overrideContentType = "multipart/form-data"
	httpResp, _, err := rawLoginWebOTP(ctx, invalidReq)
	require.NoError(t, err, "Login via /webapi/sessions/new failed unexpectedly")
	require.Equal(t, http.StatusBadRequest, httpResp.StatusCode, "HTTP status code mismatch")
}

func TestPasswordChange(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	pack := s.authPack(t, "foo")

	// invalidate the token
	s.clock.Advance(1 * time.Minute)
	validToken, err := totp.GenerateCode(pack.otpSecret, s.clock.Now())
	require.NoError(t, err)

	req := changePasswordReq{
		OldPassword:       []byte("abcdef123456"),
		NewPassword:       []byte("fedcba654321"),
		SecondFactorToken: validToken,
	}

	_, err = pack.clt.PutJSON(context.Background(), pack.clt.Endpoint("webapi", "users", "password"), req)
	require.NoError(t, err)
}

// TestValidateBearerToken tests that the bearer token's user name
// matches the user name on the cookie.
func TestValidateBearerToken(t *testing.T) {
	t.Parallel()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack1 := proxy.authPack(t, "user1", nil /* roles */)
	pack2 := proxy.authPack(t, "user2", nil /* roles */)

	// Swap pack1's session token with pack2's sessionToken
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	pack1.clt = proxy.newClient(t, roundtrip.BearerAuth(pack2.session.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(&proxy.webURL, pack1.cookies)

	// Auth protected endpoint.
	req := changePasswordReq{}
	_, err = pack1.clt.PutJSON(context.Background(), pack1.clt.Endpoint("webapi", "users", "password"), req)
	require.True(t, trace.IsAccessDenied(err))
	require.True(t, strings.Contains(err.Error(), "bad bearer token"))
}

func TestWebSessionsBadInput(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)

	authServer := s.server.Auth()
	clock := s.clock
	ctx := context.Background()

	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	require.NoError(t, err, "NewAuthPreference failed")
	_, err = authServer.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err, "UpsertAuthPreference failed")

	const user = "bob"
	const pass = "abcdef123456"
	otpSecret := newOTPSharedSecret()
	badSecret := newOTPSharedSecret()

	u, err := types.NewUser(user)
	require.NoError(t, err)
	_, err = authServer.CreateUser(ctx, u)
	require.NoError(t, err)

	err = authServer.UpsertPassword(user, []byte(pass))
	require.NoError(t, err)

	dev, err := services.NewTOTPDevice("otp", otpSecret, s.clock.Now())
	require.NoError(t, err)
	err = authServer.UpsertMFADevice(context.Background(), user, dev)
	require.NoError(t, err)

	clt := s.client(t)

	tests := []struct {
		name                  string
		user, pass, otpSecret string
	}{
		{
			name: "empty request",
		},
		{
			name:      "missing user",
			pass:      pass,
			otpSecret: otpSecret,
		},
		{
			name:      "missing pass",
			user:      user,
			otpSecret: otpSecret,
		},
		{
			name:      "bad pass",
			user:      user,
			pass:      "bla bla",
			otpSecret: otpSecret,
		},
		{
			name:      "bad otp token",
			user:      user,
			pass:      pass,
			otpSecret: badSecret,
		},
		{
			name: "missing otp token",
			user: user,
			pass: pass,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clock.Advance(1 * time.Minute) // Avoid OTP clashes.

			httpResp, body, err := rawLoginWebOTP(ctx, loginWebOTPParams{
				webClient: clt,
				clock:     clock,
				user:      test.user,
				password:  test.pass,
				otpSecret: test.otpSecret,
			})
			require.NoError(t, err, "HTTP request errored unexpectedly")

			// Assert HTTP response code.
			assert.Equal(t, http.StatusForbidden, httpResp.StatusCode, "HTTP status mismatch")

			// Assert body error message.
			var resp httpErrorResponse
			require.NoError(t,
				json.Unmarshal(body, &resp),
				"HTTP error response unmarshal",
			)
			const invalidCredentialsMessage = "invalid credentials"
			assert.Contains(t, resp.Error.Message, invalidCredentialsMessage, "HTTP error message mismatch")
		})
	}
}

type clusterNodesGetResponse struct {
	Items      []webui.Server `json:"items"`
	StartKey   string         `json:"startKey"`
	TotalCount int            `json:"totalCount"`
}

func TestClusterNodesGet(t *testing.T) {
	t.Parallel()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test-user@example.com", nil /* roles */)

	// Get the node already added by `newWebPack`
	servers, err := env.server.Auth().GetNodes(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	server1 := servers[0]

	// Add another node.
	server2, err := types.NewServerWithLabels("server2", types.KindNode, types.ServerSpecV2{Hostname: "server2"}, map[string]string{"test-field": "test-value"})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertNode(context.Background(), server2)
	require.NoError(t, err)

	// Get nodes from endpoint.
	clusterName := env.server.ClusterName()
	endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "nodes")

	query := url.Values{"sort": []string{"name"}}

	// Get nodes.
	re, err := pack.clt.Get(context.Background(), endpoint, query)
	require.NoError(t, err)

	// Test response.
	res := clusterNodesGetResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &res))
	require.Len(t, res.Items, 2)
	require.Equal(t, 2, res.TotalCount)
	require.ElementsMatch(t, res.Items, []webui.Server{
		{
			Kind:        types.KindNode,
			SubKind:     types.SubKindTeleportNode,
			ClusterName: clusterName,
			Name:        server1.GetName(),
			Hostname:    server1.GetHostname(),
			Tunnel:      server1.GetUseTunnel(),
			Addr:        server1.GetAddr(),
			Labels:      []ui.Label{},
			SSHLogins:   []string{pack.login},
		},
		{
			Kind:        types.KindNode,
			SubKind:     types.SubKindTeleportNode,
			ClusterName: clusterName,
			Name:        server2.GetName(),
			Hostname:    server2.GetHostname(),
			Labels:      []ui.Label{{Name: "test-field", Value: "test-value"}},
			Tunnel:      false,
			SSHLogins:   []string{pack.login},
		},
	})

	// Get nodes using shortcut.
	re, err = pack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites", currentSiteShortcut, "nodes"), query)
	require.NoError(t, err)

	res2 := clusterNodesGetResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &res2))
	require.Len(t, res.Items, 2)
	require.Equal(t, res, res2)
}

func TestUserGroupsGet(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test-user@example.com", nil /* roles */)

	type testResponse struct {
		Items      []webui.UserGroup `json:"items"`
		StartKey   string            `json:"startKey"`
		TotalCount int               `json:"totalCount"`
	}

	// add a user group
	ug, err := types.NewUserGroup(types.Metadata{
		Name: "ug1", Description: "ug1-description",
		Labels: map[string]string{"test-field": "test-value"},
	},
		types.UserGroupSpecV1{Applications: []string{"appnameonly"}})
	require.NoError(t, err)
	err = env.server.Auth().CreateUserGroup(ctx, ug)
	require.NoError(t, err)

	resource := &types.AppServerV3{
		Metadata: types.Metadata{Name: "test-app-server"},
		Kind:     types.KindApp,
		Version:  types.V2,
		Spec: types.AppServerSpecV3{
			HostID: "hostid",
			App: &types.AppV3{
				Metadata: types.Metadata{
					Name:        "appnameonly",
					Description: "app-description",
				},
				Spec: types.AppSpecV3{
					URI: "appname-uri",
				},
			},
		},
	}

	// Register app
	_, err = env.server.Auth().UpsertApplicationServer(ctx, resource)
	require.NoError(t, err)

	// Make the call.
	endpoint := pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "user-groups")
	re, err := pack.clt.Get(ctx, endpoint, url.Values{"sort": []string{"name"}})
	require.NoError(t, err)

	// The correct response should include application names (not app server names)
	var resp testResponse
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	require.Equal(t, 1, resp.TotalCount)
	require.ElementsMatch(t, resp.Items, []webui.UserGroup{{
		Name:        "ug1",
		Description: ug.GetMetadata().Description,
		Labels:      []ui.Label{{Name: "test-field", Value: "test-value"}},
		Applications: []webui.ApplicationAndFriendlyName{
			{Name: "appnameonly", FriendlyName: ""},
		},
	}})
}

func TestUnifiedResourcesGet(t *testing.T) {
	t.Parallel()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	username := "test-user@example.com"

	u, err := user.Current()
	require.NoError(t, err)
	loginUser := u.Username

	role := defaultRoleForNewUser(&types.UserV2{Metadata: types.Metadata{Name: username}}, loginUser)
	role.SetAWSRoleARNs(types.Allow, []string{"arn:aws:iam::999999999999:role/ProdInstance"})
	role.SetAppLabels(types.Allow, types.Labels{"env": []string{"prod"}})
	role.SetGitHubPermissions(types.Allow, []types.GitHubPermission{{Organizations: []string{types.Wildcard}}})

	// This role is used to test that DevInstance AWS Role is only available to AppServices that have env:dev label.
	roleForDev, err := types.NewRole("dev-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AWSRoleARNs: []string{"arn:aws:iam::999999999999:role/DevInstance"},
			AppLabels:   types.Labels{"env": []string{"dev"}},
		},
	})
	require.NoError(t, err)

	pack := proxy.authPack(t, username, []types.Role{role, roleForDev})

	// add aws AppServer
	awsApp, err := types.NewAppV3(
		types.Metadata{
			Name: "my-aws-app",
			Labels: map[string]string{
				"env": "prod",
			},
		},
		types.AppSpecV3{
			URI:   "localhost:8080",
			Cloud: "AWS",
		})
	require.NoError(t, err)
	awsAppServer, err := types.NewAppServerV3FromApp(
		awsApp,
		"localhost",
		"host-id",
	)
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertApplicationServer(context.Background(), awsAppServer)
	require.NoError(t, err)

	app, err := types.NewAppV3(
		types.Metadata{
			Name: "my-app",
			Labels: map[string]string{
				"env": "prod",
			},
		},
		types.AppSpecV3{
			URI: "localhost:8080",
		})
	require.NoError(t, err)
	appServer, err := types.NewAppServerV3FromApp(
		app,
		"localhost",
		"host-id",
	)
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertApplicationServer(context.Background(), appServer)
	require.NoError(t, err)

	// Add nodes
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("server-%d", i)
		node, err := types.NewServer(name, types.KindNode, types.ServerSpecV2{
			Hostname: name,
		})
		require.NoError(t, err)
		_, err = env.server.Auth().UpsertNode(context.Background(), node)
		require.NoError(t, err)
	}
	// add db
	db, err := types.NewDatabaseV3(types.Metadata{
		Name: "dbdb",
		Labels: map[string]string{
			"env": "prod",
		},
	}, types.DatabaseSpecV3{
		Protocol: "test-protocol",
		URI:      "test-uri",
	})
	require.NoError(t, err)
	dbServer, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "dddb1",
	}, types.DatabaseServerSpecV3{
		Hostname: "dddb1",
		HostID:   uuid.NewString(),
		Database: db,
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertDatabaseServer(context.Background(), dbServer)
	require.NoError(t, err)

	// add windows desktop
	win, err := types.NewWindowsDesktopV3(
		"zzzz9",
		nil,
		types.WindowsDesktopSpecV3{Addr: "localhost", HostID: "win1-host-id"},
	)
	require.NoError(t, err)
	err = env.server.Auth().UpsertWindowsDesktop(context.Background(), win)
	require.NoError(t, err)

	// add git server
	gitServer, err := types.NewGitHubServer(types.GitHubServerMetadata{
		Organization: "org1",
		Integration:  "org1",
	})
	require.NoError(t, err)
	_, err = env.server.Auth().GitServers.UpsertGitServer(context.Background(), gitServer)
	require.NoError(t, err)

	clusterName := env.server.ClusterName()
	endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "resources")

	// test sort type ascend
	query := url.Values{"sort": []string{"kind:asc"}}
	re, err := pack.clt.Get(context.Background(), endpoint, query)
	require.NoError(t, err)
	res := clusterNodesGetResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &res))
	require.Equal(t, types.KindApp, res.Items[0].Kind)
	require.Equal(t, types.KindApp, res.Items[1].Kind)
	require.Equal(t, types.KindDatabase, res.Items[2].Kind)

	// test sort type desc
	query = url.Values{"sort": []string{"kind:desc"}}
	re, err = pack.clt.Get(context.Background(), endpoint, query)
	require.NoError(t, err)
	res = clusterNodesGetResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &res))
	require.Equal(t, types.KindWindowsDesktop, res.Items[0].Kind)

	// test with no access
	noAccessRole, err := types.NewRole(services.RoleNameForUser("test-no-access@example.com"), types.RoleSpecV6{})
	require.NoError(t, err)
	noAccessPack := proxy.authPack(t, "test-no-access@example.com", []types.Role{noAccessRole})

	// shouldnt get any results with no access
	query = url.Values{"sort": []string{"name:asc"}}
	re, err = noAccessPack.clt.Get(context.Background(), endpoint, query)
	require.NoError(t, err)
	res = clusterNodesGetResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &res))
	require.Empty(t, res.Items)

	// should return first page and have a second page
	query = url.Values{"sort": []string{"name"}, "limit": []string{"15"}}
	re, err = pack.clt.Get(context.Background(), endpoint, query)
	require.NoError(t, err)
	res = clusterNodesGetResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &res))
	require.Len(t, res.Items, 15)
	require.NotEqual(t, "", res.StartKey)

	// should return second page and have no third page
	query = url.Values{"sort": []string{"name"}, "limit": []string{"15"}}
	query.Add("startKey", res.StartKey)
	re, err = pack.clt.Get(context.Background(), endpoint, query)
	require.NoError(t, err)
	res = clusterNodesGetResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &res))
	require.Len(t, res.Items, 11)
	require.Equal(t, "", res.StartKey)

	// Only list valid AWS Roles for AWS Apps
	query = url.Values{
		"search": []string{"my-aws-app"},
		"sort":   []string{"name"},
	}
	re, err = pack.clt.Get(context.Background(), endpoint, query)
	require.NoError(t, err)
	listResp := struct {
		Items []webui.App `json:"Items"`
	}{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &listResp))
	require.Len(t, listResp.Items, 1)
	expectedRoles := []utilsaws.Role{
		{Name: "ProdInstance", Display: "ProdInstance", ARN: "arn:aws:iam::999999999999:role/ProdInstance", AccountID: "999999999999"},
	}
	require.Equal(t, expectedRoles, listResp.Items[0].AWSRoles)
	t.Log(string(re.Bytes()), listResp)
}

type clusterAlertsGetResponse struct {
	Alerts []types.ClusterAlert `json:"alerts"`
}

func TestClusterAlertsGet(t *testing.T) {
	env := newWebPack(t, 1)

	// generate alert
	alert, err := types.NewClusterAlert(
		"test-alert",
		"test alert message",
		types.WithAlertSeverity(0),
		types.WithAlertLabel(types.AlertOnLogin, "yes"),
		// AlertPermitAll is necessary because the alert is only shown to
		// admin clients by default.
		types.WithAlertLabel(types.AlertPermitAll, "yes"),
	)
	require.NoError(t, err)
	err = env.server.Auth().UpsertClusterAlert(context.Background(), alert)
	require.NoError(t, err)

	// get alerts.
	clusterName := env.server.ClusterName()
	pack := env.proxies[0].authPack(t, "test-user@example.com", nil)
	endpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "alerts")
	re, err := pack.clt.Get(context.Background(), endpoint, nil)
	require.NoError(t, err)

	alerts := clusterAlertsGetResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &alerts))
	require.Len(t, alerts.Alerts, 1)
}

func TestSiteNodeConnectInvalidSessionID(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)

	ctx, cancel := context.WithCancel(s.ctx)
	t.Cleanup(cancel)

	result := make(chan error)

	_, err := connectToHost(ctx, connectConfig{
		pack:      s.authPack(t, "foo"),
		host:      s.node.ID(),
		proxy:     s.webServer.Listener.Addr().String(),
		sessionID: "/../../../foo",
		handlers: map[string]terminal.WSHandlerFunc{
			defaults.WebsocketError: func(ctx context.Context, e terminal.Envelope) {
				if e.Payload == "/../../../foo is not a valid UUID" {
					result <- errors.New(e.Payload)
				}
				close(result)
			},
		},
	})
	require.NoError(t, err)
	res := <-result
	require.Error(t, res)
}

func TestResolveServerHostPort(t *testing.T) {
	t.Parallel()
	sampleNode := types.ServerV2{}
	sampleNode.SetName("eca53e45-86a9-11e7-a893-0242ac0a0101")
	sampleNode.Spec.Hostname = "nodehostname"

	// valid cases
	validCases := []struct {
		server       string
		nodes        []types.Server
		expectedHost string
		expectedPort int
	}{
		{
			server:       "localhost",
			expectedHost: "localhost",
			expectedPort: 0,
		},
		{
			server:       "localhost:8080",
			expectedHost: "localhost",
			expectedPort: 8080,
		},
		{
			server:       "eca53e45-86a9-11e7-a893-0242ac0a0101",
			nodes:        []types.Server{&sampleNode},
			expectedHost: "nodehostname",
			expectedPort: 0,
		},
	}

	// invalid cases
	invalidCases := []struct {
		server      string
		expectedErr string
	}{
		{
			server:      ":22",
			expectedErr: "empty hostname",
		},
		{
			server:      ":",
			expectedErr: "empty hostname",
		},
		{
			server:      "",
			expectedErr: "empty server name",
		},
		{
			server:      "host:",
			expectedErr: "invalid port",
		},
		{
			server:      "host:port",
			expectedErr: "invalid port",
		},
	}

	for _, testCase := range validCases {
		host, port, err := resolveServerHostPort(testCase.server, testCase.nodes)
		require.NoError(t, err, testCase.server)
		require.Equal(t, testCase.expectedHost, host, testCase.server)
		require.Equal(t, testCase.expectedPort, port, testCase.server)
	}

	for _, testCase := range invalidCases {
		_, _, err := resolveServerHostPort(testCase.server, nil)
		require.Error(t, err, testCase.server)
		require.Regexp(t, ".*"+testCase.expectedErr+".*", err.Error(), testCase.server)
	}
}

func isFileTransferRequest(e *terminal.Envelope) bool {
	if e.GetType() != defaults.WebsocketAudit {
		return false
	}
	var ef events.EventFields
	if err := json.Unmarshal([]byte(e.GetPayload()), &ef); err != nil {
		return false
	}
	return ef.GetType() == string(srv.FileTransferUpdate)
}

func isFileTransferDecision(e *terminal.Envelope) bool {
	if e.GetType() != defaults.WebsocketAudit {
		return false
	}
	var ef events.EventFields
	if err := json.Unmarshal([]byte(e.GetPayload()), &ef); err != nil {
		return false
	}
	return ef.GetType() == string(srv.FileTransferApproved)
}

func getRequestId(e *terminal.Envelope) (string, error) {
	var ef events.EventFields
	if err := json.Unmarshal([]byte(e.GetPayload()), &ef); err != nil {
		return "", err
	}
	return ef.GetString("requestID"), nil
}

func TestFileTransferEvents(t *testing.T) {
	t.Parallel()
	s := newWebSuiteWithConfig(t, webSuiteConfig{disableDiskBasedRecording: true})

	ctx, cancel := context.WithCancel(s.ctx)
	t.Cleanup(cancel)

	// Create a new user "foo", open a terminal to a new session
	wsMessages := make(chan *terminal.Envelope)
	term, err := connectToHost(ctx, connectConfig{
		pack:  s.authPack(t, "foo"),
		host:  s.node.ID(),
		proxy: s.webServer.Listener.Addr().String(),
		handlers: map[string]terminal.WSHandlerFunc{
			defaults.WebsocketAudit: func(ctx context.Context, envelope terminal.Envelope) {
				wsMessages <- &envelope
			},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, term.Close()) })

	// Create file transfer event
	data, err := json.Marshal(events.EventFields{
		"download": true,
		"location": "~/myfile.txt",
	})

	require.NoError(t, err)
	envelope := &terminal.Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketFileTransferRequest,
		Payload: string(data),
	}
	envelopeBytes, err := proto.Marshal(envelope)
	require.NoError(t, err)
	err = term.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	require.NoError(t, err)

	done := time.After(5 * time.Second)
	for {
		select {
		case <-done:
			require.FailNow(t, "expected to receive a file transfer event")
		case e := <-wsMessages:
			if isFileTransferRequest(e) {
				requestId, err := getRequestId(e)
				require.NoError(t, err)
				data, err := json.Marshal(events.EventFields{
					"requestId": requestId,
					"approved":  true,
				})
				require.NoError(t, err)
				envelope := &terminal.Envelope{
					Version: defaults.WebsocketVersion,
					Type:    defaults.WebsocketFileTransferDecision,
					Payload: string(data),
				}
				envelopeBytes, err := proto.Marshal(envelope)
				require.NoError(t, err)
				err = term.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
				require.NoError(t, err)
			}

			if isFileTransferDecision(e) {
				return
			}
		}
	}
}

func TestNewTerminalHandler(t *testing.T) {
	ctx := context.Background()

	invalidCases := []struct {
		expectedErr string
		cfg         TerminalHandlerConfig
	}{
		{
			expectedErr: "sid: invalid session id",
			cfg: TerminalHandlerConfig{
				SessionData: session.Session{
					ID: session.ID("not a uuid"),
				},
			},
		},
		{
			expectedErr: "login: missing login",
			cfg: TerminalHandlerConfig{
				SessionData: session.Session{
					ID:    session.NewID(),
					Login: "",
				},
			},
		},
		{
			expectedErr: "server: missing server",
			cfg: TerminalHandlerConfig{
				SessionData: session.Session{
					ID:       session.NewID(),
					Login:    "root",
					ServerID: "",
				},
			},
		},
		{
			expectedErr: "term: bad dimensions(-1x0)",
			cfg: TerminalHandlerConfig{
				SessionData: session.Session{
					ID:       session.NewID(),
					Login:    "root",
					ServerID: uuid.New().String(),
				},
				Term: session.TerminalParams{
					W: -1,
					H: 0,
				},
			},
		},
		{
			expectedErr: "term: bad dimensions(1x4097)",
			cfg: TerminalHandlerConfig{
				SessionData: session.Session{
					ID:       session.NewID(),
					Login:    "root",
					ServerID: uuid.New().String(),
				},
				Term: session.TerminalParams{
					W: 1,
					H: 4097,
				},
			},
		},
	}

	for _, testCase := range invalidCases {
		_, err := NewTerminal(ctx, testCase.cfg)
		require.Equal(t, testCase.expectedErr, err.Error())
	}

	validNode := types.ServerV2{}
	validNode.SetName("eca53e45-86a9-11e7-a893-0242ac0a0101")
	validNode.Spec.Hostname = "nodehostname"

	// Valid Case
	validCfg := TerminalHandlerConfig{
		Term: session.TerminalParams{
			W: 100,
			H: 100,
		},
		SessionCtx: &SessionContext{},
		UserAuthClient: authProviderMock{
			server: validNode,
		},
		LocalAccessPoint: authProviderMock{},
		SessionData: session.Session{
			ID:       session.NewID(),
			Login:    "root",
			ServerID: uuid.New().String(),
		},
		KeepAliveInterval:  time.Duration(100),
		ProxyHostPort:      "1234",
		InteractiveCommand: make([]string, 1),
		DisplayLogin:       "tree",
		Router:             &proxy.Router{},
	}

	term, err := NewTerminal(ctx, validCfg)
	require.NoError(t, err)
	// passed through
	require.Equal(t, validCfg.SessionCtx, term.ctx)
	require.Equal(t, validCfg.UserAuthClient, term.userAuthClient)
	require.Equal(t, validCfg.SessionData, term.sessionData)
	require.Equal(t, validCfg.KeepAliveInterval, term.keepAliveInterval)
	require.Equal(t, validCfg.ProxyHostPort, term.proxyHostPort)
	require.Equal(t, validCfg.InteractiveCommand, term.interactiveCommand)
	require.Equal(t, validCfg.Term, term.term)
	require.Equal(t, validCfg.DisplayLogin, term.displayLogin)
	// newly added
	require.NotNil(t, term.logger)
}

func TestUIConfig(t *testing.T) {
	uiConfig := webclient.UIConfig{
		ScrollbackLines: 555,
		ShowResources:   constants.ShowResourcesaccessibleOnly,
	}
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	s := newWebSuiteWithConfig(t, webSuiteConfig{uiConfig: uiConfig})
	clt := s.client(t)
	endpoint := clt.Endpoint("web", "config.js")
	re, err := clt.Get(ctx, endpoint, nil)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(re.Bytes()), "var GRV_CONFIG"))
	t.Cleanup(cancel)

	// Response is type application/javascript, we need to strip off the variable name
	// and the semicolon at the end, then we are left with json like object.
	var cfg webclient.WebConfig
	str := strings.ReplaceAll(string(re.Bytes()), "var GRV_CONFIG = ", "")
	err = json.Unmarshal([]byte(str[:len(str)-1]), &cfg)
	require.NoError(t, err)
	require.Equal(t, uiConfig, cfg.UI)
}

func TestResizeTerminal(t *testing.T) {
	t.Parallel()
	s := newWebSuiteWithConfig(t, webSuiteConfig{disableDiskBasedRecording: true})
	sid := session.NewID()

	ctx, cancel := context.WithCancel(s.ctx)
	t.Cleanup(cancel)

	ws1Messages := make(chan *terminal.Envelope)
	ws1Raw := make(chan []byte)
	ws2Messages := make(chan *terminal.Envelope)
	// Create a new user "foo", open a terminal to a new session
	term, err := connectToHost(ctx, connectConfig{
		pack:  s.authPack(t, "foo"),
		host:  s.node.ID(),
		proxy: s.webServer.Listener.Addr().String(),
		handlers: map[string]terminal.WSHandlerFunc{
			defaults.WebsocketAudit: func(ctx context.Context, envelope terminal.Envelope) {
				ws1Messages <- &envelope
			},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, term.Close()) })

	sess := term.GetSession()
	// Wait for session to have started
	require.Eventually(t, func() bool {
		_, err := s.server.Auth().GetSessionTracker(context.Background(), string(sess.ID))
		return err == nil
	}, 3*time.Second, 200*time.Millisecond, "session not available")

	// Create a new user "bar" and join the session created above
	term2, err := connectToHost(ctx, connectConfig{
		pack:            s.authPack(t, "bar"),
		host:            s.node.ID(),
		proxy:           s.webServer.Listener.Addr().String(),
		sessionID:       sess.ID,
		participantMode: types.SessionPeerMode,
		handlers: map[string]terminal.WSHandlerFunc{
			defaults.WebsocketAudit: func(ctx context.Context, envelope terminal.Envelope) {
				ws2Messages <- &envelope
			},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, term2.Close()) })

	require.Equal(t, sess.ID, term2.GetSession().ID)

	go func() {
		read, err := io.ReadAll(io.LimitReader(term, 10))
		if err != nil {
			return
		}

		ws1Raw <- read
	}()

	// Consume events from the first terminal. We expect to see 2 resize events from the second user
	// joining the session (one for the default size, and one for the manual resize request). We also
	// validate at least one raw event with PTY data (indicating terminal ready) came through.
	done := time.After(10 * time.Second)
	t1ResizeEvents, t1RawEvents := 0, 0
t1ready:
	for {
		select {
		case <-done:
			require.FailNowf(t, "", "expected to receive 2 resize events (got %d)", t1ResizeEvents)
		case <-ws1Raw:
			t1RawEvents++
		case e := <-ws1Messages:
			if isResizeEventEnvelope(e) {
				t1ResizeEvents++
			}
		}

		if t1ResizeEvents == 2 && t1RawEvents > 0 {
			break t1ready
		}
	}

	// we should not expect to see a resize event on terminal 2,
	// since they are not broadcast back to the originator
	select {
	case e := <-ws2Messages:
		if isResizeEventEnvelope(e) {
			require.FailNow(t, "terminal 2 should not have received a resize event: %v", e)
		}
	case <-time.After(1 * time.Second):
	}

	// Resize the second terminal. This should only be reflected in the first terminal
	// because resize events are sent to participants but not the originator.
	params, err := session.NewTerminalParamsFromInt(300, 120)
	require.NoError(t, err)
	data, err := json.Marshal(events.EventFields{
		events.EventType:      events.ResizeEvent,
		events.EventNamespace: apidefaults.Namespace,
		events.SessionEventID: sid.String(),
		events.TerminalSize:   params.Serialize(),
	})
	require.NoError(t, err)
	envelope := &terminal.Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketResize,
		Payload: string(data),
	}
	envelopeBytes, err := proto.Marshal(envelope)
	require.NoError(t, err)
	err = term2.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	require.NoError(t, err)

	// the first terminal should see the resize event
	done = time.After(5 * time.Second)
	for {
		select {
		case <-done:
			require.FailNow(t, "expected to receive a final resize event")
		case e := <-ws1Messages:
			if isResizeEventEnvelope(e) {
				return
			}
		}
	}
}

func isResizeEventEnvelope(e *terminal.Envelope) bool {
	if e.GetType() != defaults.WebsocketAudit {
		return false
	}
	var ef events.EventFields
	if err := json.Unmarshal([]byte(e.GetPayload()), &ef); err != nil {
		return false
	}
	return ef.GetType() == events.ResizeEvent
}

// TestTerminalPing tests that the server sends continuous ping control messages.
func TestTerminalPing(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)

	ctx, cancel := context.WithCancel(s.ctx)
	t.Cleanup(cancel)

	closed := false
	done := make(chan struct{})

	term, err := connectToHost(ctx, connectConfig{
		pack:              s.authPack(t, "foo"),
		host:              s.node.ID(),
		proxy:             s.webServer.Listener.Addr().String(),
		keepAliveInterval: time.Second,
		pingHandler: func(ws terminal.WSConn, message string) error {
			if closed == false {
				close(done)
				closed = true
			}

			err := ws.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(time.Second))
			if errors.Is(err, websocket.ErrCloseSent) {
				return nil
			} else {
				var e net.Error
				if errors.As(err, &e) && e.Timeout() {
					return nil
				}
				return err
			}
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, term.Close()) })

	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("timeout waiting for ping")
	}
}

func TestTerminal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		recordingConfig types.SessionRecordingConfigV2
	}{
		{
			name: "node recording mode",
			recordingConfig: types.SessionRecordingConfigV2{
				Spec: types.SessionRecordingConfigSpecV2{
					Mode: types.RecordAtNodeSync,
				},
			},
		},
		{
			name: "proxy recording mode",
			recordingConfig: types.SessionRecordingConfigV2{
				Spec: types.SessionRecordingConfigSpecV2{
					Mode: types.RecordAtProxySync,
				},
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := newWebSuite(t)

			// Set the recording config
			_, err := s.server.Auth().UpsertSessionRecordingConfig(context.Background(), &tt.recordingConfig)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(s.ctx)
			t.Cleanup(cancel)
			// Create a new session
			term, err := connectToHost(ctx, connectConfig{
				pack:  s.authPack(t, "foo"),
				host:  s.node.ID(),
				proxy: s.webServer.Listener.Addr().String(),
			})

			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, term.Close()) })

			// Send a command and validate the output
			validateTerminal(t, term)

			// Validate that the session is active on the node
			require.Equal(t, int32(1), s.node.ActiveConnections())

			// Close the web socket to emulate a user closing the browser window
			require.NoError(t, term.Close())

			// Validate that the node terminates the session
			require.EventuallyWithT(t, func(t *assert.CollectT) {
				assert.Zero(t, s.node.ActiveConnections())
			}, 30*time.Second, 250*time.Millisecond)
		})
	}
}

func TestTerminalRouting(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)

	// add nodes with conflicting hostnames
	llama := s.addNode(t, uuid.NewString(), "llama", "127.0.0.1:0")
	s.addNode(t, uuid.NewString(), "llamas", "127.0.0.1:0")
	alpaca1 := s.addNode(t, uuid.NewString(), "alpaca", "127.0.0.1:0")
	s.addNode(t, uuid.NewString(), "alpaca", "127.0.0.1:0")

	closeNoError := func(t *testing.T, err error) {
		require.NoError(t, err)
	}

	cases := []struct {
		name             string
		target           *regular.Server
		output           string
		wsCloseAssertion func(t *testing.T, err error)
	}{
		{
			name:             "exact match by uuid",
			target:           llama,
			output:           "teleport",
			wsCloseAssertion: closeNoError,
		},
		{
			name:             "connect by uuid successful when multiple hostnames match",
			target:           alpaca1,
			output:           "teleport",
			wsCloseAssertion: closeNoError,
		},
	}

	for i, tt := range cases {
		i, tt := i, tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(s.ctx)
			t.Cleanup(cancel)

			term, err := connectToHost(ctx, connectConfig{
				pack:  s.authPack(t, fmt.Sprintf("foo-%d", i)),
				host:  tt.target.ID(),
				proxy: s.webServer.Listener.Addr().String(),
			})
			require.NoError(t, err)
			t.Cleanup(func() { tt.wsCloseAssertion(t, term.Close()) })

			sess := term.GetSession()

			metadata := tt.target.TargetMetadata()
			require.Equal(t, metadata.ServerID, sess.ServerID)
			require.Equal(t, metadata.ServerHostname, sess.ServerHostname)

			// here we intentionally run a command where the output we're looking
			// for is not present in the command itself
			_, err = io.WriteString(term, "echo txlxport | sed 's/x/e/g'\r\n")
			require.NoError(t, err)
			require.NoError(t, waitForOutput(term, tt.output))
		})
	}
}

func TestTerminalRequireSessionMFA(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	const username = "llama2999"
	pack := proxy.authPack(t, username, nil /* roles */)

	userClient, err := env.server.NewClient(auth.TestUser(username))
	require.NoError(t, err)

	cases := []struct {
		name                      string
		getAuthPreference         func(t *testing.T) types.AuthPreference
		registerDevice            func(t *testing.T) *auth.TestDevice
		getChallengeResponseBytes func(t *testing.T, chal client.MFAAuthenticateChallenge, testDev *auth.TestDevice) []byte
	}{
		{
			name: "with webauthn",
			getAuthPreference: func(t *testing.T) types.AuthPreference {
				ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorWebauthn,
					Webauthn: &types.Webauthn{
						RPID: "localhost",
					},
					RequireMFAType: types.RequireMFAType_SESSION,
				})
				require.NoError(t, err)

				return ap
			},
			registerDevice: func(t *testing.T) *auth.TestDevice {
				webauthnDev, err := auth.RegisterTestDevice(
					ctx,
					userClient,
					"webauthn", authproto.DeviceType_DEVICE_TYPE_WEBAUTHN, pack.device /* authenticator */)
				require.NoError(t, err)

				return webauthnDev
			},
			getChallengeResponseBytes: func(t *testing.T, chal client.MFAAuthenticateChallenge, testDev *auth.TestDevice) []byte {
				res, err := testDev.SolveAuthn(&authproto.MFAAuthenticateChallenge{
					WebauthnChallenge: wantypes.CredentialAssertionToProto(chal.WebauthnChallenge),
				})
				require.NoError(t, err)

				webauthnResBytes, err := json.Marshal(wantypes.CredentialAssertionResponseFromProto(res.GetWebauthn()))
				require.NoError(t, err)

				envelope := &terminal.Envelope{
					Version: defaults.WebsocketVersion,
					Type:    defaults.WebsocketMFAChallenge,
					Payload: string(webauthnResBytes),
				}
				protoBytes, err := proto.Marshal(envelope)
				require.NoError(t, err)

				return protoBytes
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err = env.server.Auth().UpsertAuthPreference(ctx, tc.getAuthPreference(t))
			require.NoError(t, err)

			dev := tc.registerDevice(t)

			termCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			// Open a terminal to a new session.
			term, err := connectToHost(termCtx, connectConfig{
				pack:  pack,
				host:  proxy.node.ID(),
				proxy: proxy.webURL.Host,
				mfaCeremony: func(challenge client.MFAAuthenticateChallenge) []byte {
					return tc.getChallengeResponseBytes(t, challenge, dev)
				},
			})
			require.NoError(t, err)

			// Test we can write.
			_, err = io.WriteString(term, "echo txlxport | sed 's/x/e/g'\r\n")
			require.NoError(t, err)
			require.NoError(t, waitForOutput(term, "teleport"))
		})
	}
}

type windowsDesktopServiceMock struct {
	listener net.Listener
}

func mustStartWindowsDesktopMock(t *testing.T, authClient *auth.Server) *windowsDesktopServiceMock {
	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, l.Close())
	})
	authID := state.IdentityID{
		Role:     types.RoleWindowsDesktop,
		HostUUID: "windows_server",
		NodeName: "windows_server",
	}
	n, err := authClient.GetClusterName()
	require.NoError(t, err)
	dns := []string{"localhost", "127.0.0.1", desktop.WildcardServiceDNS}
	identity, err := auth.LocalRegister(authID, authClient, nil, dns, "", nil)
	require.NoError(t, err)

	tlsConfig, err := identity.TLSConfig(nil)
	require.NoError(t, err)
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	require.NoError(t, err)

	ca, err := authClient.GetCertAuthority(context.Background(), types.CertAuthID{Type: types.UserCA, DomainName: n.GetClusterName()}, false)
	require.NoError(t, err)

	for _, kp := range services.GetTLSCerts(ca) {
		require.True(t, tlsConfig.ClientCAs.AppendCertsFromPEM(kp))
	}

	wd := &windowsDesktopServiceMock{
		listener: l,
	}
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		tlsConn := tls.Server(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(context.Background()); err != nil {
			t.Errorf("Unexpected error %v", err)
			return
		}
		wd.handleConn(t, tlsConn)
	}()

	return wd
}

func (w *windowsDesktopServiceMock) handleConn(t *testing.T, conn *tls.Conn) {
	tdpConn := tdp.NewConn(conn)

	// Ensure that incoming connection is MFAVerified.
	require.NotEmpty(t, conn.ConnectionState().PeerCertificates)
	cert := conn.ConnectionState().PeerCertificates[0]
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	require.NoError(t, err)
	require.NotEmpty(t, identity.MFAVerified)

	msg, err := tdpConn.ReadMessage()
	require.NoError(t, err)
	require.IsType(t, tdp.ClientUsername{}, msg)

	msg, err = tdpConn.ReadMessage()
	require.NoError(t, err)
	require.IsType(t, tdp.ClientScreenSpec{}, msg)

	err = tdpConn.WriteMessage(tdp.Alert{Message: "test", Severity: tdp.SeverityWarning})
	require.NoError(t, err)
}

func TestDesktopAccessMFARequiresMfa(t *testing.T) {
	tests := []struct {
		name           string
		authPref       types.AuthPreferenceSpecV2
		mfaHandler     func(t *testing.T, ws *websocket.Conn, dev *auth.TestDevice)
		registerDevice func(t *testing.T, ctx context.Context, clt *authclient.Client) *auth.TestDevice
	}{
		{
			name: "webauthn",
			authPref: types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorWebauthn,
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
				RequireMFAType: types.RequireMFAType_SESSION,
			},
			mfaHandler: handleDesktopMFAWebauthnChallenge,
			registerDevice: func(t *testing.T, ctx context.Context, clt *authclient.Client) *auth.TestDevice {
				webauthnDev, err := auth.RegisterTestDevice(ctx, clt, "webauthn", authproto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil /* authenticator */)
				require.NoError(t, err)
				return webauthnDev
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			env := newWebPack(t, 1)
			proxy := env.proxies[0]
			pack := proxy.authPack(t, "llama", nil /* roles */)

			clt, err := env.server.NewClient(auth.TestUser("llama"))
			require.NoError(t, err)
			wdID := uuid.New().String()

			wdMock := mustStartWindowsDesktopMock(t, env.server.Auth())
			wd, err := types.NewWindowsDesktopV3("desktop1", nil, types.WindowsDesktopSpecV3{
				Addr:   wdMock.listener.Addr().String(),
				Domain: "CORP",
				HostID: wdID,
			})
			require.NoError(t, err)

			err = env.server.Auth().UpsertWindowsDesktop(context.Background(), wd)
			require.NoError(t, err)
			wds, err := types.NewWindowsDesktopServiceV3(types.Metadata{Name: wdID}, types.WindowsDesktopServiceSpecV3{
				Addr:            wdMock.listener.Addr().String(),
				TeleportVersion: teleport.Version,
			})
			require.NoError(t, err)

			_, err = env.server.Auth().UpsertWindowsDesktopService(context.Background(), wds)
			require.NoError(t, err)

			ap, err := types.NewAuthPreference(tc.authPref)
			require.NoError(t, err)
			_, err = env.server.Auth().UpsertAuthPreference(ctx, ap)
			require.NoError(t, err)

			dev := tc.registerDevice(t, ctx, clt)

			ws := proxy.makeDesktopSession(t, pack)
			tc.mfaHandler(t, ws, dev)

			tdpClient := tdp.NewConn(&WebsocketIO{Conn: ws})

			msg, err := tdpClient.ReadMessage()
			require.NoError(t, err)
			require.IsType(t, tdp.Alert{}, msg)
		})
	}
}

func handleDesktopMFAWebauthnChallenge(t *testing.T, ws *websocket.Conn, dev *auth.TestDevice) {
	wsrwc := &WebsocketIO{Conn: ws}
	tdpConn := tdp.NewConn(wsrwc)

	// desktopConnectHandle first needs a ClientScreenSpec message in order to continue.
	tdpConn.WriteMessage(tdp.ClientScreenSpec{Width: 100, Height: 100})

	br := bufio.NewReader(wsrwc)
	mt, err := br.ReadByte()
	require.NoError(t, err)
	require.Equal(t, tdp.TypeMFA, tdp.MessageType(mt))

	mfaChallange, err := tdp.DecodeMFAChallenge(br)
	require.NoError(t, err)
	res, err := dev.SolveAuthn(&authproto.MFAAuthenticateChallenge{
		WebauthnChallenge: wantypes.CredentialAssertionToProto(mfaChallange.WebauthnChallenge),
	})
	require.NoError(t, err)
	err = tdpConn.WriteMessage(tdp.MFA{
		Type: defaults.WebsocketMFAChallenge[0],
		MFAAuthenticateResponse: &authproto.MFAAuthenticateResponse{
			Response: &authproto.MFAAuthenticateResponse_Webauthn{
				Webauthn: res.GetWebauthn(),
			},
		},
	})
	require.NoError(t, err)
}

func TestWebAgentForward(t *testing.T) {
	t.Parallel()
	s := newWebSuiteWithConfig(t, webSuiteConfig{disableDiskBasedRecording: true})

	ctx, cancel := context.WithCancel(s.ctx)
	t.Cleanup(cancel)

	term, err := connectToHost(ctx, connectConfig{
		pack:  s.authPack(t, "foo"),
		host:  s.node.ID(),
		proxy: s.webServer.Listener.Addr().String(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, term.Close()) })

	_, err = io.WriteString(term, "echo $SSH_AUTH_SOCK\r\n")
	require.NoError(t, err)

	err = waitForOutput(term, "/")
	require.NoError(t, err)
}

func TestActiveSessions(t *testing.T) {
	// Use enterprise license (required for moderated sessions).
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

	s := newWebSuite(t)
	pack := s.authPack(t, "foo")

	start := time.Now()
	kinds := []types.SessionKind{
		types.SSHSessionKind,
		types.KubernetesSessionKind,
		types.WindowsDesktopSessionKind,
		types.DatabaseSessionKind,
		types.AppSessionKind,
	}
	ids := make(map[string]struct{})

	for _, kind := range kinds {
		tracker, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
			SessionID:    string(session.NewID()),
			ClusterName:  s.server.ClusterName(),
			Kind:         string(kind),
			State:        types.SessionState_SessionStateRunning,
			Created:      start,
			Expires:      start.Add(1 * time.Hour),
			Hostname:     s.node.GetInfo().GetHostname(),
			DesktopName:  s.node.GetInfo().GetHostname(),
			AppName:      s.node.GetInfo().GetHostname(),
			DatabaseName: s.node.GetInfo().GetHostname(),
			Address:      s.srvID,
			Login:        pack.login,
			Participants: []types.Participant{
				{ID: "id", User: "user-1", LastActive: start},
			},
			HostPolicies: []*types.SessionTrackerPolicySet{
				{
					Name:    "foo",
					Version: "5",
					RequireSessionJoin: []*types.SessionRequirePolicy{
						{
							Name: "foo",
						},
					},
				},
			},
		})
		require.NoError(t, err)
		ids[tracker.GetSessionID()] = struct{}{}

		_, err = s.server.Auth().CreateSessionTracker(context.Background(), tracker)
		require.NoError(t, err)
	}

	// create an inactive session, which should not show up
	inactive, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
		SessionID:    string(session.NewID()),
		ClusterName:  s.server.ClusterName(),
		Kind:         string(types.SSHSessionKind),
		State:        types.SessionState_SessionStateTerminated,
		Created:      time.Now(),
		Expires:      time.Now().Add(1 * time.Hour),
		Hostname:     s.node.GetInfo().GetHostname(),
		Address:      s.srvID,
		Login:        pack.login,
		Participants: nil,
	})
	require.NoError(t, err)
	_, err = s.server.Auth().CreateSessionTracker(context.Background(), inactive)
	require.NoError(t, err)

	re, err := pack.clt.Get(s.ctx, pack.clt.Endpoint("webapi", "sites", s.server.ClusterName(), "sessions"), url.Values{})
	require.NoError(t, err)

	var sessResp siteSessionsGetResponse
	require.NoError(t, json.Unmarshal(re.Bytes(), &sessResp))
	require.Len(t, sessResp.Sessions, len(kinds))

	for _, session := range sessResp.Sessions {
		require.Contains(t, ids, string(session.ID))
		require.Equal(t, s.node.GetNamespace(), session.Namespace)
		require.NotNil(t, session.Parties)
		require.Greater(t, session.TerminalParams.H, 0)
		require.Greater(t, session.TerminalParams.W, 0)
		require.Equal(t, pack.login, session.Login)
		require.False(t, session.Created.IsZero())
		require.False(t, session.LastActive.IsZero())
		require.Equal(t, s.srvID, session.ServerID)
		require.Equal(t, s.node.GetInfo().GetHostname(), session.ServerHostname)
		require.Equal(t, s.srvID, session.ServerAddr)
		require.Equal(t, s.server.ClusterName(), session.ClusterName)
		require.ElementsMatch(t, []types.SessionParticipantMode{"peer"}, session.ParticipantModes)
	}
}

func TestCloseConnectionsOnLogout(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	pack := s.authPack(t, "foo")

	ctx, cancel := context.WithCancel(s.ctx)
	t.Cleanup(cancel)

	term, err := connectToHost(ctx, connectConfig{
		pack:  pack,
		host:  s.node.ID(),
		proxy: s.webServer.Listener.Addr().String(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, term.Close()) })

	// to make sure we have a session
	_, err = io.WriteString(term, "expr 137 + 39\r\n")
	require.NoError(t, err)

	// make sure the server has replied
	out := make([]byte, 100)
	_, err = term.Read(out)
	require.NoError(t, err)

	_, err = pack.clt.Delete(s.ctx, pack.clt.Endpoint("webapi", "sessions", "web"))
	require.NoError(t, err)

	// wait until timeout or detect that the connection has been closed.
	after := time.After(5 * time.Second)
	errC := make(chan error)
	go func() {
		for {
			_, err := term.Read(out)
			if err != nil {
				errC <- err
				return
			}
		}
	}()

	select {
	case <-after:
		t.Fatalf("timeout")
	case err := <-errC:
		require.ErrorIs(t, err, io.EOF)
	}
}

type httpErrorMessage struct {
	Message string `json:"message"`
}

type httpErrorResponse struct {
	Error httpErrorMessage `json:"error"`
}

func TestLogin_PrivateKeyEnabledError(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		MockAttestationData: &keys.AttestationData{
			PrivateKeyPolicy: keys.PrivateKeyPolicyNone,
		},
	})

	s := newWebSuite(t)
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:           constants.Local,
		SecondFactor:   constants.SecondFactorOff,
		RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
	})
	require.NoError(t, err)
	_, err = s.server.Auth().UpsertAuthPreference(s.ctx, ap)
	require.NoError(t, err)

	// create user
	const user = "user1"
	const pass = "password1234"
	s.createUser(t, user, "root", pass, "")

	clt := s.client(t)
	ctx := context.Background()

	const ua = "test-ua"
	_, body, err := rawLoginWebOTP(ctx, loginWebOTPParams{
		webClient: clt,
		user:      user,
		password:  pass,
		userAgent: ua,
	})
	require.NoError(t, err)

	var resErr httpErrorResponse
	require.NoError(t, json.Unmarshal(body, &resErr))
	require.Contains(t, resErr.Error.Message, keys.PrivateKeyPolicyHardwareKeyTouch)
}

func TestLogin(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOff,
	})
	require.NoError(t, err)
	_, err = s.server.Auth().UpsertAuthPreference(s.ctx, ap)
	require.NoError(t, err)

	// create user
	const user = "user1"
	const pass = "password1234"
	s.createUser(t, user, "root", pass, "")

	clt := s.client(t)
	ctx := context.Background()

	const ua = "test-ua"
	sessionResp, httpResp := loginWebOTP(t, ctx, loginWebOTPParams{
		webClient: clt,
		user:      user,
		password:  pass,
		userAgent: ua,
	})

	events, _, err := s.server.AuthServer.AuditLog.SearchEvents(ctx, events.SearchEventsRequest{
		From:       s.clock.Now().Add(-time.Hour),
		To:         s.clock.Now().Add(time.Hour),
		EventTypes: []string{events.UserLoginEvent},
		Limit:      1,
		Order:      types.EventOrderDescending,
	})
	require.NoError(t, err)
	event := events[0].(*apievents.UserLogin)
	require.True(t, event.Success)
	require.Equal(t, ua, event.UserAgent)
	require.True(t, strings.HasPrefix(event.RemoteAddr, "127.0.0.1:"))

	cookies := httpResp.Cookies()
	require.Len(t, cookies, 1)
	require.NotEmpty(t, sessionResp.SessionExpires)

	// now make sure we are logged in by calling authenticated method
	// we need to supply both session cookie and bearer token for
	// request to succeed
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	clt = s.client(t, roundtrip.BearerAuth(sessionResp.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(s.url(), cookies)

	re, err := clt.Get(s.ctx, clt.Endpoint("webapi", "sites"), url.Values{})
	require.NoError(t, err)

	var clusters []webui.Cluster
	require.NoError(t, json.Unmarshal(re.Bytes(), &clusters))

	// in absence of session cookie or bearer auth the same request fill fail

	// no session cookie:
	clt = s.client(t, roundtrip.BearerAuth(sessionResp.Token))
	_, err = clt.Get(s.ctx, clt.Endpoint("webapi", "sites"), url.Values{})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	// no bearer token:
	clt = s.client(t, roundtrip.CookieJar(jar))
	_, err = clt.Get(s.ctx, clt.Endpoint("webapi", "sites"), url.Values{})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

// TestEmptyMotD ensures that responses returned by both /webapi/ping and
// /webapi/motd work when no MotD is set
func TestEmptyMotD(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	wc := s.client(t)

	// Given an auth server configured *not* to expose a Message Of The
	// Day...

	// When I issue a ping request...
	re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "ping"), url.Values{})
	require.NoError(t, err)

	// Expect that the MotD flag in the ping response is *not* set
	var pingResponse *webclient.PingResponse
	require.NoError(t, json.Unmarshal(re.Bytes(), &pingResponse))
	require.False(t, pingResponse.Auth.HasMessageOfTheDay)

	// When I fetch the MotD...
	re, err = wc.Get(s.ctx, wc.Endpoint("webapi", "motd"), url.Values{})
	require.NoError(t, err)

	// Expect that an empty response returned
	var motdResponse *webclient.MotD
	require.NoError(t, json.Unmarshal(re.Bytes(), &motdResponse))
	require.Empty(t, motdResponse.Text)
}

// TestMotD ensures that a response is returned by both /webapi/ping and /webapi/motd
// and that that the response bodies contain their MOTD components
func TestMotD(t *testing.T) {
	t.Parallel()
	const motd = "Hello. I'm a Teleport cluster!"

	s := newWebSuite(t)
	wc := s.client(t)

	// Given an auth server configured to expose a Message Of The Day...
	prefs := types.DefaultAuthPreference()
	prefs.SetMessageOfTheDay(motd)
	_, err := s.server.AuthServer.AuthServer.UpsertAuthPreference(s.ctx, prefs)
	require.NoError(t, err)

	// When I issue a ping request...
	re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "ping"), url.Values{})
	require.NoError(t, err)

	// Expect that the MotD flag in the ping response is set to indicate
	// a MotD
	var pingResponse *webclient.PingResponse
	require.NoError(t, json.Unmarshal(re.Bytes(), &pingResponse))
	require.True(t, pingResponse.Auth.HasMessageOfTheDay)

	// When I fetch the MotD...
	re, err = wc.Get(s.ctx, wc.Endpoint("webapi", "motd"), url.Values{})
	require.NoError(t, err)

	// Expect that the text returned is the configured value
	var motdResponse *webclient.MotD
	require.NoError(t, json.Unmarshal(re.Bytes(), &motdResponse))
	require.Equal(t, motd, motdResponse.Text)
}

// TestPingAutomaticUpgrades ensures /webapi/ping returns whether AutomaticUpgrades are enabled.
func TestPingAutomaticUpgrades(t *testing.T) {
	t.Run("Automatic Upgrades are enabled", func(t *testing.T) {
		// Enable Automatic Upgrades
		modules.SetTestModules(t, &modules.TestModules{TestFeatures: modules.Features{
			AutomaticUpgrades: true,
		}})

		// Set up
		s := newWebSuite(t)
		wc := s.client(t)
		var pingResponse *webclient.PingResponse

		// Get Ping response
		re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "ping"), url.Values{})
		require.NoError(t, err)

		require.NoError(t, json.Unmarshal(re.Bytes(), &pingResponse))
		require.True(t, pingResponse.AutomaticUpgrades, "expected automatic upgrades to be enabled")
	})
	t.Run("Automatic Upgrades are disabled", func(t *testing.T) {
		// Disable Automatic Upgrades
		modules.SetTestModules(t, &modules.TestModules{TestFeatures: modules.Features{
			AutomaticUpgrades: false,
		}})

		// Set up
		s := newWebSuite(t)
		wc := s.client(t)
		var pingResponse *webclient.PingResponse

		// Get Ping response
		re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "ping"), url.Values{})
		require.NoError(t, err)

		require.NoError(t, json.Unmarshal(re.Bytes(), &pingResponse))
		require.False(t, pingResponse.AutomaticUpgrades, "expected automatic upgrades to be disabled")
	})
}

// TestInstallerRepoChannel ensures the returned installer script has the proper repo channel
func TestInstallerRepoChannel(t *testing.T) {
	t.Run("cloud with automatic upgrades", func(t *testing.T) {
		modules.SetTestModules(t, &modules.TestModules{
			TestFeatures: modules.Features{
				Cloud:             true,
				AutomaticUpgrades: true,
			},
		})

		s := newWebSuiteWithConfig(t, webSuiteConfig{
			authPreferenceSpec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOn,
				Webauthn:     &types.Webauthn{RPID: "localhost"},
			},
		})

		wc := s.client(t)
		t.Run("documented variables are injected", func(t *testing.T) {
			// Variables documented here:
			// https://goteleport.com/docs/enroll-resources/auto-discovery/servers/ec2-discovery/#step-67-optional-customize-the-default-installer-script
			err := s.server.Auth().SetInstaller(s.ctx, types.MustNewInstallerV1("custom", `#!/usr/bin/env bash
echo {{ .PublicProxyAddr }}
echo Teleport-{{ .MajorVersion }}
echo Repository Channel: {{ .RepoChannel }}
echo AutomaticUpgrades: {{ .AutomaticUpgrades }}
		`))
			require.NoError(t, err)

			re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "scripts", "installer", "custom"), url.Values{})
			require.NoError(t, err)

			responseString := string(re.Bytes())

			// Variables must be injected
			require.Contains(t, responseString, "echo Teleport-v")
			require.NotContains(t, responseString, "echo Repository Channel: stable/v")
			require.Contains(t, responseString, "echo Repository Channel: stable/cloud")
			require.Contains(t, responseString, "echo AutomaticUpgrades: true")
		})

		t.Run("default-installer", func(t *testing.T) {
			re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "scripts", "installer", "default-installer"), url.Values{})
			require.NoError(t, err)

			responseString := string(re.Bytes())

			// The repo's channel to use is stable/cloud
			require.Contains(t, responseString, "stable/cloud")
			require.NotContains(t, responseString, "stable/v")
			require.Contains(t, responseString, "--auto-upgrade=true")
			require.Contains(t, responseString, "--teleport-package=teleport-ent")
		})

		t.Run("default-agentless-installer", func(t *testing.T) {
			re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "scripts", "installer", "default-agentless-installer"), url.Values{})
			require.NoError(t, err)

			responseString := string(re.Bytes())

			// The repo's channel to use is stable/cloud
			require.Contains(t, responseString, "stable/cloud")
			require.NotContains(t, responseString, "stable/v")
			require.Contains(t, responseString, ""+
				"    # shellcheck disable=SC2050\n"+
				"    if [ \"true\" = \"true\" ]; then\n"+
				"      # automatic upgrades\n",
			)
			require.Contains(t, responseString, ""+
				"  TELEPORT_PACKAGE=\"teleport-ent\"\n"+
				"  TELEPORT_UPDATER_PACKAGE=\"teleport-ent-updater\"\n",
			)
		})
	})

	t.Run("cloud without automatic upgrades", func(t *testing.T) {
		modules.SetTestModules(t, &modules.TestModules{
			TestFeatures: modules.Features{
				Cloud:             true,
				AutomaticUpgrades: false,
			},
		})

		s := newWebSuiteWithConfig(t, webSuiteConfig{
			authPreferenceSpec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOn,
				Webauthn:     &types.Webauthn{RPID: "localhost"},
			},
		})

		wc := s.client(t)

		t.Run("documented variables are injected", func(t *testing.T) {
			// Variables documented here: https://goteleport.com/docs/enroll-resources/auto-discovery/servers/ec2-discovery/#step-67-optional-customize-the-default-installer-script
			err := s.server.Auth().SetInstaller(s.ctx, types.MustNewInstallerV1("custom", `#!/usr/bin/env bash
	echo {{ .PublicProxyAddr }}
	echo Teleport-{{ .MajorVersion }}
	echo Repository Channel: {{ .RepoChannel }}
	echo AutomaticUpgrades: {{ .AutomaticUpgrades }}
			`))
			require.NoError(t, err)

			re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "scripts", "installer", "custom"), url.Values{})
			require.NoError(t, err)

			responseString := string(re.Bytes())

			// Variables must be injected
			require.Contains(t, responseString, "echo Teleport-v")
			require.Contains(t, responseString, "echo Repository Channel: stable/v")
			require.NotContains(t, responseString, "echo Repository Channel: stable/cloud")
			require.Contains(t, responseString, "echo AutomaticUpgrades: false")
		})
		t.Run("default-installer", func(t *testing.T) {
			re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "scripts", "installer", "default-installer"), url.Values{})
			require.NoError(t, err)

			responseString := string(re.Bytes())

			require.NotContains(t, responseString, "stable/cloud")
		})
		t.Run("default-agentless-installer", func(t *testing.T) {
			re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "scripts", "installer", "default-agentless-installer"), url.Values{})
			require.NoError(t, err)

			responseString := string(re.Bytes())

			require.NotContains(t, responseString, "stable/cloud")
		})
	})

	t.Run("oss or enterprise with automatic upgrades", func(t *testing.T) {
		modules.SetTestModules(t, &modules.TestModules{
			TestBuildType: modules.BuildOSS,
			TestFeatures: modules.Features{
				Cloud:             false,
				AutomaticUpgrades: true,
			},
		})

		s := newWebSuiteWithConfig(t, webSuiteConfig{
			authPreferenceSpec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOn,
				Webauthn:     &types.Webauthn{RPID: "localhost"},
			},
		})

		wc := s.client(t)
		t.Run("documented variables are injected", func(t *testing.T) {
			// Variables documented here: https://goteleport.com/docs/enroll-resources/auto-discovery/servers/ec2-discovery/#step-67-optional-customize-the-default-installer-script
			err := s.server.Auth().SetInstaller(s.ctx, types.MustNewInstallerV1("custom", `#!/usr/bin/env bash
echo {{ .PublicProxyAddr }}
echo Teleport-{{ .MajorVersion }}
echo Repository Channel: {{ .RepoChannel }}
echo AutomaticUpgrades: {{ .AutomaticUpgrades }}
		`))
			require.NoError(t, err)

			re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "scripts", "installer", "custom"), url.Values{})
			require.NoError(t, err)

			responseString := string(re.Bytes())

			// Variables must be injected
			require.Contains(t, responseString, "echo Teleport-v")
			require.Contains(t, responseString, "echo Repository Channel: stable/v")
			require.NotContains(t, responseString, "echo Repository Channel: stable/cloud")
			require.Contains(t, responseString, "echo AutomaticUpgrades: false")
		})
		t.Run("default-installer", func(t *testing.T) {
			re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "scripts", "installer", "default-installer"), url.Values{})
			require.NoError(t, err)

			responseString := string(re.Bytes())

			// The repo's channel to use is stable/cloud
			require.NotContains(t, responseString, "stable/cloud")
			require.Contains(t, responseString, "stable/v")
			require.Contains(t, responseString, "--auto-upgrade=false")
			require.Contains(t, responseString, "--teleport-package=teleport ")
		})
		t.Run("default-agentless-installer", func(t *testing.T) {
			re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "scripts", "installer", "default-agentless-installer"), url.Values{})
			require.NoError(t, err)

			responseString := string(re.Bytes())

			// The repo's channel to use is stable/cloud
			require.NotContains(t, responseString, "stable/cloud")
			require.Contains(t, responseString, "stable/v")
			require.Contains(t, responseString, ""+
				"    # shellcheck disable=SC2050\n"+
				"    if [ \"false\" = \"true\" ]; then\n"+
				"      # automatic upgrades\n",
			)
			require.Contains(t, responseString, ""+
				"  TELEPORT_PACKAGE=\"teleport\"\n"+
				"  TELEPORT_UPDATER_PACKAGE=\"teleport-updater\"\n",
			)
		})
	})
}

func TestMultipleConnectors(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	wc := s.client(t)

	// create two oidc connectors, one named "foo" and another named "bar"
	oidcConnectorSpec := types.OIDCConnectorSpecV3{
		RedirectURLs: []string{"https://localhost:3080/v1/webapi/oidc/callback"},
		ClientID:     "000000000000-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com",
		ClientSecret: "AAAAAAAAAAAAAAAAAAAAAAAA",
		IssuerURL:    "https://oidc.example.com",
		Display:      "Login with Example",
		Scope:        []string{"group"},
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "group",
				Value: "admin",
				Roles: []string{"admin"},
			},
		},
	}
	o, err := types.NewOIDCConnector("foo", oidcConnectorSpec)
	require.NoError(t, err)
	upserted, err := s.server.Auth().UpsertOIDCConnector(s.ctx, o)
	require.NoError(t, err)
	require.NotNil(t, upserted)
	o2, err := types.NewOIDCConnector("bar", oidcConnectorSpec)
	require.NoError(t, err)
	upserted, err = s.server.Auth().UpsertOIDCConnector(s.ctx, o2)
	require.NoError(t, err)
	require.NotNil(t, upserted)

	// set the auth preferences to oidc with no connector name
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type: "oidc",
	})
	require.NoError(t, err)
	_, err = s.server.Auth().UpsertAuthPreference(s.ctx, authPreference)
	require.NoError(t, err)

	// hit the ping endpoint to get the auth type and connector name
	re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "ping"), url.Values{})
	require.NoError(t, err)
	var out *webclient.PingResponse
	require.NoError(t, json.Unmarshal(re.Bytes(), &out))

	// make sure the connector name we got back was the first connector
	// in the backend, in this case it's "bar"
	oidcConnectors, err := s.server.Auth().GetOIDCConnectors(s.ctx, false)
	require.NoError(t, err)
	require.Equal(t, oidcConnectors[0].GetName(), out.Auth.OIDC.Name)

	// update the auth preferences and this time specify the connector name
	authPreference, err = types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:          "oidc",
		ConnectorName: "foo",
	})
	require.NoError(t, err)
	_, err = s.server.Auth().UpsertAuthPreference(s.ctx, authPreference)
	require.NoError(t, err)

	// hit the ping endpoing to get the auth type and connector name
	re, err = wc.Get(s.ctx, wc.Endpoint("webapi", "ping"), url.Values{})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(re.Bytes(), &out))

	// make sure the connector we get back is "foo"
	require.Equal(t, "foo", out.Auth.OIDC.Name)
}

func TestPingSSHDialTimeout(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	wc := s.client(t)

	// hit the ping endpoint to get the ssh dial timeout
	re, err := wc.Get(s.ctx, wc.Endpoint("webapi", "ping"), url.Values{})
	require.NoError(t, err)
	var out webclient.PingResponse
	require.NoError(t, json.Unmarshal(re.Bytes(), &out))

	// Validate the timeout is the default value.
	require.Equal(t, apidefaults.DefaultIOTimeout, out.Proxy.SSH.DialTimeout)

	// Update the timeout
	cnc, err := s.server.Auth().GetClusterNetworkingConfig(s.ctx)
	require.NoError(t, err)

	cnc.SetSSHDialTimeout(time.Minute)
	cnc, err = s.server.Auth().UpsertClusterNetworkingConfig(s.ctx, cnc)
	require.NoError(t, err)

	// hit the ping endpoint again to validate that updated values are returned
	re, err = wc.Get(s.ctx, wc.Endpoint("webapi", "ping"), url.Values{})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(re.Bytes(), &out))

	// Validate the timeout is the default value.
	require.Equal(t, cnc.GetSSHDialTimeout(), out.Proxy.SSH.DialTimeout)
}

// TestConstructSSHResponse checks if the secret package uses AES-GCM to
// encrypt and decrypt data that passes through the ConstructSSHResponse
// function.
func TestConstructSSHResponse(t *testing.T) {
	key, err := secret.NewKey()
	require.NoError(t, err)

	u, err := url.Parse("http://www.example.com/callback")
	require.NoError(t, err)
	query := u.Query()
	query.Set("secret_key", key.String())
	u.RawQuery = query.Encode()

	rawresp, err := ConstructSSHResponse(AuthParams{
		Username:          "foo",
		Cert:              []byte{0x00},
		TLSCert:           []byte{0x01},
		ClientRedirectURL: u.String(),
	})
	require.NoError(t, err)

	require.Empty(t, rawresp.Query().Get("secret"))
	require.Empty(t, rawresp.Query().Get("secret_key"))
	require.NotEmpty(t, rawresp.Query().Get("response"))

	plaintext, err := key.Open([]byte(rawresp.Query().Get("response")))
	require.NoError(t, err)

	var resp *authclient.SSHLoginResponse
	err = json.Unmarshal(plaintext, &resp)
	require.NoError(t, err)
	require.Equal(t, "foo", resp.Username)
	require.EqualValues(t, []byte{0x00}, resp.Cert)
	require.EqualValues(t, []byte{0x01}, resp.TLSCert)
}

// TestConstructSSHResponseLegacy checks that the old-style NaCl encryption with
// a secret query parameter (rather than secret_key, using AES-GCM) is not
// supported.
func TestConstructSSHResponseLegacy(t *testing.T) {
	u := &url.URL{
		Scheme: "http",
		Host:   "www.example.com",
		Path:   "/callback",
		RawQuery: url.Values{
			// the old-style NaCl key is 32 bytes in base 32
			"secret": {base64.StdEncoding.EncodeToString(make([]byte, 32))},
		}.Encode(),
	}

	_, err := ConstructSSHResponse(AuthParams{
		Username:          "foo",
		Cert:              []byte{0x00},
		TLSCert:           []byte{0x01},
		ClientRedirectURL: u.String(),
	})
	require.ErrorIs(t, err, &trace.BadParameterError{Message: "missing secret_key"})
}

type byTimeAndIndex []apievents.AuditEvent

func (f byTimeAndIndex) Len() int {
	return len(f)
}

func (f byTimeAndIndex) Less(i, j int) bool {
	itime := f[i].GetTime()
	jtime := f[j].GetTime()
	if itime.Equal(jtime) && events.GetSessionID(f[i]) == events.GetSessionID(f[j]) {
		return f[i].GetIndex() < f[j].GetIndex()
	}
	return itime.Before(jtime)
}

func (f byTimeAndIndex) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

// TestSearchClusterEvents makes sure web API allows querying events by type.
func TestSearchClusterEvents(t *testing.T) {
	t.Parallel()

	s := newWebSuite(t)
	clock := s.clock
	sessionEvents := eventstest.GenerateTestSession(eventstest.SessionParams{
		PrintEvents: 3,
		Clock:       clock,
		ServerID:    s.proxy.ID(),
	})

	for _, e := range sessionEvents {
		require.NoError(t, s.proxyClient.EmitAuditEvent(s.ctx, e))
	}

	sort.Sort(sort.Reverse(byTimeAndIndex(sessionEvents)))
	sessionStart := sessionEvents[0]
	sessionPrint := sessionEvents[1]
	sessionEnd := sessionEvents[4]

	fromTime := []string{clock.Now().AddDate(0, -1, 0).UTC().Format(time.RFC3339)}
	toTime := []string{clock.Now().AddDate(0, 1, 0).UTC().Format(time.RFC3339)}

	testCases := []struct {
		// Comment is the test case description.
		Comment string
		// Query is the search query sent to the API.
		Query url.Values
		// Result is the expected returned list of events.
		Result []apievents.AuditEvent
		// TestStartKey is a flag to test start key value.
		TestStartKey bool
		// StartKeyValue is the value of start key to expect.
		StartKeyValue string
	}{
		{
			Comment: "Empty query",
			Query: url.Values{
				"from": fromTime,
				"to":   toTime,
			},
			Result: sessionEvents,
		},
		{
			Comment: "Query by session start event",
			Query: url.Values{
				"include": []string{sessionStart.GetType()},
				"from":    fromTime,
				"to":      toTime,
			},
			Result: sessionEvents[:1],
		},
		{
			Comment: "Query session start and session end events",
			Query: url.Values{
				"include": []string{sessionEnd.GetType() + "," + sessionStart.GetType()},
				"from":    fromTime,
				"to":      toTime,
			},
			Result: []apievents.AuditEvent{sessionStart, sessionEnd},
		},
		{
			Comment: "Query events with filter by type and limit",
			Query: url.Values{
				"include": []string{sessionPrint.GetType() + "," + sessionEnd.GetType()},
				"limit":   []string{"1"},
				"from":    fromTime,
				"to":      toTime,
			},
			Result: []apievents.AuditEvent{sessionPrint},
		},
		{
			Comment: "Query session start and session end events with limit and test returned start key",
			Query: url.Values{
				"include": []string{sessionEnd.GetType() + "," + sessionStart.GetType()},
				"limit":   []string{"1"},
				"from":    fromTime,
				"to":      toTime,
			},
			Result:        []apievents.AuditEvent{sessionStart},
			TestStartKey:  true,
			StartKeyValue: sessionStart.GetID(),
		},
		{
			Comment: "Query session start and session end events with limit and given start key",
			Query: url.Values{
				"include":  []string{sessionEnd.GetType() + "," + sessionStart.GetType()},
				"startKey": []string{sessionStart.GetID()},
				"from":     fromTime,
				"to":       toTime,
			},
			Result:        []apievents.AuditEvent{sessionEnd},
			TestStartKey:  true,
			StartKeyValue: "",
		},
	}

	pack := s.authPack(t, "foo")
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Comment, func(t *testing.T) {
			t.Parallel()
			response, err := pack.clt.Get(s.ctx, pack.clt.Endpoint("webapi", "sites", s.server.ClusterName(), "events", "search"), tc.Query)
			require.NoError(t, err)
			var result eventsListGetResponse
			require.NoError(t, json.Unmarshal(response.Bytes(), &result))

			// filter out irrelvant auth events
			filteredEvents := []events.EventFields{}
			for _, e := range result.Events {
				t := e.GetType()
				if t == events.SessionStartEvent ||
					t == events.SessionPrintEvent ||
					t == events.SessionEndEvent {
					filteredEvents = append(filteredEvents, e)
				}
			}

			require.Len(t, filteredEvents, len(tc.Result))
			for i, resultEvent := range filteredEvents {
				require.Equal(t, tc.Result[i].GetType(), resultEvent.GetType())
				require.Equal(t, tc.Result[i].GetID(), resultEvent.GetID())
			}

			// Session prints do not have IDs, only sessionStart and sessionEnd.
			// When retrieving events for sessionStart and sessionEnd, sessionStart is returned first.
			if tc.TestStartKey {
				require.Equal(t, tc.StartKeyValue, result.StartKey)
			}
		})
	}
}

func TestGetClusterDetails(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	site, err := s.proxyTunnel.GetSite(s.server.ClusterName())
	require.NoError(t, err)
	require.NotNil(t, site)

	cluster, err := webui.GetClusterDetails(s.ctx, site)
	require.NoError(t, err)
	require.Equal(t, s.server.ClusterName(), cluster.Name)
	require.Equal(t, teleport.Version, cluster.ProxyVersion)
	require.Equal(t, fmt.Sprintf("%v:%v", s.server.ClusterName(), defaults.HTTPListenPort), cluster.PublicURL)
	require.Equal(t, teleport.RemoteClusterStatusOnline, cluster.Status)
	require.NotNil(t, cluster.LastConnected)
	require.Equal(t, teleport.Version, cluster.AuthVersion)
}

func TestTokenGeneration(t *testing.T) {
	const username = "test-user@example.com"
	// Users should be able to create Tokens even if they can't update them
	roleTokenCRD, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindToken,
					[]string{types.VerbCreate, types.VerbRead}),
			},
		},
	})
	require.NoError(t, err)

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, username, []types.Role{roleTokenCRD})
	endpoint := pack.clt.Endpoint("webapi", "token")

	tt := []struct {
		name                        string
		roles                       types.SystemRoles
		shouldErr                   bool
		joinMethod                  types.JoinMethod
		suggestedAgentMatcherLabels types.Labels
		allow                       []*types.TokenRule
	}{
		{
			name:      "single node role",
			roles:     types.SystemRoles{types.RoleNode},
			shouldErr: false,
		},
		{
			name:      "single app role",
			roles:     types.SystemRoles{types.RoleApp},
			shouldErr: false,
		},
		{
			name:      "single db role",
			roles:     types.SystemRoles{types.RoleDatabase},
			shouldErr: false,
		},
		{
			name:      "multiple roles",
			roles:     types.SystemRoles{types.RoleNode, types.RoleApp, types.RoleDatabase},
			shouldErr: false,
		},
		{
			name:      "return error if no role is requested",
			roles:     types.SystemRoles{},
			shouldErr: true,
		},
		{
			name:       "cannot request token with IAM join method without allow field",
			roles:      types.SystemRoles{types.RoleNode},
			joinMethod: types.JoinMethodIAM,
			shouldErr:  true,
		},
		{
			name:       "can request token with IAM join method",
			roles:      types.SystemRoles{types.RoleNode},
			joinMethod: types.JoinMethodIAM,
			allow:      []*types.TokenRule{{AWSAccount: "1234"}},
			shouldErr:  false,
		},
		{
			name:  "adds the agent match labels",
			roles: types.SystemRoles{types.RoleDatabase},
			suggestedAgentMatcherLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},
			shouldErr: false,
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			re, err := pack.clt.PostJSON(context.Background(), endpoint, types.ProvisionTokenSpecV2{
				Roles:                       tc.roles,
				JoinMethod:                  tc.joinMethod,
				Allow:                       tc.allow,
				SuggestedAgentMatcherLabels: tc.suggestedAgentMatcherLabels,
			})

			if tc.shouldErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			var responseToken nodeJoinToken
			err = json.Unmarshal(re.Bytes(), &responseToken)
			require.NoError(t, err)

			require.NotEmpty(t, responseToken.SuggestedLabels)
			require.Condition(t, func() (success bool) {
				for _, uiLabel := range responseToken.SuggestedLabels {
					if uiLabel.Name == types.InternalResourceIDLabel && uiLabel.Value != "" {
						return true
					}
				}
				return false
			})

			// generated token roles should match the requested ones
			generatedToken, err := proxy.auth.Auth().GetToken(context.Background(), responseToken.ID)
			require.NoError(t, err)
			require.Equal(t, tc.roles, generatedToken.GetRoles())

			expectedJoinMethod := tc.joinMethod
			if tc.joinMethod == "" {
				expectedJoinMethod = types.JoinMethodToken
			}
			// if no joinMethod is provided, expect token method
			require.Equal(t, expectedJoinMethod, generatedToken.GetJoinMethod())

			require.Equal(t, tc.suggestedAgentMatcherLabels, generatedToken.GetSuggestedAgentMatcherLabels())
		})
	}
}

func TestEndpointNotFoundHandling(t *testing.T) {
	t.Parallel()
	const username = "test-user@example.com"
	// Allow user to create tokens.
	roleTokenCRD, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindToken,
					[]string{types.VerbCreate}),
			},
		},
	})
	require.NoError(t, err)

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, username, []types.Role{roleTokenCRD})

	tt := []struct {
		name      string
		endpoint  string
		shouldErr bool
	}{
		{
			name:     "valid endpoint without v1 prefix",
			endpoint: "webapi/token",
		},
		{
			name:     "valid endpoint with v1 prefix",
			endpoint: "v1/webapi/token",
		},
		{
			name:     "valid endpoint with v2 prefix",
			endpoint: "v2/webapi/token",
		},
		{
			name:      "invalid double version prefixes",
			endpoint:  "v1/v2/webapi/token",
			shouldErr: true,
		},
		{
			name:      "route not matched version prefix",
			endpoint:  "v9999999/webapi/token",
			shouldErr: true,
		},
		{
			name:      "non api route with prefix",
			endpoint:  "v1/something/else",
			shouldErr: true,
		},
		{
			name:      "invalid triple version prefixes",
			endpoint:  "v1/v1/v1/webapi/token",
			shouldErr: true,
		},
		{
			name:      "invalid just prefix",
			endpoint:  "v1",
			shouldErr: true,
		},
		{
			name:      "invalid prefix",
			endpoint:  "v1s/webapi/token",
			shouldErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			re, err := pack.clt.PostJSON(context.Background(), fmt.Sprintf("%s/%s", proxy.web.URL, tc.endpoint), types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodToken,
			})

			if tc.shouldErr {
				require.True(t, trace.IsNotFound(err))

				jsonResp := struct {
					Error struct {
						Message string
					}
					Fields struct {
						ProxyVersion httplib.ProxyVersion
					}
				}{}

				require.NoError(t, json.Unmarshal(re.Bytes(), &jsonResp))
				require.Equal(t, "path not found", jsonResp.Error.Message)
				require.Equal(t, teleport.Version, jsonResp.Fields.ProxyVersion.String)

				ver, err := semver.NewVersion(teleport.Version)
				require.NoError(t, err)
				require.Equal(t, ver.Major, jsonResp.Fields.ProxyVersion.Major)
				require.Equal(t, ver.Minor, jsonResp.Fields.ProxyVersion.Minor)
				require.Equal(t, ver.Patch, jsonResp.Fields.ProxyVersion.Patch)
				require.Equal(t, string(ver.PreRelease), jsonResp.Fields.ProxyVersion.PreRelease)

			} else {
				require.NoError(t, err)

				var responseToken nodeJoinToken
				err = json.Unmarshal(re.Bytes(), &responseToken)
				require.NoError(t, err)
				require.Equal(t, types.JoinMethodToken, responseToken.Method)
			}
		})
	}
}

func TestKnownWebPathsWithAndWithoutV1Prefix(t *testing.T) {
	t.Parallel()
	const username = "test-user@example.com"
	// Allow user to create tokens.
	roleTokenCRD, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindToken,
					[]string{types.VerbCreate}),
			},
		},
	})
	require.NoError(t, err)

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, username, []types.Role{roleTokenCRD})

	res, err := pack.clt.PostJSON(context.Background(), pack.clt.Endpoint("webapi", "token"), types.ProvisionTokenSpecV2{
		Roles: types.SystemRoles{types.RoleNode},
	})
	require.NoError(t, err)

	var responseToken nodeJoinToken
	err = json.Unmarshal(res.Bytes(), &responseToken)
	require.NoError(t, err)

	tt := []struct {
		name     string
		endpoint string
	}{
		{
			name:     "web path with prefix",
			endpoint: "v1/web/config.js",
		},
		{
			name:     "web path without prefix",
			endpoint: "web/config.js",
		},
		{
			name:     "webapi path with prefix",
			endpoint: "v1/webapi/spiffe/bundle.json",
		},
		{
			name:     "webapi path without prefix",
			endpoint: "webapi/spiffe/bundle.json",
		},
		{
			name:     ".well-known path with prefix",
			endpoint: "v1/.well-known/jwks.json",
		},
		{
			name:     ".well-known path without prefix",
			endpoint: ".well-known/jwks.json",
		},
		{
			name:     "workload-identity path with prefix",
			endpoint: "v1/workload-identity/jwt-jwks.json",
		},
		{
			name:     "workload-identity path without prefix",
			endpoint: "workload-identity/jwt-jwks.json",
		},
		{
			name:     "scripts path with prefix",
			endpoint: fmt.Sprintf("v1/scripts/%s/install-node.sh", responseToken.ID),
		},
		{
			name:     "scripts path without prefix",
			endpoint: fmt.Sprintf("scripts/%s/install-node.sh", responseToken.ID),
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := pack.clt.Get(context.Background(), fmt.Sprintf("%s/%s", proxy.web.URL, tc.endpoint), url.Values{})

			require.NoError(t, err)
		})
	}
}

func TestInstallDatabaseScriptGeneration(t *testing.T) {
	const username = "test-user@example.com"
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildCommunity})

	// Users should be able to create Tokens even if they can't update them
	roleTokenCRD, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindToken,
					[]string{types.VerbCreate, types.VerbRead}),
			},
		},
	})
	require.NoError(t, err)

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, username, []types.Role{roleTokenCRD})

	// Create a new token with the desired SuggestedAgentMatcherLabels
	endpointGenerateToken := pack.clt.Endpoint("webapi", "token")
	re, err := pack.clt.PostJSON(
		context.Background(),
		endpointGenerateToken,
		types.ProvisionTokenSpecV2{
			Roles: types.SystemRoles{types.RoleDatabase},
			SuggestedAgentMatcherLabels: types.Labels{
				"stage": apiutils.Strings{"prod"},
			},
		})
	require.NoError(t, err)

	var responseToken nodeJoinToken
	require.NoError(t, json.Unmarshal(re.Bytes(), &responseToken))

	// Generating the script with the token should return the SuggestedAgentMatcherLabels provided in the first request
	endpointInstallDatabase := pack.clt.Endpoint("scripts", responseToken.ID, "install-database.sh")

	t.Log(responseToken, endpointInstallDatabase)
	req, err := http.NewRequest(http.MethodGet, endpointInstallDatabase, nil)
	require.NoError(t, err)

	anonHTTPClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := anonHTTPClient.Do(req)
	require.NoError(t, err)

	scriptBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.NoError(t, resp.Body.Close())

	script := string(scriptBytes)

	// It contains the agenbtMatchLabels
	require.Contains(t, script, "stage: prod")
}

func TestSignMTLS(t *testing.T) {
	env := newWebPack(t, 1)
	clusterName := env.server.ClusterName()

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test-user@example.com", nil)

	endpoint := pack.clt.Endpoint("webapi", "token")
	re, err := pack.clt.PostJSON(context.Background(), endpoint, types.ProvisionTokenSpecV2{
		Roles: types.SystemRoles{types.RoleDatabase},
	})
	require.NoError(t, err)

	var responseToken nodeJoinToken
	err = json.Unmarshal(re.Bytes(), &responseToken)
	require.NoError(t, err)

	// download mTLS files from /webapi/sites/:site/sign/db
	endpointSign := pack.clt.Endpoint("webapi", "sites", clusterName, "sign", "db")

	bs, err := json.Marshal(struct {
		Hostname string `json:"hostname"`
		TTL      string `json:"ttl"`
	}{
		Hostname: "mypg.example.com",
		TTL:      "2h",
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, endpointSign, bytes.NewReader(bs))
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+responseToken.ID)

	anonHTTPClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := anonHTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	gzipReader, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)

	tarReader := tar.NewReader(gzipReader)

	tarContentFileNames := []string{}
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		require.Equal(t, byte(tar.TypeReg), header.Typeflag)
		require.Equal(t, int64(0o600), header.Mode)
		tarContentFileNames = append(tarContentFileNames, header.Name)
	}

	expectedFileNames := []string{"server.cas", "server.key", "server.crt"}
	require.ElementsMatch(t, tarContentFileNames, expectedFileNames)

	// the token is no longer valid, so trying again should return an error
	req, err = http.NewRequest(http.MethodPost, endpointSign, bytes.NewReader(bs))
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+responseToken.ID)

	respSecondCall, err := anonHTTPClient.Do(req)
	require.NoError(t, err)
	defer respSecondCall.Body.Close()
	require.Equal(t, http.StatusForbidden, respSecondCall.StatusCode)
}

func TestSignMTLS_failsAccessDenied(t *testing.T) {
	env := newWebPack(t, 1)
	clusterName := env.server.ClusterName()
	username := "test-user@example.com"

	roleUserUpdate, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindUser, []string{types.VerbUpdate}),
				types.NewRule(types.KindToken, []string{types.VerbCreate}),
			},
		},
	})
	require.NoError(t, err)

	proxy := env.proxies[0]
	pack := proxy.authPack(t, username, []types.Role{roleUserUpdate})

	endpoint := pack.clt.Endpoint("webapi", "token")
	re, err := pack.clt.PostJSON(context.Background(), endpoint, types.ProvisionTokenSpecV2{
		Roles: types.SystemRoles{types.RoleProxy},
	})
	require.NoError(t, err)

	var responseToken nodeJoinToken
	err = json.Unmarshal(re.Bytes(), &responseToken)
	require.NoError(t, err)

	// download mTLS files from /webapi/sites/:site/sign/db
	endpointSign := pack.clt.Endpoint("webapi", "sites", clusterName, "sign", "db")

	bs, err := json.Marshal(struct {
		Hostname string `json:"hostname"`
		TTL      string `json:"ttl"`
		Format   string `json:"format"`
	}{
		Hostname: "mypg.example.com",
		TTL:      "2h",
		Format:   "db",
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, endpointSign, bytes.NewReader(bs))
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+responseToken.ID)

	anonHTTPClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := anonHTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// It fails because we passed a Provision Token with the wrong Role: Proxy
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	// using a user token also returns Forbidden
	endpointResetToken := pack.clt.Endpoint("webapi", "users", "password", "token")
	_, err = pack.clt.PostJSON(context.Background(), endpointResetToken, authclient.CreateUserTokenRequest{
		Name: username,
		TTL:  time.Minute,
		Type: authclient.UserTokenTypeResetPassword,
	})
	require.NoError(t, err)

	req, err = http.NewRequest(http.MethodPost, endpointSign, bytes.NewReader(bs))
	require.NoError(t, err)

	resp, err = anonHTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestCheckAccessToRegisteredResource_AccessDenied tests that access denied error
// is ignored.
// TODO(kiosion): DELETE in 18.0
func TestCheckAccessToRegisteredResource_AccessDenied(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "foo", nil /* roles */)

	// newWebPack already registers 1 node.
	n, err := env.server.Auth().GetNodes(ctx, env.node.GetNamespace())
	require.NoError(t, err)
	require.Len(t, n, 1)

	// Checking for access returns true.
	endpoint := pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "resources", "check")
	re, err := pack.clt.Get(ctx, endpoint, url.Values{})
	require.NoError(t, err)
	resp := checkAccessToRegisteredResourceResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.True(t, resp.HasResource)

	// Deny this resource.
	fooRole, err := env.server.Auth().GetRole(ctx, "user:foo")
	require.NoError(t, err)
	fooRole.SetRules(types.Deny, []types.Rule{types.NewRule(types.KindNode, services.RW())})
	_, err = env.server.Auth().UpsertRole(ctx, fooRole)
	require.NoError(t, err)

	// Direct querying should return a access denied error.
	endpoint = pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "nodes")
	_, err = pack.clt.Get(ctx, endpoint, url.Values{})
	require.True(t, trace.IsAccessDenied(err))

	// Checking for access returns false, not an error.
	endpoint = pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "resources", "check")
	re, err = pack.clt.Get(ctx, endpoint, url.Values{})
	require.NoError(t, err)
	resp = checkAccessToRegisteredResourceResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.False(t, resp.HasResource)
}

// TODO(kiosion): DELETE in 18.0
func TestCheckAccessToRegisteredResource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "foo", nil /* roles */)

	// Delete the node that was created by the `newWebPack` to start afresh.
	require.NoError(t, env.server.Auth().DeleteNode(ctx, env.node.GetNamespace(), env.node.ID()))
	n, err := env.server.Auth().GetNodes(ctx, env.node.GetNamespace())
	require.NoError(t, err)
	require.Empty(t, n)

	// Double check we start of with no resources.
	endpoint := pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "resources", "check")
	re, err := pack.clt.Get(ctx, endpoint, url.Values{})
	require.NoError(t, err)
	resp := checkAccessToRegisteredResourceResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.False(t, resp.HasResource)

	// Test all cases return true.
	tests := []struct {
		name           string
		resourceKind   string
		insertResource func()
		deleteResource func()
	}{
		{
			name: "has registered windows desktop",
			insertResource: func() {
				wd, err := types.NewWindowsDesktopV3("test-desktop", nil, types.WindowsDesktopSpecV3{
					Addr:   "addr",
					HostID: "hostid",
				})
				require.NoError(t, err)
				require.NoError(t, env.server.Auth().UpsertWindowsDesktop(ctx, wd))
			},
			deleteResource: func() {
				require.NoError(t, env.server.Auth().DeleteWindowsDesktop(ctx, "hostid", "test-desktop"))
				wds, err := env.server.Auth().GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
				require.NoError(t, err)
				require.Empty(t, wds)
			},
		},
		{
			name: "has registered node",
			insertResource: func() {
				resource, err := types.NewServer("test-node", types.KindNode, types.ServerSpecV2{})
				require.NoError(t, err)
				_, err = env.server.Auth().UpsertNode(ctx, resource)
				require.NoError(t, err)
			},
			deleteResource: func() {
				require.NoError(t, env.server.Auth().DeleteNode(ctx, apidefaults.Namespace, "test-node"))
				nodes, err := env.server.Auth().GetNodes(ctx, apidefaults.Namespace)
				require.NoError(t, err)
				require.Empty(t, nodes)
			},
		},
		{
			name: "has registered app server",
			insertResource: func() {
				resource := &types.AppServerV3{
					Metadata: types.Metadata{Name: "test-app"},
					Kind:     types.KindApp,
					Version:  types.V2,
					Spec: types.AppServerSpecV3{
						HostID: "hostid",
						App: &types.AppV3{
							Metadata: types.Metadata{
								Name: "app-name",
							},
							Spec: types.AppSpecV3{
								URI: "https://console.aws.amazon.com",
							},
						},
					},
				}
				_, err := env.server.Auth().UpsertApplicationServer(ctx, resource)
				require.NoError(t, err)
			},
			deleteResource: func() {
				require.NoError(t, env.server.Auth().DeleteApplicationServer(ctx, apidefaults.Namespace, "hostid", "test-app"))
				apps, err := env.server.Auth().GetApplicationServers(ctx, apidefaults.Namespace)
				require.NoError(t, err)
				require.Empty(t, apps)
			},
		},
		{
			name: "has registered db server",
			insertResource: func() {
				db, err := types.NewDatabaseServerV3(types.Metadata{
					Name: "test-db",
				}, types.DatabaseServerSpecV3{
					Database: mustCreateDatabase(t, "test-db", "test-protocol", "test-uri"),
					Hostname: "test-hostname",
					HostID:   "test-hostID",
				})
				require.NoError(t, err)
				_, err = env.server.Auth().UpsertDatabaseServer(ctx, db)
				require.NoError(t, err)
			},
			deleteResource: func() {
				require.NoError(t, env.server.Auth().DeleteDatabaseServer(ctx, apidefaults.Namespace, "test-hostID", "test-db"))
				dbs, err := env.server.Auth().GetDatabaseServers(ctx, apidefaults.Namespace)
				require.NoError(t, err)
				require.Empty(t, dbs)
			},
		},
		{
			name: "has registered kube server",
			insertResource: func() {
				kubeCluster, err := types.NewKubernetesClusterV3(types.Metadata{Name: "test-kube-name"}, types.KubernetesClusterSpecV3{})
				require.NoError(t, err)
				kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeCluster, "test-kube", "test-kube")
				require.NoError(t, err)
				_, err = env.server.Auth().UpsertKubernetesServer(ctx, kubeServer)
				require.NoError(t, err)
			},
			deleteResource: func() {
				require.NoError(t, env.server.Auth().DeleteKubernetesServer(ctx, "test-kube", "test-kube-name"))
				kubes, err := env.server.Auth().GetKubernetesServers(ctx)
				require.NoError(t, err)
				require.Empty(t, kubes)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.insertResource()

			re, err := pack.clt.Get(ctx, endpoint, url.Values{})
			require.NoError(t, err)
			resp := checkAccessToRegisteredResourceResponse{}
			require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
			require.True(t, resp.HasResource)

			tc.deleteResource()
		})
	}
}

func mustCreateDatabase(t *testing.T, name, protocol, uri string) *types.DatabaseV3 {
	database, err := types.NewDatabaseV3(
		types.Metadata{
			Name: name,
		},
		types.DatabaseSpecV3{
			Protocol: protocol,
			URI:      uri,
		},
	)
	require.NoError(t, err)
	return database
}

func TestClusterDatabasesGet_NoRole(t *testing.T) {
	env := newWebPack(t, 1)

	proxy := env.proxies[0]

	pack := proxy.authPack(t, "test-user@example.com", nil /* roles */)

	query := url.Values{"sort": []string{"name"}}
	endpoint := pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "databases")

	type testResponse struct {
		Items      []webui.Database `json:"items"`
		TotalCount int              `json:"totalCount"`
	}

	// add db
	db, err := types.NewDatabaseV3(types.Metadata{
		Name: "dbdb",
		Labels: map[string]string{
			"env": "prod",
		},
	}, types.DatabaseSpecV3{
		Protocol: "test-protocol",
		URI:      "test-uri:1234",
	})
	require.NoError(t, err)
	dbServer, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "dddb1",
	}, types.DatabaseServerSpecV3{
		Hostname: "dddb1",
		HostID:   uuid.NewString(),
		Database: db,
	})
	require.NoError(t, err)

	_, err = env.server.Auth().UpsertDatabaseServer(context.Background(), dbServer)
	require.NoError(t, err)

	// Test without defined database names or users in role.
	re, err := pack.clt.Get(context.Background(), endpoint, query)
	require.NoError(t, err)

	resp := testResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	require.ElementsMatch(t, resp.Items, []webui.Database{{
		Kind:     types.KindDatabase,
		Name:     "dbdb",
		Type:     types.DatabaseTypeSelfHosted,
		Labels:   []ui.Label{{Name: "env", Value: "prod"}},
		Protocol: "test-protocol",
		Hostname: "test-uri",
		URI:      "test-uri:1234",
	}})
}

func TestClusterDatabasesGet_WithRole(t *testing.T) {
	env := newWebPack(t, 1)

	proxy := env.proxies[0]

	type testResponse struct {
		Items      []webui.Database `json:"items"`
		TotalCount int              `json:"totalCount"`
	}

	// Register databases.
	db, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "dbServer1",
	}, types.DatabaseServerSpecV3{
		Hostname: "test-hostname",
		HostID:   "test-hostID",
		Database: &types.DatabaseV3{
			Metadata: types.Metadata{
				Name:        "db1",
				Description: "test-description",
			},
			Spec: types.DatabaseSpecV3{
				Protocol: "test-protocol",
				URI:      "test-uri:1234",
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, err)

	_, err = env.server.Auth().UpsertDatabaseServer(context.Background(), db)
	require.NoError(t, err)

	// Test with a role that defines database names and users.
	extraRole := &types.RoleV6{
		Metadata: types.Metadata{Name: "extra-role"},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				DatabaseNames: []string{"name1"},
				DatabaseUsers: []string{"user1"},
				DatabaseLabels: types.Labels{
					"*": []string{"*"},
				},
			},
		},
	}

	query := url.Values{"sort": []string{"name"}}
	pack := proxy.authPack(t, "test-user2@example.com", services.NewRoleSet(extraRole))
	endpoint := pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "databases")
	re, err := pack.clt.Get(context.Background(), endpoint, query)
	require.NoError(t, err)

	resp := testResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
}

func TestClusterKubesGet(t *testing.T) {
	env := newWebPack(t, 1)

	proxy := env.proxies[0]

	extraRole := &types.RoleV6{
		Metadata: types.Metadata{Name: "extra-role"},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				KubeUsers:  []string{"user1"},
				KubeGroups: []string{"group1"},
				KubernetesLabels: types.Labels{
					"*": []string{"*"},
				},
			},
		},
	}

	cluster1, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name:   "test-kube1",
			Labels: map[string]string{"test-field": "test-value"},
		},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)

	// duplicate same server
	for i := 0; i < 3; i++ {
		server, err := types.NewKubernetesServerV3FromCluster(
			cluster1,
			fmt.Sprintf("hostname-%d", i),
			fmt.Sprintf("uid-%d", i),
		)
		require.NoError(t, err)
		// Register a kube service.
		_, err = env.server.Auth().UpsertKubernetesServer(context.Background(), server)
		require.NoError(t, err)
	}

	cluster2, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: "test-kube2",
		},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)
	server2, err := types.NewKubernetesServerV3FromCluster(
		cluster2,
		"test-kube2-hostname",
		"test-kube2-hostid",
	)
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertKubernetesServer(context.Background(), server2)
	require.NoError(t, err)

	type testResponse struct {
		Items      []webui.KubeCluster `json:"items"`
		TotalCount int                 `json:"totalCount"`
	}

	tt := []struct {
		name             string
		user             string
		extraRoles       services.RoleSet
		expectedResponse []webui.KubeCluster
	}{
		{
			name: "user with no extra roles",
			user: "test-user@example.com",
			expectedResponse: []webui.KubeCluster{
				{
					Name:       "test-kube1",
					Labels:     []ui.Label{{Name: "test-field", Value: "test-value"}},
					KubeUsers:  nil,
					KubeGroups: nil,
				},
				{
					Name:       "test-kube2",
					Labels:     []ui.Label{},
					KubeUsers:  nil,
					KubeGroups: nil,
				},
			},
		},
		{
			name:       "user with extra roles",
			user:       "test-user2@example.com",
			extraRoles: services.NewRoleSet(extraRole),
			expectedResponse: []webui.KubeCluster{
				{
					Name:       "test-kube1",
					Labels:     []ui.Label{{Name: "test-field", Value: "test-value"}},
					KubeUsers:  []string{"user1"},
					KubeGroups: []string{"group1"},
				},
				{
					Name:       "test-kube2",
					Labels:     []ui.Label{},
					KubeUsers:  []string{"user1"},
					KubeGroups: []string{"group1"},
				},
			},
		},
	}

	for _, tc := range tt {
		pack := proxy.authPack(t, tc.user, tc.extraRoles)

		endpoint := pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "kubernetes")

		re, err := pack.clt.Get(context.Background(), endpoint, url.Values{})
		require.NoError(t, err)

		resp := testResponse{}
		require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
		require.Len(t, resp.Items, 2)
		require.Equal(t, 2, resp.TotalCount)
		require.ElementsMatch(t, tc.expectedResponse, resp.Items)
	}
}

func TestClusterKubeResourcesGet(t *testing.T) {
	t.Parallel()
	kubeClusterName := "kube_cluster"

	roleWithFullAccess := func(username string) []types.Role {
		ret, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:       []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				Rules: []types.Rule{
					types.NewRule(types.KindConnectionDiagnostic, services.RW()),
				},
				KubeGroups: []string{"groups"},
				KubernetesResources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: types.Wildcard,
						Name:      types.Wildcard,
					},
					{
						Kind: types.KindKubeNamespace,
						Name: types.Wildcard,
					},
				},
			},
		})
		require.NoError(t, err)
		return []types.Role{ret}
	}
	require.NotNil(t, roleWithFullAccess)

	env := newWebPack(t, 1)

	type testResponse struct {
		Items      []webui.KubeResource `json:"items"`
		TotalCount int                  `json:"totalCount"`
	}

	tt := []struct {
		name             string
		user             string
		kind             string
		kubeCluster      string
		expectedResponse []webui.KubeResource
		wantErr          bool
	}{
		{
			name:        "get pods from gRPC server",
			kind:        types.KindKubePod,
			kubeCluster: kubeClusterName,
			expectedResponse: []webui.KubeResource{
				{
					Kind:        types.KindKubePod,
					Name:        "test-pod",
					Namespace:   "default",
					Labels:      []ui.Label{{Name: "app", Value: "test"}},
					KubeCluster: kubeClusterName,
				},
				{
					Kind:        types.KindKubePod,
					Name:        "test-pod2",
					Namespace:   "default",
					Labels:      []ui.Label{{Name: "app", Value: "test2"}},
					KubeCluster: kubeClusterName,
				},
			},
		},
		{
			name:        "get namespaces",
			kind:        types.KindKubeNamespace,
			kubeCluster: kubeClusterName,
			expectedResponse: []webui.KubeResource{
				{
					Kind:        types.KindKubeNamespace,
					Name:        "default",
					Namespace:   "",
					Labels:      []ui.Label{{Name: "app", Value: "test"}},
					KubeCluster: kubeClusterName,
				},
			},
		},
		{
			name:        "missing kind",
			kind:        "",
			kubeCluster: kubeClusterName,
			wantErr:     true,
		},
		{
			name:        "invalid kind",
			kind:        "invalid-kind",
			kubeCluster: kubeClusterName,
			wantErr:     true,
		},
		{
			name:        "missing kube cluster",
			kind:        types.KindKubeNamespace,
			kubeCluster: "",
			wantErr:     true,
		},
	}
	proxy := env.proxies[0]
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	// Init fake gRPC Kube service.
	initGRPCServer(t, env, listener)
	addr := utils.MustParseAddr(listener.Addr().String())
	proxy.handler.handler.cfg.ProxyWebAddr = *addr

	user := "test-user@example.com"
	pack := proxy.authPack(t, user, roleWithFullAccess(user))

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			endpoint := pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "kubernetes", "resources")
			params := url.Values{}
			params.Add("kubeCluster", tc.kubeCluster)
			params.Add("kind", tc.kind)
			re, err := pack.clt.Get(context.Background(), endpoint, params)

			if tc.wantErr {
				require.True(t, trace.IsBadParameter(err))
			} else {
				require.NoError(t, err)
				resp := testResponse{}
				require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
				require.ElementsMatch(t, tc.expectedResponse, resp.Items)
			}
		})
	}
}

// DELETE IN 16.0
func TestClusterAppsGet(t *testing.T) {
	env := newWebPack(t, 1)

	// Set license to enterprise in order to be able to list SAML IdP Service Providers.
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
	})

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test-user@example.com", nil /* roles */)

	type testResponse struct {
		Items      []webui.App `json:"items"`
		TotalCount int         `json:"totalCount"`
	}

	// add a user group
	ug, err := types.NewUserGroup(types.Metadata{
		Name: "ug1", Description: "ug1-description",
	},
		types.UserGroupSpecV1{Applications: []string{"app1"}})
	require.NoError(t, err)
	err = env.server.Auth().CreateUserGroup(context.Background(), ug)
	require.NoError(t, err)

	resource := &types.AppServerV3{
		Metadata: types.Metadata{Name: "test-app"},
		Kind:     types.KindApp,
		Version:  types.V2,
		Spec: types.AppServerSpecV3{
			HostID: "hostid",
			App: &types.AppV3{
				Metadata: types.Metadata{
					Name:        "app1",
					Description: "description",
					Labels:      map[string]string{"test-field": "test-value"},
				},
				Spec: types.AppSpecV3{
					URI:        "https://console.aws.amazon.com", // sets field awsConsole to true
					PublicAddr: "publicaddrs",
					UserGroups: []string{"ug1", "ug2"}, // ug2 doesn't exist in the backend, so its lookup will fail.
				},
			},
		},
	}

	resource2, err := types.NewAppServerV3(types.Metadata{Name: "server2"}, types.AppServerSpecV3{
		HostID: "hostid",
		App: &types.AppV3{
			Metadata: types.Metadata{Name: "app2"},
			Spec:     types.AppSpecV3{URI: "uri", PublicAddr: "publicaddrs"},
		},
	})
	require.NoError(t, err)

	// Register apps and service providers.
	_, err = env.server.Auth().UpsertApplicationServer(context.Background(), resource)
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertApplicationServer(context.Background(), resource2)
	require.NoError(t, err)

	// Make the call.
	endpoint := pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "apps")
	re, err := pack.clt.Get(context.Background(), endpoint, url.Values{"sort": []string{"name"}})
	require.NoError(t, err)

	// Test correct response.
	resp := testResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.Len(t, resp.Items, 2)
	require.Equal(t, 2, resp.TotalCount)
	require.ElementsMatch(t, resp.Items, []webui.App{{
		Kind:        types.KindApp,
		Name:        "app1",
		Description: resource.Spec.App.GetDescription(),
		URI:         resource.Spec.App.GetURI(),
		PublicAddr:  resource.Spec.App.GetPublicAddr(),
		Labels:      []ui.Label{{Name: "test-field", Value: "test-value"}},
		FQDN:        resource.Spec.App.GetPublicAddr(),
		ClusterID:   env.server.ClusterName(),
		AWSConsole:  true,
		UserGroups:  []webui.UserGroupAndDescription{{Name: "ug1", Description: "ug1-description"}},
	}, {
		Kind:       types.KindApp,
		Name:       "app2",
		URI:        "uri",
		Labels:     []ui.Label{},
		ClusterID:  env.server.ClusterName(),
		FQDN:       "publicaddrs",
		PublicAddr: "publicaddrs",
		AWSConsole: false,
	}})
}

// TestApplicationAccessDisabled makes sure application access can be disabled
// via modules.
func TestApplicationAccessDisabled(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.App: {Enabled: false},
			},
		},
	})

	env := newWebPack(t, 1)

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "foo@example.com", nil /* roles */)

	// Register an application.
	app, err := types.NewAppV3(types.Metadata{
		Name: "panel",
	}, types.AppSpecV3{
		URI:        "localhost",
		PublicAddr: "panel.example.com",
	})
	require.NoError(t, err)
	server, err := types.NewAppServerV3FromApp(app, "host", uuid.New().String())
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertApplicationServer(context.Background(), server)
	require.NoError(t, err)

	endpoint := pack.clt.Endpoint("webapi", "sessions", "app")
	_, err = pack.clt.PostJSON(context.Background(), endpoint, &CreateAppSessionRequest{
		ResolveAppParams: ResolveAppParams{
			FQDNHint:    "panel.example.com",
			PublicAddr:  "panel.example.com",
			ClusterName: "localhost",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "this Teleport cluster is not licensed for application access")
}

// TestApplicationWebSessionsDeletedAfterLogout makes sure user's application
// sessions are deleted after user logout.
func TestApplicationWebSessionsDeletedAfterLogout(t *testing.T) {
	env := newWebPack(t, 1)

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "foo@example.com", nil /* roles */)

	// Register multiple applications.
	applications := []struct {
		name       string
		publicAddr string
	}{
		{name: "panel", publicAddr: "panel.example.com"},
		{name: "admin", publicAddr: "admin.example.com"},
		{name: "metrics", publicAddr: "metrics.example.com"},
	}

	// Register and create a session for each application.
	for _, application := range applications {
		// Register an application.
		app, err := types.NewAppV3(types.Metadata{
			Name: application.name,
		}, types.AppSpecV3{
			URI:        "localhost",
			PublicAddr: application.publicAddr,
		})
		require.NoError(t, err)
		server, err := types.NewAppServerV3FromApp(app, "host", uuid.New().String())
		require.NoError(t, err)
		_, err = env.server.Auth().UpsertApplicationServer(context.Background(), server)
		require.NoError(t, err)

		// Create application session
		endpoint := pack.clt.Endpoint("webapi", "sessions", "app")
		_, err = pack.clt.PostJSON(context.Background(), endpoint, &CreateAppSessionRequest{
			ResolveAppParams: ResolveAppParams{
				FQDNHint:    application.publicAddr,
				PublicAddr:  application.publicAddr,
				ClusterName: "localhost",
			},
		})
		require.NoError(t, err)
	}

	collectAppSessions := func(ctx context.Context) []types.WebSession {
		var (
			nextToken string
			sessions  []types.WebSession
		)
		for {
			webSessions, token, err := proxy.client.ListAppSessions(ctx, apidefaults.DefaultChunkSize, nextToken, "")
			require.NoError(t, err)
			sessions = append(sessions, webSessions...)
			if token == "" {
				break
			}

			nextToken = token
		}

		return sessions
	}

	// List sessions, should have one for each application.
	require.Len(t, collectAppSessions(context.Background()), len(applications))

	// Logout from Telport.
	_, err := pack.clt.Delete(context.Background(), pack.clt.Endpoint("webapi", "sessions", "web"))
	require.NoError(t, err)

	// Check sessions after logout, should be empty.
	require.Empty(t, collectAppSessions(context.Background()))
}

func TestGetAppDetails(t *testing.T) {
	ctx := context.Background()
	s := newWebSuite(t)
	pack := s.authPack(t, "foo@example.com")

	// Register an application called "api".
	apiApp, err := types.NewAppV3(types.Metadata{
		Name: "api",
	}, types.AppSpecV3{
		URI:        "http://127.0.0.1:8080",
		PublicAddr: "api.example.com",
	})
	require.NoError(t, err)
	server, err := types.NewAppServerV3FromApp(apiApp, "host", uuid.New().String())
	require.NoError(t, err)
	_, err = s.server.Auth().UpsertApplicationServer(s.ctx, server)
	require.NoError(t, err)

	// Register an application called "client" and have "api" required.
	clientApp, err := types.NewAppV3(types.Metadata{
		Name: "client",
	}, types.AppSpecV3{
		URI:              "http://127.0.0.1:8080",
		PublicAddr:       "client.example.com",
		RequiredAppNames: []string{"api"},
	})
	require.NoError(t, err)
	server2, err := types.NewAppServerV3FromApp(clientApp, "host", uuid.New().String())
	require.NoError(t, err)
	_, err = s.server.Auth().UpsertApplicationServer(s.ctx, server2)
	require.NoError(t, err)

	clientFQDN := "client.example.com"

	tests := []struct {
		name     string
		endpoint string
	}{
		{
			name:     "request app details with clientName and publicAddr",
			endpoint: pack.clt.Endpoint("webapi", "apps", clientFQDN, s.server.ClusterName(), clientApp.GetPublicAddr()),
		},
		{
			name:     "request app details with fqdn only",
			endpoint: pack.clt.Endpoint("webapi", "apps", clientFQDN),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			re, err := pack.clt.Get(ctx, tc.endpoint, url.Values{})
			require.NoError(t, err)
			resp := GetAppDetailsResponse{}

			require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
			require.Equal(t, GetAppDetailsResponse{
				FQDN:             "client.example.com",
				RequiredAppFQDNs: []string{"api.example.com", "client.example.com"},
			}, resp)
		})
	}
}

func TestGetWebConfig_WithEntitlements(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)
	handler := env.proxies[0].handler.handler

	// Set auth preference with passwordless.
	const MOTD = "Welcome to cluster, your activity will be recorded."
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:          constants.Local,
		SecondFactor:  constants.SecondFactorOn,
		ConnectorName: constants.PasswordlessConnector,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
		MessageOfTheDay: MOTD,
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAuthPreference(ctx, ap)
	require.NoError(t, err)

	// Add a test connector.
	github, err := types.NewGithubConnector("test-github", types.GithubConnectorSpecV3{
		TeamsToLogins: []types.TeamMapping{
			{
				Organization: "octocats",
				Team:         "dummy",
				Logins:       []string{"dummy"},
			},
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertGithubConnector(ctx, github)
	require.NoError(t, err)

	// start the feature watcher so the web config gets new features
	env.clock.Advance(DefaultFeatureWatchInterval * 2)

	expectedCfg := webclient.WebConfig{
		Auth: webclient.WebConfigAuthSettings{
			SecondFactor: constants.SecondFactorOn,
			Providers: []webclient.WebConfigAuthProvider{{
				Name:      "test-github",
				Type:      constants.Github,
				WebAPIURL: webclient.WebConfigAuthProviderGitHubURL,
			}},
			LocalAuthEnabled:   true,
			AllowPasswordless:  true,
			AuthType:           constants.Local,
			PreferredLocalMFA:  constants.SecondFactorWebauthn,
			LocalConnectorName: constants.PasswordlessConnector,
			PrivateKeyPolicy:   keys.PrivateKeyPolicyNone,
			MOTD:               MOTD,
		},
		CanJoinSessions:    true,
		ProxyClusterName:   env.server.ClusterName(),
		IsCloud:            false,
		AutomaticUpgrades:  false,
		JoinActiveSessions: true,
		Edition:            modules.BuildOSS, // testBuildType is empty
		Entitlements: map[string]webclient.EntitlementInfo{
			string(entitlements.AccessLists):            {Enabled: false},
			string(entitlements.AccessMonitoring):       {Enabled: false},
			string(entitlements.AccessRequests):         {Enabled: false},
			string(entitlements.App):                    {Enabled: true},
			string(entitlements.CloudAuditLogRetention): {Enabled: false},
			string(entitlements.DB):                     {Enabled: true},
			string(entitlements.Desktop):                {Enabled: true},
			string(entitlements.DeviceTrust):            {Enabled: false},
			string(entitlements.ExternalAuditStorage):   {Enabled: false},
			string(entitlements.FeatureHiding):          {Enabled: false},
			string(entitlements.HSM):                    {Enabled: false},
			string(entitlements.Identity):               {Enabled: false},
			string(entitlements.JoinActiveSessions):     {Enabled: true},
			string(entitlements.K8s):                    {Enabled: true},
			string(entitlements.MobileDeviceManagement): {Enabled: false},
			string(entitlements.OIDC):                   {Enabled: false},
			string(entitlements.OktaSCIM):               {Enabled: false},
			string(entitlements.OktaUserSync):           {Enabled: false},
			string(entitlements.Policy):                 {Enabled: false},
			string(entitlements.SAML):                   {Enabled: false},
			string(entitlements.SessionLocks):           {Enabled: false},
			string(entitlements.UpsellAlert):            {Enabled: false},
			string(entitlements.UsageReporting):         {Enabled: false},
			string(entitlements.LicenseAutoUpdate):      {Enabled: false},
		},
		TunnelPublicAddress:            "",
		RecoveryCodesEnabled:           false,
		UI:                             webclient.UIConfig{},
		IsPolicyRoleVisualizerEnabled:  true,
		IsDashboard:                    false,
		IsUsageBasedBilling:            false,
		AutomaticUpgradesTargetVersion: "",
		CustomTheme:                    "",
		Questionnaire:                  false,
		IsStripeManaged:                false,
		PremiumSupport:                 false,
		PlayableDatabaseProtocols:      player.SupportedDatabaseProtocols,
	}

	// Make a request.
	clt := env.proxies[0].newClient(t)
	endpoint := clt.Endpoint("web", "config.js")
	re, err := clt.Get(ctx, endpoint, nil)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(re.Bytes()), "var GRV_CONFIG"))

	// Response is type application/javascript, we need to strip off the variable name
	// and the semicolon at the end, then we are left with json like object.
	var cfg webclient.WebConfig
	str := strings.ReplaceAll(string(re.Bytes()), "var GRV_CONFIG = ", "")
	err = json.Unmarshal([]byte(str[:len(str)-1]), &cfg)
	require.NoError(t, err)
	require.Equal(t, expectedCfg, cfg)

	// update features and assert that it is properly updated on the config object
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Cloud:               true,
			IsUsageBasedBilling: true,
			AutomaticUpgrades:   true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DB:          {Enabled: true, Limit: 22},
				entitlements.DeviceTrust: {Enabled: true, Limit: 33},
				entitlements.Desktop:     {Enabled: true, Limit: 44},
			},
		},
	})
	env.clock.Advance(DefaultFeatureWatchInterval * 2)

	require.NoError(t, err)
	// This version is too high and MUST NOT be used
	testVersion := "v99.0.1"
	channels := automaticupgrades.Channels{
		automaticupgrades.DefaultCloudChannelName: {
			StaticVersion: testVersion,
		},
	}
	require.NoError(t, channels.CheckAndSetDefaults())
	handler.cfg.AutomaticUpgradesChannels = channels

	expectedCfg.IsCloud = true
	expectedCfg.IsUsageBasedBilling = true
	expectedCfg.AutomaticUpgrades = true
	expectedCfg.AutomaticUpgradesTargetVersion = "v" + teleport.Version
	expectedCfg.JoinActiveSessions = false
	expectedCfg.Edition = "" // testBuildType is empty
	expectedCfg.TrustedDevices = true
	expectedCfg.Entitlements[string(entitlements.App)] = webclient.EntitlementInfo{Enabled: false}
	expectedCfg.Entitlements[string(entitlements.DB)] = webclient.EntitlementInfo{Enabled: true, Limit: 22}
	expectedCfg.Entitlements[string(entitlements.DeviceTrust)] = webclient.EntitlementInfo{Enabled: true, Limit: 33}
	expectedCfg.Entitlements[string(entitlements.Desktop)] = webclient.EntitlementInfo{Enabled: true, Limit: 44}
	expectedCfg.Entitlements[string(entitlements.JoinActiveSessions)] = webclient.EntitlementInfo{Enabled: false}
	expectedCfg.Entitlements[string(entitlements.K8s)] = webclient.EntitlementInfo{Enabled: false}

	// request and verify enabled features are eventually enabled.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		re, err := clt.Get(ctx, endpoint, nil)
		if !assert.NoError(t, err) {
			return
		}
		assert.True(t, bytes.HasPrefix(re.Bytes(), []byte("var GRV_CONFIG")))
		res := bytes.ReplaceAll(re.Bytes(), []byte("var GRV_CONFIG = "), []byte{})
		err = json.Unmarshal(res[:len(res)-1], &cfg)
		assert.NoError(t, err)
		diff := cmp.Diff(expectedCfg, cfg)
		assert.Empty(t, diff)
	}, time.Second*5, time.Millisecond*50)

	// use mock client to assert that if ping returns an error, we'll default to
	// cluster config
	mockClient := mockedPingTestProxy{
		mockedPing: func(ctx context.Context) (authproto.PingResponse, error) {
			return authproto.PingResponse{}, errors.New("err")
		},
	}
	env.proxies[0].client = mockClient
	expectedCfg.AutomaticUpgrades = false
	expectedCfg.TrustedDevices = false
	expectedCfg.Entitlements[string(entitlements.DB)] = webclient.EntitlementInfo{Enabled: false}
	expectedCfg.Entitlements[string(entitlements.Desktop)] = webclient.EntitlementInfo{Enabled: false}
	expectedCfg.Entitlements[string(entitlements.DeviceTrust)] = webclient.EntitlementInfo{Enabled: false}

	// update modules but NOT the expected config
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Cloud:               false,
			IsUsageBasedBilling: false,
		},
	})
	env.clock.Advance(DefaultFeatureWatchInterval * 2)

	// request and verify again
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		re, err := clt.Get(ctx, endpoint, nil)
		if !assert.NoError(t, err) {
			return
		}
		assert.True(t, bytes.HasPrefix(re.Bytes(), []byte("var GRV_CONFIG")))
		res := bytes.ReplaceAll(re.Bytes(), []byte("var GRV_CONFIG = "), []byte{})
		err = json.Unmarshal(res[:len(res)-1], &cfg)
		assert.NoError(t, err)
		diff := cmp.Diff(expectedCfg, cfg)
		assert.Empty(t, diff)
	}, time.Second*5, time.Millisecond*50)
}

func TestGetWebConfig_LegacyFeatureLimits(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)

	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			ProductType:         modules.ProductTypeTeam,
			IsUsageBasedBilling: true,
			IsStripeManaged:     true,
			Questionnaire:       true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity:         {Enabled: true},
				entitlements.AccessLists:      {Enabled: true, Limit: 5},
				entitlements.AccessMonitoring: {Enabled: true, Limit: 10},
			},
		},
	})
	// start the feature watcher so the web config gets new features
	env.clock.Advance(DefaultFeatureWatchInterval * 2)

	expectedCfg := webclient.WebConfig{
		Auth: webclient.WebConfigAuthSettings{
			SecondFactor:     constants.SecondFactorOff,
			LocalAuthEnabled: true,
			AuthType:         constants.Local,
			PrivateKeyPolicy: keys.PrivateKeyPolicyNone,
		},
		CanJoinSessions:  true,
		ProxyClusterName: env.server.ClusterName(),
		FeatureLimits: webclient.FeatureLimits{
			AccessListCreateLimit:               5,
			AccessMonitoringMaxReportRangeLimit: 10,
		},
		IsTeam:              false,
		IsIGSEnabled:        true,
		IsStripeManaged:     true,
		Questionnaire:       true,
		IsUsageBasedBilling: true,
		Entitlements: map[string]webclient.EntitlementInfo{
			string(entitlements.AccessLists):            {Enabled: true, Limit: 5},
			string(entitlements.AccessMonitoring):       {Enabled: true, Limit: 10},
			string(entitlements.AccessRequests):         {Enabled: false},
			string(entitlements.App):                    {Enabled: false},
			string(entitlements.CloudAuditLogRetention): {Enabled: false},
			string(entitlements.DB):                     {Enabled: false},
			string(entitlements.Desktop):                {Enabled: false},
			string(entitlements.DeviceTrust):            {Enabled: false},
			string(entitlements.ExternalAuditStorage):   {Enabled: false},
			string(entitlements.FeatureHiding):          {Enabled: false},
			string(entitlements.HSM):                    {Enabled: false},
			string(entitlements.Identity):               {Enabled: true},
			string(entitlements.JoinActiveSessions):     {Enabled: false},
			string(entitlements.K8s):                    {Enabled: false},
			string(entitlements.MobileDeviceManagement): {Enabled: false},
			string(entitlements.OIDC):                   {Enabled: false},
			string(entitlements.OktaSCIM):               {Enabled: false},
			string(entitlements.OktaUserSync):           {Enabled: false},
			string(entitlements.Policy):                 {Enabled: false},
			string(entitlements.SAML):                   {Enabled: false},
			string(entitlements.SessionLocks):           {Enabled: false},
			string(entitlements.UpsellAlert):            {Enabled: false},
			string(entitlements.UsageReporting):         {Enabled: false},
			string(entitlements.LicenseAutoUpdate):      {Enabled: false},
		},
		PlayableDatabaseProtocols:     player.SupportedDatabaseProtocols,
		IsPolicyRoleVisualizerEnabled: true,
	}

	clt := env.proxies[0].newClient(t)
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Make a request.
		endpoint := clt.Endpoint("web", "config.js")
		re, err := clt.Get(ctx, endpoint, nil)
		if !assert.NoError(t, err) {
			return
		}
		assert.True(t, bytes.HasPrefix(re.Bytes(), []byte("var GRV_CONFIG")))

		// Response is type application/javascript, we need to strip off the variable name
		// and the semicolon at the end, then we are left with json like object.
		var cfg webclient.WebConfig
		res := bytes.ReplaceAll(re.Bytes(), []byte("var GRV_CONFIG = "), []byte{})
		err = json.Unmarshal(res[:len(res)-1], &cfg)
		assert.NoError(t, err)
		diff := cmp.Diff(expectedCfg, cfg)
		assert.Empty(t, diff)
	}, time.Second*5, time.Millisecond*50)
}

func TestCreatePrivilegeToken(t *testing.T) {
	t.Parallel()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	// Create a user with second factor totp.
	pack := proxy.authPack(t, "foo@example.com", nil /* roles */)

	// Get a totp code.
	totpCode, err := totp.GenerateCode(pack.otpSecret, env.clock.Now().Add(30*time.Second))
	require.NoError(t, err)

	endpoint := pack.clt.Endpoint("webapi", "users", "privilege", "token")
	re, err := pack.clt.PostJSON(context.Background(), endpoint, &privilegeTokenRequest{
		SecondFactorToken: totpCode,
	})
	require.NoError(t, err)

	var privilegeToken string
	err = json.Unmarshal(re.Bytes(), &privilegeToken)
	require.NoError(t, err)
	require.NotEmpty(t, privilegeToken)
}

func TestAddMFADevice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "foo@example.com", nil /* roles */)

	// Enable second factor.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAuthPreference(ctx, ap)
	require.NoError(t, err)

	// Get a totp code to re-auth.
	totpCode, err := totp.GenerateCode(pack.otpSecret, env.clock.Now().Add(30*time.Second))
	require.NoError(t, err)

	// Obtain a privilege token.
	endpoint := pack.clt.Endpoint("webapi", "users", "privilege", "token")
	re, err := pack.clt.PostJSON(ctx, endpoint, &privilegeTokenRequest{
		SecondFactorToken: totpCode,
	})
	require.NoError(t, err)
	var privilegeToken string
	require.NoError(t, json.Unmarshal(re.Bytes(), &privilegeToken))

	tests := []struct {
		name            string
		deviceName      string
		getTOTPCode     func() string
		getWebauthnResp func() *wantypes.CredentialCreationResponse
	}{
		{
			name:       "new TOTP device",
			deviceName: "new-totp",
			getTOTPCode: func() string {
				// Create totp secrets.
				res, err := env.server.Auth().CreateRegisterChallenge(ctx, &authproto.CreateRegisterChallengeRequest{
					TokenID:    privilegeToken,
					DeviceType: authproto.DeviceType_DEVICE_TYPE_TOTP,
				})
				require.NoError(t, err)

				_, regRes, err := auth.NewTestDeviceFromChallenge(res, auth.WithTestDeviceClock(env.clock))
				require.NoError(t, err)

				return regRes.GetTOTP().Code
			},
		},
		{
			name:       "new Webauthn device",
			deviceName: "new-webauthn",
			getWebauthnResp: func() *wantypes.CredentialCreationResponse {
				// Get webauthn register challenge.
				res, err := env.server.Auth().CreateRegisterChallenge(ctx, &authproto.CreateRegisterChallengeRequest{
					TokenID:    privilegeToken,
					DeviceType: authproto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				})
				require.NoError(t, err)

				_, regRes, err := auth.NewTestDeviceFromChallenge(res)
				require.NoError(t, err)

				return wantypes.CredentialCreationResponseFromProto(regRes.GetWebauthn())
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var totpCode string
			var webauthnRegResp *wantypes.CredentialCreationResponse

			if tc.getWebauthnResp != nil {
				webauthnRegResp = tc.getWebauthnResp()
			} else {
				totpCode = tc.getTOTPCode()
			}

			// Add device.
			endpoint := pack.clt.Endpoint("webapi", "mfa", "devices")
			_, err := pack.clt.PostJSON(ctx, endpoint, addMFADeviceRequest{
				PrivilegeTokenID:         privilegeToken,
				DeviceName:               tc.deviceName,
				SecondFactorToken:        totpCode,
				WebauthnRegisterResponse: webauthnRegResp,
			})
			require.NoError(t, err)
		})
	}
}

func TestDeleteMFA(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "foo@example.com", nil /* roles */)

	// setting up client manually because we need sanitizer off
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	opts := []roundtrip.ClientParam{roundtrip.BearerAuth(pack.session.Token), roundtrip.CookieJar(jar), roundtrip.HTTPClient(client.NewInsecureWebClient())}
	rclt, err := roundtrip.NewClient(proxy.webURL.String(), "", opts...)
	require.NoError(t, err)
	clt := client.WebClient{Client: rclt}
	jar.SetCookies(&proxy.webURL, pack.cookies)

	totpCode, err := totp.GenerateCode(pack.otpSecret, env.clock.Now().Add(30*time.Second))
	require.NoError(t, err)

	// Obtain a privilege token.
	endpoint := pack.clt.Endpoint("webapi", "users", "privilege", "token")
	re, err := pack.clt.PostJSON(ctx, endpoint, &privilegeTokenRequest{
		SecondFactorToken: totpCode,
	})
	require.NoError(t, err)

	var privilegeToken string
	require.NoError(t, json.Unmarshal(re.Bytes(), &privilegeToken))

	names := []string{"x", "??", "%123/", "///", "my/device", "?/%&*1"}
	for _, devName := range names {
		devName := devName
		t.Run(devName, func(t *testing.T) {
			t.Parallel()
			otpSecret := newOTPSharedSecret()
			dev, err := services.NewTOTPDevice(devName, otpSecret, env.clock.Now())
			require.NoError(t, err)
			err = env.server.Auth().UpsertMFADevice(ctx, pack.user, dev)
			require.NoError(t, err)

			enc := url.PathEscape(devName)
			_, err = clt.Delete(ctx, pack.clt.Endpoint("webapi", "mfa", "token", privilegeToken, "devices", enc))
			require.NoError(t, err)
		})
	}
}

func TestGetMFADevicesWithAuth(t *testing.T) {
	t.Parallel()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "foo@example.com", nil /* roles */)

	endpoint := pack.clt.Endpoint("webapi", "mfa", "devices")
	re, err := pack.clt.Get(context.Background(), endpoint, url.Values{})
	require.NoError(t, err)

	var devices []webui.MFADevice
	err = json.Unmarshal(re.Bytes(), &devices)
	require.NoError(t, err)
	require.Len(t, devices, 1)
}

func TestGetAndDeleteMFADevices_WithRecoveryApprovedToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	// Create a user with a TOTP device.
	username := "llama"
	proxy.createUser(ctx, t, username, "root", "password1234", "some-otp-secret", nil /* roles */)

	// Enable second factor.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: env.server.ClusterName(),
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAuthPreference(ctx, ap)
	require.NoError(t, err)

	// Acquire an approved token.
	approvedToken, err := types.NewUserToken("some-token-id")
	require.NoError(t, err)
	approvedToken.SetUser(username)
	approvedToken.SetSubKind(authclient.UserTokenTypeRecoveryApproved)
	approvedToken.SetExpiry(env.clock.Now().Add(5 * time.Minute))
	_, err = env.server.Auth().CreateUserToken(ctx, approvedToken)
	require.NoError(t, err)

	// Call the getter endpoint.
	clt := proxy.newClient(t)
	getDevicesEndpoint := clt.Endpoint("webapi", "mfa", "token", approvedToken.GetName(), "devices")
	res, err := clt.Get(ctx, getDevicesEndpoint, url.Values{})
	require.NoError(t, err)

	var devices []webui.MFADevice
	err = json.Unmarshal(res.Bytes(), &devices)
	require.NoError(t, err)
	require.Len(t, devices, 1)

	// Call the delete endpoint.
	_, err = clt.Delete(ctx, clt.Endpoint("webapi", "mfa", "token", approvedToken.GetName(), "devices", devices[0].Name))
	require.NoError(t, err)

	// Check device has been deleted.
	res, err = clt.Get(ctx, getDevicesEndpoint, url.Values{})
	require.NoError(t, err)

	err = json.Unmarshal(res.Bytes(), &devices)
	require.NoError(t, err)
	require.Empty(t, devices)
}

func TestCreateAuthenticateChallenge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	// Create a user with a TOTP device, with second factor preference to OTP only.
	authPack := proxy.authPack(t, "llama@example.com", nil /* roles */)

	// Authenticated client for private endpoints.
	authnClt := authPack.clt

	// Unauthenticated client for public endpoints.
	publicClt := proxy.newClient(t)

	// Acquire a start token, for the request the requires it.
	startToken, err := types.NewUserToken("some-token-id")
	require.NoError(t, err)
	startToken.SetUser(authPack.user)
	startToken.SetSubKind(authclient.UserTokenTypeRecoveryStart)
	startToken.SetExpiry(env.clock.Now().Add(5 * time.Minute))
	_, err = env.server.Auth().CreateUserToken(ctx, startToken)
	require.NoError(t, err)

	tests := []struct {
		name    string
		clt     *TestWebClient
		ep      []string
		reqBody any
	}{
		{
			name: "/webapi/mfa/authenticatechallenge/password",
			clt:  authnClt,
			ep:   []string{"webapi", "mfa", "authenticatechallenge", "password"},
			reqBody: client.MFAChallengeRequest{
				Pass: authPack.password,
			},
		},
		{
			name: "/webapi/mfa/login/begin",
			clt:  publicClt,
			ep:   []string{"webapi", "mfa", "login", "begin"},
			reqBody: client.MFAChallengeRequest{
				User: authPack.user,
				Pass: authPack.password,
			},
		},
		{
			name: "/webapi/mfa/authenticatechallenge",
			clt:  authnClt,
			ep:   []string{"webapi", "mfa", "authenticatechallenge"},
			reqBody: CreateAuthenticateChallengeRequest{
				ChallengeScope: int(mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN),
			},
		},
		{
			name: "/webapi/mfa/token/:token/authenticatechallenge",
			clt:  publicClt,
			ep:   []string{"webapi", "mfa", "token", startToken.GetName(), "authenticatechallenge"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			endpoint := tc.clt.Endpoint(tc.ep...)
			res, err := tc.clt.PostJSON(ctx, endpoint, tc.reqBody)
			require.NoError(t, err)

			var chal client.MFAAuthenticateChallenge
			err = json.Unmarshal(res.Bytes(), &chal)
			require.NoError(t, err)
			require.True(t, chal.TOTPChallenge)
			require.Empty(t, chal.WebauthnChallenge)
		})
	}
}

func TestCreateRegisterChallenge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	clt := proxy.newClient(t)

	// Enable second factor.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: env.server.ClusterName(),
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAuthPreference(ctx, ap)
	require.NoError(t, err)

	// Acquire an accepted token.
	token, err := types.NewUserToken("some-token-id")
	require.NoError(t, err)
	token.SetUser("llama")
	token.SetSubKind(authclient.UserTokenTypePrivilege)
	token.SetExpiry(env.clock.Now().Add(5 * time.Minute))
	_, err = env.server.Auth().CreateUserToken(ctx, token)
	require.NoError(t, err)

	tests := []struct {
		name            string
		req             *createRegisterChallengeWithTokenRequest
		assertChallenge func(t *testing.T, c *client.MFARegisterChallenge)
	}{
		{
			name: "totp",
			req: &createRegisterChallengeWithTokenRequest{
				DeviceType: "totp",
			},
		},
		{
			name: "webauthn",
			req: &createRegisterChallengeWithTokenRequest{
				DeviceType: "webauthn",
			},
		},
		{
			name: "passwordless",
			req: &createRegisterChallengeWithTokenRequest{
				DeviceType:  "webauthn",
				DeviceUsage: "passwordless",
			},
			assertChallenge: func(t *testing.T, c *client.MFARegisterChallenge) {
				// rrk=true is a good proxy for passwordless.
				require.NotNil(t, c.Webauthn.Response.AuthenticatorSelection.RequireResidentKey, "rrk cannot be nil")
				require.True(t, *c.Webauthn.Response.AuthenticatorSelection.RequireResidentKey, "rrk cannot be false")
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			endpoint := clt.Endpoint("webapi", "mfa", "token", token.GetName(), "registerchallenge")
			res, err := clt.PostJSON(ctx, endpoint, tc.req)
			require.NoError(t, err)

			var chal client.MFARegisterChallenge
			require.NoError(t, json.Unmarshal(res.Bytes(), &chal))

			switch tc.req.DeviceType {
			case "totp":
				require.NotNil(t, chal.TOTP.QRCode, "TOTP QR code cannot be nil")
			case "webauthn":
				require.NotNil(t, chal.Webauthn, "WebAuthn challenge cannot be nil")
			}

			if tc.assertChallenge != nil {
				tc.assertChallenge(t, &chal)
			}
		})
	}
}

// TestCreateAppSession verifies that an existing session to the Web UI can
// be exchanged for an application specific session.
func TestCreateAppSession(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	pack := s.authPack(t, "foo@example.com")

	// Register an application called "panel".
	app, err := types.NewAppV3(types.Metadata{
		Name: "panel",
	}, types.AppSpecV3{
		URI:        "http://127.0.0.1:8080",
		PublicAddr: "panel.example.com",
	})
	require.NoError(t, err)
	server, err := types.NewAppServerV3FromApp(app, "host", uuid.New().String())
	require.NoError(t, err)
	_, err = s.server.Auth().UpsertApplicationServer(s.ctx, server)
	require.NoError(t, err)

	// Extract the session ID and bearer token for the current session.
	rawCookie := *pack.cookies[0]
	cookieBytes, err := hex.DecodeString(rawCookie.Value)
	require.NoError(t, err)
	var sessionCookie websession.Cookie
	err = json.Unmarshal(cookieBytes, &sessionCookie)
	require.NoError(t, err)

	tests := []struct {
		name            string
		inCreateRequest *CreateAppSessionRequest
		outError        require.ErrorAssertionFunc
		outFQDN         string
		outUsername     string
	}{
		{
			name: "Valid request: all fields",
			inCreateRequest: &CreateAppSessionRequest{
				ResolveAppParams: ResolveAppParams{
					FQDNHint:    "panel.example.com",
					PublicAddr:  "panel.example.com",
					ClusterName: "localhost",
				},
			},
			outError:    require.NoError,
			outFQDN:     "panel.example.com",
			outUsername: "foo@example.com",
		},
		{
			name: "Valid request: without FQDN",
			inCreateRequest: &CreateAppSessionRequest{
				ResolveAppParams: ResolveAppParams{
					PublicAddr:  "panel.example.com",
					ClusterName: "localhost",
				},
			},
			outError:    require.NoError,
			outFQDN:     "panel.example.com",
			outUsername: "foo@example.com",
		},
		{
			name: "Valid request: only FQDN",
			inCreateRequest: &CreateAppSessionRequest{
				ResolveAppParams: ResolveAppParams{
					FQDNHint: "panel.example.com",
				},
			},
			outError:    require.NoError,
			outFQDN:     "panel.example.com",
			outUsername: "foo@example.com",
		},
		{
			name: "Invalid request: only public address",
			inCreateRequest: &CreateAppSessionRequest{
				ResolveAppParams: ResolveAppParams{
					PublicAddr: "panel.example.com",
				},
			},
			outError: require.Error,
		},
		{
			name: "Invalid request: only cluster name",
			inCreateRequest: &CreateAppSessionRequest{
				ResolveAppParams: ResolveAppParams{
					ClusterName: "localhost",
				},
			},
			outError: require.Error,
		},
		{
			name: "Invalid application",
			inCreateRequest: &CreateAppSessionRequest{
				ResolveAppParams: ResolveAppParams{
					FQDNHint:    "panel.example.com",
					PublicAddr:  "invalid.example.com",
					ClusterName: "localhost",
				},
			},
			outError: require.Error,
		},
		{
			name: "Invalid cluster name",
			inCreateRequest: &CreateAppSessionRequest{
				ResolveAppParams: ResolveAppParams{
					FQDNHint:    "panel.example.com",
					PublicAddr:  "panel.example.com",
					ClusterName: "example.com",
				},
			},
			outError: require.Error,
		},
		{
			name: "Malicious request: all fields",
			inCreateRequest: &CreateAppSessionRequest{
				ResolveAppParams: ResolveAppParams{
					FQDNHint:    "panel.example.com@malicious.com",
					PublicAddr:  "panel.example.com",
					ClusterName: "localhost",
				},
			},
			outError:    require.NoError,
			outFQDN:     "panel.example.com",
			outUsername: "foo@example.com",
		},
		{
			name: "Malicious request: only FQDN",
			inCreateRequest: &CreateAppSessionRequest{
				ResolveAppParams: ResolveAppParams{
					FQDNHint: "panel.example.com@malicious.com",
				},
			},
			outError: require.Error,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Make a request to create an application session for "panel".
			endpoint := pack.clt.Endpoint("webapi", "sessions", "app")
			resp, err := pack.clt.PostJSON(s.ctx, endpoint, tt.inCreateRequest)
			tt.outError(t, err)
			if err != nil {
				return
			}

			// Unmarshal the response.
			var response *CreateAppSessionResponse
			require.NoError(t, json.Unmarshal(resp.Bytes(), &response))
			require.Equal(t, tt.outFQDN, response.FQDN)

			// Verify that the application session was created.
			sess, err := s.server.Auth().GetAppSession(s.ctx, types.GetAppSessionRequest{
				SessionID: response.CookieValue,
			})
			require.NoError(t, err)
			require.Equal(t, tt.outUsername, sess.GetUser())
			require.NotEmpty(t, response.CookieValue)
			require.Equal(t, response.CookieValue, sess.GetName())
			require.NotEmpty(t, response.SubjectCookieValue, "every session should create a secret token")
			require.Equal(t, response.SubjectCookieValue, sess.GetBearerToken())
		})
	}
}

func TestCreateAppSessionHealthCheckAppServer(t *testing.T) {
	t.Parallel()

	validApp, err := types.NewAppV3(types.Metadata{
		Name: "valid",
	}, types.AppSpecV3{
		URI:        "http://127.0.0.1:8080",
		PublicAddr: "valid.example.com",
	})
	require.NoError(t, err)

	invalidApp, err := types.NewAppV3(types.Metadata{
		Name: "invalid",
	}, types.AppSpecV3{
		URI:        "http://127.0.0.1:8080",
		PublicAddr: "invalid.example.com",
	})
	require.NoError(t, err)

	s := newWebSuiteWithConfig(t, webSuiteConfig{
		HealthCheckAppServer: func(_ context.Context, publicAddr string, _ string) error {
			// Can only serve "validApp".
			if publicAddr == validApp.GetPublicAddr() {
				return nil
			}

			return trace.ConnectionProblem(nil, "offline AppServer")
		},
	})

	for _, app := range []*types.AppV3{validApp, invalidApp} {
		server, err := types.NewAppServerV3FromApp(app, "host", uuid.New().String())
		require.NoError(t, err)
		_, err = s.server.Auth().UpsertApplicationServer(s.ctx, server)
		require.NoError(t, err)
	}

	pack := s.authPack(t, "foo@example.com")
	rawCookie := *pack.cookies[0]
	cookieBytes, err := hex.DecodeString(rawCookie.Value)
	require.NoError(t, err)
	var sessionCookie websession.Cookie
	err = json.Unmarshal(cookieBytes, &sessionCookie)
	require.NoError(t, err)

	for _, tc := range []struct {
		desc       string
		publicAddr string
		expectErr  require.ErrorAssertionFunc
	}{
		{
			desc:       "request to application that can be served",
			publicAddr: validApp.GetPublicAddr(),
			expectErr:  require.NoError,
		},
		{
			desc:       "request to application that cannot be served",
			publicAddr: invalidApp.GetPublicAddr(),
			expectErr:  require.Error,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			endpoint := pack.clt.Endpoint("webapi", "sessions", "app")
			_, err := pack.clt.PostJSON(s.ctx, endpoint, &CreateAppSessionRequest{
				ResolveAppParams: ResolveAppParams{
					FQDNHint: tc.publicAddr,
				},
			})
			tc.expectErr(t, err)
		})
	}
}

func TestCreateAppSession_RequireSessionMFA(t *testing.T) {
	ctx := context.Background()
	t.Parallel()
	s := newWebSuite(t)
	pack := s.authPack(t, "foo@example.com")

	// Register an application called "panel".
	app, err := types.NewAppV3(types.Metadata{
		Name: "panel",
	}, types.AppSpecV3{
		URI:        "http://127.0.0.1:8080",
		PublicAddr: "panel.example.com",
	})
	require.NoError(t, err)
	server, err := types.NewAppServerV3FromApp(app, "host", uuid.New().String())
	require.NoError(t, err)
	_, err = s.server.Auth().UpsertApplicationServer(s.ctx, server)
	require.NoError(t, err)

	// Enable per session MFA.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorWebauthn,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
		RequireMFAType: types.RequireMFAType_SESSION,
	})
	require.NoError(t, err)
	_, err = s.server.Auth().UpsertAuthPreference(ctx, ap)
	require.NoError(t, err)

	// register an mfa device for the user.
	userClient, err := s.server.NewClient(auth.TestUser(pack.user))
	require.NoError(t, err)
	webauthnDev, err := auth.RegisterTestDevice(
		ctx,
		userClient,
		"webauthn", authproto.DeviceType_DEVICE_TYPE_WEBAUTHN, pack.device /* authenticator */)
	require.NoError(t, err)

	// Prepare a valid mfa response for the user.
	chal, err := userClient.CreateAuthenticateChallenge(ctx, &authproto.CreateAuthenticateChallengeRequest{
		Request: &authproto.CreateAuthenticateChallengeRequest_ContextUser{},
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
		},
	})
	require.NoError(t, err)
	mfaResp, err := webauthnDev.SolveAuthn(chal)
	require.NoError(t, err)

	// Extract the session ID and bearer token for the current session.
	rawCookie := *pack.cookies[0]
	cookieBytes, err := hex.DecodeString(rawCookie.Value)
	require.NoError(t, err)
	var sessionCookie websession.Cookie
	err = json.Unmarshal(cookieBytes, &sessionCookie)
	require.NoError(t, err)

	tests := []struct {
		name              string
		inCreateRequest   *CreateAppSessionRequest
		expectMFAVerified bool
	}{
		{
			name: "NOK MFA not provided",
			inCreateRequest: &CreateAppSessionRequest{
				ResolveAppParams: ResolveAppParams{
					FQDNHint:    "panel.example.com",
					PublicAddr:  "panel.example.com",
					ClusterName: "localhost",
				},
			},
			expectMFAVerified: false,
		},
		{
			name: "OK MFA provided",
			inCreateRequest: &CreateAppSessionRequest{
				ResolveAppParams: ResolveAppParams{
					FQDNHint:    "panel.example.com",
					PublicAddr:  "panel.example.com",
					ClusterName: "localhost",
				},
				MFAResponse: client.MFAChallengeResponse{
					WebauthnAssertionResponse: wantypes.CredentialAssertionResponseFromProto(mfaResp.GetWebauthn()),
				},
			},
			expectMFAVerified: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Make a request to create an application session for "panel".
			endpoint := pack.clt.Endpoint("webapi", "sessions", "app")
			resp, err := pack.clt.PostJSON(s.ctx, endpoint, tt.inCreateRequest)
			require.NoError(t, err)

			// Unmarshal the response.
			var response *CreateAppSessionResponse
			require.NoError(t, json.Unmarshal(resp.Bytes(), &response))

			// Verify that the application session was created.
			sess, err := s.server.Auth().GetAppSession(s.ctx, types.GetAppSessionRequest{
				SessionID: response.CookieValue,
			})
			require.NoError(t, err)

			// Verify that the session is MFA verified
			certificate, err := tlsca.ParseCertificatePEM(sess.GetTLSCert())
			require.NoError(t, err)
			identity, err := tlsca.FromSubject(certificate.Subject, certificate.NotAfter)
			require.NoError(t, err)

			if tt.expectMFAVerified {
				require.NotEmpty(t, identity.MFAVerified, "expected app session to be MFA verified")
			} else {
				require.Empty(t, identity.MFAVerified, "expected app session to not be MFA verified")
			}
		})
	}
}

func TestNewSessionResponseWithRenewSession(t *testing.T) {
	t.Parallel()
	env := newWebPack(t, 1)

	// Set a web idle timeout.
	duration := time.Duration(5) * time.Minute
	cfg := types.DefaultClusterNetworkingConfig()
	cfg.SetWebIdleTimeout(duration)
	_, err := env.server.Auth().UpsertClusterNetworkingConfig(context.Background(), cfg)
	require.NoError(t, err)

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "foo", nil /* roles */)

	assertSession := func(t *testing.T, session *CreateSessionResponse) {
		t.Helper()

		assert.Equal(t, roundtrip.AuthBearer, session.TokenType, "token type mismatch")
		assert.NotEmpty(t, session.Token, "token")
		assert.NotZero(t, session.TokenExpiresIn, "expires_in")
		assert.GreaterOrEqual(t, session.SessionExpiresIn, session.TokenExpiresIn, "sessionExpiresIn must be >= expires_in")
		assert.NotZero(t, session.SessionExpires, "sessionExpires")
		assert.Equal(t, int(duration.Milliseconds()), session.SessionInactiveTimeoutMS, "sessionInactiveTimeout")
	}

	t.Run("pack.session", func(t *testing.T) {
		assertSession(t, pack.session)
	})

	t.Run("renew", func(t *testing.T) {
		resp := pack.renewSession(context.Background(), t)

		session := &CreateSessionResponse{}
		require.NoError(t, json.Unmarshal(resp.Bytes(), &session), "unmarshal renewed session")
		assertSession(t, session)
	})
}

// TestWebSessionsRenewDoesNotBreakExistingTerminalSession validates that the
// session renewed via one proxy does not force the terminals created by another
// proxy to disconnect
//
// See https://github.com/gravitational/teleport/issues/5265
func TestWebSessionsRenewDoesNotBreakExistingTerminalSession(t *testing.T) {
	env := newWebPack(t, 2)

	proxy1, proxy2 := env.proxies[0], env.proxies[1]
	// Connect to both proxies
	pack1 := proxy1.authPack(t, "foo", nil /* roles */)
	pack2 := proxy2.authPackFromPack(t, pack1)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	term, err := connectToHost(ctx, connectConfig{
		pack:  pack2,
		host:  proxy2.node.ID(),
		proxy: proxy2.webURL.Host,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, term.Close()) })

	// Advance the time before renewing the session.
	// This will allow the new session to have a more plausible
	// expiration
	const delta = 30 * time.Second
	env.clock.Advance(defaults.BearerTokenTTL - delta)

	// Renew the session using the 1st proxy
	resp := pack1.renewSession(context.Background(), t)

	// Expire the old session and make sure it has been removed.
	// The bearer token is also removed after this point, so we have to
	// use the new session data for future connects
	env.clock.Advance(delta + 1*time.Second)
	pack2 = proxy2.authPackFromResponse(t, resp)

	// Verify that access via the 2nd proxy also works for the same session
	pack2.validateAPI(context.Background(), t)

	// Check whether the terminal session is still active
	validateTerminal(t, term)
}

// TestWebSessionsRenewAllowsOldBearerTokenToLinger validates that the
// bearer token bound to the previous session is still active after the
// session renewal, if the renewal happens with a time margin.
//
// See https://github.com/gravitational/teleport/issues/5265
func TestWebSessionsRenewAllowsOldBearerTokenToLinger(t *testing.T) {
	// Login to implicitly create a new web session
	env := newWebPack(t, 1)

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "foo", nil /* roles */)

	delta := 30 * time.Second
	// Advance the time before renewing the session.
	// This will allow the new session to have a more plausible
	// expiration
	env.clock.Advance(defaults.BearerTokenTTL - delta)

	// make sure we can use client to make authenticated requests
	// before we issue this request, we will recover session id and bearer token
	//
	prevSessionCookie := *pack.cookies[0]
	prevBearerToken := pack.session.Token
	resp := pack.renewSession(context.Background(), t)

	newPack := proxy.authPackFromResponse(t, resp)

	// new session is functioning
	newPack.validateAPI(context.Background(), t)

	sessionCookie := *newPack.cookies[0]
	bearerToken := newPack.session.Token
	require.NotEmpty(t, bearerToken)
	require.NotEmpty(t, cmp.Diff(bearerToken, prevBearerToken))

	prevSessionID := decodeSessionCookie(t, prevSessionCookie.Value)
	activeSessionID := decodeSessionCookie(t, sessionCookie.Value)
	require.NotEmpty(t, cmp.Diff(prevSessionID, activeSessionID))

	// old session is still valid
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	oldClt := proxy.newClient(t, roundtrip.BearerAuth(prevBearerToken), roundtrip.CookieJar(jar))
	jar.SetCookies(&proxy.webURL, []*http.Cookie{&prevSessionCookie})
	_, err = oldClt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites"), url.Values{})
	require.NoError(t, err)

	// now expire the old session and make sure it has been removed
	env.clock.Advance(delta)

	_, err = proxy.client.GetWebSession(context.Background(), types.GetWebSessionRequest{
		User:      "foo",
		SessionID: prevSessionID,
	})
	require.Regexp(t, "^key.*not found$", err.Error())

	// now delete session
	_, err = newPack.clt.Delete(
		context.Background(),
		pack.clt.Endpoint("webapi", "sessions", "web"))
	require.NoError(t, err)

	// subsequent requests to use this session will fail
	_, err = newPack.clt.Get(context.Background(), pack.clt.Endpoint("webapi", "sites"), url.Values{})
	require.True(t, trace.IsAccessDenied(err))
}

// TestChangeUserAuthentication_recoveryCodesReturnedForCloud tests for following:
// - Recovery codes are not returned for usernames that are not emails
// - Recovery codes are returned for usernames that are valid emails
func TestChangeUserAuthentication_recoveryCodesReturnedForCloud(t *testing.T) {
	env := newWebPack(t, 1)
	ctx := context.Background()

	// Enable second factor.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAuthPreference(ctx, ap)
	require.NoError(t, err)

	// Enable cloud feature.
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			RecoveryCodes: true,
		},
	})

	// Creaet a username that is not a valid email format for recovery.
	teleUser, err := types.NewUser("invalid-name-for-recovery")
	require.NoError(t, err)
	_, err = env.server.Auth().CreateUser(ctx, teleUser)
	require.NoError(t, err)

	// Create a reset password token and secrets.
	resetToken, err := env.server.Auth().CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
		Name: "invalid-name-for-recovery",
	})
	require.NoError(t, err)
	res, err := env.server.Auth().CreateRegisterChallenge(ctx, &authproto.CreateRegisterChallengeRequest{
		TokenID:    resetToken.GetName(),
		DeviceType: authproto.DeviceType_DEVICE_TYPE_TOTP,
	})
	require.NoError(t, err)
	totpCode, err := totp.GenerateCode(res.GetTOTP().GetSecret(), env.clock.Now())
	require.NoError(t, err)

	// Test invalid username does not receive codes.
	clt := env.proxies[0].client
	re, err := clt.ChangeUserAuthentication(ctx, &authproto.ChangeUserAuthenticationRequest{
		TokenID:     resetToken.GetName(),
		NewPassword: []byte("abcdef123456"),
		NewMFARegisterResponse: &authproto.MFARegisterResponse{Response: &authproto.MFARegisterResponse_TOTP{
			TOTP: &authproto.TOTPRegisterResponse{Code: totpCode},
		}},
	})
	require.NoError(t, err)
	require.Nil(t, re.Recovery)
	require.False(t, re.PrivateKeyPolicyEnabled)

	// Create a user that is valid for recovery.
	teleUser, err = types.NewUser("valid-username@example.com")
	require.NoError(t, err)
	_, err = env.server.Auth().CreateUser(ctx, teleUser)
	require.NoError(t, err)

	// Create a reset password token and secrets.
	resetToken, err = env.server.Auth().CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
		Name: "valid-username@example.com",
	})
	require.NoError(t, err)
	res, err = env.server.Auth().CreateRegisterChallenge(ctx, &authproto.CreateRegisterChallengeRequest{
		TokenID:    resetToken.GetName(),
		DeviceType: authproto.DeviceType_DEVICE_TYPE_TOTP,
	})
	require.NoError(t, err)
	totpCode, err = totp.GenerateCode(res.GetTOTP().GetSecret(), env.clock.Now())
	require.NoError(t, err)

	// Test valid username (email) returns codes.
	re, err = clt.ChangeUserAuthentication(ctx, &authproto.ChangeUserAuthenticationRequest{
		TokenID:     resetToken.GetName(),
		NewPassword: []byte("abcdef123456"),
		NewMFARegisterResponse: &authproto.MFARegisterResponse{Response: &authproto.MFARegisterResponse_TOTP{
			TOTP: &authproto.TOTPRegisterResponse{Code: totpCode},
		}},
	})
	require.NoError(t, err)
	require.Len(t, re.Recovery.Codes, 3)
	require.NotEmpty(t, re.Recovery.Created)
	require.False(t, re.PrivateKeyPolicyEnabled)
}

// TestChangeUserAuthentication_WithPrivacyPolicyEnabledError tests
// that when there is a privacy policy enabled error, we still get
// a non error response with recovery codes and a privacy policy
// flag set to true.
func TestChangeUserAuthentication_WithPrivacyPolicyEnabledError(t *testing.T) {
	env := newWebPack(t, 1)
	ctx := context.Background()

	// Enable second factor required by cloud and a privacy policy.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:           constants.Local,
		SecondFactor:   constants.SecondFactorOTP,
		RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAuthPreference(ctx, ap)
	require.NoError(t, err)

	// Enable cloud feature.
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			RecoveryCodes: true,
		},
		MockAttestationData: &keys.AttestationData{
			PrivateKeyPolicy: keys.PrivateKeyPolicyNone,
		},
	})

	// Create a user that is valid for recovery.
	teleUser, err := types.NewUser("valid-username@example.com")
	require.NoError(t, err)
	_, err = env.server.Auth().CreateUser(ctx, teleUser)
	require.NoError(t, err)

	// Create a reset password token and secrets.
	resetToken, err := env.server.Auth().CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
		Name: "valid-username@example.com",
	})
	require.NoError(t, err)
	res, err := env.server.Auth().CreateRegisterChallenge(ctx, &authproto.CreateRegisterChallengeRequest{
		TokenID:    resetToken.GetName(),
		DeviceType: authproto.DeviceType_DEVICE_TYPE_TOTP,
	})
	require.NoError(t, err)
	totpCode, err := totp.GenerateCode(res.GetTOTP().GetSecret(), env.clock.Now())
	require.NoError(t, err)

	// Craft http request data.
	clt := env.proxies[0].newClient(t)
	req := changeUserAuthenticationRequest{
		SecondFactorToken: totpCode,
		Password:          []byte("abcdef123456"),
		TokenID:           resetToken.GetName(),
	}
	httpReqData, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq, err := http.NewRequest("PUT", clt.Endpoint("webapi", "users", "password", "token"), bytes.NewBuffer(httpReqData))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")
	httpRes, err := httplib.ConvertResponse(clt.RoundTrip(func() (*http.Response, error) {
		return clt.HTTPClient().Do(httpReq)
	}))
	require.NoError(t, err)

	var apiRes webui.ChangedUserAuthn
	require.NoError(t, json.Unmarshal(httpRes.Bytes(), &apiRes))
	require.Len(t, apiRes.Recovery.Codes, 3)
	require.NotEmpty(t, apiRes.Recovery.Created)
	require.True(t, apiRes.PrivateKeyPolicyEnabled)
}

func TestChangeUserAuthentication_settingDefaultClusterAuthPreference(t *testing.T) {
	tt := []struct {
		name                 string
		cloud                bool
		numberOfUsers        int
		password             []byte
		authPreferenceType   string
		initialConnectorName string
		resultConnectorName  string
	}{{
		name:                 "first cloud sign-in changes connector to passwordless",
		cloud:                true,
		numberOfUsers:        1,
		authPreferenceType:   constants.Local,
		initialConnectorName: "",
		resultConnectorName:  constants.PasswordlessConnector,
	}, {
		name:                 "first non-cloud sign-in doesn't change the connector",
		cloud:                false,
		numberOfUsers:        1,
		authPreferenceType:   constants.Local,
		initialConnectorName: "",
		resultConnectorName:  "",
	}, {
		name:                 "second cloud sign-in doesn't change the connector",
		cloud:                true,
		numberOfUsers:        2,
		authPreferenceType:   constants.Local,
		initialConnectorName: "",
		resultConnectorName:  "",
	}, {
		name:                 "first cloud sign-in does not change custom connector",
		cloud:                true,
		numberOfUsers:        1,
		authPreferenceType:   constants.OIDC,
		initialConnectorName: "custom",
		resultConnectorName:  "custom",
	}, {
		name:                 "first cloud sign-in with password does not change connector",
		cloud:                true,
		numberOfUsers:        1,
		password:             []byte("abcdef123456"),
		authPreferenceType:   constants.Local,
		initialConnectorName: "",
		resultConnectorName:  "",
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			modules.SetTestModules(t, &modules.TestModules{
				TestFeatures: modules.Features{
					Cloud: tc.cloud,
				},
			})

			const RPID = "localhost"

			s := newWebSuiteWithConfig(t, webSuiteConfig{
				authPreferenceSpec: &types.AuthPreferenceSpecV2{
					Type:          tc.authPreferenceType,
					ConnectorName: tc.initialConnectorName,
					SecondFactor:  constants.SecondFactorOn,
					Webauthn: &types.Webauthn{
						RPID: RPID,
					},
				},
			})

			// user and role
			users := make([]types.User, tc.numberOfUsers)

			for i := 0; i < tc.numberOfUsers; i++ {
				user, err := types.NewUser(fmt.Sprintf("test_user_%v", i))
				require.NoError(t, err)

				user.SetCreatedBy(types.CreatedBy{
					User: types.UserRef{Name: "other_user"},
				})

				role := services.RoleForUser(user)

				role, err = s.server.Auth().UpsertRole(s.ctx, role)
				require.NoError(t, err)

				user.AddRole(role.GetName())

				user, err = s.server.Auth().CreateUser(s.ctx, user)
				require.NoError(t, err)

				users[i] = user
			}

			initialUser := users[0]

			clt := s.client(t)

			// create register challenge
			token, err := s.server.Auth().CreateResetPasswordToken(s.ctx, authclient.CreateUserTokenRequest{
				Name: initialUser.GetName(),
			})
			require.NoError(t, err)

			res, err := s.server.Auth().CreateRegisterChallenge(s.ctx, &authproto.CreateRegisterChallengeRequest{
				TokenID:     token.GetName(),
				DeviceType:  authproto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				DeviceUsage: authproto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
			})
			require.NoError(t, err)

			cc := wantypes.CredentialCreationFromProto(res.GetWebauthn())

			// use passwordless as auth method
			device, err := mocku2f.Create()
			require.NoError(t, err)

			device.SetPasswordless()

			ccr, err := device.SignCredentialCreation("https://"+RPID, cc)
			require.NoError(t, err)

			// send sign-in response to server
			body, err := json.Marshal(changeUserAuthenticationRequest{
				WebauthnCreationResponse: ccr,
				TokenID:                  token.GetName(),
				DeviceName:               "passwordless-device",
				Password:                 tc.password,
			})
			require.NoError(t, err)

			req, err := http.NewRequest("PUT", clt.Endpoint("webapi", "users", "password", "token"), bytes.NewBuffer(body))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")

			re, err := clt.Client.RoundTrip(func() (*http.Response, error) {
				return clt.Client.HTTPClient().Do(req)
			})

			require.NoError(t, err)
			require.Equal(t, http.StatusOK, re.Code())

			// check if auth preference connectorName is set
			authPreference, err := s.server.Auth().GetAuthPreference(s.ctx)
			require.NoError(t, err)

			require.Equal(t, tc.resultConnectorName, authPreference.GetConnectorName(), "Found unexpected auth connector name")
		})
	}
}

func TestParseSSORequestParams(t *testing.T) {
	t.Parallel()

	token := "someMeaninglessTokenString"

	tests := []struct {
		name, url string
		wantErr   bool
		expected  *SSORequestParams
	}{
		{
			name: "preserve redirect's query params (escaped)",
			url:  "https://localhost/login?connector_id=oidc&redirect_url=https:%2F%2Flocalhost:8080%2Fweb%2Fcluster%2Fim-a-cluster-name%2Fnodes%3Fsearch=tunnel&sort=hostname:asc",
			expected: &SSORequestParams{
				ClientRedirectURL: "https://localhost:8080/web/cluster/im-a-cluster-name/nodes?search=tunnel&sort=hostname:asc",
				ConnectorID:       "oidc",
				CSRFToken:         token,
			},
		},
		{
			name: "preserve redirect's query params (unescaped)",
			url:  "https://localhost/login?connector_id=github&redirect_url=https://localhost:8080/web/cluster/im-a-cluster-name/nodes?search=tunnel&sort=hostname:asc",
			expected: &SSORequestParams{
				ClientRedirectURL: "https://localhost:8080/web/cluster/im-a-cluster-name/nodes?search=tunnel&sort=hostname:asc",
				ConnectorID:       "github",
				CSRFToken:         token,
			},
		},
		{
			name: "preserve various encoded chars",
			url:  "https://localhost/login?connector_id=saml&redirect_url=https:%2F%2Flocalhost:8080%2Fweb%2Fcluster%2Fim-a-cluster-name%2Fapps%3Fquery=search(%2522watermelon%2522%252C%2520%2522this%2522)%2520%2526%2526%2520labels%255B%2522unique-id%2522%255D%2520%253D%253D%2520%2522hi%2522&sort=name:asc",
			expected: &SSORequestParams{
				ClientRedirectURL: "https://localhost:8080/web/cluster/im-a-cluster-name/apps?query=search(%22watermelon%22%2C%20%22this%22)%20%26%26%20labels%5B%22unique-id%22%5D%20%3D%3D%20%22hi%22&sort=name:asc",
				ConnectorID:       "saml",
				CSRFToken:         token,
			},
		},
		{
			name:    "invalid redirect_url query param",
			url:     "https://localhost/login?redirect=https://localhost/nodes&connector_id=oidc",
			wantErr: true,
		},
		{
			name:    "invalid connector_id query param",
			url:     "https://localhost/login?redirect_url=https://localhost/nodes&connector=oidc",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest("", tc.url, nil)
			require.NoError(t, err)

			req.AddCookie(&http.Cookie{
				Name:  csrf.CookieName,
				Value: token,
			})

			params, err := ParseSSORequestParams(req)

			switch {
			case tc.wantErr:
				require.Error(t, err)
			default:
				require.NoError(t, err)
				require.Equal(t, tc.expected, params)
			}
		})
	}
}

func TestClusterDesktopsGet(t *testing.T) {
	t.Parallel()
	env := newWebPack(t, 1)

	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test-user@example.com", nil /* roles */)

	type testResponse struct {
		Items      []webui.Desktop `json:"items"`
		TotalCount int             `json:"totalCount"`
	}

	// Add a few desktops.
	resource, err := types.NewWindowsDesktopV3("desktop1", map[string]string{"test-field": "test-value"}, types.WindowsDesktopSpecV3{
		Addr:   "addr:3389", // test stripping off rdp port
		HostID: "host",
	})
	require.NoError(t, err)
	resource2, err := types.NewWindowsDesktopV3("desktop2", map[string]string{"test-field": "test-value2"}, types.WindowsDesktopSpecV3{
		Addr:   "addr",
		HostID: "host",
	})
	require.NoError(t, err)

	err = env.server.Auth().UpsertWindowsDesktop(context.Background(), resource)
	require.NoError(t, err)
	err = env.server.Auth().UpsertWindowsDesktop(context.Background(), resource2)
	require.NoError(t, err)

	// Make the call.
	query := url.Values{"sort": []string{"name"}}
	endpoint := pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "desktops")
	re, err := pack.clt.Get(context.Background(), endpoint, query)
	require.NoError(t, err)

	// Test correct response.
	resp := testResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.Len(t, resp.Items, 2)
	require.Equal(t, 2, resp.TotalCount)
	require.ElementsMatch(t, resp.Items, []webui.Desktop{{
		Kind:   types.KindWindowsDesktop,
		OS:     constants.WindowsOS,
		Name:   "desktop1",
		Addr:   "addr",
		Labels: []ui.Label{{Name: "test-field", Value: "test-value"}},
		HostID: "host",
	}, {
		Kind:   types.KindWindowsDesktop,
		OS:     constants.WindowsOS,
		Name:   "desktop2",
		Addr:   "addr",
		Labels: []ui.Label{{Name: "test-field", Value: "test-value2"}},
		HostID: "host",
	}})
}

func TestDesktopActive(t *testing.T) {
	desktopName := "rickey-rock"
	env := newWebPack(t, 1)
	ctx := context.Background()

	role, err := types.NewRole("admin", types.RoleSpecV6{
		Allow: types.RoleConditions{
			WindowsDesktopLabels: types.Labels{"environment": []string{"dev"}},
		},
	})
	require.NoError(t, err)

	pack := env.proxies[0].authPack(t, "foo", []types.Role{role})

	check := func(match string) {
		resp, err := pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "desktops", desktopName, "active"), url.Values{})
		require.NoError(t, err)
		require.Contains(t, string(resp.Bytes()), match)
	}

	check("\"active\":false")
	desktop, err := types.NewWindowsDesktopV3(desktopName, map[string]string{"environment": "dev"}, types.WindowsDesktopSpecV3{
		Domain: "ad",
		Addr:   "foo",
		HostID: "bar",
	})
	require.NoError(t, err)
	err = env.server.Auth().CreateWindowsDesktop(ctx, desktop)
	require.NoError(t, err)
	tracker, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
		SessionID:   "foo",
		Kind:        string(types.WindowsDesktopSessionKind),
		State:       types.SessionState_SessionStateRunning,
		DesktopName: desktopName,
	})
	require.NoError(t, err)
	_, err = env.server.Auth().CreateSessionTracker(ctx, tracker)
	require.NoError(t, err)
	check("\"active\":true")
}

func TestGetUserOrResetToken(t *testing.T) {
	env := newWebPack(t, 1)
	ctx := context.Background()
	username := "someuser"

	// Create a username.
	teleUser, err := types.NewUser(username)
	require.NoError(t, err)
	teleUser.SetLogins([]string{"login1"})
	_, err = env.server.Auth().CreateUser(ctx, teleUser)
	require.NoError(t, err)

	// Create a reset password token and secrets.
	resetToken, err := env.server.Auth().CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
		Name: username,
		Type: authclient.UserTokenTypeResetPasswordInvite,
	})
	require.NoError(t, err)

	pack := env.proxies[0].authPack(t, "foo", nil /* roles */)

	// the default roles of foo don't have users read but we need it on our tests
	fooRole, err := env.server.Auth().GetRole(ctx, "user:foo")
	require.NoError(t, err)
	fooAllowRules := fooRole.GetRules(types.Allow)
	fooAllowRules = append(fooAllowRules, types.NewRule(types.KindUser, services.RO()))
	fooRole.SetRules(types.Allow, fooAllowRules)
	_, err = env.server.Auth().UpsertRole(ctx, fooRole)
	require.NoError(t, err)

	resp, err := pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "users", username), url.Values{})
	require.NoError(t, err)
	require.Contains(t, string(resp.Bytes()), "login1")

	resp, err = pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "users", "password", "token", resetToken.GetName()), url.Values{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	_, err = pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "users", "password", "notToken", resetToken.GetName()), url.Values{})
	require.True(t, trace.IsNotFound(err))
}

func TestListConnectionsDiagnostic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	username := "someuser"
	diagName := "diag1"
	roleROConnectionDiagnostics, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindConnectionDiagnostic,
					[]string{types.VerbRead}),
			},
		},
	})
	require.NoError(t, err)

	env := newWebPack(t, 1)
	clusterName := env.server.ClusterName()
	pack := env.proxies[0].authPack(t, username, []types.Role{roleROConnectionDiagnostics})

	connectionsEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "diagnostics", "connections", diagName)

	// No connection diagnostics so far, should return not found
	_, err = pack.clt.Get(ctx, connectionsEndpoint, url.Values{})
	require.True(t, trace.IsNotFound(err))

	connectionDiagnostic, err := types.NewConnectionDiagnosticV1(diagName, map[string]string{}, types.ConnectionDiagnosticSpecV1{
		Success: true,
		Message: "success for cd0",
	})
	require.NoError(t, err)
	require.NoError(t, env.server.Auth().CreateConnectionDiagnostic(ctx, connectionDiagnostic))

	resp, err := pack.clt.Get(ctx, connectionsEndpoint, url.Values{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	var receivedConnectionDiagnostic webui.ConnectionDiagnostic
	require.NoError(t, json.Unmarshal(resp.Bytes(), &receivedConnectionDiagnostic))

	require.True(t, receivedConnectionDiagnostic.Success)
	require.Equal(t, diagName, receivedConnectionDiagnostic.ID)
	require.Equal(t, "success for cd0", receivedConnectionDiagnostic.Message)

	diag, err := env.server.Auth().GetConnectionDiagnostic(ctx, diagName)
	require.NoError(t, err)

	// Adding traces
	diag.AppendTrace(&types.ConnectionDiagnosticTrace{
		Type:    types.ConnectionDiagnosticTrace_RBAC_NODE,
		Status:  types.ConnectionDiagnosticTrace_SUCCESS,
		Details: "some details",
	})
	diag.SetMessage("after update")
	require.NoError(t, env.server.Auth().UpdateConnectionDiagnostic(ctx, diag))

	resp, err = pack.clt.Get(ctx, connectionsEndpoint, url.Values{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	require.NoError(t, json.Unmarshal(resp.Bytes(), &receivedConnectionDiagnostic))

	require.True(t, receivedConnectionDiagnostic.Success)
	require.Equal(t, diagName, receivedConnectionDiagnostic.ID)
	require.Equal(t, "after update", receivedConnectionDiagnostic.Message)
	require.Len(t, receivedConnectionDiagnostic.Traces, 1)
	require.NotNil(t, receivedConnectionDiagnostic.Traces[0])
	require.Equal(t, "some details", receivedConnectionDiagnostic.Traces[0].Details)
}

func TestDiagnoseSSHConnection(t *testing.T) {
	ctx := context.Background()

	osUser, err := user.Current()
	require.NoError(t, err)

	osUsername := osUser.Username
	require.NotEmpty(t, osUsername)

	roleWithFullAccess := func(username string, login string) []types.Role {
		ret, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
				NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				Rules: []types.Rule{
					types.NewRule(types.KindConnectionDiagnostic, services.RW()),
				},
				Logins: []string{login},
			},
		})
		require.NoError(t, err)
		return []types.Role{ret}
	}
	require.NotNil(t, roleWithFullAccess)

	rolesWithoutAccessToNode := func(username string, login string) []types.Role {
		ret, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
				NodeLabels: types.Labels{"forbidden": []string{"yes"}},
				Rules: []types.Rule{
					types.NewRule(types.KindConnectionDiagnostic, services.RW()),
				},
				Logins: []string{login},
			},
		})
		require.NoError(t, err)
		return []types.Role{ret}
	}
	require.NotNil(t, rolesWithoutAccessToNode)

	roleWithPrincipal := func(username string, principal string) []types.Role {
		ret, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
				NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				Rules: []types.Rule{
					types.NewRule(types.KindConnectionDiagnostic, services.RW()),
				},
				Logins: []string{principal},
			},
		})
		require.NoError(t, err)
		return []types.Role{ret}
	}
	require.NotNil(t, roleWithPrincipal)

	env := newWebPack(t, 1)
	nodeName := env.node.GetInfo().GetHostname()

	// Wait for node to show up
	require.Eventually(t, func() bool {
		_, err := env.server.Auth().GetNode(ctx, apidefaults.Namespace, nodeName)
		if trace.IsNotFound(err) {
			return false
		}
		assert.NoError(t, err, "GetNode returned an unexpected error")
		return true
	}, 5*time.Second, 250*time.Millisecond)

	for _, tt := range []struct {
		name            string
		teleportUser    string
		roles           []types.Role
		resourceName    string
		nodeUser        string
		nodeOS          string
		setupMethod     string
		stopNode        bool
		expectedSuccess bool
		expectedMessage string
		expectedTraces  []types.ConnectionDiagnosticTrace
	}{
		{
			name:            "success",
			roles:           roleWithFullAccess("success", osUsername),
			teleportUser:    "success",
			resourceName:    nodeName,
			nodeUser:        osUsername,
			expectedSuccess: true,
			expectedMessage: "success",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_NODE,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "You have access to the Node.",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Node is alive and reachable.",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "The requested principal is allowed.",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_NODE_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: fmt.Sprintf("%q user exists in target node", osUsername),
				},
			},
		},
		{
			name:            "Linux node not found",
			roles:           roleWithFullAccess("nodenotfound-linux", osUsername),
			teleportUser:    "nodenotfound-linux",
			resourceName:    "notanode",
			nodeUser:        osUsername,
			nodeOS:          constants.LinuxOS,
			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: `Failed to connect to the Node. Ensure teleport service is running using "systemctl status teleport".`,
					Error:   "direct dialing to nodes not found in inventory is not supported",
				},
			},
		},
		{
			name:            "Darwin node not found",
			roles:           roleWithFullAccess("nodenotfound-darwin", osUsername),
			teleportUser:    "nodenotfound-darwin",
			resourceName:    "notanode",
			nodeUser:        osUsername,
			nodeOS:          constants.DarwinOS,
			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: `Failed to connect to the Node. Ensure teleport service is running using "launchctl print 'system/Teleport Service'".`,
					Error:   "direct dialing to nodes not found in inventory is not supported",
				},
			},
		},
		{
			name:            "Connect My Computer node not found",
			roles:           roleWithFullAccess("nodenotfound-connect-my-computer", osUsername),
			teleportUser:    "nodenotfound-connect-my-computer",
			resourceName:    "notanode",
			nodeUser:        osUsername,
			nodeOS:          constants.DarwinOS,
			setupMethod:     conntest.SSHNodeSetupMethodConnectMyComputer,
			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: `Open the Connect My Computer tab in Teleport Connect and make sure that the agent is running.`,
					Error:   "direct dialing to nodes not found in inventory is not supported",
				},
			},
		},
		{
			name:            "node not reachable",
			teleportUser:    "nodenotreachable",
			roles:           roleWithFullAccess("nodenotreachable", osUsername),
			resourceName:    nodeName,
			nodeUser:        osUsername,
			stopNode:        true,
			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: `Failed to connect to the Node. Ensure teleport service is running using "systemctl status teleport".`,
					Error:   "Teleport proxy failed to connect to",
				},
			},
		},
		{
			name:            "no access to node",
			teleportUser:    "userwithoutaccess",
			roles:           rolesWithoutAccessToNode("userwithoutaccess", osUsername),
			resourceName:    nodeName,
			nodeUser:        osUsername,
			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_NODE,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: "You are not authorized to access this node. Ensure your role grants access by adding it to the 'node_labels' property.",
					Error:   fmt.Sprintf("user userwithoutaccess@localhost is not authorized to login as %s@localhost: access to node denied", osUsername),
				},
			},
		},
		{
			name:            "selected principal is not part of the allowed principals",
			teleportUser:    "deniedprincipal",
			roles:           roleWithFullAccess("deniedprincipal", "otherprincipal"),
			resourceName:    nodeName,
			nodeUser:        osUsername,
			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: `Principal "` + osUsername + `" is not allowed by this certificate. Ensure your roles grants access by adding it to the 'login' property.`,
					Error:   `ssh: principal "` + osUsername + `" not in the set of valid principals for given certificate: ["otherprincipal" "-teleport-internal-join"]`,
				},
			},
		},
		{
			name:            "principal does not exist in target host",
			teleportUser:    "principaldoesnotexist",
			roles:           roleWithPrincipal("principaldoesnotexist", "nonvalidlinuxuser"),
			resourceName:    nodeName,
			nodeUser:        "nonvalidlinuxuser",
			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_NODE_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: `Invalid user. Please ensure the principal "nonvalidlinuxuser" is a valid login in the target node. Output from Node: Failed to launch: user:`,
					Error:   "Process exited with status 255",
				},
			},
		},
		{
			name:            "principal does not exist in target Connect My Computer host",
			teleportUser:    "principaldoesnotexist-connect-my-computer",
			roles:           roleWithPrincipal("principaldoesnotexist-connect-my-computer", "nonvaliduser"),
			resourceName:    nodeName,
			nodeUser:        "nonvaliduser",
			setupMethod:     conntest.SSHNodeSetupMethodConnectMyComputer,
			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_NODE_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: `Invalid user`,
					Error:   `The role "connect-my-computer-principaldoesnotexist-connect-my-computer" includes only the login "nonvaliduser" and "nonvaliduser" is not a valid principal for this node`,
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			localEnv := env

			if tt.stopNode {
				localEnv = newWebPack(t, 1)
				require.NoError(t, localEnv.node.Close())
			}

			clusterName := localEnv.server.ClusterName()
			pack := localEnv.proxies[0].authPack(t, tt.teleportUser, tt.roles)

			createConnectionEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "diagnostics", "connections")

			resp, err := pack.clt.PostJSON(ctx, createConnectionEndpoint, conntest.TestConnectionRequest{
				ResourceKind:       types.KindNode,
				ResourceName:       tt.resourceName,
				SSHPrincipal:       tt.nodeUser,
				SSHNodeOS:          tt.nodeOS,
				SSHNodeSetupMethod: tt.setupMethod,
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())

			var connectionDiagnostic webui.ConnectionDiagnostic
			require.NoError(t, json.Unmarshal(resp.Bytes(), &connectionDiagnostic))

			gotFailedTraces := 0
			expectedFailedTraces := 0

			t.Log(tt.name)
			t.Log(connectionDiagnostic.Message, connectionDiagnostic.Success)
			for i, trace := range connectionDiagnostic.Traces {
				if trace.Status == types.ConnectionDiagnosticTrace_FAILED.String() {
					gotFailedTraces++
				}

				t.Logf("%d status='%s' type='%s' details='%s' error='%s'\n", i, trace.Status, trace.TraceType, trace.Details, trace.Error)
			}

			require.Equal(t, tt.expectedSuccess, connectionDiagnostic.Success)
			require.Equal(t, tt.expectedMessage, connectionDiagnostic.Message)

			for _, expectedTrace := range tt.expectedTraces {
				if expectedTrace.Status == types.ConnectionDiagnosticTrace_FAILED {
					expectedFailedTraces++
				}

				foundTrace := false
				for _, returnedTrace := range connectionDiagnostic.Traces {
					if expectedTrace.Type.String() != returnedTrace.TraceType {
						continue
					}

					foundTrace = true
					require.Equal(t, returnedTrace.Status, expectedTrace.Status.String())
					require.Contains(t, returnedTrace.Details, expectedTrace.Details)
					require.Contains(t, returnedTrace.Error, expectedTrace.Error)
				}

				require.True(t, foundTrace, "expected trace %v was not found", expectedTrace)
			}
			require.Equal(t, expectedFailedTraces, gotFailedTraces)
		})
	}

	// Test success with per-session MFA.

	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:           constants.Local,
		SecondFactor:   constants.SecondFactorOTP,
		RequireMFAType: types.RequireMFAType_SESSION,
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAuthPreference(ctx, ap)
	require.NoError(t, err)

	// Get a totp code to re-auth.
	pack := env.proxies[0].authPack(t, "llama", roleWithFullAccess("success", osUsername))
	totpCode, err := totp.GenerateCode(pack.otpSecret, env.clock.Now().Add(30*time.Second))
	require.NoError(t, err)

	clusterName := env.server.ClusterName()
	createConnectionEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "diagnostics", "connections")

	resp, err := pack.clt.PostJSON(ctx, createConnectionEndpoint, conntest.TestConnectionRequest{
		ResourceKind: types.KindNode,
		ResourceName: nodeName,
		SSHPrincipal: osUsername,
		MFAResponse:  client.MFAChallengeResponse{TOTPCode: totpCode},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	var connectionDiagnostic webui.ConnectionDiagnostic
	require.NoError(t, json.Unmarshal(resp.Bytes(), &connectionDiagnostic))
	require.True(t, connectionDiagnostic.Success)
}

func TestDiagnoseKubeConnection(t *testing.T) {
	var (
		validKubeUsers              = []string{}
		multiKubeUsers              = []string{"user1", "user2"}
		validKubeGroups             = []string{"validKubeGroup"}
		invalidKubeGroups           = []string{"invalidKubeGroups"}
		kubeClusterName             = "kube_cluster"
		disconnectedKubeClustername = "dis_kube_cluster"
		ctx                         = context.Background()
	)

	roleWithFullAccess := func(username string, kubeUsers, kubeGroups []string) []types.Role {
		ret, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:       []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				Rules: []types.Rule{
					types.NewRule(types.KindConnectionDiagnostic, services.RW()),
				},
				KubeGroups: kubeGroups,
				KubeUsers:  kubeUsers,
				KubernetesResources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: types.Wildcard,
						Name:      types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
				},
			},
		})
		require.NoError(t, err)
		return []types.Role{ret}
	}
	require.NotNil(t, roleWithFullAccess)

	rolesWithoutAccessToKubeCluster := func(username string, kubeUsers, kubeGroups []string) []types.Role {
		ret, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:       []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{"forbidden": []string{"yes"}},
				Rules: []types.Rule{
					types.NewRule(types.KindConnectionDiagnostic, services.RW()),
				},
				KubeGroups: kubeGroups,
				KubeUsers:  kubeUsers,
				KubernetesResources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: types.Wildcard,
						Name:      types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
				},
			},
		})
		require.NoError(t, err)
		return []types.Role{ret}
	}
	require.NotNil(t, rolesWithoutAccessToKubeCluster)

	env := newWebPack(t, 1)

	rt := http.NewServeMux()
	rt.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if slices.Contains(r.Header.Values("Impersonate-Group"), invalidKubeGroups[0]) {
			marshalRBACError(t, w)
			return
		}
		marshalValidPodList(t, w)
	})
	testKube := httptest.NewTLSServer(rt)

	t.Cleanup(func() {
		testKube.Close()
	})

	startKube(
		ctx,
		t,
		startKubeOptions{
			serviceType: kubeproxy.KubeService,
			authServer:  env.server.TLS,
			clusters: []kubeClusterConfig{
				{
					name:        kubeClusterName,
					apiEndpoint: testKube.URL,
				},
			},
		},
	)

	for _, tt := range []struct {
		name               string
		teleportUser       string
		roleFunc           func(string, []string, []string) []types.Role
		kubeUsers          []string
		kubeGroups         []string
		resourceName       string
		selectedKubeUser   string
		selectedKubeGroups []string
		expectedSuccess    bool
		disconnectedKube   bool
		expectedMessage    string
		expectedTraces     []types.ConnectionDiagnosticTrace
	}{
		{
			name:            "kube cluster not found",
			roleFunc:        roleWithFullAccess,
			kubeGroups:      validKubeGroups,
			kubeUsers:       validKubeUsers,
			teleportUser:    "notfound",
			resourceName:    "notregistered",
			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: `Failed to connect to Kubernetes cluster. Ensure the cluster is registered and online.`,
					Error:   "kubernetes cluster \"notregistered\" is not registered or is offline",
				},
			},
		},
		{
			name:             "kube cluster disconnected",
			roleFunc:         roleWithFullAccess,
			kubeGroups:       validKubeGroups,
			kubeUsers:        validKubeUsers,
			teleportUser:     "disconnected",
			resourceName:     disconnectedKubeClustername,
			disconnectedKube: true,
			expectedSuccess:  false,
			expectedMessage:  "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: `Failed to connect to Kubernetes cluster. Ensure the cluster is registered and online.`,
					Error:   fmt.Sprintf("kubernetes cluster %q is not registered or is offline", disconnectedKubeClustername),
				},
			},
		},
		{
			name:            "no access to kube cluster",
			teleportUser:    "userwithoutaccess",
			roleFunc:        rolesWithoutAccessToKubeCluster,
			kubeGroups:      validKubeGroups,
			kubeUsers:       validKubeUsers,
			resourceName:    kubeClusterName,
			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Kubernetes Cluster is registered in Teleport.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "User-associated roles define valid Kubernetes principals.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_KUBE,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: "You are not authorized to access this Kubernetes Cluster. Ensure your role grants access by adding it to the 'kubernetes_labels' property.",
					Error:   "[00] access denied",
				},
			},
		},
		{
			name:            "no kube principals",
			teleportUser:    "userwithoutprincipals",
			roleFunc:        roleWithFullAccess,
			kubeGroups:      nil,
			kubeUsers:       nil,
			resourceName:    kubeClusterName,
			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Kubernetes Cluster is registered in Teleport.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: "User-associated roles do not configure \"kubernetes_groups\" or \"kubernetes_users\". Make sure that at least one is configured for the user.",
					Error: "Your user's Teleport role does not allow Kubernetes access." +
						" Please ask cluster administrator to ensure your role has appropriate kubernetes_groups and kubernetes_users set.",
				},
			},
		},
		{
			name:            "teleport access but Kube RBAC fails",
			teleportUser:    "userbadrbac",
			roleFunc:        roleWithFullAccess,
			kubeGroups:      invalidKubeGroups,
			kubeUsers:       validKubeUsers,
			resourceName:    kubeClusterName,
			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Kubernetes Cluster is registered in Teleport.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "User-associated roles define valid Kubernetes principals.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_KUBE_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: "You are not allowed to list pods in the \"default\" namespace. Make sure your \"kubernetes_groups\" or \"kubernetes_users\" exist in the cluster and grant you access to list pods.",
					Error:   "pods is forbidden: User \"USER\" cannot list resource \"pods\" in API group \"\" in the namespace \"default\"",
				},
			},
		},
		{
			name:            "user with multiple defined kube_users",
			roleFunc:        roleWithFullAccess,
			kubeGroups:      validKubeGroups,
			kubeUsers:       multiKubeUsers,
			teleportUser:    "multiuser",
			resourceName:    kubeClusterName,
			expectedSuccess: false,
			expectedMessage: "failed",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Kubernetes Cluster is registered in Teleport.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: `User-associated roles define multiple "kubernetes_users". Make sure that only one value is defined or that you select the target user.`,
					Error:   "please select a user to impersonate, refusing to select a user due to several kubernetes_users set up for this user",
				},
			},
		},
		{
			name:             "user chose to impersonate invalid kube_users",
			roleFunc:         roleWithFullAccess,
			kubeGroups:       validKubeGroups,
			kubeUsers:        multiKubeUsers,
			teleportUser:     "userwithWrongImpUser",
			resourceName:     kubeClusterName,
			expectedSuccess:  false,
			expectedMessage:  "failed",
			selectedKubeUser: "missingUser",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Kubernetes Cluster is registered in Teleport.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: `User-associated roles do now allow the desired "kubernetes_user" impersonation. Please define a "kubernetes_user" that your roles allow to impersonate.`,
					Error:   `impersonation request has been denied, user header "missingUser" is not allowed in roles`,
				},
			},
		},
		{
			name:               "user chose to impersonate invalid kube_group",
			roleFunc:           roleWithFullAccess,
			kubeGroups:         validKubeGroups,
			kubeUsers:          multiKubeUsers,
			teleportUser:       "userwithWrongImpGroup",
			resourceName:       kubeClusterName,
			expectedSuccess:    false,
			expectedMessage:    "failed",
			selectedKubeUser:   "user1",
			selectedKubeGroups: []string{"missingGroup"},
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Kubernetes Cluster is registered in Teleport.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_FAILED,
					Details: `User-associated roles do now allow the desired "kubernetes_group" impersonation. Please define a "kubernetes_group" that your roles allow to impersonate.`,
					Error:   `impersonation request has been denied, group header "missingGroup" value is not allowed in roles`,
				},
			},
		},
		{
			name:            "user with multiple defined kube_users",
			roleFunc:        roleWithFullAccess,
			kubeGroups:      validKubeGroups,
			kubeUsers:       validKubeUsers,
			teleportUser:    "successwithmultiusers",
			resourceName:    kubeClusterName,
			expectedSuccess: true,
			expectedMessage: "success",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Kubernetes Cluster is registered in Teleport.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "User-associated roles define valid Kubernetes principals.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_KUBE,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "You are authorized to access this Kubernetes Cluster.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_KUBE_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Access to the Kubernetes Cluster granted.",
					Error:   "",
				},
			},
		},
		{
			name:            "success",
			roleFunc:        roleWithFullAccess,
			kubeGroups:      validKubeGroups,
			kubeUsers:       validKubeUsers,
			teleportUser:    "success",
			resourceName:    kubeClusterName,
			expectedSuccess: true,
			expectedMessage: "success",
			expectedTraces: []types.ConnectionDiagnosticTrace{
				{
					Type:    types.ConnectionDiagnosticTrace_CONNECTIVITY,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Kubernetes Cluster is registered in Teleport.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "User-associated roles define valid Kubernetes principals.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_RBAC_KUBE,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "You are authorized to access this Kubernetes Cluster.",
					Error:   "",
				},
				{
					Type:    types.ConnectionDiagnosticTrace_KUBE_PRINCIPAL,
					Status:  types.ConnectionDiagnosticTrace_SUCCESS,
					Details: "Access to the Kubernetes Cluster granted.",
					Error:   "",
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			localEnv := env

			if tt.disconnectedKube {
				kubeServer, cleanup, _ := startKubeWithoutCleanup(ctx, t, startKubeOptions{
					serviceType: kubeproxy.KubeService,
					authServer:  env.server.TLS,
					clusters: []kubeClusterConfig{
						{
							name:        tt.resourceName,
							apiEndpoint: testKube.URL,
						},
					},
				})
				err := kubeServer.Close()
				require.NoError(t, err)
				require.NoError(t, cleanup())
			}

			clusterName := localEnv.server.ClusterName()
			roles := tt.roleFunc(tt.teleportUser, tt.kubeUsers, tt.kubeGroups)
			pack := localEnv.proxies[0].authPack(t, tt.teleportUser, roles)

			createConnectionEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "diagnostics", "connections")

			resp, err := pack.clt.PostJSON(ctx, createConnectionEndpoint, conntest.TestConnectionRequest{
				ResourceKind: types.KindKubernetesCluster,
				ResourceName: tt.resourceName,
				// Default is 30 seconds but since tests run locally, we can reduce this value to also improve test responsiveness
				DialTimeout: time.Second,
				KubernetesImpersonation: conntest.KubernetesImpersonation{
					KubernetesUser:   tt.selectedKubeUser,
					KubernetesGroups: tt.selectedKubeGroups,
				},
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.Code())

			var connectionDiagnostic webui.ConnectionDiagnostic
			require.NoError(t, json.Unmarshal(resp.Bytes(), &connectionDiagnostic))
			gotFailedTraces := 0
			expectedFailedTraces := 0

			t.Log(tt.name)
			t.Log(connectionDiagnostic.Message, connectionDiagnostic.Success)
			for i, trace := range connectionDiagnostic.Traces {
				if trace.Status == types.ConnectionDiagnosticTrace_FAILED.String() {
					gotFailedTraces++
				}

				t.Logf("%d status='%s' type='%s' details='%s' error='%s'\n", i, trace.Status, trace.TraceType, trace.Details, trace.Error)
			}

			require.Equal(t, tt.expectedSuccess, connectionDiagnostic.Success)
			require.Equal(t, tt.expectedMessage, connectionDiagnostic.Message)

			for _, expectedTrace := range tt.expectedTraces {
				if expectedTrace.Status == types.ConnectionDiagnosticTrace_FAILED {
					expectedFailedTraces++
				}

				foundTrace := false
				for _, returnedTrace := range connectionDiagnostic.Traces {
					if expectedTrace.Type.String() != returnedTrace.TraceType {
						continue
					}

					foundTrace = true
					require.Equal(t, expectedTrace.Status.String(), returnedTrace.Status)
					require.Equal(t, expectedTrace.Details, returnedTrace.Details)
					require.Contains(t, expectedTrace.Error, returnedTrace.Error)
				}

				require.True(t, foundTrace, expectedTrace)
			}

			require.Equal(t, expectedFailedTraces, gotFailedTraces)
		})
	}

	// Test success with per-session MFA.

	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:           constants.Local,
		SecondFactor:   constants.SecondFactorOTP,
		RequireMFAType: types.RequireMFAType_SESSION,
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAuthPreference(ctx, ap)
	require.NoError(t, err)

	// Get a totp code to re-auth.
	pack := env.proxies[0].authPack(t, "llama", roleWithFullAccess("llama", validKubeUsers, validKubeGroups))
	totpCode, err := totp.GenerateCode(pack.otpSecret, env.clock.Now().Add(30*time.Second))
	require.NoError(t, err)

	clusterName := env.server.ClusterName()
	createConnectionEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "diagnostics", "connections")

	resp, err := pack.clt.PostJSON(ctx, createConnectionEndpoint, conntest.TestConnectionRequest{
		ResourceKind: types.KindKubernetesCluster,
		ResourceName: kubeClusterName,
		// Default is 30 seconds but since tests run locally, we can reduce this value to also improve test responsiveness
		DialTimeout:             time.Second,
		KubernetesImpersonation: conntest.KubernetesImpersonation{},
		MFAResponse:             client.MFAChallengeResponse{TOTPCode: totpCode},
	})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	var connectionDiagnostic webui.ConnectionDiagnostic
	require.NoError(t, json.Unmarshal(resp.Bytes(), &connectionDiagnostic))
	require.True(t, connectionDiagnostic.Success)
}

func TestCreateDatabase(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	username := "someuser"
	roleCreateDatabase, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
		Allow: types.RoleConditions{
			DatabaseNames: []string{"name1"},
			DatabaseUsers: []string{"user1"},
			Rules: []types.Rule{
				types.NewRule(types.KindDatabase,
					[]string{types.VerbCreate}),
			},
			DatabaseLabels: types.Labels{
				types.Wildcard: {types.Wildcard},
			},
		},
	})
	require.NoError(t, err)

	env := newWebPack(t, 1)
	clusterName := env.server.ClusterName()
	pack := env.proxies[0].authPack(t, username, []types.Role{roleCreateDatabase})

	createDatabaseEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "databases")

	// Create an initial database to table test a duplicate creation
	_, err = pack.clt.PostJSON(ctx, createDatabaseEndpoint, createOrOverwriteDatabaseRequest{
		Name:     "duplicatedb",
		Protocol: "mysql",
		URI:      "someuri:3306",
	})
	require.NoError(t, err)

	for _, tt := range []struct {
		name           string
		req            createOrOverwriteDatabaseRequest
		expectedStatus int
		errAssert      require.ErrorAssertionFunc
	}{
		{
			name: "valid",
			req: createOrOverwriteDatabaseRequest{
				Name:     "mydatabase",
				Protocol: "mysql",
				URI:      "someuri:3306",
				Labels: []ui.Label{
					{
						Name:  "teleport.dev/origin",
						Value: "dynamic",
					},
				},
			},
			expectedStatus: http.StatusOK,
			errAssert:      require.NoError,
		},
		{
			name: "valid with labels",
			req: createOrOverwriteDatabaseRequest{
				Name:     "dbwithlabels",
				Protocol: "mysql",
				URI:      "someuri:3306",
				Labels: []ui.Label{
					{
						Name:  "env",
						Value: "prod",
					},
					{
						Name:  "teleport.dev/origin",
						Value: "dynamic",
					},
				},
			},
			expectedStatus: http.StatusOK,
			errAssert:      require.NoError,
		},
		{
			name: "empty name",
			req: createOrOverwriteDatabaseRequest{
				Name:     "",
				Protocol: "mysql",
				URI:      "someuri:3306",
			},
			expectedStatus: http.StatusBadRequest,
			errAssert: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "missing database name")
			},
		},
		{
			name: "empty protocol",
			req: createOrOverwriteDatabaseRequest{
				Name:     "emptyprotocol",
				Protocol: "",
				URI:      "someuri:3306",
			},
			expectedStatus: http.StatusBadRequest,
			errAssert: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "missing protocol")
			},
		},
		{
			name: "empty uri",
			req: createOrOverwriteDatabaseRequest{
				Name:     "emptyuri",
				Protocol: "mysql",
				URI:      "",
			},
			expectedStatus: http.StatusBadRequest,
			errAssert: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "missing uri")
			},
		},
		{
			name: "missing port",
			req: createOrOverwriteDatabaseRequest{
				Name:     "missingport",
				Protocol: "mysql",
				URI:      "someuri",
			},
			expectedStatus: http.StatusBadRequest,
			errAssert: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "missing port in address")
			},
		},
		{
			name: "duplicatedb",
			req: createOrOverwriteDatabaseRequest{
				Name:     "duplicatedb",
				Protocol: "mysql",
				URI:      "someuri:3306",
			},
			expectedStatus: http.StatusConflict,
			errAssert: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAlreadyExists(err), "expected already exists error, got %v", err)
				require.Contains(t, err.Error(), `failed to create database ("duplicatedb" already exists), please use another name`)
			},
		},
	} {
		// Create database
		resp, err := pack.clt.PostJSON(ctx, createDatabaseEndpoint, tt.req)
		tt.errAssert(t, err)

		require.Equal(t, tt.expectedStatus, resp.Code(), "invalid status code received")

		if err != nil {
			continue
		}

		// Ensure database exists
		database, err := env.proxies[0].client.GetDatabase(ctx, tt.req.Name)
		require.NoError(t, err)

		require.Equal(t, tt.req.Name, database.GetName())
		require.Equal(t, tt.req.Protocol, database.GetProtocol())
		require.Equal(t, tt.req.URI, database.GetURI())

		// At least the provided labels exist in the database resource
		databaseLabels := database.GetAllLabels()
		for _, label := range tt.req.Labels {
			require.Contains(t, databaseLabels, label.Name, "label not found")
			require.Equal(t, label.Value, databaseLabels[label.Name], "label exists but has unexpected value")
		}

		// Check response value:
		if tt.expectedStatus == http.StatusOK {
			result := webui.Database{}
			require.NoError(t, json.Unmarshal(resp.Bytes(), &result))
			expected := webui.Database{
				Kind:          types.KindDatabase,
				Name:          tt.req.Name,
				Protocol:      tt.req.Protocol,
				Type:          types.DatabaseTypeSelfHosted,
				Labels:        tt.req.Labels,
				Hostname:      "someuri",
				DatabaseUsers: []string{"user1"},
				DatabaseNames: []string{"name1"},
				URI:           "someuri:3306",
			}
			require.Equal(t, expected, result)
		}
	}
}

func TestOverwriteDatabase(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "user", nil /* roles */)
	accessChecker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, env.server.ClusterName(), nil)

	initDb, err := types.NewDatabaseV3(types.Metadata{
		Name: "postgres",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "localhost:5432",
		AWS: types.AWS{
			AccountID: "123456789012",
		},
	})
	require.NoError(t, err)

	err = env.server.Auth().CreateDatabase(context.Background(), initDb)
	require.NoError(t, err)

	tests := []struct {
		name           string
		req            createOrOverwriteDatabaseRequest
		verifyResponse func(*testing.T, *roundtrip.Response, createOrOverwriteDatabaseRequest, error)
	}{
		{
			name: "overwrite",
			req: createOrOverwriteDatabaseRequest{
				Name:      initDb.GetName(),
				Overwrite: true,
				URI:       "some-other-uri:3306",
				Protocol:  "postgres",
			},
			verifyResponse: func(t *testing.T, resp *roundtrip.Response, req createOrOverwriteDatabaseRequest, err error) {
				require.NoError(t, err)

				var gotDb webui.Database
				require.NoError(t, json.Unmarshal(resp.Bytes(), &gotDb))
				require.Equal(t, req.URI, gotDb.URI)
				require.Equal(t, req.Protocol, gotDb.Protocol)
				require.Empty(t, req.AWSRDS)
				require.Equal(t, initDb.GetName(), gotDb.Name)

				backendDb, err := env.server.Auth().GetDatabase(context.Background(), req.Name)
				require.NoError(t, err)

				require.Equal(t, webui.MakeDatabase(backendDb, accessChecker, proxy.handler.handler.cfg.DatabaseREPLRegistry, false), gotDb)
			},
		},
		{
			name: "overwrite error: database does not exist",
			req: createOrOverwriteDatabaseRequest{
				Name:      "this-db-does-not-exist",
				URI:       "some-uri",
				Protocol:  "mysql",
				Overwrite: true,
			},
			verifyResponse: func(t *testing.T, resp *roundtrip.Response, req createOrOverwriteDatabaseRequest, err error) {
				require.True(t, trace.IsNotFound(err))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			endpoint := pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "databases")
			resp, err := pack.clt.PostJSON(context.Background(), endpoint, test.req)

			test.verifyResponse(t, resp, test.req, err)
		})
	}
}

func TestUpdateDatabase_Errors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	databaseName := "somedb"
	username := "someuser"
	roleCreateUpdateDatabase, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindDatabase,
					[]string{types.VerbCreate, types.VerbUpdate, types.VerbRead}),
			},
			DatabaseLabels: types.Labels{
				types.Wildcard: {types.Wildcard},
			},
		},
	})
	require.NoError(t, err)

	env := newWebPack(t, 1)
	clusterName := env.server.ClusterName()
	pack := env.proxies[0].authPack(t, username, []types.Role{roleCreateUpdateDatabase})

	// Create database
	createDatabaseEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "databases")
	_, err = pack.clt.PostJSON(ctx, createDatabaseEndpoint, createOrOverwriteDatabaseRequest{
		Name:     databaseName,
		Protocol: "mysql",
		URI:      "someuri:3306",
	})
	require.NoError(t, err)

	for _, tt := range []struct {
		name           string
		req            updateDatabaseRequest
		expectedStatus int
		errAssert      require.ErrorAssertionFunc
	}{
		{
			name: "empty ca_cert",
			req: updateDatabaseRequest{
				CACert: strPtr(""),
			},
			expectedStatus: http.StatusBadRequest,
			errAssert: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "missing CA certificate data")
			},
		},
		{
			name: "invalid certificate",
			req: updateDatabaseRequest{
				CACert: strPtr("Not a certificate"),
			},
			expectedStatus: http.StatusBadRequest,
			errAssert: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "could not parse provided CA as X.509 PEM certificate")
			},
		},

		{
			name: "invalid awsRDS missing resourceID field",
			req: updateDatabaseRequest{
				AWSRDS: &awsRDS{
					AccountID: "123123123123",
				},
			},
			expectedStatus: http.StatusBadRequest,
			errAssert: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "missing aws rds field resource id")
			},
		},
		{
			name: "invalid awsRDS missing accountID field",
			req: updateDatabaseRequest{
				AWSRDS: &awsRDS{
					ResourceID: "123123123123",
				},
			},
			expectedStatus: http.StatusBadRequest,
			errAssert: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "missing aws rds field account id")
			},
		},
		{
			name:           "no fields defined",
			req:            updateDatabaseRequest{},
			expectedStatus: http.StatusBadRequest,
			errAssert: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(tt, err, "missing fields to update the database")
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Update database's CA Cert
			updateDatabaseEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "databases", databaseName)
			resp, err := pack.clt.PutJSON(ctx, updateDatabaseEndpoint, tt.req)
			tt.errAssert(t, err)

			require.Equal(t, tt.expectedStatus, resp.Code(), "invalid status code received")
		})
	}
}

func TestUpdateDatabase_NonErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	databaseName := "somedb"
	username := "someuser"
	roleCreateUpdateDatabase, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindDatabase,
					[]string{types.VerbCreate, types.VerbUpdate, types.VerbRead}),
			},
			DatabaseLabels: types.Labels{
				types.Wildcard: {types.Wildcard},
			},
		},
	})
	require.NoError(t, err)

	env := newWebPack(t, 1)
	clusterName := env.server.ClusterName()
	pack := env.proxies[0].authPack(t, username, []types.Role{roleCreateUpdateDatabase})

	// Create a database.
	dbProtocol := "mysql"
	database, err := getNewDatabaseResource(createOrOverwriteDatabaseRequest{
		Name:     databaseName,
		Protocol: dbProtocol,
		URI:      "someuri:3306",
	})
	require.NoError(t, err)
	require.NoError(t, env.server.Auth().CreateDatabase(ctx, database))

	requiredOriginLabel := ui.Label{Name: types.OriginLabel, Value: types.OriginDynamic}

	// Each test case builds on top of each other.
	for _, tt := range []struct {
		name           string
		req            updateDatabaseRequest
		expectedFields webui.Database
		expectedAWSRDS awsRDS
	}{
		{
			name: "update caCert",
			req: updateDatabaseRequest{
				CACert: &fakeValidTLSCert,
			},
			expectedFields: webui.Database{
				Kind:     types.KindDatabase,
				Name:     databaseName,
				Protocol: dbProtocol,
				Type:     "self-hosted",
				Hostname: "someuri",
				Labels:   []ui.Label{requiredOriginLabel},
				URI:      "someuri:3306",
			},
		},
		{
			name: "update URI",
			req: updateDatabaseRequest{
				URI: "something-else:3306",
			},
			expectedFields: webui.Database{
				Kind:     types.KindDatabase,
				Name:     databaseName,
				Protocol: dbProtocol,
				Type:     "self-hosted",
				Hostname: "something-else",
				Labels:   []ui.Label{requiredOriginLabel},
				URI:      "something-else:3306",
			},
		},
		{
			name: "update aws rds fields",
			req: updateDatabaseRequest{
				URI: "llama.cgi8.us-west-2.rds.amazonaws.com:3306",
				AWSRDS: &awsRDS{
					AccountID:  "123123123123",
					ResourceID: "db-1234",
				},
			},
			expectedAWSRDS: awsRDS{
				AccountID:  "123123123123",
				ResourceID: "db-1234",
			},
			expectedFields: webui.Database{
				Kind:     types.KindDatabase,
				Name:     databaseName,
				Protocol: dbProtocol,
				Type:     "rds",
				Hostname: "llama.cgi8.us-west-2.rds.amazonaws.com",
				Labels:   []ui.Label{requiredOriginLabel},
				URI:      "llama.cgi8.us-west-2.rds.amazonaws.com:3306",
				AWS: &webui.AWS{
					AWS: types.AWS{
						Region:    "us-west-2",
						AccountID: "123123123123",
						RDS: types.RDS{
							ResourceID: "db-1234",
							InstanceID: "llama",
						},
					},
				},
			},
		},
		{
			name: "update labels",
			req: updateDatabaseRequest{
				Labels: []ui.Label{{Name: "env", Value: "prod"}},
			},
			expectedAWSRDS: awsRDS{
				AccountID:  "123123123123",
				ResourceID: "db-1234",
			},
			expectedFields: webui.Database{
				Kind:     types.KindDatabase,
				Name:     databaseName,
				Protocol: dbProtocol,
				Type:     "rds",
				Hostname: "llama.cgi8.us-west-2.rds.amazonaws.com",
				Labels:   []ui.Label{{Name: "env", Value: "prod"}, requiredOriginLabel},
				URI:      "llama.cgi8.us-west-2.rds.amazonaws.com:3306",
				AWS: &webui.AWS{
					AWS: types.AWS{
						Region:    "us-west-2",
						AccountID: "123123123123",
						RDS: types.RDS{
							ResourceID: "db-1234",
							InstanceID: "llama",
						},
					},
				},
			},
		},
		{
			name: "update multiple fields",
			req: updateDatabaseRequest{
				URI: "alpaca.cgi8.us-east-1.rds.amazonaws.com:3306",
				AWSRDS: &awsRDS{
					AccountID:  "000000000000",
					ResourceID: "db-0000",
				},
			},
			expectedAWSRDS: awsRDS{
				AccountID:  "000000000000",
				ResourceID: "db-0000",
			},
			expectedFields: webui.Database{
				Kind:     types.KindDatabase,
				Name:     databaseName,
				Protocol: dbProtocol,
				Type:     "rds",
				Hostname: "alpaca.cgi8.us-east-1.rds.amazonaws.com",
				Labels:   []ui.Label{{Name: "env", Value: "prod"}, requiredOriginLabel},
				URI:      "alpaca.cgi8.us-east-1.rds.amazonaws.com:3306",
				AWS: &webui.AWS{
					AWS: types.AWS{
						Region:    "us-east-1",
						AccountID: "000000000000",
						RDS: types.RDS{
							ResourceID: "db-0000",
							InstanceID: "alpaca",
						},
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			updateDatabaseEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "databases", databaseName)
			resp, err := pack.clt.PutJSON(ctx, updateDatabaseEndpoint, tt.req)
			require.NoError(t, err)
			var dbResp webui.Database
			require.NoError(t, json.Unmarshal(resp.Bytes(), &dbResp))
			require.Equal(t, tt.expectedFields, dbResp)

			// Ensure database was updated
			database, err := env.proxies[0].client.GetDatabase(ctx, databaseName)
			require.NoError(t, err)

			require.Equal(t, database.GetCA(), fakeValidTLSCert) // should not have changed
			require.Equal(t, database.GetType(), tt.expectedFields.Type)
			require.Equal(t, database.GetProtocol(), tt.expectedFields.Protocol)
			require.Equal(t, database.GetURI(), fmt.Sprintf("%s:3306", tt.expectedFields.Hostname))

			require.Equal(t, database.GetAWS().AccountID, tt.expectedAWSRDS.AccountID)
			require.Equal(t, database.GetAWS().RDS.ResourceID, tt.expectedAWSRDS.ResourceID)
		})
	}
}

type authProviderMock struct {
	server types.ServerV2
}

func (mock authProviderMock) ListUnifiedResources(ctx context.Context, req *authproto.ListUnifiedResourcesRequest) (*authproto.ListUnifiedResourcesResponse, error) {
	return nil, nil
}

func (mock authProviderMock) GetNode(ctx context.Context, namespace, name string) (types.Server, error) {
	return &mock.server, nil
}

func (mock authProviderMock) GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error) {
	return nil, trace.NotFound("foo")
}

func (mock authProviderMock) IsMFARequired(ctx context.Context, req *authproto.IsMFARequiredRequest) (*authproto.IsMFARequiredResponse, error) {
	return nil, nil
}

func (mock authProviderMock) CreateAuthenticateChallenge(ctx context.Context, req *authproto.CreateAuthenticateChallengeRequest) (*authproto.MFAAuthenticateChallenge, error) {
	return nil, nil
}

func (mock authProviderMock) GenerateUserCerts(ctx context.Context, req authproto.UserCertsRequest) (*authproto.Certs, error) {
	return nil, nil
}

func (mock authProviderMock) GenerateOpenSSHCert(ctx context.Context, req *authproto.OpenSSHCertRequest) (*authproto.OpenSSHCert, error) {
	return nil, nil
}

func (mock authProviderMock) MaintainSessionPresence(ctx context.Context) (authproto.AuthService_MaintainSessionPresenceClient, error) {
	return nil, nil
}

func (mock authProviderMock) GetUser(_ context.Context, _ string, _ bool) (types.User, error) {
	return nil, nil
}

func (mock authProviderMock) GetRole(_ context.Context, _ string) (types.Role, error) {
	return nil, nil
}

func waitForOutputWithDuration(r ReaderWithDeadline, substr string, timeout time.Duration) error {
	timeoutCh := time.After(timeout)

	var prev string
	out := make([]byte, int64(len(substr)*3))
	for {
		select {
		case <-timeoutCh:
			return trace.BadParameter("timeout waiting on terminal for output: %v", substr)
		default:
		}

		if err := r.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return trace.Wrap(err)
		}
		n, err := r.Read(out)
		outStr := removeSpace(string(out[:n]))

		// Check for [substr] before checking the error,
		// as it's valid for n > 0 even when there is an error.
		// The [substr] is checked against the current and previous
		// output to account for scenarios where the [substr] is split
		// across two reads. While we try to prevent this by reading
		// twice the length of [substr] there are no guarantees the
		// whole thing will arrive in a single read.
		if n > 0 && strings.Contains(prev+outStr, substr) {
			return nil
		}
		if err != nil {
			return trace.Wrap(err)
		}
		prev = outStr
	}
}

type ReaderWithDeadline interface {
	io.Reader
	SetReadDeadline(time.Time) error
}

type ReadWriterWithDeadline interface {
	io.Reader
	io.Writer
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
}

func waitForOutput(r ReaderWithDeadline, substr string) error {
	return waitForOutputWithDuration(r, substr, 10*time.Second)
}

func (s *WebSuite) client(t *testing.T, opts ...roundtrip.ClientParam) *TestWebClient {
	opts = append(opts, roundtrip.HTTPClient(client.NewInsecureWebClient()))
	wc, err := client.NewWebClient(s.url().String(), opts...)
	if err != nil {
		panic(err)
	}
	return &TestWebClient{wc, t}
}

type TestWebClient struct {
	*client.WebClient
	t *testing.T
}

// It is understood that implementing RoundTrip here will NOT result in calls from `Get`, or `PostJSON` from
// client.WebClient getting this verification.  Those functions would additionally need to be specified here.
// Despite that, currently our use of RoundTrip directly is providing us enough broad coverage to verify these headers.
func (c *TestWebClient) RoundTrip(fn roundtrip.RoundTripFn) (*roundtrip.Response, error) {
	c.t.Helper()
	resp, err := c.WebClient.RoundTrip(fn)

	verifySecurityResponseHeaders(c.t, resp.Headers())

	return resp, err
}

func (s *WebSuite) url() *url.URL {
	u, err := url.Parse("https://" + s.webServer.Listener.Addr().String())
	if err != nil {
		panic(err)
	}
	return u
}

func removeSpace(in string) string {
	for _, c := range []string{"\n", "\r", "\t"} {
		in = strings.Replace(in, c, " ", -1)
	}
	return strings.TrimSpace(in)
}

func decodeSessionCookie(t *testing.T, value string) (sessionID string) {
	sessionBytes, err := hex.DecodeString(value)
	require.NoError(t, err)
	var cookie struct {
		User      string `json:"user"`
		SessionID string `json:"sid"`
	}
	require.NoError(t, json.Unmarshal(sessionBytes, &cookie))
	return cookie.SessionID
}

func newWebPack(t *testing.T, numProxies int, opts ...proxyOption) *webPack {
	ctx := context.Background()
	clock := clockwork.NewFakeClockAt(time.Now())

	server, err := auth.NewTestServer(auth.TestServerConfig{
		Auth: auth.TestAuthServerConfig{
			ClusterName: "localhost",
			Dir:         t.TempDir(),
			Clock:       clock,
			AuditLog:    events.NewDiscardAuditLog(),
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, server.Shutdown(ctx)) })

	// use a sync recording mode because the disk-based uploader
	// that runs in the background introduces races with test cleanup
	recConfig := types.DefaultSessionRecordingConfig()
	recConfig.SetMode(types.RecordAtNodeSync)
	_, err = server.AuthServer.AuthServer.UpsertSessionRecordingConfig(context.Background(), recConfig)
	require.NoError(t, err)

	// Register the auth server, since test auth server doesn't start its own
	// heartbeat.
	err = server.Auth().UpsertAuthServer(ctx, &types.ServerV2{
		Kind:    types.KindAuthServer,
		Version: types.V2,
		Metadata: types.Metadata{
			Namespace: apidefaults.Namespace,
			Name:      "auth",
		},
		Spec: types.ServerSpecV2{
			Addr:     server.TLS.Listener.Addr().String(),
			Hostname: "localhost",
			Version:  teleport.Version,
		},
	})
	require.NoError(t, err)

	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPub, err := auth.PrivateKeyToPublicKeyTLS(priv)
	require.NoError(t, err)

	const nodeID = "node"
	// start auth server
	certs, err := server.Auth().GenerateHostCerts(ctx,
		&authproto.HostCertsRequest{
			HostID:       hostID,
			NodeName:     nodeID,
			Role:         types.RoleNode,
			PublicSSHKey: pub,
			PublicTLSKey: tlsPub,
		})
	require.NoError(t, err)

	signer, err := sshutils.NewSigner(priv, certs.SSH)
	require.NoError(t, err)
	hostSigners := []ssh.Signer{signer}

	nodeClient, err := server.TLS.NewClient(auth.TestIdentity{
		I: authz.BuiltinRole{
			Role:     types.RoleNode,
			Username: nodeID,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, nodeClient.Close()) })

	nodeLockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentNode,
			Client:    nodeClient,
		},
	})
	require.NoError(t, err)
	t.Cleanup(nodeLockWatcher.Close)

	nodeSessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   nodeClient,
		AccessPoint:  nodeClient,
		LockEnforcer: nodeLockWatcher,
		Emitter:      nodeClient,
		Component:    teleport.ComponentNode,
		ServerID:     nodeID,
	})
	require.NoError(t, err)

	// create SSH service:
	nodeDataDir := t.TempDir()
	node, err := regular.New(
		ctx,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		nodeID,
		sshutils.StaticHostSigners(hostSigners...),
		nodeClient,
		nodeDataDir,
		"",
		utils.NetAddr{},
		nodeClient,
		regular.SetUUID(nodeID),
		regular.SetNamespace(apidefaults.Namespace),
		regular.SetEmitter(nodeClient),
		regular.SetPAMConfig(&servicecfg.PAMConfig{Enabled: false}),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetClock(clock),
		regular.SetLockWatcher(nodeLockWatcher),
		regular.SetSessionController(nodeSessionController),
	)
	require.NoError(t, err)

	require.NoError(t, node.Start())
	t.Cleanup(func() {
		require.NoError(t, node.Close())
		node.Wait()
	})

	var proxies []*testProxy
	for p := 0; p < numProxies; p++ {
		proxyID := fmt.Sprintf("proxy%v", p)
		proxies = append(proxies, createProxy(ctx, t, proxyID, node, server.TLS, hostSigners, clock, opts...))
	}

	// Wait for proxies to fully register before starting the test.
	for start := time.Now(); ; {
		proxies, err := proxies[0].client.GetProxies()
		require.NoError(t, err)
		if len(proxies) == numProxies {
			break
		}
		if time.Since(start) > 5*time.Second {
			t.Fatalf("Proxies didn't register within 5s after startup; registered: %d, want: %d", len(proxies), numProxies)
		}
	}

	return &webPack{
		proxies: proxies,
		server:  server,
		node:    node,
		clock:   clock,
	}
}

// wrappedAuthClient is used when tests need to mock or replace parts of the
// underlying auth.Client used by the Proxy.
type wrappedAuthClient struct {
	*authclient.Client
	devicesClient devicepb.DeviceTrustServiceClient
}

func (w *wrappedAuthClient) DevicesClient() devicepb.DeviceTrustServiceClient {
	return w.devicesClient
}

type proxyConfig struct {
	minimalHandler        bool
	devicesClientOverride devicepb.DeviceTrustServiceClient
}

type proxyOption func(cfg *proxyConfig)

func withDevicesClientOverride(c devicepb.DeviceTrustServiceClient) proxyOption {
	return func(cfg *proxyConfig) {
		cfg.devicesClientOverride = c
	}
}

func createProxy(ctx context.Context, t *testing.T, proxyID string, node *regular.Server, authServer *auth.TestTLSServer,
	hostSigners []ssh.Signer, clock *clockwork.FakeClock, opts ...proxyOption,
) *testProxy {
	cfg := proxyConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	// create reverse tunnel service:
	authClient, err := authServer.NewClient(auth.TestIdentity{
		I: authz.BuiltinRole{
			Role:     types.RoleProxy,
			Username: proxyID,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authClient.Close()) })

	// Replace underlying devicesClient, if the option was supplied.
	var client authclient.ClientI
	if cfg.devicesClientOverride != nil {
		client = &wrappedAuthClient{
			Client:        authClient,
			devicesClient: cfg.devicesClientOverride,
		}
	} else {
		client = authClient
	}
	// Favor client instead of authClient from here on.

	revTunListener, err := net.Listen("tcp", fmt.Sprintf("%v:0", authServer.ClusterName()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, revTunListener.Close()) })

	proxyLockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    client,
		},
	})
	require.NoError(t, err)
	t.Cleanup(proxyLockWatcher.Close)

	proxyCAWatcher, err := services.NewCertAuthorityWatcher(ctx, services.CertAuthorityWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    client,
		},
		Types: []types.CertAuthType{types.HostCA, types.UserCA},
	})
	require.NoError(t, err)
	t.Cleanup(proxyLockWatcher.Close)

	proxyNodeWatcher, err := services.NewNodeWatcher(ctx, services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    client,
		},
		NodesGetter: client,
	})
	require.NoError(t, err)
	t.Cleanup(proxyNodeWatcher.Close)

	proxyGitServerWatcher, err := services.NewGitServerWatcher(ctx, services.GitServerWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    client,
		},
		GitServerGetter: client.GitServerReadOnlyClient(),
	})
	require.NoError(t, err)
	t.Cleanup(proxyGitServerWatcher.Close)

	revTunServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ID:       node.ID(),
		Listener: revTunListener,
		GetClientTLSCertificate: func() (*tls.Certificate, error) {
			return &authClient.TLSConfig().Certificates[0], nil
		},
		ClusterName:           authServer.ClusterName(),
		GetHostSigners:        sshutils.StaticHostSigners(hostSigners...),
		LocalAuthClient:       client,
		LocalAccessPoint:      client,
		Emitter:               client,
		NewCachingAccessPoint: noCache,
		DataDir:               t.TempDir(),
		LockWatcher:           proxyLockWatcher,
		NodeWatcher:           proxyNodeWatcher,
		GitServerWatcher:      proxyGitServerWatcher,
		CertAuthorityWatcher:  proxyCAWatcher,
		CircuitBreakerConfig:  breaker.NoopBreakerConfig(),
		LocalAuthAddresses:    []string{authServer.Listener.Addr().String()},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, revTunServer.Close()) })

	clustername := authServer.ClusterName()
	router, err := proxy.NewRouter(proxy.RouterConfig{
		ClusterName:      clustername,
		LocalAccessPoint: client,
		SiteGetter:       revTunServer,
		TracerProvider:   tracing.NoopProvider(),
	})
	require.NoError(t, err)

	sessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   client,
		AccessPoint:  client,
		LockEnforcer: proxyLockWatcher,
		Emitter:      client,
		Component:    teleport.ComponentProxy,
		ServerID:     proxyID,
	})
	require.NoError(t, err)

	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	mux, err := multiplexer.New(multiplexer.Config{
		Listener:          proxyListener,
		PROXYProtocolMode: multiplexer.PROXYProtocolOff,
		ID:                teleport.Component(teleport.ComponentProxy, "ssh"),
		CertAuthorityGetter: func(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
			return client.GetCertAuthority(ctx, id, loadKeys)
		},
		LocalClusterName: clustername,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mux.Close()) })

	go func() {
		if err := mux.Serve(); err != nil && !utils.IsOKNetworkError(err) {
			slog.ErrorContext(context.Background(), "Mux encountered error serving", "error", err)
		}
	}()

	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: clustername,
		AccessPoint: authServer.Auth(),
		LockWatcher: proxyLockWatcher,
	})
	require.NoError(t, err)

	tlscfg, err := authServer.Identity.TLSConfig(utils.DefaultCipherSuites())
	require.NoError(t, err)
	tlscfg.ClientAuth = tls.RequireAndVerifyClientCert
	if lib.IsInsecureDevMode() {
		tlscfg.InsecureSkipVerify = true
		tlscfg.ClientAuth = tls.RequireAnyClientCert
	}
	tlscfg.GetConfigForClient = func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		tlsClone := tlscfg.Clone()

		// Build the client CA pool containing the cluster's user CA in
		// order to be able to validate certificates provided by users.
		var err error
		tlsClone.ClientCAs, _, _, err = authclient.DefaultClientCertPool(info.Context(), authServer.Auth(), clustername)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return tlsClone, nil
	}

	creds, err := auth.NewTransportCredentials(auth.TransportCredentialsConfig{
		TransportCredentials: credentials.NewTLS(tlscfg),
		UserGetter: &auth.Middleware{
			ClusterName: authServer.ClusterName(),
		},
		Authorizer: authorizer,
	})
	require.NoError(t, err)

	sshGRPCServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
		grpc.ChainStreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
		grpc.Creds(creds),
	)
	t.Cleanup(sshGRPCServer.Stop)

	connMonitor, err := srv.NewConnectionMonitor(srv.ConnectionMonitorConfig{
		AccessPoint:    client,
		LockWatcher:    proxyLockWatcher,
		Clock:          clock,
		ServerID:       proxyID,
		Emitter:        client,
		EmitterContext: ctx,
		Logger:         utils.NewSlogLoggerForTests(),
	})
	require.NoError(t, err)

	transportService, err := transportv1.NewService(transportv1.ServerConfig{
		FIPS:   false,
		Logger: utils.NewSlogLoggerForTests(),
		Dialer: router,
		SignerFn: func(authzCtx *authz.Context, clusterName string) agentless.SignerCreator {
			return agentless.SignerFromAuthzContext(authzCtx, client, clusterName)
		},
		ConnectionMonitor: connMonitor,
		LocalAddr:         proxyListener.Addr(),
	})
	require.NoError(t, err)
	transportpb.RegisterTransportServiceServer(sshGRPCServer, transportService)

	go func() {
		if err := sshGRPCServer.Serve(mux.TLS()); err != nil && !utils.IsOKNetworkError(err) {
			slog.ErrorContext(context.Background(), "gRPC proxy server terminated unexpectedly", "error", err)
		}
	}()

	proxyServer, err := regular.New(
		ctx,
		utils.NetAddr{AddrNetwork: proxyListener.Addr().Network(), Addr: mux.SSH().Addr().String()},
		authServer.ClusterName(),
		sshutils.StaticHostSigners(hostSigners...),
		client,
		t.TempDir(),
		"",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "proxy-1.example.com:443"},
		client,
		regular.SetUUID(proxyID),
		regular.SetProxyMode("", revTunServer, client, router),
		regular.SetEmitter(client),
		regular.SetNamespace(apidefaults.Namespace),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetClock(clock),
		regular.SetLockWatcher(proxyLockWatcher),
		regular.SetSessionController(sessionController),
		regular.SetPublicAddrs([]utils.NetAddr{{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, proxyServer.Close()) })

	fs, err := NewDebugFileSystem(false)
	require.NoError(t, err)

	authID := state.IdentityID{
		Role:     types.RoleProxy,
		HostUUID: proxyID,
	}
	dns := []string{"localhost", "127.0.0.1"}
	proxyIdentity, err := auth.LocalRegister(authID, authServer.Auth(), nil, dns, "", nil)
	require.NoError(t, err)
	proxyClientCert, err := keys.X509KeyPair(proxyIdentity.TLSCertBytes, proxyIdentity.KeyBytes)
	require.NoError(t, err)
	handler, err := NewHandler(Config{
		Proxy:            revTunServer,
		AuthServers:      utils.FromAddr(authServer.Addr()),
		ProxyClient:      client,
		ProxyPublicAddrs: utils.MustParseAddrList("proxy-1.example.com", "proxy-2.example.com"),
		CipherSuites:     utils.DefaultCipherSuites(),
		AccessPoint:      client,
		Context:          ctx,
		HostUUID:         proxyID,
		Emitter:          client,
		StaticFS:         fs,
		ProxySettings: &ProxySettings{
			ServiceConfig: servicecfg.MakeDefaultConfig(),
			ProxySSHAddr:  "127.0.0.1",
			AccessPoint:   client,
		},
		SessionControl: SessionControllerFunc(func(ctx context.Context, sctx *SessionContext, login, localAddr, remoteAddr string) (context.Context, error) {
			controller := srv.WebSessionController(sessionController)
			ctx, err := controller(ctx, sctx, login, localAddr, remoteAddr)
			return ctx, trace.Wrap(err)
		}),
		Router:                         router,
		HealthCheckAppServer:           func(context.Context, string, string) error { return nil },
		MinimalReverseTunnelRoutesOnly: cfg.minimalHandler,
		GetProxyClientCertificate: func() (*tls.Certificate, error) {
			return &proxyClientCert, nil
		},
		IntegrationAppHandler: &mockIntegrationAppHandler{},
		DatabaseREPLRegistry:  &mockDatabaseREPLRegistry{repl: map[string]dbrepl.REPLNewFunc{}},
	}, SetClock(clock))
	require.NoError(t, err)

	webServer := httptest.NewTLSServer(handler)
	t.Cleanup(webServer.Close)
	go func() {
		if err := proxyServer.Serve(mux.SSH()); err != nil && !utils.IsOKNetworkError(err) {
			slog.ErrorContext(context.Background(), "SSH proxy server terminated unexpectedly", "error", err)
		}
	}()

	proxyAddr := mux.Listener.Addr()
	addr := utils.MustParseAddr(webServer.Listener.Addr().String())
	handler.handler.cfg.ProxyWebAddr = *addr
	handler.handler.cfg.ProxySSHAddr = utils.NetAddr{AddrNetwork: proxyAddr.Network(), Addr: proxyAddr.String()}

	_, sshPort, err := net.SplitHostPort(proxyAddr.String())
	require.NoError(t, err)
	handler.handler.sshPort = sshPort

	kubeProxyAddr := startKube(
		ctx,
		t,
		startKubeOptions{
			serviceType: kubeproxy.ProxyService,
			authServer:  authServer,
			revTunnel:   revTunServer,
		},
	)
	handler.handler.cfg.ProxyKubeAddr = utils.FromAddr(kubeProxyAddr)
	handler.handler.cfg.PublicProxyAddr = webServer.Listener.Addr().String()
	url, err := url.Parse("https://" + webServer.Listener.Addr().String())
	require.NoError(t, err)

	return &testProxy{
		clock:   clock,
		auth:    authServer,
		client:  client,
		revTun:  revTunServer,
		node:    node,
		proxy:   proxyServer,
		web:     webServer,
		handler: handler,
		webURL:  *url,
	}
}

type mockIntegrationAppHandler struct{}

func (m *mockIntegrationAppHandler) HandleConnection(_ net.Conn) {}

// webPack represents the state of a single web test.
// It replicates most of the WebSuite and serves to gradually
// transition the test suite to use the testing package
// directly.
type webPack struct {
	proxies []*testProxy
	server  *auth.TestServer
	node    *regular.Server
	clock   *clockwork.FakeClock
}

type testProxy struct {
	clock   *clockwork.FakeClock
	client  authclient.ClientI
	auth    *auth.TestTLSServer
	revTun  reversetunnelclient.Server
	node    *regular.Server
	proxy   *regular.Server
	handler *APIHandler
	web     *httptest.Server
	webURL  url.URL
}

// authPack returns new authenticated package consisting of created valid
// user, otp token, created web session and authenticated client.
func (r *testProxy) authPack(t *testing.T, teleportUser string, roles []types.Role) *authPack {
	ctx := context.Background()
	const (
		pass      = "abcdef123456"
		rawSecret = "def456"
	)

	u, err := user.Current()
	require.NoError(t, err)
	loginUser := u.Username

	otpSecret := newOTPSharedSecret()

	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	require.NoError(t, err)

	_, err = r.auth.Auth().UpsertAuthPreference(ctx, ap)
	require.NoError(t, err)

	r.createUser(context.Background(), t, teleportUser, loginUser, pass, otpSecret, roles)

	sessionResp, httpResp := loginWebOTP(t, ctx, loginWebOTPParams{
		webClient: r.newClient(t),
		clock:     r.clock,
		user:      teleportUser,
		password:  pass,
		otpSecret: otpSecret,
	})

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	clt := r.newClient(t, roundtrip.BearerAuth(sessionResp.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(&r.webURL, httpResp.Cookies())

	return &authPack{
		otpSecret: otpSecret,
		user:      teleportUser,
		login:     loginUser,
		session:   sessionResp,
		clt:       clt,
		cookies:   httpResp.Cookies(),
		password:  pass,
		device: &auth.TestDevice{
			TOTPSecret: otpSecret,
		},
	}
}

func (r *testProxy) authPackFromPack(t *testing.T, pack *authPack) *authPack {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	clt := r.newClient(t, roundtrip.BearerAuth(pack.session.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(&r.webURL, pack.cookies)

	result := *pack
	result.clt = clt
	return &result
}

func (r *testProxy) authPackFromResponse(t *testing.T, httpResp *roundtrip.Response) *authPack {
	var resp *CreateSessionResponse
	require.NoError(t, json.Unmarshal(httpResp.Bytes(), &resp))
	if resp.TokenExpiresIn < 0 {
		t.Errorf("Expected expiry time to be in the future but got %v", resp.TokenExpiresIn)
	}

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	clt := r.newClient(t, roundtrip.BearerAuth(resp.Token), roundtrip.CookieJar(jar))
	jar.SetCookies(&r.webURL, httpResp.Cookies())

	return &authPack{
		session: resp,
		clt:     clt,
		cookies: httpResp.Cookies(),
	}
}

func defaultRoleForNewUser(teleUser types.User, login string) types.Role {
	role := services.RoleForUser(teleUser)
	role.SetLogins(types.Allow, []string{login})
	role.SetWindowsDesktopLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	options := role.GetOptions()
	options.ForwardAgent = types.NewBool(true)
	role.SetOptions(options)
	return role
}

func (r *testProxy) createUser(ctx context.Context, t *testing.T, user, login, pass, otpSecret string, roles []types.Role) {
	teleUser, err := types.NewUser(user)
	require.NoError(t, err)

	if len(roles) == 0 {
		roles = []types.Role{defaultRoleForNewUser(teleUser, login)}
	}

	for _, role := range roles {
		role, err = r.auth.Auth().UpsertRole(ctx, role)
		require.NoError(t, err)

		teleUser.AddRole(role.GetName())
	}

	teleUser.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: "some-auth-user"},
	})

	_, err = r.auth.Auth().CreateUser(ctx, teleUser)
	require.NoError(t, err)

	err = r.auth.Auth().UpsertPassword(user, []byte(pass))
	require.NoError(t, err)

	if otpSecret != "" {
		dev, err := services.NewTOTPDevice("otp", otpSecret, r.clock.Now())
		require.NoError(t, err)
		err = r.auth.Auth().UpsertMFADevice(ctx, user, dev)
		require.NoError(t, err)
	}
}

func (r *testProxy) newClient(t *testing.T, opts ...roundtrip.ClientParam) *TestWebClient {
	opts = append(opts, roundtrip.HTTPClient(client.NewInsecureWebClient()))
	clt, err := client.NewWebClient(r.webURL.String(), opts...)
	require.NoError(t, err)
	return &TestWebClient{clt, t}
}

func makeAuthReqOverWS(ws *websocket.Conn, token string) error {
	authReq, err := json.Marshal(struct {
		Token string `json:"token"`
	}{Token: token})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := ws.WriteMessage(websocket.TextMessage, authReq); err != nil {
		return trace.Wrap(err)
	}
	_, authRes, err := ws.ReadMessage()
	if err != nil {
		return trace.Wrap(err)
	}
	if !strings.Contains(string(authRes), `"status":"ok"`) {
		return trace.AccessDenied("unexpected response")
	}
	return nil
}

func (r *testProxy) makeDesktopSession(t *testing.T, pack *authPack) *websocket.Conn {
	u := url.URL{
		Host:   r.webURL.Host,
		Scheme: client.WSS,
		Path:   fmt.Sprintf("/webapi/sites/%s/desktops/%s/connect/ws", currentSiteShortcut, "desktop1"),
	}

	q := u.Query()
	q.Set("username", "marek")
	q.Set("width", "100")
	q.Set("height", "100")
	u.RawQuery = q.Encode()

	dialer := websocket.Dialer{}
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	header := http.Header{}
	for _, cookie := range pack.cookies {
		header.Add("Cookie", cookie.String())
	}

	ws, resp, err := dialer.Dial(u.String(), header)
	require.NoError(t, err)

	err = makeAuthReqOverWS(ws, pack.session.Token)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, ws.Close())
		require.NoError(t, resp.Body.Close())
	})
	return ws
}

func validateTerminal(t *testing.T, term ReadWriterWithDeadline) {
	t.Helper()

	// here we intentionally run a command where the output we're looking
	// for is not present in the command itself
	_, err := io.WriteString(term, "echo txlxport | sed 's/x/e/g'\r\n")
	require.NoError(t, err)
	require.NoError(t, waitForOutput(term, "teleport"))
}

// TestUserContextWithAccessRequest checks that the userContext includes the ID of the
// access request after it has been consumed and the web session has been renewed.
func TestUserContextWithAccessRequest(t *testing.T) {
	t.Parallel()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	ctx := context.Background()

	// Set user and role names.
	username := "user"
	baseRoleName := "role"
	requestableRolename := "requestable-role"

	// Create user's base role with the ability to request the requestable role.
	baseRole, err := types.NewRole(baseRoleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles: []string{requestableRolename},
			},
		},
	})
	require.NoError(t, err)

	// Create user with the base role.
	pack := proxy.authPack(t, username, []types.Role{baseRole})

	// Create the requestable role.
	requestableRole, err := types.NewRole(requestableRolename, types.RoleSpecV6{})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertRole(ctx, requestableRole)
	require.NoError(t, err)

	identity := tlsca.Identity{
		Expires: env.clock.Now().Add(1 * time.Hour),
	}

	// Create and approve an access request for the requestable role.
	accessReq, err := services.NewAccessRequest(username, requestableRolename)
	require.NoError(t, err)
	accessReq.SetState(types.RequestState_APPROVED)
	accessReq, err = env.server.Auth().CreateAccessRequestV2(ctx, accessReq, identity)
	require.NoError(t, err)

	// Get the ID of the created and approved access request.
	accessRequestID := accessReq.GetMetadata().Name

	// Make a request to renew the session with the ID of the access request.
	_, err = pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "sessions", "web", "renew"), renewSessionRequest{
		AccessRequestID: accessRequestID,
	})
	require.NoError(t, err)

	// Make a request to fetch the userContext.
	endpoint := pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "context")
	response, err := pack.clt.Get(context.Background(), endpoint, url.Values{})
	require.NoError(t, err)

	// Process the JSON response of the request.
	var userContext webui.UserContext
	err = json.Unmarshal(response.Bytes(), &userContext)
	require.NoError(t, err)

	// Verify that the userContext returned contains the correct Access Request ID.
	require.Equal(t, accessRequestID, userContext.ConsumedAccessRequestID)
}

// TestIsMFARequired_AcceptedRequests mostly tests that requests
// are formatted correctly.
func TestIsMFARequired_AcceptedRequests(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "llama", nil /* roles */)

	cfg, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:           constants.Local,
		SecondFactor:   constants.SecondFactorWebauthn,
		RequireMFAType: types.RequireMFAType_SESSION,
		Webauthn: &types.Webauthn{
			RPID: env.server.ClusterName(),
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAuthPreference(ctx, cfg)
	require.NoError(t, err)

	// Register an application called "panel".
	app, err := types.NewAppV3(types.Metadata{
		Name: "panel",
	}, types.AppSpecV3{
		URI:        "http://127.0.0.1:8080",
		PublicAddr: "panel.example.com",
	})
	require.NoError(t, err)
	server, err := types.NewAppServerV3FromApp(app, "host", uuid.New().String())
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertApplicationServer(ctx, server)
	require.NoError(t, err)

	for _, test := range []struct {
		name       string
		errMsg     string
		getRequest func() IsMFARequiredRequest
	}{
		{
			name: "valid db req",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{
					Database: &isMFARequiredDatabase{
						ServiceName: "name",
						Protocol:    "protocol",
					},
				}
			},
		},
		{
			name:   "invalid db req",
			errMsg: "missing service_name",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{Database: &isMFARequiredDatabase{}}
			},
		},
		{
			name: "valid node req",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{
					Node: &isMFARequiredNode{
						NodeName: "name",
						Login:    "login",
					},
				}
			},
		},
		{
			name:   "invalid node req",
			errMsg: "missing login",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{Node: &isMFARequiredNode{}}
			},
		},
		{
			name: "valid kube req",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{
					Kube: &isMFARequiredKube{
						ClusterName: "name",
					},
				}
			},
		},
		{
			name:   "invalid kube req",
			errMsg: "missing cluster_name",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{Kube: &isMFARequiredKube{}}
			},
		},
		{
			name: "valid windows desktop req",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{
					WindowsDesktop: &isMFARequiredWindowsDesktop{
						DesktopName: "name",
						Login:       "login",
					},
				}
			},
		},
		{
			name:   "invalid windows desktop req",
			errMsg: "missing desktop_name",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{WindowsDesktop: &isMFARequiredWindowsDesktop{}}
			},
		},
		{
			name: "valid app req - resolve addr",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{
					App: &IsMFARequiredApp{
						ResolveAppParams: ResolveAppParams{
							PublicAddr:  app.GetPublicAddr(),
							ClusterName: env.server.ClusterName(),
						},
					},
				}
			},
		},
		{
			name: "valid app req - resolve fqdn",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{
					App: &IsMFARequiredApp{
						ResolveAppParams: ResolveAppParams{
							FQDNHint: fmt.Sprintf("%v.%v", app.GetName(), "proxy-1.example.com"),
						},
					},
				}
			},
		},
		{
			name:   "invalid app req",
			errMsg: "no inputs to resolve application",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{App: &IsMFARequiredApp{}}
			},
		},
		{
			name: "valid admin action req",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{
					AdminAction: &isMFARequiredAdminAction{},
				}
			},
		},
		{
			name:   "invalid empty req",
			errMsg: "missing target",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{}
			},
		},
		{
			name:   "invalid multi field",
			errMsg: "only one target is allowed",
			getRequest: func() IsMFARequiredRequest {
				return IsMFARequiredRequest{
					Kube: &isMFARequiredKube{
						ClusterName: "name",
					},
					Node: &isMFARequiredNode{
						NodeName: "name",
						Login:    "login",
					},
				}
			},
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			endpoint := pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "mfa", "required")
			re, err := pack.clt.PostJSON(ctx, endpoint, test.getRequest())

			if test.errMsg != "" {
				require.True(t, trace.IsBadParameter(err), "isMFARequired returned err = %v (%T), wanted trace.BadParameter", err, err)
				require.ErrorContains(t, err, test.errMsg)
				return
			}

			require.NoError(t, err)
			resp := isMfaRequiredResponse{}
			require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
			require.True(t, resp.Required, "isMFARequired returned response with unexpected value for Required field")
		})
	}
}

func TestWithLimiterHandlerFunc(t *testing.T) {
	const burst = 20
	limiter, err := limiter.NewRateLimiter(limiter.Config{
		Rates: []limiter.Rate{
			{
				Period:  time.Minute,
				Average: 10,
				Burst:   burst,
			},
		},
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	h := &Handler{limiter: limiter}
	hf := h.WithLimiterHandlerFunc(func(http.ResponseWriter, *http.Request, httprouter.Params) (interface{}, error) {
		return nil, nil
	})

	// Verify that a valid burst is allowed.
	r := &http.Request{}
	for i := 0; i < burst; i++ {
		r.RemoteAddr = fmt.Sprintf("127.0.0.1:%v", i)
		_, err = hf(nil, r, nil)
		require.NoError(t, err, "WithLimiterHandlerFunc failed unexpectedly")
	}

	// Verify that exceeding the limit causes errors.
	r.RemoteAddr = fmt.Sprintf("127.0.0.1:%v", burst)
	_, err = hf(nil, r, nil)
	require.True(t, trace.IsLimitExceeded(err), "WithLimiterHandlerFunc returned err = %T, want trace.LimitExceededError", err)
}

// kubeClusterConfig defines the cluster to be created
type kubeClusterConfig struct {
	name        string
	apiEndpoint string
}

func newKubeConfigFile(ctx context.Context, t *testing.T, clusters ...kubeClusterConfig) string {
	tmpDir := t.TempDir()

	kubeConf := clientcmdapi.NewConfig()
	for _, cluster := range clusters {
		kubeConf.Clusters[cluster.name] = &clientcmdapi.Cluster{
			Server:                cluster.apiEndpoint,
			InsecureSkipTLSVerify: true,
		}
		kubeConf.AuthInfos[cluster.name] = &clientcmdapi.AuthInfo{}

		kubeConf.Contexts[cluster.name] = &clientcmdapi.Context{
			Cluster:  cluster.name,
			AuthInfo: cluster.name,
		}
	}
	kubeConfigLocation := filepath.Join(tmpDir, "kubeconfig")
	err := clientcmd.WriteToFile(*kubeConf, kubeConfigLocation)
	require.NoError(t, err)
	return kubeConfigLocation
}

type startKubeOptions struct {
	clusters    []kubeClusterConfig
	authServer  *auth.TestTLSServer
	revTunnel   reversetunnelclient.Server
	serviceType kubeproxy.KubeServiceType
}

func startKube(ctx context.Context, t *testing.T, cfg startKubeOptions) net.Addr {
	server, cleanup, addr := startKubeWithoutCleanup(ctx, t, cfg)
	t.Cleanup(func() {
		err := server.Close()
		require.NoError(t, err)
		require.NoError(t, cleanup())
	})
	return addr
}

type cleanupFunc func() error

func startKubeWithoutCleanup(ctx context.Context, t *testing.T, cfg startKubeOptions) (*kubeproxy.TLSServer, cleanupFunc, net.Addr) {
	role := types.RoleProxy
	if cfg.serviceType == kubeproxy.KubeService {
		role = types.RoleKube
	}
	var kubeConfigLocation string
	if len(cfg.clusters) > 0 {
		kubeConfigLocation = newKubeConfigFile(ctx, t, cfg.clusters...)
	}

	keyGen := tlsutils.New(ctx)
	hostID := uuid.New().String()
	// heartbeatsWaitChannel waits for clusters heartbeats to start.
	heartbeatsWaitChannel := make(chan struct{}, len(cfg.clusters))
	client, err := cfg.authServer.NewClient(auth.TestServerID(role, hostID))
	require.NoError(t, err)

	// Auth client, lock watcher and authorizer for Kube proxy.
	proxyAuthClient, err := cfg.authServer.NewClient(auth.TestBuiltin(types.RoleProxy))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, proxyAuthClient.Close()) })

	proxyLockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    proxyAuthClient,
		},
	})
	require.NoError(t, err)
	proxyAuthorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: cfg.authServer.ClusterName(),
		AccessPoint: proxyAuthClient,
		LockWatcher: proxyLockWatcher,
	})
	require.NoError(t, err)

	// TLS config for kube proxy and Kube service.
	authID := state.IdentityID{
		Role:     role,
		HostUUID: hostID,
		NodeName: "kube_server",
	}
	dns := []string{"localhost", "127.0.0.1", constants.KubeTeleportProxyALPNPrefix + constants.APIDomain, "*" + constants.APIDomain}
	identity, err := auth.LocalRegister(authID, cfg.authServer.Auth(), nil, dns, "", nil)
	require.NoError(t, err)

	tlsConfig, err := identity.TLSConfig(nil)
	require.NoError(t, err)

	component := teleport.Component(teleport.ComponentProxy, teleport.ComponentProxyKube)
	if cfg.serviceType == kubeproxy.KubeService {
		component = teleport.ComponentKube
	}

	proxySigner := &mockPROXYSigner{}
	if cfg.serviceType == kubeproxy.KubeService {
		proxySigner = nil
	}
	clock := clockwork.NewRealClock()
	watcher, err := services.NewKubeServerWatcher(ctx, services.KubeServerWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: component,
			Client:    client,
			Clock:     clock,
		},
		KubernetesServerGetter: client,
	})
	require.NoError(t, err)

	inventoryHandle := inventory.NewDownstreamHandle(client.InventoryControlStream, clientproto.UpstreamInventoryHello{
		ServerID: hostID,
		Version:  teleport.Version,
		Services: []types.SystemRole{role},
		Hostname: "test",
	})
	t.Cleanup(func() { require.NoError(t, inventoryHandle.Close()) })

	kubeServer, err := kubeproxy.NewTLSServer(kubeproxy.TLSServerConfig{
		ForwarderConfig: kubeproxy.ForwarderConfig{
			Namespace:         apidefaults.Namespace,
			Keygen:            keyGen,
			ClusterName:       cfg.authServer.ClusterName(),
			Authz:             proxyAuthorizer,
			AuthClient:        client,
			Emitter:           client,
			DataDir:           t.TempDir(),
			CachingAuthClient: client,
			HostID:            hostID,
			Context:           ctx,
			KubeconfigPath:    kubeConfigLocation,
			KubeServiceType:   cfg.serviceType,
			Component:         component,
			LockWatcher:       proxyLockWatcher,
			ReverseTunnelSrv:  cfg.revTunnel,
			PROXYSigner:       proxySigner,
			// skip Impersonation validation
			CheckImpersonationPermissions: func(ctx context.Context, clusterName string, sarClient authztypes.SelfSubjectAccessReviewInterface) error {
				return nil
			},

			GetConnTLSCertificate: func() (*tls.Certificate, error) {
				return &tlsConfig.Certificates[0], nil
			},
			GetConnTLSRoots: func() (*x509.CertPool, error) {
				return tlsConfig.RootCAs, nil
			},

			Clock: clockwork.NewRealClock(),
			ClusterFeatures: func() authproto.Features {
				return authproto.Features{
					Entitlements: map[string]*authproto.EntitlementInfo{
						string(entitlements.K8s): {Enabled: true},
					},
				}
			},
		},
		TLS:           tlsConfig.Clone(),
		AccessPoint:   client,
		DynamicLabels: nil,
		LimiterConfig: limiter.Config{
			MaxConnections: 1000,
		},
		// each time heartbeat is called we insert data into the channel.
		// this is used to make sure that heartbeat started and the clusters
		// are registered in the auth server
		OnHeartbeat: func(err error) {
			select {
			case heartbeatsWaitChannel <- struct{}{}:
			default:
			}
		},
		GetRotation:              func(role types.SystemRole) (*types.Rotation, error) { return &types.Rotation{}, nil },
		ResourceMatchers:         nil,
		OnReconcile:              func(kc types.KubeClusters) {},
		KubernetesServersWatcher: watcher,
		InventoryHandle:          inventoryHandle,
	})
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)
		err := kubeServer.Serve(listener)
		// ignore server closed error returned when .Close is called.
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		errChan <- err
	}()
	// wait for the watcher to init or it may race with test cleanup.
	require.NoError(t, watcher.WaitInitialization())

	// Waits for len(clusters) heartbeats to start
	heartbeatsToExpect := len(cfg.clusters)
	for i := 0; i < heartbeatsToExpect; i++ {
		<-heartbeatsWaitChannel
	}

	return kubeServer, func() error {
		return <-errChan
	}, listener.Addr()
}

func marshalRBACError(t *testing.T, w http.ResponseWriter) {
	status := &metav1.Status{
		Message: "pods is forbidden: User \"USER\" cannot list resource \"pods\" in API group \"\" in the namespace \"default\"",
		Code:    http.StatusForbidden,
		Reason:  metav1.StatusReasonForbidden,
		Status:  metav1.StatusFailure,
	}

	data, err := runtime.Encode(statusCodecs.LegacyCodec(), status)
	require.NoError(t, err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_, err = w.Write(data)
	require.NoError(t, err)
}

func marshalValidPodList(t *testing.T, w http.ResponseWriter) {
	result := &corev1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodList",
			APIVersion: "v1",
		},
		ListMeta: metav1.ListMeta{
			SelfLink:           "",
			ResourceVersion:    "1231415",
			Continue:           "",
			RemainingItemCount: nil,
		},
		Items: []corev1.Pod{},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err := json.NewEncoder(w).Encode(result)
	require.NoError(t, err)
}

// statusScheme is private scheme for the decoding here until someone fixes the TODO in NewConnection
var statusScheme = runtime.NewScheme()

// ParameterCodec knows about query parameters used with the meta v1 API spec.
var statusCodecs = serializer.NewCodecFactory(statusScheme)

func init() {
	statusScheme.AddUnversionedTypes(metav1.SchemeGroupVersion,
		&metav1.Status{},
	)
}

// TestForwardingTraces checks that the userContext includes the ID of the
// access request after it has been consumed and the web session has been renewed.
func TestForwardingTraces(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	p := env.proxies[0]

	newRequest := func(t *testing.T) *http.Request {
		req, err := http.NewRequest(http.MethodGet, "", nil)
		require.NoError(t, err)

		return req
	}

	// Span captured from the UI which was marshaled by opentelemetry-js.
	const rawSpan = `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"web-ui"}},{"key":"telemetry.sdk.language","value":{"stringValue":"webjs"}},{"key":"telemetry.sdk.name","value":{"stringValue":"opentelemetry"}},{"key":"telemetry.sdk.version","value":{"stringValue":"1.7.0"}},{"key":"service.version","value":{"stringValue":"0.1.0"}}],"droppedAttributesCount":0},"scopeSpans":[{"scope":{"name":"@opentelemetry/instrumentation-fetch","version":"0.33.0"},"spans":[{"traceId":"255c8d876e7dbf3707ee8451ad518652","spanId":"d9edec516e598d8c","name":"HTTP GET","kind":3,"startTimeUnixNano":1668606426497000000,"endTimeUnixNano":1668502943215499800,"attributes":[{"key":"component","value":{"stringValue":"fetch"}},{"key":"http.method","value":{"stringValue":"GET"}},{"key":"http.url","value":{"stringValue":"https://proxy.example.com/v1/webapi/user/status"}},{"key":"http.status_code","value":{"intValue":0}},{"key":"http.status_text","value":{"stringValue":"Failed to fetch"}},{"key":"http.host","value":{"stringValue":"proxy.example.com"}},{"key":"http.scheme","value":{"stringValue":"https"}},{"key":"http.user_agent","value":{"stringValue":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36    "}},{"key":"http.response_content_length","value":{"intValue":0}}],"droppedAttributesCount":0,"events":[{"attributes":[],"name":"fetchStart","timeUnixNano":1668502943210900000,"droppedAttributesCount":0},{"attributes":[],"name":"domainLookupStart","timeUnixNano":1668502687491499800,"droppedAttributesCount":0},{"attributes":[],"name":"domainLookupEnd","timeUnixNano":1668502687491499800,"droppedAttributesCount":0},{"attributes":[],"name":"connectStart","timeUnixNano":1668502687491499800,"droppedAttributesCount":0},{"attributes":[],"name":"secureConnectionStart","timeUnixNano":1668502687491499800,"droppedAttributesCount":0},{"attributes":[],"name":"connectEnd","timeUnixNano":1668502687491499800,"droppedAttributesCount":0},{"attributes":[],"name":"requestStart","timeUnixNano":1668502687491499800,"droppedAttributesCount":0},{"attributes":[],"name":"responseStart","timeUnixNano":1668502687491499800,"droppedAttributesCount":0},{"attributes":[],"name":"responseEnd","timeUnixNano":1668502943215100000,"droppedAttributesCount":0}],"droppedEventsCount":0,"status":{"code":0},"links":[],"droppedLinksCount":0}]}]}]}`

	// dummy span with arbitrary data, needed to be able to protojson.Marshal in tests
	span := &tracepb.TracesData{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcev1.Resource{
					Attributes: []*commonv1.KeyValue{
						{
							Key: "test",
							Value: &commonv1.AnyValue{
								Value: &commonv1.AnyValue_IntValue{
									IntValue: 0,
								},
							},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte{1, 2, 3, 4},
								SpanId:            []byte{5, 6, 7, 8},
								TraceState:        "",
								ParentSpanId:      []byte{9, 10, 11, 12},
								Name:              "test",
								Kind:              tracepb.Span_SPAN_KIND_CLIENT,
								StartTimeUnixNano: uint64(time.Now().Add(-1 * time.Minute).Unix()),
								EndTimeUnixNano:   uint64(time.Now().Unix()),
								Attributes: []*commonv1.KeyValue{
									{
										Key: "test",
										Value: &commonv1.AnyValue{
											Value: &commonv1.AnyValue_IntValue{
												IntValue: 11,
											},
										},
									},
								},
								Status: &tracepb.Status{
									Message: "success!",
									Code:    tracepb.Status_STATUS_CODE_OK,
								},
							},
						},
					},
				},
			},
		},
	}

	cases := []struct {
		name      string
		req       func(t *testing.T) *http.Request
		assertion func(t *testing.T, spans []*tracepb.ResourceSpans, err error, code int)
	}{
		{
			name: "no data",
			req: func(t *testing.T) *http.Request {
				r := newRequest(t)
				r.Body = io.NopCloser(&bytes.Buffer{})
				return r
			},
			assertion: func(t *testing.T, spans []*tracepb.ResourceSpans, err error, code int) {
				require.NoError(t, err)
				require.Equal(t, http.StatusBadRequest, code)
				require.Empty(t, spans)
			},
		},
		{
			name: "invalid data",
			req: func(t *testing.T) *http.Request {
				r := newRequest(t)
				r.Body = io.NopCloser(strings.NewReader(`{"test": "abc"}`))
				return r
			},
			assertion: func(t *testing.T, spans []*tracepb.ResourceSpans, err error, code int) {
				require.NoError(t, err)
				require.Equal(t, http.StatusBadRequest, code)
				require.Empty(t, spans)
			},
		},
		{
			name: "no traces",
			req: func(t *testing.T) *http.Request {
				r := newRequest(t)

				raw, err := protojson.Marshal(&tracepb.ResourceSpans{})
				require.NoError(t, err)
				r.Body = io.NopCloser(bytes.NewBuffer(raw))

				return r
			},
			assertion: func(t *testing.T, spans []*tracepb.ResourceSpans, err error, code int) {
				require.NoError(t, err)
				require.Equal(t, http.StatusBadRequest, code)
				require.Empty(t, spans)
			},
		},
		{
			name: "traces with base64 encoded ids",
			req: func(t *testing.T) *http.Request {
				r := newRequest(t)

				// Since the id fields of the span are all []byte,
				// protojson will marshal them into base64
				raw, err := protojson.Marshal(span)
				require.NoError(t, err)
				r.Body = io.NopCloser(bytes.NewBuffer(raw))

				return r
			},
			assertion: func(t *testing.T, spans []*tracepb.ResourceSpans, err error, code int) {
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, code)
				require.Len(t, spans, 1)
				require.Empty(t, cmp.Diff(span.ResourceSpans[0], spans[0], protocmp.Transform()))
			},
		},
		{
			name: "traces with hex encoded ids",
			req: func(t *testing.T) *http.Request {
				r := newRequest(t)

				// The id fields are hex encoded instead of base64 encoded
				// by opentelemetry-js for the rawSpan
				r.Body = io.NopCloser(strings.NewReader(rawSpan))

				return r
			},
			assertion: func(t *testing.T, spans []*tracepb.ResourceSpans, err error, code int) {
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, code)
				require.Len(t, spans, 1)

				var data tracepb.TracesData
				require.NoError(t, protojson.Unmarshal([]byte(rawSpan), &data))

				// compare the spans, but ignore the ids since we know that the rawSpan
				// has hex encoded ids and protojson.Unmarshal will give us an invalid value
				require.Empty(t, cmp.Diff(data.ResourceSpans[0], spans[0], protocmp.Transform(), protocmp.IgnoreFields(&tracepb.Span{}, "span_id", "trace_id")))

				// compare the ids separately
				sid1 := spans[0].ScopeSpans[0].Spans[0].SpanId
				tid1 := spans[0].ScopeSpans[0].Spans[0].TraceId

				sid2 := data.ResourceSpans[0].ScopeSpans[0].Spans[0].SpanId
				tid2 := data.ResourceSpans[0].ScopeSpans[0].Spans[0].TraceId

				require.Equal(t, hex.EncodeToString(sid1), base64.StdEncoding.EncodeToString(sid2))
				require.Equal(t, hex.EncodeToString(tid1), base64.StdEncoding.EncodeToString(tid2))
			},
		},
	}

	// NOTE: resetting the tracing client prevents
	// the test cases from running in parallel
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			clt := &mockTraceClient{
				uploadReceived: make(chan struct{}),
			}
			p.handler.handler.cfg.TraceClient = clt

			recorder := httptest.NewRecorder()

			// use the handler directly because there is no easy way to pipe in our tracing
			// data using the pack client in a format that would match the webui.
			_, err := p.handler.handler.traces(recorder, tt.req(t), nil, nil)

			// if traces weren't uploaded perform the assertion
			// without waiting for traces to be forwarded
			if err != nil || recorder.Code != http.StatusOK {
				tt.assertion(t, clt.spans, err, recorder.Code)
				return
			}

			// traces are forwarded in a goroutine, wait for them
			// to be received by the trace client before doing the
			// assertion
			select {
			case <-clt.uploadReceived:
			case <-time.After(10 * time.Second):
				t.Fatal("Timed out waiting for traces to be uploaded")
			}

			tt.assertion(t, clt.spans, err, recorder.Code)
		})
	}
}

type mockPROXYSigner struct{}

func (m *mockPROXYSigner) SignPROXYHeader(source, destination net.Addr) ([]byte, error) {
	return nil, nil
}

type mockTraceClient struct {
	uploadError    error
	uploadReceived chan struct{}
	spans          []*tracepb.ResourceSpans
}

func (m *mockTraceClient) Start(ctx context.Context) error {
	return nil
}

func (m *mockTraceClient) Stop(ctx context.Context) error {
	return nil
}

func (m *mockTraceClient) UploadTraces(ctx context.Context, protoSpans []*tracepb.ResourceSpans) error {
	m.spans = append(m.spans, protoSpans...)
	m.uploadReceived <- struct{}{}
	return m.uploadError
}

func TestLogout(t *testing.T) {
	ctx := context.Background()
	t.Parallel()
	env := newWebPack(t, 2)

	// create a logged in user for proxy 1
	pack := env.proxies[0].authPack(t, "llama", nil /* roles */)

	// ensure the client is authenticated
	re, err := pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "sites"), url.Values{})
	require.NoError(t, err)
	var clusters []webui.Cluster
	require.NoError(t, json.Unmarshal(re.Bytes(), &clusters))
	require.Len(t, clusters, 1)

	// create a client for proxy 2 with the token and cookies from proxy 1
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	jar.SetCookies(&env.proxies[1].webURL, pack.cookies)
	clt2 := env.proxies[1].newClient(t, roundtrip.BearerAuth(pack.session.Token), roundtrip.CookieJar(jar))

	// ensure the second client is authenticated
	re, err = clt2.Get(ctx, clt2.Endpoint("webapi", "sites"), url.Values{})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(re.Bytes(), &clusters))
	require.Len(t, clusters, 1)

	// logout from proxy 1
	_, err = pack.clt.Delete(ctx, pack.clt.Endpoint("webapi", "sessions", "web"))
	require.NoError(t, err)

	// ensure proxy 1 invalidated the session
	_, err = pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "sites"), url.Values{})
	require.Error(t, err)
	require.ErrorIs(t, err, trace.AccessDenied("missing session cookie"))

	// should still be authenticated to proxy 2 until the expiration loop kicks in
	re, err = clt2.Get(ctx, clt2.Endpoint("webapi", "sites"), url.Values{})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(re.Bytes(), &clusters))
	require.Len(t, clusters, 1)

	// advance the clock to fire the expiration ticker
	env.clock.Advance(time.Second)

	// wait for the expiration loop to purge the session
	require.Eventually(t, func() bool {
		return env.proxies[1].handler.handler.auth.ActiveSessions() == 0
	}, 5*time.Second, 100*time.Millisecond)

	// ensure proxy 2 invalidated the session
	_, err = clt2.Get(ctx, clt2.Endpoint("webapi", "sites"), url.Values{})
	require.True(t, trace.IsAccessDenied(err))
	require.ErrorIs(t, err, trace.AccessDenied("need auth"))
}

// initGRPCServer creates a gRPC server serving on the provided listener.
func initGRPCServer(t *testing.T, env *webPack, listener net.Listener) {
	clusterName := env.server.ClusterName()
	// Auth client, lock watcher and authorizer for Kube proxy.
	proxyAuthClient, err := env.server.TLS.NewClient(auth.TestBuiltin(types.RoleProxy))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, proxyAuthClient.Close()) })

	serverIdentity, err := auth.NewServerIdentity(env.server.Auth(), uuid.NewString(), types.RoleProxy)
	require.NoError(t, err)
	tlsConfig, err := serverIdentity.TLSConfig(nil)
	require.NoError(t, err)
	limiter, err := limiter.NewLimiter(limiter.Config{MaxConnections: 100})
	require.NoError(t, err)
	// authMiddleware authenticates request assuming TLS client authentication
	// adds authentication information to the context
	// and passes it to the API server
	authMiddleware := &auth.Middleware{
		ClusterName:   clusterName,
		Limiter:       limiter,
		AcceptedUsage: []string{teleport.UsageKubeOnly},
	}

	tlsConf := copyAndConfigureTLS(tlsConfig, proxyAuthClient, clusterName)
	creds, err := auth.NewTransportCredentials(auth.TransportCredentialsConfig{
		TransportCredentials: credentials.NewTLS(tlsConf),
		UserGetter:           authMiddleware,
	})
	require.NoError(t, err)

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(authMiddleware.UnaryInterceptors()...),
		grpc.ChainStreamInterceptor(authMiddleware.StreamInterceptors()...),
		grpc.Creds(creds),
	)

	kubeproto.RegisterKubeServiceServer(grpcServer, &fakeKubeService{})
	errC := make(chan error, 1)
	t.Cleanup(func() {
		grpcServer.GracefulStop()
		require.NoError(t, <-errC)
	})
	go func() {
		err := grpcServer.Serve(listener)
		errC <- trace.Wrap(err)
	}()
}

// copyAndConfigureTLS can be used to copy and modify an existing *tls.Config
// for Teleport application proxy servers.
func copyAndConfigureTLS(config *tls.Config, accessPoint authclient.AccessCache, clusterName string) *tls.Config {
	tlsConfig := config.Clone()

	// Require clients to present a certificate
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	// Configure function that will be used to fetch the CA that signed the
	// client's certificate to verify the chain presented. If the client does not
	// pass in the cluster name, this functions pulls back all CA to try and
	// match the certificate presented against any CA.
	tlsConfig.GetConfigForClient = authclient.WithClusterCAs(tlsConfig.Clone(), accessPoint, clusterName, slog.Default())

	return tlsConfig
}

type fakeKubeService struct {
	kubeproto.UnimplementedKubeServiceServer
}

func (s *fakeKubeService) ListKubernetesResources(ctx context.Context, req *kubeproto.ListKubernetesResourcesRequest) (*kubeproto.ListKubernetesResourcesResponse, error) {
	switch req.GetResourceType() {
	case types.KindKubePod:
		{
			return &kubeproto.ListKubernetesResourcesResponse{
				Resources: []*types.KubernetesResourceV1{
					{
						Kind: types.KindKubePod,
						Metadata: types.Metadata{
							Name: "test-pod",
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "default",
						},
					},
					{
						Kind: types.KindKubePod,
						Metadata: types.Metadata{
							Name: "test-pod2",
							Labels: map[string]string{
								"app": "test2",
							},
						},
						Spec: types.KubernetesResourceSpecV1{
							Namespace: "default",
						},
					},
				},
				TotalCount: 2,
			}, nil
		}
	case types.KindKubeNamespace:
		{
			return &kubeproto.ListKubernetesResourcesResponse{
				Resources: []*types.KubernetesResourceV1{
					{
						Kind: types.KindNamespace,
						Metadata: types.Metadata{
							Name: "default",
							Labels: map[string]string{
								"app": "test",
							},
						},
					},
				},
				TotalCount: 1,
			}, nil
		}
	default:
		return nil, trace.BadParameter("kubernetes resource kind %q is not mocked", req.GetResourceType())
	}
}

func TestWebSocketAuthenticateRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "test-user@example.com", nil)
	for _, tc := range []struct {
		name              string
		serverExpectError string
		expectResponse    wsStatus
		token             string
		writeTimeout      func()
		readTimeout       func()
	}{
		{
			name: "valid token",
			expectResponse: wsStatus{
				Type:   "create_session_response",
				Status: "ok",
			},
			token: pack.session.Token,
		},
		{
			name:              "invalid token",
			serverExpectError: "not found",
			expectResponse: wsStatus{
				Type:    "create_session_response",
				Status:  "error",
				Message: "invalid token",
			},
			token: "honk",
		},
		{
			name:              "server read timeout",
			serverExpectError: "i/o timeout",
			token:             pack.session.Token,
			readTimeout: func() {
				<-time.After(wsIODeadline * 3)
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sctx, ws, err := proxy.handler.handler.AuthenticateRequestWS(w, r)
				if err != nil {
					if tc.serverExpectError == "" {
						t.Errorf("unexpected error: %v", err)
					}
					if !strings.Contains(err.Error(), tc.serverExpectError) {
						t.Errorf("unexpected error: %v", err)
						return
					}
					return
				}
				t.Cleanup(func() { ws.Close() })
				if tc.serverExpectError != "" {
					t.Errorf("expected error, got nil")
					return
				}

				clt, err := sctx.GetClient()
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				_, err = clt.GetDomainName(ctx)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
			}))

			header := http.Header{}
			for _, cookie := range pack.cookies {
				header.Add("Cookie", cookie.String())
			}

			u := strings.Replace(server.URL, "http:", "ws:", 1)
			conn, resp, err := websocket.DefaultDialer.Dial(u, header)
			require.NoError(t, err)
			t.Cleanup(func() { conn.Close() })
			t.Cleanup(func() { resp.Body.Close() })

			if tc.readTimeout != nil {
				tc.readTimeout()
			}
			err = conn.WriteJSON(wsBearerToken{
				Token: tc.token,
			})
			require.NoError(t, err)
			if tc.readTimeout != nil {
				return // Reading will fail as the server will have closed the connection
			}

			var status wsStatus
			err = conn.ReadJSON(&status)
			require.NoError(t, err)
			require.Equal(t, tc.expectResponse, status)
		})
	}
}

func TestGetKubeExecClusterData(t *testing.T) {
	testCases := []struct {
		name          string
		listenerMode  types.ProxyListenerMode
		proxyKubeAddr string
		proxyWebAddr  string

		expectedServerAddr string
		expectedTLSName    string

		errCheck require.ErrorAssertionFunc
	}{
		{
			name:               "separate, regular addr",
			listenerMode:       types.ProxyListenerMode_Separate,
			proxyKubeAddr:      "kube.example.com:555",
			expectedServerAddr: "https://kube.example.com:555",
		},
		{
			name:               "separate, specified ip addr",
			listenerMode:       types.ProxyListenerMode_Separate,
			proxyKubeAddr:      "1.2.3.4:444",
			expectedServerAddr: "https://1.2.3.4:444",
		},
		{
			name:               "separate, unspecified ip addr",
			listenerMode:       types.ProxyListenerMode_Separate,
			proxyKubeAddr:      "0.0.0.0:444",
			expectedServerAddr: "https://localhost:444",
		},
		{
			name:               "multiplex, regular proxy web addr",
			listenerMode:       types.ProxyListenerMode_Multiplex,
			proxyWebAddr:       "web.example.com:777",
			expectedServerAddr: "https://web.example.com:777",
			expectedTLSName:    "kube-teleport-proxy-alpn.web.example.com",
		},
		{
			name:               "multiplex, proxy web addr unspecified ip",
			listenerMode:       types.ProxyListenerMode_Multiplex,
			proxyWebAddr:       "0.0.0.0:888",
			expectedServerAddr: "https://localhost:888",
			expectedTLSName:    "kube-teleport-proxy-alpn.teleport.cluster.local",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			h := Handler{}
			if tt.proxyWebAddr != "" {
				h.cfg.ProxyWebAddr = *utils.MustParseAddr(tt.proxyWebAddr)
			}
			if tt.proxyKubeAddr != "" {
				h.cfg.ProxyKubeAddr = *utils.MustParseAddr(tt.proxyKubeAddr)
			}

			netConfig := types.ClusterNetworkingConfigV2{Spec: types.ClusterNetworkingConfigSpecV2{
				ProxyListenerMode: tt.listenerMode,
			}}

			serverAddr, tlsServerName, err := h.getKubeExecClusterData(&netConfig)
			if tt.errCheck != nil {
				tt.errCheck(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedServerAddr, serverAddr)
			require.Equal(t, tt.expectedTLSName, tlsServerName)
		})
	}
}

// TestSimultaneousAuthenticateRequest ensures that multiple authenticated
// requests do not race to create a SessionContext. This would happen when
// Proxies were deployed behind a round-robin load balancer. Only the Proxy
// that handled the login will have initially created a SessionContext for
// the particular user+session. All subsequent requests to the other Proxies
// in the load balancer pool attempt to create a SessionContext in
// [Handler.AuthenticateRequest] if one didn't already exist. If the web UI
// makes enough requests fast enough it can result in the Proxy trying to
// create multiple SessionContext for a user+session. Since only one SessionContext
// is stored in the sessionCache all previous SessionContext and their underlying
// auth client get closed, which results in an ugly and unfriendly
// `grpc: the client connection is closing` error banner on the web webui.
func TestSimultaneousAuthenticateRequest(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)

	proxy := env.proxies[0]

	// Authenticate to get a session token and cookies.
	pack := proxy.authPack(t, "test-user@example.com", nil)

	// Reset the sessions so that all future requests will race to create
	// a new SessionContext for the user + session pair to simulate multiple
	// proxies behind a load balancer.
	proxy.handler.handler.auth.sessions = map[string]*SessionContext{}

	// Create a request with the auth header and cookies for the session.
	endpoint := pack.clt.Endpoint("webapi", "sites")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	require.NoError(t, err)

	req.Header.Set("Authorization", "Bearer "+pack.session.Token)
	for _, cookie := range pack.cookies {
		req.AddCookie(cookie)
	}

	// Spawn several requests in parallel and attempt to use the auth client.
	type res struct {
		domain string
		err    error
	}
	const requests = 10
	respC := make(chan res, requests)
	for i := 0; i < requests; i++ {
		go func() {
			sctx, err := proxy.handler.handler.AuthenticateRequest(httptest.NewRecorder(), req.Clone(ctx), false)
			if err != nil {
				respC <- res{err: err}
				return
			}

			clt, err := sctx.GetClient()
			if err != nil {
				respC <- res{err: err}
				return
			}

			domain, err := clt.GetDomainName(ctx)
			respC <- res{domain: domain, err: err}
		}()
	}

	// Assert that all requests were successful and each one was able to
	// get the domain name without its auth client being closed.
	for i := 0; i < requests; i++ {
		select {
		case res := <-respC:
			require.NoError(t, res.err)
			require.Equal(t, "localhost", res.domain)
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for responses")
		}
	}
}

// mockedPingTestProxy is a test proxy with a mocked Ping method
type mockedPingTestProxy struct {
	authclient.ClientI
	mockedPing func(ctx context.Context) (authproto.PingResponse, error)
}

func (m mockedPingTestProxy) Ping(ctx context.Context) (authproto.PingResponse, error) {
	return m.mockedPing(ctx)
}

// TestModeratedSession validates that peers are able to start Moderated
// Sessions and remain in the waiting room until the required number of
// moderators are present. Only when the moderator is present the peer
// is allowed to access the host and start entering input and receiving
// output until the moderator terminates the session.
func TestModeratedSession(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

	s := newWebSuiteWithConfig(t, webSuiteConfig{disableDiskBasedRecording: true})

	peerRole, err := types.NewRole("moderated", types.RoleSpecV6{
		Allow: types.RoleConditions{
			RequireSessionJoin: []*types.SessionRequirePolicy{
				{
					Name:   "moderated",
					Filter: "contains(user.roles, \"moderator\")",
					Kinds:  []string{string(types.SSHSessionKind)},
					Count:  1,
					Modes:  []string{string(types.SessionModeratorMode)},
				},
			},
		},
	})
	require.NoError(t, err)
	peerRole, err = s.server.Auth().UpsertRole(s.ctx, peerRole)
	require.NoError(t, err)

	moderatorRole, err := types.NewRole("moderator", types.RoleSpecV6{
		Allow: types.RoleConditions{
			JoinSessions: []*types.SessionJoinPolicy{
				{
					Name:  "moderated",
					Roles: []string{peerRole.GetName()},
					Kinds: []string{string(types.SSHSessionKind)},
					Modes: []string{string(types.SessionModeratorMode), string(types.SessionObserverMode)},
				},
			},
		},
	})
	require.NoError(t, err)
	moderatorRole, err = s.server.Auth().UpsertRole(s.ctx, moderatorRole)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(s.ctx)
	t.Cleanup(cancel)

	peerTerm, err := connectToHost(ctx, connectConfig{
		pack:  s.authPack(t, "foo", peerRole.GetName()),
		host:  s.node.ID(),
		proxy: s.webServer.Listener.Addr().String(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, peerTerm.Close()) })

	require.NoError(t, waitForOutput(peerTerm, "Teleport > Waiting for required participants..."), "waiting for peer to enter session")

	moderatorTerm, err := connectToHost(ctx, connectConfig{
		pack:            s.authPack(t, "bar", moderatorRole.GetName()),
		host:            s.node.ID(),
		proxy:           s.webServer.Listener.Addr().String(),
		sessionID:       peerTerm.GetSession().ID,
		participantMode: types.SessionModeratorMode,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, moderatorTerm.Close()) })

	require.NoError(t, waitForOutput(peerTerm, "Teleport > Connecting to node over SSH"), "waiting for peer connection to node after moderator joins")

	// here we intentionally run a command where the output we're looking
	// for is not present in the command itself
	_, err = io.WriteString(peerTerm, "echo llxmx | sed 's/x/a/g'\r\n")
	require.NoError(t, err)
	require.NoError(t, waitForOutput(peerTerm, "llama"), "waiting for output on peer terminal")
	require.NoError(t, waitForOutput(moderatorTerm, "llama"), "waiting for output on moderator terminal")

	// the moderator terminates the session
	_, err = io.WriteString(moderatorTerm, "t")
	require.NoError(t, err)

	require.NoError(t, waitForOutput(moderatorTerm, "Stopping session..."), "waiting for moderator to terminate session")
	require.NoError(t, waitForOutput(peerTerm, "Process exited with status 255"), "waiting for peer session to be terminated")
}

// TestModeratedSessionWithMFA validates the same behavior as TestModeratedSession while
// also ensuring that MFA is performed prior to accessing the host and that periodic
// presence checks are performed by the moderator. When presence checks are not performed
// the session is aborted.
func TestModeratedSessionWithMFA(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

	const RPID = "localhost"

	presenceClock := clockwork.NewFakeClock()
	s := newWebSuiteWithConfig(t, webSuiteConfig{
		clock:                     clockwork.NewFakeClockAt(presenceClock.Now()),
		disableDiskBasedRecording: true,
		authPreferenceSpec: &types.AuthPreferenceSpecV2{
			Type:           constants.Local,
			ConnectorName:  constants.PasswordlessConnector,
			SecondFactor:   constants.SecondFactorOn,
			RequireMFAType: types.RequireMFAType_SESSION,
			Webauthn: &types.Webauthn{
				RPID: RPID,
			},
		},
		presenceChecker: func(ctx context.Context, term io.Writer, maintainer client.PresenceMaintainer, sessionID string, mfaCeremony *mfa.Ceremony, opts ...client.PresenceOption) error {
			return trace.Wrap(client.RunPresenceTask(ctx, term, maintainer, sessionID, mfaCeremony, client.WithPresenceClock(presenceClock)))
		},
	})

	peerRole, err := types.NewRole("moderated", types.RoleSpecV6{
		Allow: types.RoleConditions{
			RequireSessionJoin: []*types.SessionRequirePolicy{
				{
					Name:   "moderated",
					Filter: "contains(user.roles, \"moderator\")",
					Kinds:  []string{string(types.SSHSessionKind)},
					Count:  1,
					Modes:  []string{string(types.SessionModeratorMode)},
				},
			},
		},
	})
	require.NoError(t, err)

	moderatorRole, err := types.NewRole("moderator", types.RoleSpecV6{
		Allow: types.RoleConditions{
			JoinSessions: []*types.SessionJoinPolicy{
				{
					Name:  "moderated",
					Roles: []string{peerRole.GetName()},
					Kinds: []string{string(types.SSHSessionKind)},
					Modes: []string{string(types.SessionModeratorMode), string(types.SessionObserverMode)},
				},
			},
		},
	})
	require.NoError(t, err)

	peer := s.authPackWithMFA(t, "foo", peerRole)
	moderator := s.authPackWithMFA(t, "bar", moderatorRole)

	ctx, cancel := context.WithCancel(s.ctx)
	t.Cleanup(cancel)

	peerTerm, err := connectToHost(ctx, connectConfig{
		pack:  peer,
		host:  s.node.ID(),
		proxy: s.webServer.Listener.Addr().String(),
		mfaCeremony: func(challenge client.MFAAuthenticateChallenge) []byte {
			res, err := peer.device.SolveAuthn(&authproto.MFAAuthenticateChallenge{
				WebauthnChallenge: wantypes.CredentialAssertionToProto(challenge.WebauthnChallenge),
			})
			require.NoError(t, err)

			webauthnResBytes, err := json.Marshal(wantypes.CredentialAssertionResponseFromProto(res.GetWebauthn()))
			require.NoError(t, err)

			envelope := &terminal.Envelope{
				Version: defaults.WebsocketVersion,
				Type:    defaults.WebsocketMFAChallenge,
				Payload: string(webauthnResBytes),
			}
			envelopeBytes, err := proto.Marshal(envelope)
			require.NoError(t, err)

			return envelopeBytes
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, peerTerm.Close()) })

	require.NoError(t, waitForOutput(peerTerm, "Teleport > Waiting for required participants..."), "waiting for peer to start session")

	moderatorTerm, err := connectToHost(ctx, connectConfig{
		pack:            moderator,
		host:            s.node.ID(),
		proxy:           s.webServer.Listener.Addr().String(),
		sessionID:       peerTerm.GetSession().ID,
		participantMode: types.SessionModeratorMode,
		mfaCeremony: func(challenge client.MFAAuthenticateChallenge) []byte {
			res, err := moderator.device.SolveAuthn(&authproto.MFAAuthenticateChallenge{
				WebauthnChallenge: wantypes.CredentialAssertionToProto(challenge.WebauthnChallenge),
			})
			require.NoError(t, err)

			webauthnResBytes, err := json.Marshal(wantypes.CredentialAssertionResponseFromProto(res.GetWebauthn()))
			require.NoError(t, err)

			envelope := &terminal.Envelope{
				Version: defaults.WebsocketVersion,
				Type:    defaults.WebsocketMFAChallenge,
				Payload: string(webauthnResBytes),
			}
			envelopeBytes, err := proto.Marshal(envelope)
			require.NoError(t, err)

			return envelopeBytes
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, moderatorTerm.Close()) })

	require.NoError(t, waitForOutput(peerTerm, "Teleport > Connecting to node over SSH"), "waiting for peer to connect after moderator joins")

	// here we intentionally run a command where the output we're looking
	// for is not present in the command itself
	_, err = io.WriteString(peerTerm, "echo llxmx | sed 's/x/a/g'\r\n")
	require.NoError(t, err)
	require.NoError(t, waitForOutput(peerTerm, "llama"), "waiting for output in peer terminal")
	require.NoError(t, waitForOutput(moderatorTerm, "llama"), "waiting for output in moderator terminal")

	// run the presence check a few times
	for i := 0; i < 3; i++ {
		presenceClock.BlockUntil(1)
		presenceClock.Advance(30 * time.Second)
		require.NoError(t, waitForOutput(moderatorTerm, "Teleport > Please tap your MFA key"), "waiting for moderator mfa prompt")

		challenge, err := moderatorTerm.stream.ReadChallenge(protobufMFACodec{})
		require.NoError(t, err)

		res, err := moderator.device.SolveAuthn(challenge)
		require.NoError(t, err)

		webauthnResBytes, err := json.Marshal(wantypes.CredentialAssertionResponseFromProto(res.GetWebauthn()))
		require.NoError(t, err)

		envelope := &terminal.Envelope{
			Version: defaults.WebsocketVersion,
			Type:    defaults.WebsocketMFAChallenge,
			Payload: string(webauthnResBytes),
		}
		envelopeBytes, err := proto.Marshal(envelope)
		require.NoError(t, err)

		require.NoError(t, moderatorTerm.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes))
	}

	// Advance the clock far enough in the future to make the moderator stale
	// which will terminate the session - because the clock is used by ALL server
	// components, it's not practical to use BlockUntil here, so we use EventuallyWithT instead.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		s.clock.Advance(3 * time.Minute)
		assert.NoError(t, waitForOutputWithDuration(moderatorTerm, "wait: remote command exited without exit status or exit signal", 3*time.Second))
		assert.NoError(t, waitForOutputWithDuration(peerTerm, "Process exited with status 255", 3*time.Second))
	}, 15*time.Second, 500*time.Millisecond)
}

type proxyClientMock struct {
	authclient.ClientI
	tokens map[string]types.ProvisionToken
}

// GetToken returns provisioning token
func (pc *proxyClientMock) GetToken(_ context.Context, token string) (types.ProvisionToken, error) {
	tok, ok := pc.tokens[token]
	if ok {
		return tok, nil
	}

	return nil, trace.NotFound("%s", token)
}

func (pc *proxyClientMock) DeleteToken(_ context.Context, token string) error {
	_, ok := pc.tokens[token]
	if ok {
		delete(pc.tokens, token)
		return nil
	}
	return trace.NotFound("%s", token)
}

func Test_consumeTokenForAPICall(t *testing.T) {
	pc := &proxyClientMock{tokens: map[string]types.ProvisionToken{}}

	tests := []struct {
		name     string
		getToken func() (string, types.ProvisionToken)
		wantErr  require.ErrorAssertionFunc
	}{
		{
			name: "missing token is rejected",
			getToken: func() (string, types.ProvisionToken) {
				return "fake", nil
			},
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name: "valid token is accepted",
			getToken: func() (string, types.ProvisionToken) {
				tok, err := types.NewProvisionToken(uuid.New().String(), []types.SystemRole{types.RoleDatabase}, time.Now().Add(time.Hour))
				require.NoError(t, err)
				pc.tokens[tok.GetName()] = tok
				return tok.GetName(), tok
			},
		},
		{
			name: "token with no expiry is accepted",
			getToken: func() (string, types.ProvisionToken) {
				tok, err := types.NewProvisionToken(uuid.New().String(), []types.SystemRole{types.RoleDatabase}, time.Time{})
				require.NoError(t, err)
				pc.tokens[tok.GetName()] = tok
				return tok.GetName(), tok
			},
		},
		{
			name: "expired token is rejected",
			getToken: func() (string, types.ProvisionToken) {
				tok, err := types.NewProvisionToken(uuid.New().String(), []types.SystemRole{types.RoleDatabase}, time.Now().Add(-time.Hour))
				require.NoError(t, err)
				pc.tokens[tok.GetName()] = tok
				return tok.GetName(), tok
			},
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "expired token")
			},
		},
		{
			name: "token with invalid join type is rejected",
			getToken: func() (string, types.ProvisionToken) {
				tok, err := types.NewProvisionTokenFromSpec("ec2-token", time.Now().Add(time.Hour), types.ProvisionTokenSpecV2{
					Roles:      []types.SystemRole{types.RoleDatabase},
					Allow:      []*types.TokenRule{{AWSAccount: "1234"}},
					JoinMethod: types.JoinMethodEC2,
				})

				require.NoError(t, err)
				pc.tokens[tok.GetName()] = tok
				return tok.GetName(), tok
			},
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "unexpected join method \"ec2\" for token \"ec2-token\"")
			},
		},
	}

	tokenExists := func(tokenName string) bool {
		tok, _ := pc.GetToken(context.Background(), tokenName)
		return tok != nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokName, tok := tt.getToken()
			tokenInitiallyPresent := tokenExists(tokName)
			result, err := consumeTokenForAPICall(context.Background(), pc, tokName)
			if tt.wantErr != nil {
				tt.wantErr(t, err)
				// verify that if token was present, then it has not been deleted.
				require.Equal(t, tokenInitiallyPresent, tokenExists(tokName))
			} else {
				require.NoError(t, err)
				require.Equal(t, tok, result)
				// verify that token does not exist now, even if it did.
				require.False(t, tokenExists(tokName))
			}
		})
	}
}

// TestGithubAuthCompat asserts that the proxy can handle github login flows
// from tsh clients that send split keys in the new format, or only a single key
// in the old format.
func TestGithubAuthCompat(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)

	env.server.Auth().GithubUserAndTeamsOverride = func() (*auth.GithubUserResponse, []auth.GithubTeamResponse, error) {
		return &auth.GithubUserResponse{
				Login: "alice",
			}, []auth.GithubTeamResponse{{
				Name: "devs",
				Slug: "devs",
				Org:  auth.GithubOrgResponse{Login: "octocats"},
			}}, nil
	}

	connector, err := types.NewGithubConnector("github", types.GithubConnectorSpecV3{
		ClientID:     "12345",
		ClientSecret: "678910",
		RedirectURL:  "https://proxy.example.com/v1/webapi/github/callback",
		Display:      "Github",
		TeamsToRoles: []types.TeamRolesMapping{
			{
				Organization: "octocats",
				Team:         "devs",
				Roles:        []string{"access"},
			},
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertGithubConnector(ctx, connector)
	require.NoError(t, err)

	access, err := types.NewRole("access", types.RoleSpecV6{})
	require.NoError(t, err)
	_, err = env.server.Auth().CreateRole(ctx, access)
	require.NoError(t, err)

	clt := env.proxies[0].newClient(t)

	// The response in the callback will be encrypted with this key.
	secretKey, err := secret.NewKey()
	require.NoError(t, err)

	sshKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(sshKey.Public())
	require.NoError(t, err)
	sshPubBytes := ssh.MarshalAuthorizedKey(sshPub)

	tlsKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPubBytes, err := keys.MarshalPublicKey(tlsKey.Public())
	require.NoError(t, err)

	for _, tc := range []struct {
		desc                         string
		pubKey, sshPubKey, tlsPubKey []byte
		expectLoginError             string
		expectSSHSubjectKey          ssh.PublicKey
		expectTLSSubjectKey          crypto.PublicKey
	}{
		{
			desc:             "no keys",
			expectLoginError: "Failed to login",
		},
		{
			desc:                "single key",
			pubKey:              sshPubBytes,
			expectSSHSubjectKey: sshPub,
			expectTLSSubjectKey: sshKey.Public(),
		},
		{
			desc:                "split keys",
			sshPubKey:           sshPubBytes,
			tlsPubKey:           tlsPubBytes,
			expectSSHSubjectKey: sshPub,
			expectTLSSubjectKey: tlsKey.Public(),
		},
		{
			desc:                "only ssh",
			sshPubKey:           sshPubBytes,
			expectSSHSubjectKey: sshPub,
		},
		{
			desc:                "only tls",
			tlsPubKey:           tlsPubBytes,
			expectTLSSubjectKey: tlsKey.Public(),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// Initiate the github SSO login.
			loginResponse, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "github", "login", "console"), client.SSOLoginConsoleReq{
				RedirectURL: (&url.URL{
					Scheme:   "http",
					Host:     "localhost",
					Path:     "callback",
					RawQuery: url.Values{"secret_key": []string{secretKey.String()}}.Encode(),
				}).String(),
				ConnectorID: "github",
				SSOUserPublicKeys: client.SSOUserPublicKeys{
					PublicKey: tc.pubKey,
					SSHPubKey: tc.sshPubKey,
					TLSPubKey: tc.tlsPubKey,
				},
				CertTTL: time.Hour,
			})
			if tc.expectLoginError != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.expectLoginError)
				return
			}
			require.NoError(t, err)

			// Retrieve the state token from the redirect URL in the response.
			stateToken := stateTokenFromConsoleLoginResponse(t, loginResponse.Bytes())

			// Send the callback to the proxy to complete the login.
			callbackResponse, err := clt.Get(ctx, clt.Endpoint("webapi", "github", "callback"), url.Values{
				"state": []string{stateToken},
				"code":  []string{"success"},
			})
			require.NoError(t, err)

			// Retrieve the login response from the callback response HTML.
			sshLoginResponse := sshLoginResponseFromCallbackResponse(t, callbackResponse.Reader(), secretKey)

			// Make sure the subject key in the issued SSH cert matches the
			// expected key and didn't get accidentally switched.
			if tc.expectSSHSubjectKey != nil {
				sshCert, err := apisshutils.ParseCertificate(sshLoginResponse.Cert)
				require.NoError(t, err)
				require.Equal(t, tc.expectSSHSubjectKey, sshCert.Key)
			} else {
				// No SSH cert should be issued if we didn't ask for one.
				require.Empty(t, sshLoginResponse.Cert)
			}

			// Make sure the subject key in the issued TLS cert matches the
			// expected key and didn't get accidentally switched.
			if tc.expectTLSSubjectKey != nil {
				tlsCert, err := tlsca.ParseCertificatePEM(sshLoginResponse.TLSCert)
				require.NoError(t, err)
				require.Equal(t, tc.expectTLSSubjectKey, tlsCert.PublicKey)
			} else {
				// No TLS cert should be issued if we didn't ask for one.
				require.Empty(t, sshLoginResponse.TLSCert)
			}
		})
	}
}

func stateTokenFromConsoleLoginResponse(t *testing.T, responseBody []byte) string {
	var loginResp client.SSOLoginConsoleResponse
	require.NoError(t, json.Unmarshal(responseBody, &loginResp))
	u, err := url.Parse(loginResp.RedirectURL)
	require.NoError(t, err, "parsing login redirect URL")
	q, err := url.ParseQuery(u.RawQuery)
	require.NoError(t, err, "parsing login redirect URL query params")
	require.Contains(t, q, "state", "redirect query did not contain state token")
	return q["state"][0]
}

// The login response we're after with the certs in it is:
// - JSON
// - encrypted with [secretKey]
// - in a query param
// - in a redirect URL
// - in an HTML document
func sshLoginResponseFromCallbackResponse(t *testing.T, responseBody io.Reader, secretKey secret.Key) *authclient.SSHLoginResponse {
	// First pull the URL from the HTML meta redirect.
	redirectURL, err := app.GetURLFromMetaRedirect(responseBody)
	require.NoError(t, err)

	// Then get the encrypted JSON out of the query param.
	u, err := url.Parse(redirectURL)
	require.NoError(t, err)
	require.Contains(t, u.Query(), "response", "redirect query did not contain response")
	ciphertext := u.Query().Get("response")

	// Then unencrypt.
	callbackPlaintext, err := secretKey.Open([]byte(ciphertext))
	require.NoError(t, err, "unencrypting github callback response")

	// Then unmarshal the JSON.
	var sshLoginResponse authclient.SSHLoginResponse
	require.NoError(t, json.Unmarshal(callbackPlaintext, &sshLoginResponse))
	return &sshLoginResponse
}

func TestGithubConnector(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)

	proxy := env.proxies[0]

	// Authenticate to get a session token and cookies.
	pack := proxy.authPack(t, "test-user@example.com", nil)

	expected, err := types.NewGithubConnector("github", types.GithubConnectorSpecV3{
		ClientID:     "12345",
		ClientSecret: "678910",
		RedirectURL:  "https://proxy.example.com/v1/webapi/github/callback",
		Display:      "Github",
		TeamsToRoles: []types.TeamRolesMapping{
			{
				Organization: "acme",
				Team:         "users",
				Roles:        []string{"access", "editor", "auditor"},
			},
		},
	})
	require.NoError(t, err, "creating initial connector resource")

	createPayload := func(connector types.GithubConnector) webui.ResourceItem {
		raw, err := services.MarshalGithubConnector(connector, services.PreserveRevision())
		require.NoError(t, err, "marshaling connector")

		return webui.ResourceItem{
			Kind:    types.KindGithubConnector,
			Name:    connector.GetName(),
			Content: string(raw),
		}
	}

	unmarshalResponse := func(resp []byte) types.GithubConnector {
		var item webui.ResourceItem
		require.NoError(t, json.Unmarshal(resp, &item), "response from server contained an invalid resource item")

		var conn types.GithubConnectorV3
		require.NoError(t, yaml.Unmarshal([]byte(item.Content), &conn), "resource item content was not a github connector")
		return &conn
	}

	// Create the initial connector.
	resp, err := pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "github"), createPayload(expected))
	require.NoError(t, err, "expected creating the initial connector to succeed")
	require.Equal(t, http.StatusOK, resp.Code(), "unexpected status code creating connector")

	created := unmarshalResponse(resp.Bytes())

	// Validate that creating the connector again fails.
	resp, err = pack.clt.PostJSON(ctx, pack.clt.Endpoint("webapi", "github"), createPayload(expected))
	assert.Error(t, err, "expected an error creating a duplicate connector")
	assert.True(t, trace.IsAlreadyExists(err), "expected an already exists error got %T", err)
	assert.Equal(t, http.StatusConflict, resp.Code(), "unexpected status code creating duplicate connector")

	// Update the connector.
	created.SetDisplay("test")
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("webapi", "github", expected.GetName()), createPayload(created))
	require.NoError(t, err, "unexpected error updating the connector")
	require.Equal(t, http.StatusOK, resp.Code(), "unexpected status code updating the connector")

	updated := unmarshalResponse(resp.Bytes())

	require.Empty(t, cmp.Diff(created, updated, cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
		cmpopts.IgnoreFields(types.GithubConnectorSpecV3{}, "Display", "ClientSecret"),
	))
	require.NotEqual(t, expected.GetDisplay(), updated.GetDisplay(), "expected update to modify the display name")
	require.Equal(t, "test", updated.GetDisplay(), "display name should have been updated to test. got %s", updated.GetDisplay())

	// Validate that a stale revision prevents updates.
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("webapi", "github", expected.GetName()), createPayload(expected))
	assert.Error(t, err, "expected an error updating a connector with a stale revision")
	assert.True(t, trace.IsCompareFailed(err), "expected a compare failed error got %T", err)
	assert.Equal(t, http.StatusPreconditionFailed, resp.Code(), "unexpected status code updating the connector")

	// Validate that renaming the connector prevents updates.
	updated.SetName(uuid.NewString())
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("webapi", "github", expected.GetName()), createPayload(updated))
	assert.Error(t, err, "expected and error when renaming a connector")
	assert.True(t, trace.IsBadParameter(err), "expected a bad parameter error got %T", err)
	assert.Equal(t, http.StatusBadRequest, resp.Code(), "unexpected status code updating the connector")

	// Validate that updating a nonexistent connector fails.
	updated.SetName(uuid.NewString())
	resp, err = pack.clt.PutJSON(ctx, pack.clt.Endpoint("webapi", "github", updated.GetName()), createPayload(updated))
	assert.Error(t, err, "expected updating a nonexistent connector to fail")
	assert.True(t, trace.IsCompareFailed(err), "expected a compare failed error got %T", err)
	assert.Equal(t, http.StatusPreconditionFailed, resp.Code(), "unexpected status code updating the connector")

	// Validate that the connector can be deleted
	_, err = pack.clt.Delete(ctx, pack.clt.Endpoint("webapi", "github", expected.GetName()))
	require.NoError(t, err, "unexpected error deleting connector")

	resp, err = pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "github"), nil)
	assert.NoError(t, err, "unexpected error listing github connectors")

	authConnectorsResp := webui.ListAuthConnectorsResponse{}
	require.NoError(t, json.Unmarshal(resp.Bytes(), &authConnectorsResp), "invalid response received")

	assert.Empty(t, authConnectorsResp.Connectors)
	assert.Equal(t, http.StatusOK, resp.Code(), "unexpected status code getting connectors")
}

func TestCalculateSSHLogins(t *testing.T) {
	cases := []struct {
		name              string
		allowedLogins     []string
		grantedPrincipals []string
		expectedLogins    []string
	}{
		{
			name:              "no matching logins",
			allowedLogins:     []string{"llama"},
			grantedPrincipals: []string{"fish"},
		},
		{
			name:              "identical logins",
			allowedLogins:     []string{"llama", "shark", "goose"},
			grantedPrincipals: []string{"shark", "goose", "llama"},
			expectedLogins:    []string{"goose", "shark", "llama"},
		},
		{
			name:              "subset of logins",
			allowedLogins:     []string{"llama"},
			grantedPrincipals: []string{"shark", "goose", "llama"},
			expectedLogins:    []string{"llama"},
		},
		{
			name:              "no allowed logins",
			grantedPrincipals: []string{"shark", "goose", "llama"},
		},
		{
			name:          "no granted logins",
			allowedLogins: []string{"shark", "goose", "llama"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			identity := &tlsca.Identity{Principals: test.grantedPrincipals}

			logins, err := calculateSSHLogins(identity, test.allowedLogins)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(logins, test.expectedLogins, cmpopts.SortSlices(func(a, b string) bool {
				return strings.Compare(a, b) < 0
			})))
		})
	}
}

func TestCalculateAppLogins(t *testing.T) {
	cases := []struct {
		name           string
		allowedLogins  []string
		expectedLogins []string
		loginGetter    loginGetterFunc
	}{
		{
			name:           "allowed logins",
			allowedLogins:  []string{"llama", "fish", "dog"},
			expectedLogins: []string{"llama", "fish", "dog"},
			loginGetter: func(_ services.AccessCheckable) ([]string, error) {
				return nil, nil
			},
		},
		{
			name: "no allowed logins",
			loginGetter: func(_ services.AccessCheckable) ([]string, error) {
				return nil, nil
			},
		},
		{
			name:           "no allowed logins with fallback",
			expectedLogins: []string{"apple", "banana"},
			loginGetter: func(_ services.AccessCheckable) ([]string, error) {
				return []string{"apple", "banana"}, nil
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			logins, err := calculateAppLogins(test.loginGetter, &types.AppServerV3{}, test.allowedLogins)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(logins, test.expectedLogins, cmpopts.SortSlices(func(a, b string) bool {
				return strings.Compare(a, b) < 0
			})))
		})
	}
}

type loginGetterFunc func(resource services.AccessCheckable) ([]string, error)

func (f loginGetterFunc) GetAllowedLoginsForResource(resource services.AccessCheckable) ([]string, error) {
	return f(resource)
}

func TestWebSocketClosedBeforeSSHSessionCreated(t *testing.T) {
	t.Parallel()
	s := newWebSuiteWithConfig(t, webSuiteConfig{disableDiskBasedRecording: true})

	ctx, cancel := context.WithCancel(s.ctx)
	t.Cleanup(cancel)

	pack := s.authPack(t, "foo")

	req := TerminalRequest{
		Server: s.node.ID(),
		Login:  pack.login,
		Term: session.TerminalParams{
			W: 100,
			H: 100,
		},
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	u := url.URL{
		Host:   s.webServer.Listener.Addr().String(),
		Scheme: client.WSS,
		Path:   "/v1/webapi/sites/-current-/connect/ws",
	}

	q := u.Query()
	q.Set("params", string(data))
	u.RawQuery = q.Encode()

	header := http.Header{}
	header.Add("Origin", "http://localhost")
	for _, cookie := range pack.cookies {
		header.Add("Cookie", cookie.String())
	}

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	ws, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		var sb strings.Builder
		sb.WriteString("websocket dial")
		if resp != nil {
			fmt.Fprintf(&sb, "; status code %v;", resp.StatusCode)
			fmt.Fprintf(&sb, "headers: %v; body: ", resp.Header)
			io.Copy(&sb, resp.Body)
		}
		require.NoError(t, err, sb.String())
	}
	require.NoError(t, resp.Body.Close())

	require.NoError(t, makeAuthReqOverWS(ws, pack.session.Token))

	wsClosedChan := make(chan struct{})

	// Create a stream that closes the web socket when the server writes the session metadata
	// to the client. At this point, the SSH connection to the target should be in flight but
	// not yet established.
	stream := terminal.NewStream(ctx, terminal.StreamConfig{
		WS:     ws,
		Logger: utils.NewSlogLoggerForTests(),
		Handlers: map[string]terminal.WSHandlerFunc{
			defaults.WebsocketSessionMetadata: func(ctx context.Context, envelope terminal.Envelope) {
				if envelope.Type != defaults.WebsocketSessionMetadata {
					return
				}

				var sessResp siteSessionGenerateResponse
				if err := json.Unmarshal([]byte(envelope.Payload), &sessResp); err != nil {
					return
				}

				assert.NoError(t, ws.WriteControl(websocket.CloseMessage, nil, time.Now().Add(time.Second)))
				close(wsClosedChan)
			},
		},
	})
	t.Cleanup(func() { require.NoError(t, stream.Close()) })

	// Set a read deadline to unblock ReadAll below in the event of a bug
	// preventing the ws from closing above.
	require.NoError(t, stream.SetReadDeadline(time.Now().Add(30*time.Second)))

	// Wait for the web socket to be closed above.
	select {
	case <-wsClosedChan:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for session metadata")
	}

	// Validate that the SSH connection is terminated in response to the WS closing.
	require.Eventually(t, func() bool {
		return s.node.ActiveConnections() == 0
	}, 10*time.Second, 100*time.Millisecond)

	// Validate that reading nothing was permitted.
	out, err := io.ReadAll(stream)
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestUnstartedServerShutdown(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)
	srv, err := NewServer(ServerConfig{
		Server:  &http.Server{},
		Handler: s.webHandler,
	})

	require.NoError(t, err)

	// Shutdown the server before starting it shouldn't panic.
	require.NoError(t, srv.Shutdown(context.Background()))
}

func Test_setEntitlementsWithLegacyLogic(t *testing.T) {
	tests := []struct {
		name            string
		config          *webclient.WebConfig
		clusterFeatures authproto.Features
		expected        *webclient.WebConfig
	}{
		{
			name:   "sets entitlements",
			config: &webclient.WebConfig{},
			clusterFeatures: authproto.Features{
				AccessControls: false,
				AccessGraph:    false,
				AccessList: &clientproto.AccessListFeature{
					CreateLimit: 10,
				},
				AccessMonitoring: &clientproto.AccessMonitoringFeature{
					Enabled:             false,
					MaxReportRangeLimit: 20,
				},
				AccessMonitoringConfigured: false,
				AccessRequests: &clientproto.AccessRequestsFeature{
					MonthlyRequestLimit: 30,
				},
				AdvancedAccessWorkflows: false,
				App:                     false,
				Assist:                  false,
				AutomaticUpgrades:       false,
				Cloud:                   false,
				CustomTheme:             "theme",
				DB:                      false,
				Desktop:                 false,
				DeviceTrust: &clientproto.DeviceTrustFeature{
					Enabled:           false,
					DevicesUsageLimit: 40,
				},
				ExternalAuditStorage:   false,
				FeatureHiding:          false,
				HSM:                    false,
				IdentityGovernance:     false,
				IsStripeManaged:        false,
				IsUsageBased:           false,
				JoinActiveSessions:     false,
				Kubernetes:             false,
				MobileDeviceManagement: false,
				OIDC:                   false,
				Plugins:                false,
				Policy:                 nil,
				ProductType:            0,
				Questionnaire:          false,
				RecoveryCodes:          false,
				SAML:                   false,
				SupportType:            0,
				// since present, becomes source of truth  for feature enablement
				Entitlements: map[string]*authproto.EntitlementInfo{
					string(entitlements.AccessLists):            {Enabled: true, Limit: 99},
					string(entitlements.AccessMonitoring):       {Enabled: true, Limit: 99},
					string(entitlements.AccessRequests):         {Enabled: true, Limit: 99},
					string(entitlements.App):                    {Enabled: true, Limit: 99},
					string(entitlements.CloudAuditLogRetention): {Enabled: true, Limit: 99},
					string(entitlements.DB):                     {Enabled: true, Limit: 99},
					string(entitlements.Desktop):                {Enabled: true, Limit: 99},
					string(entitlements.DeviceTrust):            {Enabled: true, Limit: 99},
					string(entitlements.ExternalAuditStorage):   {Enabled: true, Limit: 99},
					string(entitlements.FeatureHiding):          {Enabled: true, Limit: 99},
					string(entitlements.HSM):                    {Enabled: true, Limit: 99},
					string(entitlements.Identity):               {Enabled: true, Limit: 99},
					string(entitlements.JoinActiveSessions):     {Enabled: true, Limit: 99},
					string(entitlements.K8s):                    {Enabled: true, Limit: 99},
					string(entitlements.MobileDeviceManagement): {Enabled: true, Limit: 99},
					string(entitlements.OIDC):                   {Enabled: true, Limit: 99},
					string(entitlements.OktaSCIM):               {Enabled: true, Limit: 99},
					string(entitlements.OktaUserSync):           {Enabled: true, Limit: 99},
					string(entitlements.Policy):                 {Enabled: true, Limit: 99},
					string(entitlements.SAML):                   {Enabled: true, Limit: 99},
					string(entitlements.SessionLocks):           {Enabled: true, Limit: 99},
					string(entitlements.UpsellAlert):            {Enabled: true, Limit: 99},
					string(entitlements.UsageReporting):         {Enabled: true, Limit: 99},
					string(entitlements.LicenseAutoUpdate):      {Enabled: true, Limit: 99},
				},
			},
			expected: &webclient.WebConfig{
				Auth:                           webclient.WebConfigAuthSettings{},
				AutomaticUpgrades:              false,
				AutomaticUpgradesTargetVersion: "",
				CanJoinSessions:                false,
				CustomTheme:                    "",
				Edition:                        "",
				IsCloud:                        false,
				IsDashboard:                    false,
				IsStripeManaged:                false,
				IsTeam:                         false,
				IsUsageBasedBilling:            false,
				PlayableDatabaseProtocols:      nil,
				PremiumSupport:                 false,
				ProxyClusterName:               "",
				Questionnaire:                  false,
				RecoveryCodesEnabled:           false,
				TunnelPublicAddress:            "",
				UI:                             webclient.UIConfig{},
				// set by the equivalent entitlement value
				AccessRequests:           true,
				ExternalAuditStorage:     true,
				HideInaccessibleFeatures: true,
				IsIGSEnabled:             true,
				IsPolicyEnabled:          true,
				JoinActiveSessions:       true,
				MobileDeviceManagement:   true,
				OIDC:                     true,
				SAML:                     true,
				TrustedDevices:           true,
				FeatureLimits: webclient.FeatureLimits{
					AccessListCreateLimit:               99,
					AccessMonitoringMaxReportRangeLimit: 99,
					AccessRequestMonthlyRequestLimit:    99,
				},
				Entitlements: map[string]webclient.EntitlementInfo{
					string(entitlements.AccessLists):            {Enabled: true, Limit: 99},
					string(entitlements.AccessMonitoring):       {Enabled: true, Limit: 99},
					string(entitlements.AccessRequests):         {Enabled: true, Limit: 99},
					string(entitlements.App):                    {Enabled: true, Limit: 99},
					string(entitlements.CloudAuditLogRetention): {Enabled: true, Limit: 99},
					string(entitlements.DB):                     {Enabled: true, Limit: 99},
					string(entitlements.Desktop):                {Enabled: true, Limit: 99},
					string(entitlements.DeviceTrust):            {Enabled: true, Limit: 99},
					string(entitlements.ExternalAuditStorage):   {Enabled: true, Limit: 99},
					string(entitlements.FeatureHiding):          {Enabled: true, Limit: 99},
					string(entitlements.HSM):                    {Enabled: true, Limit: 99},
					string(entitlements.Identity):               {Enabled: true, Limit: 99},
					string(entitlements.JoinActiveSessions):     {Enabled: true, Limit: 99},
					string(entitlements.K8s):                    {Enabled: true, Limit: 99},
					string(entitlements.MobileDeviceManagement): {Enabled: true, Limit: 99},
					string(entitlements.OIDC):                   {Enabled: true, Limit: 99},
					string(entitlements.OktaSCIM):               {Enabled: true, Limit: 99},
					string(entitlements.OktaUserSync):           {Enabled: true, Limit: 99},
					string(entitlements.Policy):                 {Enabled: true, Limit: 99},
					string(entitlements.SAML):                   {Enabled: true, Limit: 99},
					string(entitlements.SessionLocks):           {Enabled: true, Limit: 99},
					string(entitlements.UpsellAlert):            {Enabled: true, Limit: 99},
					string(entitlements.UsageReporting):         {Enabled: true, Limit: 99},
					string(entitlements.LicenseAutoUpdate):      {Enabled: true, Limit: 99},
				},
			},
		},
		{
			name:   "sets legacy features when no entitlements are present (Identity true)",
			config: &webclient.WebConfig{},
			clusterFeatures: authproto.Features{
				AccessControls:             false,
				AccessGraph:                false,
				AccessMonitoringConfigured: false,
				AdvancedAccessWorkflows:    false,
				App:                        false,
				Assist:                     false,
				AutomaticUpgrades:          false,
				Cloud:                      false,
				CustomTheme:                "",
				DB:                         false,
				Desktop:                    false,
				HSM:                        false,
				IsStripeManaged:            false,
				IsUsageBased:               false,
				Kubernetes:                 false,
				Plugins:                    false,
				ProductType:                0,
				Questionnaire:              false,
				RecoveryCodes:              false,
				SupportType:                0,
				// not present
				Entitlements: nil,
				// will set equivalent entitlement values
				ExternalAuditStorage:   true,
				FeatureHiding:          true,
				IdentityGovernance:     true,
				JoinActiveSessions:     true,
				MobileDeviceManagement: true,
				OIDC:                   true,
				SAML:                   true,
				AccessRequests: &clientproto.AccessRequestsFeature{
					MonthlyRequestLimit: 88,
				},
				AccessList: &clientproto.AccessListFeature{
					CreateLimit: 88,
				},
				AccessMonitoring: &clientproto.AccessMonitoringFeature{
					Enabled:             true,
					MaxReportRangeLimit: 88,
				},
				DeviceTrust: &clientproto.DeviceTrustFeature{
					Enabled:           true,
					DevicesUsageLimit: 88,
				},
				Policy: &clientproto.PolicyFeature{
					Enabled: true,
				},
			},
			expected: &webclient.WebConfig{
				Auth:                           webclient.WebConfigAuthSettings{},
				AutomaticUpgrades:              false,
				AutomaticUpgradesTargetVersion: "",
				CanJoinSessions:                false,
				CustomTheme:                    "",
				Edition:                        "",
				IsCloud:                        false,
				IsDashboard:                    false,
				IsStripeManaged:                false,
				IsTeam:                         false,
				IsUsageBasedBilling:            false,
				PlayableDatabaseProtocols:      nil,
				PremiumSupport:                 false,
				ProxyClusterName:               "",
				Questionnaire:                  false,
				RecoveryCodesEnabled:           false,
				TunnelPublicAddress:            "",
				UI:                             webclient.UIConfig{},
				// set to legacy feature
				AccessRequests:           true,
				ExternalAuditStorage:     true,
				HideInaccessibleFeatures: true,
				IsIGSEnabled:             true,
				IsPolicyEnabled:          true,
				JoinActiveSessions:       true,
				MobileDeviceManagement:   true,
				OIDC:                     true,
				SAML:                     true,
				TrustedDevices:           true,
				FeatureLimits: webclient.FeatureLimits{
					AccessListCreateLimit:               88,
					AccessMonitoringMaxReportRangeLimit: 88,
					AccessRequestMonthlyRequestLimit:    88,
				},
				Entitlements: map[string]webclient.EntitlementInfo{
					// no equivalent legacy feature; defaults to false
					string(entitlements.App):                    {Enabled: false},
					string(entitlements.CloudAuditLogRetention): {Enabled: false},
					string(entitlements.DB):                     {Enabled: false},
					string(entitlements.Desktop):                {Enabled: false},
					string(entitlements.HSM):                    {Enabled: false},
					string(entitlements.K8s):                    {Enabled: false},
					string(entitlements.UpsellAlert):            {Enabled: false},
					string(entitlements.UsageReporting):         {Enabled: false},
					string(entitlements.LicenseAutoUpdate):      {Enabled: false},

					// set to equivalent legacy feature
					string(entitlements.ExternalAuditStorage):   {Enabled: true},
					string(entitlements.FeatureHiding):          {Enabled: true},
					string(entitlements.Identity):               {Enabled: true},
					string(entitlements.JoinActiveSessions):     {Enabled: true},
					string(entitlements.MobileDeviceManagement): {Enabled: true},
					string(entitlements.OIDC):                   {Enabled: true},
					string(entitlements.Policy):                 {Enabled: true},
					string(entitlements.SAML):                   {Enabled: true},
					// set to legacy feature "IsIGSEnabled"; true so set true and clear limits
					string(entitlements.AccessLists):      {Enabled: true},
					string(entitlements.AccessMonitoring): {Enabled: true},
					string(entitlements.AccessRequests):   {Enabled: true},
					string(entitlements.DeviceTrust):      {Enabled: true},
					string(entitlements.OktaSCIM):         {Enabled: true},
					string(entitlements.OktaUserSync):     {Enabled: true},
					string(entitlements.SessionLocks):     {Enabled: true},
				},
			},
		},
		{
			name:   "sets legacy features when no entitlements are present (Identity false)",
			config: &webclient.WebConfig{},
			clusterFeatures: authproto.Features{
				AccessControls:             false,
				AccessGraph:                false,
				AccessMonitoringConfigured: false,
				AdvancedAccessWorkflows:    false,
				App:                        false,
				Assist:                     false,
				AutomaticUpgrades:          false,
				Cloud:                      false,
				CustomTheme:                "",
				DB:                         false,
				Desktop:                    false,
				HSM:                        false,
				IsStripeManaged:            false,
				IsUsageBased:               false,
				Kubernetes:                 false,
				Plugins:                    false,
				ProductType:                0,
				Questionnaire:              false,
				RecoveryCodes:              false,
				SupportType:                0,
				// not present
				Entitlements: nil,
				// will set equivalent entitlement values
				ExternalAuditStorage:   true,
				FeatureHiding:          true,
				IdentityGovernance:     false,
				JoinActiveSessions:     true,
				MobileDeviceManagement: true,
				OIDC:                   true,
				SAML:                   true,
				AccessRequests: &clientproto.AccessRequestsFeature{
					MonthlyRequestLimit: 88,
				},
				AccessList: &clientproto.AccessListFeature{
					CreateLimit: 88,
				},
				AccessMonitoring: &clientproto.AccessMonitoringFeature{
					Enabled:             true,
					MaxReportRangeLimit: 88,
				},
				DeviceTrust: &clientproto.DeviceTrustFeature{
					Enabled:           true,
					DevicesUsageLimit: 88,
				},
				Policy: &clientproto.PolicyFeature{
					Enabled: true,
				},
			},
			expected: &webclient.WebConfig{
				Auth:                           webclient.WebConfigAuthSettings{},
				AutomaticUpgrades:              false,
				AutomaticUpgradesTargetVersion: "",
				CanJoinSessions:                false,
				CustomTheme:                    "",
				Edition:                        "",
				IsCloud:                        false,
				IsDashboard:                    false,
				IsStripeManaged:                false,
				IsTeam:                         false,
				IsUsageBasedBilling:            false,
				PlayableDatabaseProtocols:      nil,
				PremiumSupport:                 false,
				ProxyClusterName:               "",
				Questionnaire:                  false,
				RecoveryCodesEnabled:           false,
				TunnelPublicAddress:            "",
				UI:                             webclient.UIConfig{},
				// set to legacy feature
				AccessRequests:           true,
				ExternalAuditStorage:     true,
				HideInaccessibleFeatures: true,
				IsIGSEnabled:             false,
				IsPolicyEnabled:          true,
				JoinActiveSessions:       true,
				MobileDeviceManagement:   true,
				OIDC:                     true,
				SAML:                     true,
				TrustedDevices:           true,
				FeatureLimits: webclient.FeatureLimits{
					AccessListCreateLimit:               88,
					AccessMonitoringMaxReportRangeLimit: 88,
					AccessRequestMonthlyRequestLimit:    88,
				},
				Entitlements: map[string]webclient.EntitlementInfo{
					// no equivalent legacy feature; defaults to false
					string(entitlements.App):                    {Enabled: false},
					string(entitlements.CloudAuditLogRetention): {Enabled: false},
					string(entitlements.DB):                     {Enabled: false},
					string(entitlements.Desktop):                {Enabled: false},
					string(entitlements.HSM):                    {Enabled: false},
					string(entitlements.K8s):                    {Enabled: false},
					string(entitlements.UpsellAlert):            {Enabled: false},
					string(entitlements.UsageReporting):         {Enabled: false},

					// set to equivalent legacy feature
					string(entitlements.ExternalAuditStorage):   {Enabled: true},
					string(entitlements.FeatureHiding):          {Enabled: true},
					string(entitlements.Identity):               {Enabled: false},
					string(entitlements.JoinActiveSessions):     {Enabled: true},
					string(entitlements.MobileDeviceManagement): {Enabled: true},
					string(entitlements.OIDC):                   {Enabled: true},
					string(entitlements.Policy):                 {Enabled: true},
					string(entitlements.SAML):                   {Enabled: true},
					// set to legacy feature "IsIGSEnabled"; false so set value and keep limits
					string(entitlements.AccessLists):       {Enabled: true, Limit: 88},
					string(entitlements.AccessMonitoring):  {Enabled: true, Limit: 88},
					string(entitlements.AccessRequests):    {Enabled: true, Limit: 88},
					string(entitlements.DeviceTrust):       {Enabled: true, Limit: 88},
					string(entitlements.OktaSCIM):          {Enabled: false},
					string(entitlements.OktaUserSync):      {Enabled: false},
					string(entitlements.SessionLocks):      {Enabled: false},
					string(entitlements.LicenseAutoUpdate): {Enabled: false},
				},
			},
		},
		{
			name: "retains non-feature field values",
			config: &webclient.WebConfig{
				Auth: webclient.WebConfigAuthSettings{
					LocalAuthEnabled:  true,
					AllowPasswordless: true,
					MOTD:              "some-message",
				},
				PlayableDatabaseProtocols: []string{"play-able"},
				UI: webclient.UIConfig{
					ScrollbackLines: 10,
					ShowResources:   "foo",
				},
				Edition:                        "edition",
				TunnelPublicAddress:            "0000",
				AutomaticUpgradesTargetVersion: "99",
				CustomTheme:                    "theme",
				CanJoinSessions:                true,
				IsCloud:                        true,
				RecoveryCodesEnabled:           true,
				IsDashboard:                    true,
				IsUsageBasedBilling:            true,
				AutomaticUpgrades:              true,
				Questionnaire:                  true,
				IsStripeManaged:                true,
				PremiumSupport:                 true,
			},
			clusterFeatures: authproto.Features{
				DeviceTrust:      &clientproto.DeviceTrustFeature{},
				AccessRequests:   &clientproto.AccessRequestsFeature{},
				AccessList:       &clientproto.AccessListFeature{},
				AccessMonitoring: &clientproto.AccessMonitoringFeature{},
				Policy:           &clientproto.PolicyFeature{},
			},
			expected: &webclient.WebConfig{
				Auth: webclient.WebConfigAuthSettings{
					LocalAuthEnabled:  true,
					AllowPasswordless: true,
					MOTD:              "some-message",
				},
				PlayableDatabaseProtocols: []string{"play-able"},
				UI: webclient.UIConfig{
					ScrollbackLines: 10,
					ShowResources:   "foo",
				},
				Edition:                        "edition",
				TunnelPublicAddress:            "0000",
				AutomaticUpgradesTargetVersion: "99",
				CustomTheme:                    "theme",
				CanJoinSessions:                true,
				IsCloud:                        true,
				RecoveryCodesEnabled:           true,
				IsDashboard:                    true,
				IsUsageBasedBilling:            true,
				AutomaticUpgrades:              true,
				Questionnaire:                  true,
				IsStripeManaged:                true,
				PremiumSupport:                 true,
				// Default; not under test
				ProxyClusterName:         "",
				FeatureLimits:            webclient.FeatureLimits{},
				IsTeam:                   false,
				HideInaccessibleFeatures: false,
				IsIGSEnabled:             false,
				IsPolicyEnabled:          false,
				ExternalAuditStorage:     false,
				JoinActiveSessions:       false,
				AccessRequests:           false,
				TrustedDevices:           false,
				OIDC:                     false,
				SAML:                     false,
				MobileDeviceManagement:   false,
				Entitlements: map[string]webclient.EntitlementInfo{
					string(entitlements.AccessLists):            {Enabled: true}, // AccessLists had no previous behavior from an enablement perspective; so we default to true
					string(entitlements.AccessMonitoring):       {Enabled: false},
					string(entitlements.AccessRequests):         {Enabled: false},
					string(entitlements.App):                    {Enabled: false},
					string(entitlements.CloudAuditLogRetention): {Enabled: false},
					string(entitlements.DB):                     {Enabled: false},
					string(entitlements.Desktop):                {Enabled: false},
					string(entitlements.DeviceTrust):            {Enabled: false},
					string(entitlements.ExternalAuditStorage):   {Enabled: false},
					string(entitlements.FeatureHiding):          {Enabled: false},
					string(entitlements.HSM):                    {Enabled: false},
					string(entitlements.Identity):               {Enabled: false},
					string(entitlements.JoinActiveSessions):     {Enabled: false},
					string(entitlements.K8s):                    {Enabled: false},
					string(entitlements.MobileDeviceManagement): {Enabled: false},
					string(entitlements.OIDC):                   {Enabled: false},
					string(entitlements.OktaSCIM):               {Enabled: false},
					string(entitlements.OktaUserSync):           {Enabled: false},
					string(entitlements.Policy):                 {Enabled: false},
					string(entitlements.SAML):                   {Enabled: false},
					string(entitlements.SessionLocks):           {Enabled: false},
					string(entitlements.UpsellAlert):            {Enabled: false},
					string(entitlements.UsageReporting):         {Enabled: false},
					string(entitlements.LicenseAutoUpdate):      {Enabled: false},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEntitlementsWithLegacyLogic(tt.config, tt.clusterFeatures)

			assert.Equal(t, tt.expected, tt.config)
		})
	}
}
