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
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

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
		cfg.Clock = clockwork.NewFakeClockAt(time.Now())
	}
	if len(cfg.CipherSuites) == 0 {
		cfg.CipherSuites = utils.DefaultCipherSuites()
	}
	return nil
}

// CreateUploaderDir creates directory for file uploader service
func CreateUploaderDir(dir string) error {
	err := os.MkdirAll(filepath.Join(dir, teleport.LogsDir, teleport.ComponentUpload,
		events.SessionLogsDir, defaults.Namespace), teleport.SharedDirMode)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	return nil
}

// TestAuthServer is auth server using local filesystem backend
// and test certificate authority key generation that speeds up
// keygen by using the same private key
type TestAuthServer struct {
	// TestAuthServer config is configuration used for auth server setup
	TestAuthServerConfig
	// AuthServer is an auth server
	AuthServer *AuthServer
	// AuditLog is an event audit log
	AuditLog events.IAuditLog
	// SessionLogger is a session logger
	SessionServer session.Service
	// Backend is a backend for auth server
	Backend backend.Backend
	// Authorizer is an authorizer used in tests
	Authorizer Authorizer
}

// NewTestAuthServer returns new instances of Auth server
func NewTestAuthServer(cfg TestAuthServerConfig) (*TestAuthServer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	srv := &TestAuthServer{
		TestAuthServerConfig: cfg,
	}
	var err error
	b, err := memory.New(memory.Config{
		Context:   context.Background(),
		Clock:     cfg.Clock,
		EventsOff: false,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Wrap backend in sanitizer like in production.
	srv.Backend = backend.NewSanitizer(b)

	srv.AuditLog, err = events.NewAuditLog(events.AuditLogConfig{
		DataDir:        cfg.Dir,
		RecordSessions: true,
		ServerID:       cfg.ClusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	srv.SessionServer, err = session.New(srv.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	access := local.NewAccessService(srv.Backend)
	identity := local.NewIdentityService(srv.Backend)

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: cfg.ClusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	srv.AuthServer, err = NewAuthServer(&InitConfig{
		Backend:                srv.Backend,
		Authority:              authority.New(),
		Access:                 access,
		Identity:               identity,
		AuditLog:               srv.AuditLog,
		SkipPeriodicOperations: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = srv.AuthServer.SetClusterConfig(services.DefaultClusterConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// set cluster name in the backend
	err = srv.AuthServer.SetClusterName(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OFF,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = srv.AuthServer.SetAuthPreference(authPreference)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// set static tokens
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionTokenV1{},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = srv.AuthServer.SetStaticTokens(staticTokens)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx := context.Background()

	// create the default role
	err = srv.AuthServer.UpsertRole(ctx, services.NewAdminRole())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// set up host private key and certificate
	err = srv.AuthServer.UpsertCertAuthority(suite.NewTestCA(services.HostCA, srv.ClusterName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = srv.AuthServer.UpsertCertAuthority(suite.NewTestCA(services.UserCA, srv.ClusterName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	srv.Authorizer, err = NewAuthorizer(srv.AuthServer.Access, srv.AuthServer.Identity, srv.AuthServer.Trust)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return srv, nil
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
	return certs.ssh, nil
}

// generateCertificate generates certificate for identity,
// returns private public key pair
func generateCertificate(authServer *AuthServer, identity TestIdentity) ([]byte, []byte, error) {
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
		priv, pub, err := authServer.GenerateKeyPair("")
		if err != nil {
			return nil, nil, trace.Wrap(err)
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
		return certs.tls, priv, nil
	case BuiltinRole:
		keys, err := authServer.GenerateServerKeys(GenerateServerKeysRequest{
			HostID:   id.Username,
			NodeName: id.Username,
			Roles:    teleport.Roles{id.Role},
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return keys.TLSCert, keys.Key, nil
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
func (a *TestAuthServer) Trust(remote *TestAuthServer, roleMap services.RoleMap) error {
	remoteCA, err := remote.AuthServer.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: remote.ClusterName,
	}, false)
	if err != nil {
		return trace.Wrap(err)
	}
	err = a.AuthServer.UpsertCertAuthority(remoteCA)
	if err != nil {
		return trace.Wrap(err)
	}
	remoteCA, err = remote.AuthServer.GetCertAuthority(services.CertAuthID{
		Type:       services.UserCA,
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
	tlsConfig.ServerName = EncodeClusterName(a.ClusterName)
	addrs := []utils.NetAddr{{
		AddrNetwork: addr.Network(),
		Addr:        addr.String()}}
	return NewTLSClient(ClientConfig{Addrs: addrs, TLS: tlsConfig})
}

// TestTLSServerConfig is a configuration for test TLS server
type TestTLSServerConfig struct {
	// APIConfig is a configuration of API server
	APIConfig *APIConfig
	// AuthServer is a test auth server used to serve requests
	AuthServer *TestAuthServer
	// Limiter is a connection and request limiter
	Limiter *limiter.LimiterConfig
	// Listener is a listener to serve requests on
	Listener net.Listener
	// AcceptedUsage is a list of accepted usage restrictions
	AcceptedUsage []string
}

// Auth returns auth server used by this TLS server
func (t *TestTLSServer) Auth() *AuthServer {
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
		cfg.Limiter = &limiter.LimiterConfig{
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
	srv.Identity, err = NewServerIdentity(srv.AuthServer.AuthServer, "test-tls-server", teleport.RoleAuth)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Register TLS endpoint of the auth service
	tlsConfig, err := srv.Identity.TLSConfig(srv.AuthServer.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessPoint, err := NewAdminAuthServer(srv.AuthServer.AuthServer, srv.AuthServer.SessionServer, srv.AuthServer.AuditLog)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	srv.TLSServer, err = NewTLSServer(TLSServerConfig{
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
	return TestBuiltin(teleport.RoleAdmin)
}

// TestBuiltin returns TestIdentity for builtin user
func TestBuiltin(role teleport.Role) TestIdentity {
	return TestIdentity{
		I: BuiltinRole{
			Role:     role,
			Username: string(role),
		},
	}
}

// TestServerID returns a TestIdentity for a node with the passed in serverID.
func TestServerID(serverID string) TestIdentity {
	return TestIdentity{
		I: BuiltinRole{
			Role:     teleport.RoleNode,
			Username: serverID,
		},
	}
}

// NewClientFromWebSession returns new authenticated client from web session
func (t *TestTLSServer) NewClientFromWebSession(sess services.WebSession) (*Client, error) {
	tlsConfig, err := t.Identity.TLSConfig(t.AuthServer.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := tls.X509KeyPair(sess.GetTLSCert(), sess.GetPriv())
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS cert and key")
	}
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	addrs := []utils.NetAddr{utils.FromAddr(t.Listener.Addr())}
	return NewTLSClient(ClientConfig{Addrs: addrs, TLS: tlsConfig})
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
	return tlsConfig, nil
}

// CloneClient uses the same credentials as the passed client
// but forces the client to be recreated
func (t *TestTLSServer) CloneClient(clt *Client) *Client {
	addr := []utils.NetAddr{{Addr: t.Addr().String(), AddrNetwork: t.Addr().Network()}}
	newClient, err := NewTLSClient(ClientConfig{Addrs: addr, TLS: clt.TLSConfig()})
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
	addrs := []utils.NetAddr{utils.FromAddr(t.Listener.Addr())}
	return NewTLSClient(ClientConfig{Addrs: addrs, TLS: tlsConfig})
}

// Addr returns address of TLS server
func (t *TestTLSServer) Addr() net.Addr {
	return t.Listener.Addr()
}

// Start starts TLS server on loopback address on the first lisenting socket
func (t *TestTLSServer) Start() error {
	var err error
	if t.Listener == nil {
		t.Listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return trace.Wrap(err)
		}
	}
	go t.TLSServer.Serve(t.Listener)
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

// Stop stops listening server, but does not close the auth backend
func (t *TestTLSServer) Stop() error {
	err := t.TLSServer.Close()
	if t.Listener != nil {
		t.Listener.Close()
	}
	return err
}

// NewServerIdentity generates new server identity, used in tests
func NewServerIdentity(clt *AuthServer, hostID string, role teleport.Role) (*Identity, error) {
	keys, err := clt.GenerateServerKeys(GenerateServerKeysRequest{
		HostID:   hostID,
		NodeName: hostID,
		Roles:    teleport.Roles{teleport.RoleAuth},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ReadIdentityFromKeyPair(keys)
}

// clt limits required interface to the necessary methods
// used to pass different clients in tests
type clt interface {
	UpsertRole(context.Context, services.Role) error
	UpsertUser(services.User) error
}

// CreateUserRoleAndRequestable creates two roles for a user, one base role with allowed login
// matching username, and another role with a login matching rolename that can be requested.
func CreateUserRoleAndRequestable(clt clt, username string, rolename string) (services.User, error) {
	ctx := context.TODO()
	user, err := services.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	baseRole := services.RoleForUser(user)
	baseRole.SetLogins(services.Allow, []string{username})
	baseRole.SetAccessRequestConditions(services.Allow, services.AccessRequestConditions{
		Roles: []string{rolename},
	})
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

// CreateUserAndRole creates user and role and assignes role to a user, used in tests
func CreateUserAndRole(clt clt, username string, allowedLogins []string) (services.User, services.Role, error) {
	ctx := context.TODO()
	user, err := services.NewUser(username)
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
func CreateUserAndRoleWithoutRoles(clt clt, username string, allowedLogins []string) (services.User, services.Role, error) {
	ctx := context.TODO()
	user, err := services.NewUser(username)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	role := services.RoleForUser(user)
	set := services.MakeRuleSet(role.GetRules(services.Allow))
	delete(set, services.KindRole)
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
