/*
Copyright 2017-2019 Gravitational, Inc.

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

package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// TestAuthServerConfig is auth server test config
type TestAuthServerConfig struct {
	// ClusterName is cluster name
	ClusterName string
	// Dir is directory for local backend
	Dir string
	// AcceptedUsage is an optional list of restricted
	// server usage
	AcceptedUsage []string
	// CipherSuites is the list of ciphers that the server supports.
	CipherSuites []uint16
	// Clock is used to control time in tests.
	Clock clockwork.FakeClock
	// ClusterNetworkingConfig allows a test to change the default
	// networking configuration.
	ClusterNetworkingConfig types.ClusterNetworkingConfig
}

// CheckAndSetDefaults checks and sets defaults
func (cfg *TestAuthServerConfig) CheckAndSetDefaults() error {
	if cfg.ClusterName == "" {
		cfg.ClusterName = "localhost"
	}
	if cfg.Dir == "" {
		return trace.BadParameter("missing parameter Dir")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewFakeClock()
	}
	if len(cfg.CipherSuites) == 0 {
		cfg.CipherSuites = utils.DefaultCipherSuites()
	}
	return nil
}

// CreateUploaderDir creates directory for file uploader service
func CreateUploaderDir(dir string) error {
	// DELETE IN(5.1.0)
	// this folder is no longer used past 5.0 upgrade
	err := os.MkdirAll(filepath.Join(dir, teleport.LogsDir, teleport.ComponentUpload,
		events.SessionLogsDir, apidefaults.Namespace), teleport.SharedDirMode)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	err = os.MkdirAll(filepath.Join(dir, teleport.LogsDir, teleport.ComponentUpload,
		events.StreamingLogsDir, apidefaults.Namespace), teleport.SharedDirMode)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	return nil
}

// TestServer defines the set of server components for a test
type TestServer struct {
	TLS        *TestTLSServer
	AuthServer *TestAuthServer
}

// TestServerConfig defines the configuration for all server components
type TestServerConfig struct {
	// Auth specifies the auth server configuration
	Auth TestAuthServerConfig
	// TLS optionally specifies the configuration for the TLS server.
	// If unspecified, will be generated automatically
	TLS *TestTLSServerConfig
}

// NewTestServer creates a new test server configuration
func NewTestServer(cfg TestServerConfig) (*TestServer, error) {
	authServer, err := NewTestAuthServer(cfg.Auth)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var tlsServer *TestTLSServer
	if cfg.TLS != nil {
		tlsServer, err = NewTestTLSServer(*cfg.TLS)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		tlsServer, err = authServer.NewTestTLSServer()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return &TestServer{
		AuthServer: authServer,
		TLS:        tlsServer,
	}, nil
}

// Auth returns the underlying auth server instance
func (a *TestServer) Auth() *Server {
	return a.AuthServer.AuthServer
}

func (a *TestServer) NewClient(identity TestIdentity) (*Client, error) {
	return a.TLS.NewClient(identity)
}

func (a *TestServer) ClusterName() string {
	return a.TLS.ClusterName()
}

// Shutdown stops this server instance gracefully
func (a *TestServer) Shutdown(ctx context.Context) error {
	return trace.NewAggregate(
		a.TLS.Shutdown(ctx),
		a.AuthServer.Close(),
	)
}

// TestAuthServer is auth server using local filesystem backend
// and test certificate authority key generation that speeds up
// keygen by using the same private key
type TestAuthServer struct {
	// TestAuthServer config is configuration used for auth server setup
	TestAuthServerConfig
	// AuthServer is an auth server
	AuthServer *Server
	// AuditLog is an event audit log
	AuditLog events.IAuditLog
	// SessionLogger is a session logger
	SessionServer session.Service
	// Backend is a backend for auth server
	Backend backend.Backend
	// Authorizer is an authorizer used in tests
	Authorizer Authorizer
	// LockWatcher is a lock watcher used in tests.
	LockWatcher *services.LockWatcher
}

// NewTestAuthServer returns new instances of Auth server
func NewTestAuthServer(cfg TestAuthServerConfig) (*TestAuthServer, error) {
	ctx := context.Background()

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	srv := &TestAuthServer{
		TestAuthServerConfig: cfg,
	}
	b, err := memory.New(memory.Config{
		Context:   ctx,
		Clock:     cfg.Clock,
		EventsOff: false,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Wrap backend in sanitizer like in production.
	srv.Backend = backend.NewSanitizer(b)

	localLog, err := events.NewAuditLog(events.AuditLogConfig{
		DataDir:        cfg.Dir,
		RecordSessions: true,
		ServerID:       cfg.ClusterName,
		UploadHandler:  events.NewMemoryUploader(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	srv.AuditLog = localLog

	srv.SessionServer, err = session.New(srv.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	access := local.NewAccessService(srv.Backend)
	identity := local.NewIdentityService(srv.Backend)

	srv.AuthServer, err = NewServer(&InitConfig{
		Backend:                srv.Backend,
		Authority:              authority.NewWithClock(cfg.Clock),
		Access:                 access,
		Identity:               identity,
		AuditLog:               srv.AuditLog,
		SkipPeriodicOperations: true,
		Emitter:                localLog,
	}, WithClock(cfg.Clock))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = srv.AuthServer.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterNetworkingCfg := cfg.ClusterNetworkingConfig
	if clusterNetworkingCfg == nil {
		clusterNetworkingCfg = types.DefaultClusterNetworkingConfig()
	}

	err = srv.AuthServer.SetClusterNetworkingConfig(ctx, clusterNetworkingCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = srv.AuthServer.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: cfg.ClusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = srv.AuthServer.SetClusterName(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPreference, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOff,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = srv.AuthServer.SetAuthPreference(ctx, authPreference)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// set static tokens
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = srv.AuthServer.SetStaticTokens(staticTokens)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create the default role
	err = srv.AuthServer.UpsertRole(ctx, services.NewAdminRole())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Setup certificate and signing authorities.
	if err = srv.AuthServer.UpsertCertAuthority(suite.NewTestCAWithConfig(suite.TestCAConfig{
		Type:        types.HostCA,
		ClusterName: srv.ClusterName,
		Clock:       cfg.Clock,
	})); err != nil {
		return nil, trace.Wrap(err)
	}
	if err = srv.AuthServer.UpsertCertAuthority(suite.NewTestCAWithConfig(suite.TestCAConfig{
		Type:        types.UserCA,
		ClusterName: srv.ClusterName,
		Clock:       cfg.Clock,
	})); err != nil {
		return nil, trace.Wrap(err)
	}
	if err = srv.AuthServer.UpsertCertAuthority(suite.NewTestCAWithConfig(suite.TestCAConfig{
		Type:        types.JWTSigner,
		ClusterName: srv.ClusterName,
		Clock:       cfg.Clock,
	})); err != nil {
		return nil, trace.Wrap(err)
	}

	srv.LockWatcher, err = services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentAuth,
			Client:    srv.AuthServer,
			Clock:     cfg.Clock,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	srv.AuthServer.SetLockWatcher(srv.LockWatcher)

	srv.Authorizer, err = NewAuthorizer(srv.ClusterName, srv.AuthServer, srv.LockWatcher)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return srv, nil
}

func (a *TestAuthServer) Close() error {
	return trace.NewAggregate(
		a.AuthServer.Close(),
		a.Backend.Close(),
		a.AuditLog.Close(),
	)
}

// GenerateUserCert takes the public key in the OpenSSH `authorized_keys`
// plain text format, signs it using User Certificate Authority signing key and returns the
// resulting certificate.
func (a *TestAuthServer) GenerateUserCert(key []byte, username string, ttl time.Duration, compatibility string) ([]byte, error) {
	user, err := a.AuthServer.GetUser(username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.FetchRoles(user.GetRoles(), a.AuthServer, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := a.AuthServer.generateUserCert(certRequest{
		user:          user,
		ttl:           ttl,
		compatibility: compatibility,
		publicKey:     key,
		checker:       checker,
		traits:        user.GetTraits(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certs.SSH, nil
}

// PrivateKeyToPublicKeyTLS gets the TLS public key from a raw private key.
func PrivateKeyToPublicKeyTLS(privateKey []byte) (tlsPublicKey []byte, err error) {
	sshPrivate, err := ssh.ParseRawPrivateKey(privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsPublicKey, err = tlsca.MarshalPublicKeyFromPrivateKeyPEM(sshPrivate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return tlsPublicKey, nil
}

// generateCertificate generates certificate for identity,
// returns private public key pair
func generateCertificate(authServer *Server, identity TestIdentity) ([]byte, []byte, error) {
	priv, pub, err := authServer.GenerateKeyPair("")
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(priv)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	switch id := identity.I.(type) {
	case LocalUser:
		user, err := authServer.GetUser(id.Username, false)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		checker, err := services.FetchRoles(user.GetRoles(), authServer, user.GetTraits())
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if identity.TTL == 0 {
			identity.TTL = time.Hour
		}
		certs, err := authServer.generateUserCert(certRequest{
			publicKey:      pub,
			user:           user,
			ttl:            identity.TTL,
			usage:          identity.AcceptedUsage,
			routeToCluster: identity.RouteToCluster,
			checker:        checker,
			traits:         user.GetTraits(),
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return certs.TLS, priv, nil
	case BuiltinRole:
		certs, err := authServer.GenerateHostCerts(context.Background(),
			&proto.HostCertsRequest{
				HostID:       id.Username,
				NodeName:     id.Username,
				Role:         id.Role,
				PublicTLSKey: tlsPublicKey,
				PublicSSHKey: pub,
			})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return certs.TLS, priv, nil
	case RemoteBuiltinRole:
		certs, err := authServer.GenerateHostCerts(context.Background(),
			&proto.HostCertsRequest{
				HostID:       id.Username,
				NodeName:     id.Username,
				Role:         id.Role,
				PublicTLSKey: tlsPublicKey,
				PublicSSHKey: pub,
			})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return certs.TLS, priv, nil
	default:
		return nil, nil, trace.BadParameter("identity of unknown type %T is unsupported", identity)
	}
}

// NewCertificate returns new TLS credentials generated by test auth server
func (a *TestAuthServer) NewCertificate(identity TestIdentity) (*tls.Certificate, error) {
	cert, key, err := generateCertificate(a.AuthServer, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &tlsCert, nil
}

// Clock returns clock used by auth server
func (a *TestAuthServer) Clock() clockwork.Clock {
	return a.AuthServer.GetClock()
}

// Trust adds other server host certificate authority as trusted
func (a *TestAuthServer) Trust(remote *TestAuthServer, roleMap types.RoleMap) error {
	remoteCA, err := remote.AuthServer.GetCertAuthority(types.CertAuthID{
		Type:       types.HostCA,
		DomainName: remote.ClusterName,
	}, false)
	if err != nil {
		return trace.Wrap(err)
	}
	err = a.AuthServer.UpsertCertAuthority(remoteCA)
	if err != nil {
		return trace.Wrap(err)
	}
	remoteCA, err = remote.AuthServer.GetCertAuthority(types.CertAuthID{
		Type:       types.UserCA,
		DomainName: remote.ClusterName,
	}, false)
	if err != nil {
		return trace.Wrap(err)
	}
	remoteCA.SetRoleMap(roleMap)
	err = a.AuthServer.UpsertCertAuthority(remoteCA)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// NewTestTLSServer returns new test TLS server
func (a *TestAuthServer) NewTestTLSServer() (*TestTLSServer, error) {
	apiConfig := &APIConfig{
		AuthServer:     a.AuthServer,
		Authorizer:     a.Authorizer,
		SessionService: a.SessionServer,
		AuditLog:       a.AuditLog,
		Emitter:        a.AuthServer.emitter,
	}
	srv, err := NewTestTLSServer(TestTLSServerConfig{
		APIConfig:     apiConfig,
		AuthServer:    a,
		AcceptedUsage: a.AcceptedUsage,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return srv, nil
}

// NewRemoteClient creates new client to the remote server using identity
// generated for this certificate authority
func (a *TestAuthServer) NewRemoteClient(identity TestIdentity, addr net.Addr, pool *x509.CertPool) (*Client, error) {
	tlsConfig := utils.TLSConfig(a.CipherSuites)
	cert, err := a.NewCertificate(identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.Certificates = []tls.Certificate{*cert}
	tlsConfig.RootCAs = pool
	tlsConfig.ServerName = apiutils.EncodeClusterName(a.ClusterName)
	tlsConfig.Time = a.AuthServer.clock.Now

	return NewClient(client.Config{
		Addrs: []string{addr.String()},
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
	})
}

// TestTLSServerConfig is a configuration for test TLS server
type TestTLSServerConfig struct {
	// APIConfig is a configuration of API server
	APIConfig *APIConfig
	// AuthServer is a test auth server used to serve requests
	AuthServer *TestAuthServer
	// Limiter is a connection and request limiter
	Limiter *limiter.Config
	// Listener is a listener to serve requests on
	Listener net.Listener
	// AcceptedUsage is a list of accepted usage restrictions
	AcceptedUsage []string
}

// Auth returns auth server used by this TLS server
func (t *TestTLSServer) Auth() *Server {
	return t.AuthServer.AuthServer
}

// TestTLSServer is a test TLS server
type TestTLSServer struct {
	// TestTLSServerConfig is a configuration for TLS server
	TestTLSServerConfig
	// Identity is a generated TLS/SSH identity used to answer in TLS
	Identity *Identity
	// TLSServer is a configured TLS server
	TLSServer *TLSServer
}

// ClusterName returns name of test TLS server cluster
func (t *TestTLSServer) ClusterName() string {
	return t.AuthServer.ClusterName
}

// Clock returns clock used by auth server
func (t *TestTLSServer) Clock() clockwork.Clock {
	return t.AuthServer.Clock()
}

// CheckAndSetDefaults checks and sets limiter defaults
func (cfg *TestTLSServerConfig) CheckAndSetDefaults() error {
	if cfg.APIConfig == nil {
		return trace.BadParameter("missing parameter APIConfig")
	}
	if cfg.AuthServer == nil {
		return trace.BadParameter("missing parameter AuthServer")
	}
	// use very permissive limiter configuration by default
	if cfg.Limiter == nil {
		cfg.Limiter = &limiter.Config{
			MaxConnections:   1000,
			MaxNumberOfUsers: 1000,
		}
	}
	return nil
}

// NewTestTLSServer returns new test TLS server that is started and is listening
// on 127.0.0.1 loopback on any available port
func NewTestTLSServer(cfg TestTLSServerConfig) (*TestTLSServer, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	srv := &TestTLSServer{
		TestTLSServerConfig: cfg,
	}
	srv.Identity, err = NewServerIdentity(srv.AuthServer.AuthServer, "test-tls-server", types.RoleAuth)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Register TLS endpoint of the auth service
	tlsConfig, err := srv.Identity.TLSConfig(srv.AuthServer.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.Time = cfg.AuthServer.Clock().Now

	accessPoint, err := NewAdminAuthServer(srv.AuthServer.AuthServer, srv.AuthServer.SessionServer, srv.AuthServer.AuditLog)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	srv.Listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	srv.TLSServer, err = NewTLSServer(TLSServerConfig{
		Listener:      srv.Listener,
		AccessPoint:   accessPoint,
		TLS:           tlsConfig,
		APIConfig:     *srv.APIConfig,
		LimiterConfig: *srv.Limiter,
		AcceptedUsage: cfg.AcceptedUsage,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := srv.Start(); err != nil {
		return nil, trace.Wrap(err)
	}
	return srv, nil
}

// TestIdentity is test identity spec used to generate identities in tests
type TestIdentity struct {
	I              interface{}
	TTL            time.Duration
	AcceptedUsage  []string
	RouteToCluster string
}

// TestUser returns TestIdentity for local user
func TestUser(username string) TestIdentity {
	return TestIdentity{
		I: LocalUser{
			Username: username,
			Identity: tlsca.Identity{Username: username},
		},
	}
}

// TestNop returns "Nop" - unauthenticated identity
func TestNop() TestIdentity {
	return TestIdentity{
		I: nil,
	}
}

// TestAdmin returns TestIdentity for admin user
func TestAdmin() TestIdentity {
	return TestBuiltin(types.RoleAdmin)
}

// TestBuiltin returns TestIdentity for builtin user
func TestBuiltin(role types.SystemRole) TestIdentity {
	return TestIdentity{
		I: BuiltinRole{
			Role:     role,
			Username: string(role),
		},
	}
}

// TestServerID returns a TestIdentity for a node with the passed in serverID.
func TestServerID(role types.SystemRole, serverID string) TestIdentity {
	return TestIdentity{
		I: BuiltinRole{
			Role:     role,
			Username: serverID,
		},
	}
}

// TestRemoteBuiltin returns TestIdentity for a remote builtin role.
func TestRemoteBuiltin(role types.SystemRole, remoteCluster string) TestIdentity {
	return TestIdentity{
		I: RemoteBuiltinRole{
			Role:        role,
			Username:    string(role),
			ClusterName: remoteCluster,
		},
	}
}

// NewClientFromWebSession returns new authenticated client from web session
func (t *TestTLSServer) NewClientFromWebSession(sess types.WebSession) (*Client, error) {
	tlsConfig, err := t.Identity.TLSConfig(t.AuthServer.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := tls.X509KeyPair(sess.GetTLSCert(), sess.GetPriv())
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS cert and key")
	}
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConfig.Time = t.AuthServer.AuthServer.clock.Now

	return NewClient(client.Config{
		Addrs: []string{t.Addr().String()},
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
	})
}

// CertPool returns cert pool that auth server represents
func (t *TestTLSServer) CertPool() (*x509.CertPool, error) {
	tlsConfig, err := t.Identity.TLSConfig(t.AuthServer.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tlsConfig.RootCAs, nil
}

// ClientTLSConfig returns client TLS config based on the identity
func (t *TestTLSServer) ClientTLSConfig(identity TestIdentity) (*tls.Config, error) {
	tlsConfig, err := t.Identity.TLSConfig(t.AuthServer.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if identity.I != nil {
		cert, err := t.AuthServer.NewCertificate(identity)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tlsConfig.Certificates = []tls.Certificate{*cert}
	} else {
		// this client is not authenticated, which means that auth
		// server should apply Nop builtin role
		tlsConfig.Certificates = nil
	}
	tlsConfig.Time = t.AuthServer.AuthServer.clock.Now
	return tlsConfig, nil
}

// CloneClient uses the same credentials as the passed client
// but forces the client to be recreated
func (t *TestTLSServer) CloneClient(clt *Client) *Client {
	newClient, err := NewClient(client.Config{
		Addrs: []string{t.Addr().String()},
		Credentials: []client.Credentials{
			client.LoadTLS(clt.Config()),
		},
	})
	if err != nil {
		panic(err)
	}
	return newClient
}

// NewClientWithCert creates a new client using given cert and private key
func (t *TestTLSServer) NewClientWithCert(clientCert tls.Certificate) *Client {
	tlsConfig, err := t.Identity.TLSConfig(t.AuthServer.CipherSuites)
	if err != nil {
		panic(err)
	}
	tlsConfig.Time = t.AuthServer.AuthServer.clock.Now
	tlsConfig.Certificates = []tls.Certificate{clientCert}
	newClient, err := NewClient(client.Config{
		Addrs: []string{t.Addr().String()},
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
	})
	if err != nil {
		panic(err)
	}
	return newClient
}

// NewClient returns new client to test server authenticated with identity
func (t *TestTLSServer) NewClient(identity TestIdentity) (*Client, error) {
	tlsConfig, err := t.ClientTLSConfig(identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newClient, err := NewClient(client.Config{
		DialInBackground: true,
		Addrs:            []string{t.Addr().String()},
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newClient, nil
}

// Addr returns address of TLS server
func (t *TestTLSServer) Addr() net.Addr {
	return t.Listener.Addr()
}

// Start starts TLS server on loopback address on the first listening socket
func (t *TestTLSServer) Start() error {
	go t.TLSServer.Serve()
	return nil
}

// Close closes the listener and HTTP server
func (t *TestTLSServer) Close() error {
	err := t.TLSServer.Close()
	if t.Listener != nil {
		t.Listener.Close()
	}
	if t.AuthServer.Backend != nil {
		t.AuthServer.Backend.Close()
	}
	return err
}

// Shutdown closes the listener and HTTP server gracefully
func (t *TestTLSServer) Shutdown(ctx context.Context) error {
	errs := []error{t.TLSServer.Shutdown(ctx)}
	if t.Listener != nil {
		errs = append(errs, t.Listener.Close())
	}
	if t.AuthServer.Backend != nil {
		errs = append(errs, t.AuthServer.Backend.Close())
	}
	return trace.NewAggregate(errs...)
}

// Stop stops listening server, but does not close the auth backend
func (t *TestTLSServer) Stop() error {
	err := t.TLSServer.Close()
	if t.Listener != nil {
		t.Listener.Close()
	}
	return err
}

// NewServerIdentity generates new server identity, used in tests
func NewServerIdentity(clt *Server, hostID string, role types.SystemRole) (*Identity, error) {
	priv, pub, err := clt.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicTLS, err := PrivateKeyToPublicKeyTLS(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := clt.GenerateHostCerts(context.Background(),
		&proto.HostCertsRequest{
			HostID:       hostID,
			NodeName:     hostID,
			Role:         types.RoleAuth,
			PublicTLSKey: publicTLS,
			PublicSSHKey: pub,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ReadIdentityFromKeyPair(priv, certs)
}

// clt limits required interface to the necessary methods
// used to pass different clients in tests
type clt interface {
	UpsertRole(context.Context, types.Role) error
	UpsertUser(types.User) error
}

// CreateUserRoleAndRequestable creates two roles for a user, one base role with allowed login
// matching username, and another role with a login matching rolename that can be requested.
func CreateUserRoleAndRequestable(clt clt, username string, rolename string) (types.User, error) {
	ctx := context.TODO()
	user, err := types.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	baseRole := services.RoleForUser(user)
	baseRole.SetAccessRequestConditions(services.Allow, types.AccessRequestConditions{
		Roles: []string{rolename},
	})
	baseRole.SetLogins(services.Allow, nil)
	err = clt.UpsertRole(ctx, baseRole)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.AddRole(baseRole.GetName())
	err = clt.UpsertUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	requestableRole := services.RoleForUser(user)
	requestableRole.SetName(rolename)
	requestableRole.SetLogins(services.Allow, []string{rolename})
	err = clt.UpsertRole(ctx, requestableRole)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// CreateAccessPluginUser creates a user with list/read abilites for access requests, and list/read/update
// abilities for access plugin data.
func CreateAccessPluginUser(ctx context.Context, clt clt, username string) (types.User, error) {
	user, err := types.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role := services.RoleForUser(user)
	rules := role.GetRules(types.Allow)
	rules = append(rules,
		types.Rule{
			Resources: []string{types.KindAccessRequest},
			Verbs:     []string{types.VerbRead, types.VerbList},
		},
		types.Rule{
			Resources: []string{types.KindAccessPluginData},
			Verbs:     []string{types.VerbRead, types.VerbList, types.VerbUpdate},
		},
	)
	role.SetRules(types.Allow, rules)
	role.SetLogins(types.Allow, nil)
	if err := clt.UpsertRole(ctx, role); err != nil {
		return nil, trace.Wrap(err)
	}
	user.AddRole(role.GetName())
	if err := clt.UpsertUser(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// CreateUser creates user and role and assignes role to a user, used in tests
func CreateUser(clt clt, username string, roles ...types.Role) (types.User, error) {
	ctx := context.TODO()
	user, err := types.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, role := range roles {
		err = clt.UpsertRole(ctx, role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		user.AddRole(role.GetName())
	}

	err = clt.UpsertUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// CreateUserAndRole creates user and role and assignes role to a user, used in tests
func CreateUserAndRole(clt clt, username string, allowedLogins []string) (types.User, types.Role, error) {
	ctx := context.TODO()
	user, err := types.NewUser(username)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	role := services.RoleForUser(user)
	role.SetLogins(services.Allow, []string{user.GetName()})
	err = clt.UpsertRole(ctx, role)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	user.AddRole(role.GetName())
	err = clt.UpsertUser(user)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return user, role, nil
}

// CreateUserAndRoleWithoutRoles creates user and role, but does not assign user to a role, used in tests
func CreateUserAndRoleWithoutRoles(clt clt, username string, allowedLogins []string) (types.User, types.Role, error) {
	ctx := context.TODO()
	user, err := types.NewUser(username)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	role := services.RoleForUser(user)
	set := services.MakeRuleSet(role.GetRules(services.Allow))
	delete(set, types.KindRole)
	role.SetRules(services.Allow, set.Slice())
	role.SetLogins(services.Allow, []string{user.GetName()})
	err = clt.UpsertRole(ctx, role)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	user.AddRole(role.GetName())
	err = clt.UpsertUser(user)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return user, role, nil
}
