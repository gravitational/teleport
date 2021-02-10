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

package services

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/client"
	"github.com/gravitational/teleport/lib/auth/local"
	"github.com/gravitational/teleport/lib/auth/server"
	"github.com/gravitational/teleport/lib/auth/test"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// AuthServerConfig is auth server test config
type AuthServerConfig struct {
	// ClusterName is cluster name
	ClusterName string
	// Dir is directory for local backend
	Dir string
	// AcceptedUsage is an optional list of restricted
	// server usage
	AcceptedUsage []string
	// CipherSuites is the list of ciphers that the server supports.
	CipherSuites []uint16
	// PrivateKey optionally specifies the private key for auth server
	PrivateKey []byte
	// Clock is used to control time in tests.
	Clock clockwork.FakeClock
	// Emitter optionally specifies the events emitter
	Emitter events.Emitter
}

// CheckAndSetDefaults checks and sets defaults
func (cfg *AuthServerConfig) CheckAndSetDefaults() error {
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
		events.SessionLogsDir, defaults.Namespace), teleport.SharedDirMode)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	err = os.MkdirAll(filepath.Join(dir, teleport.LogsDir, teleport.ComponentUpload,
		events.StreamingLogsDir, defaults.Namespace), teleport.SharedDirMode)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	return nil
}

// AuthServer is auth server using local filesystem backend
// and test certificate authority key generation that speeds up
// keygen by using the same private key
type AuthServer struct {
	// AuthServerConfig is configuration used for auth server setup
	AuthServerConfig
	// AuthServer is an auth server
	AuthServer *server.Server
	// AuditLog is an event audit log
	AuditLog events.IAuditLog
	// SessionLogger is a session logger
	SessionServer session.Service
	// Backend is a backend for auth server
	Backend backend.Backend
	// Authorizer is an authorizer used in tests
	Authorizer server.Authorizer
}

// NewAuthServer returns new instances of Auth server
func NewAuthServer(cfg AuthServerConfig) (*AuthServer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	srv := &AuthServer{
		AuthServerConfig: cfg,
	}
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

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: cfg.ClusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opts := []server.Option{server.WithClock(cfg.Clock)}
	if cfg.PrivateKey != nil {
		opts = append(opts, server.WithPrivateKey(cfg.PrivateKey))
	}
	var emitter events.Emitter = localLog
	if cfg.Emitter != nil {
		emitter = cfg.Emitter
	}
	srv.AuthServer, err = server.New(&server.InitConfig{
		Backend:                srv.Backend,
		Authority:              authority.NewWithClock(cfg.Clock),
		Access:                 access,
		Identity:               identity,
		AuditLog:               srv.AuditLog,
		SkipPeriodicOperations: true,
		Emitter:                emitter,
	}, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = srv.AuthServer.SetClusterConfig(auth.DefaultClusterConfig())
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
		SecondFactor: constants.SecondFactorOff,
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
	err = srv.AuthServer.UpsertRole(ctx, auth.NewAdminRole())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Setup certificate and signing authorities.
	if err = srv.AuthServer.UpsertCertAuthority(test.NewCAWithConfig(test.CAConfig{
		Type:        services.HostCA,
		ClusterName: srv.ClusterName,
		Clock:       cfg.Clock,
	})); err != nil {
		return nil, trace.Wrap(err)
	}
	if err = srv.AuthServer.UpsertCertAuthority(test.NewCAWithConfig(test.CAConfig{
		Type:        services.UserCA,
		ClusterName: srv.ClusterName,
		Clock:       cfg.Clock,
	})); err != nil {
		return nil, trace.Wrap(err)
	}
	if err = srv.AuthServer.UpsertCertAuthority(test.NewCAWithConfig(test.CAConfig{
		Type:        services.JWTSigner,
		ClusterName: srv.ClusterName,
		Clock:       cfg.Clock,
	})); err != nil {
		return nil, trace.Wrap(err)
	}

	srv.Authorizer, err = server.NewAuthorizer(srv.ClusterName, srv.AuthServer.Access, srv.AuthServer.Identity, srv.AuthServer.Trust)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return srv, nil
}

func (a *AuthServer) Close() error {
	return trace.NewAggregate(
		a.Backend.Close(),
		a.AuditLog.Close(),
		a.AuthServer.Close(),
	)
}

// GenerateUserCert takes the public key in the OpenSSH `authorized_keys`
// plain text format, signs it using User Certificate Authority signing key and returns the
// resulting certificate.
func (a *AuthServer) GenerateUserCert(key []byte, username string, ttl time.Duration, compatibility string) ([]byte, error) {
	sshCert, _, err := a.AuthServer.GenerateUserTestCerts(server.TestUserCertRequest{
		Key:           key,
		User:          username,
		TTL:           ttl,
		Compatibility: compatibility,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshCert, nil
}

// generateCertificate generates certificate for identity,
// returns private public key pair
func generateCertificate(authServer *server.Server, identity Identity) ([]byte, []byte, error) {
	switch id := identity.I.(type) {
	case server.LocalUser:
		if identity.TTL == 0 {
			identity.TTL = time.Hour
		}
		priv, pub, err := authServer.GenerateKeyPair("")
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		_, tlsCert, err := authServer.GenerateUserTestCerts(server.TestUserCertRequest{
			Key:            pub,
			User:           id.Username,
			TTL:            identity.TTL,
			RouteToCluster: identity.RouteToCluster,
			Usage:          identity.AcceptedUsage,
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return tlsCert, priv, nil
	case server.BuiltinRole:
		keys, err := authServer.GenerateServerKeys(server.GenerateServerKeysRequest{
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
func (a *AuthServer) NewCertificate(identity Identity) (*tls.Certificate, error) {
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
func (a *AuthServer) Clock() clockwork.Clock {
	return a.AuthServer.GetClock()
}

// Trust adds other server host certificate authority as trusted
func (a *AuthServer) Trust(remote *AuthServer, roleMap services.RoleMap) error {
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

// NewTLSServer returns new test TLS server
func (a *AuthServer) NewTLSServer() (*TLSServer, error) {
	apiConfig := &server.APIConfig{
		AuthServer:     a.AuthServer,
		Authorizer:     a.Authorizer,
		SessionService: a.SessionServer,
		AuditLog:       a.AuditLog,
		Emitter:        a.AuthServerConfig.Emitter,
	}
	srv, err := NewTLSServer(TLSServerConfig{
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
func (a *AuthServer) NewRemoteClient(identity Identity, addr net.Addr, pool *x509.CertPool) (*client.Client, error) {
	tlsConfig := utils.TLSConfig(a.CipherSuites)
	cert, err := a.NewCertificate(identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.Certificates = []tls.Certificate{*cert}
	tlsConfig.RootCAs = pool
	tlsConfig.ServerName = auth.EncodeClusterName(a.ClusterName)
	tlsConfig.Time = a.AuthServer.GetClock().Now

	return client.New(apiclient.Config{
		Addrs: []string{addr.String()},
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(tlsConfig),
		},
	})
}

// TLSServerConfig is a configuration for test TLS server
type TLSServerConfig struct {
	// APIConfig is a configuration of API server
	APIConfig *server.APIConfig
	// AuthServer is a test auth server used to serve requests
	AuthServer *AuthServer
	// Limiter is a connection and request limiter
	Limiter *limiter.Config
	// Listener is a listener to serve requests on
	Listener net.Listener
	// AcceptedUsage is a list of accepted usage restrictions
	AcceptedUsage []string
}

// Auth returns auth server used by this TLS server
func (t *TLSServer) Auth() *server.Server {
	return t.AuthServer.AuthServer
}

// TLSServer is a test TLS server
type TLSServer struct {
	// TLSServerConfig is a configuration for TLS server
	TLSServerConfig
	// Identity is a generated TLS/SSH identity used to answer in TLS
	Identity *server.Identity
	// TLSServer is a configured TLS server
	TLSServer *server.TLSServer
}

// ClusterName returns name of test TLS server cluster
func (t *TLSServer) ClusterName() string {
	return t.AuthServer.ClusterName
}

// Clock returns clock used by auth server
func (t *TLSServer) Clock() clockwork.Clock {
	return t.AuthServer.Clock()
}

// CheckAndSetDefaults checks and sets limiter defaults
func (cfg *TLSServerConfig) CheckAndSetDefaults() error {
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

// NewTLSServer returns new test TLS server that is started and is listening
// on 127.0.0.1 loopback on any available port
func NewTLSServer(cfg TLSServerConfig) (*TLSServer, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	srv := &TLSServer{
		TLSServerConfig: cfg,
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
	tlsConfig.Time = cfg.AuthServer.Clock().Now

	accessPoint, err := server.NewAdminAuthServer(srv.AuthServer.AuthServer, srv.AuthServer.SessionServer, srv.AuthServer.AuditLog)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	srv.Listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	srv.TLSServer, err = server.NewTLSServer(server.TLSServerConfig{
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

// Identity is test identity spec used to generate identities in tests
type Identity struct {
	I              interface{}
	TTL            time.Duration
	AcceptedUsage  []string
	RouteToCluster string
}

// User returns Identity for local user
func User(username string) Identity {
	return Identity{
		I: server.LocalUser{
			Username: username,
		},
	}
}

// Nop returns "Nop" - unauthenticated identity
func Nop() Identity {
	return Identity{
		I: nil,
	}
}

// Admin returns Identity for admin user
func Admin() Identity {
	return Builtin(teleport.RoleAdmin)
}

// Builtin returns Identity for builtin user
func Builtin(role teleport.Role) Identity {
	return Identity{
		I: server.BuiltinRole{
			Role:     role,
			Username: string(role),
		},
	}
}

// ServerID returns a Identity for a node with the passed in serverID.
func ServerID(role teleport.Role, serverID string) Identity {
	return Identity{
		I: server.BuiltinRole{
			Role:     role,
			Username: serverID,
		},
	}
}

// NewClientFromWebSession returns new authenticated client from web session
func (t *TLSServer) NewClientFromWebSession(sess types.WebSession) (*client.Client, error) {
	tlsConfig, err := t.Identity.TLSConfig(t.AuthServer.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := tls.X509KeyPair(sess.GetTLSCert(), sess.GetPriv())
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS cert and key")
	}
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConfig.Time = t.AuthServer.AuthServer.GetClock().Now

	return client.New(apiclient.Config{
		Addrs: []string{t.Addr().String()},
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(tlsConfig),
		},
	})
}

// CertPool returns cert pool that auth server represents
func (t *TLSServer) CertPool() (*x509.CertPool, error) {
	tlsConfig, err := t.Identity.TLSConfig(t.AuthServer.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tlsConfig.RootCAs, nil
}

// ClientTLSConfig returns client TLS config based on the identity
func (t *TLSServer) ClientTLSConfig(identity Identity) (*tls.Config, error) {
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
	tlsConfig.Time = t.AuthServer.AuthServer.GetClock().Now
	return tlsConfig, nil
}

// CloneClient uses the same credentials as the passed client
// but forces the client to be recreated
func (t *TLSServer) CloneClient(clt *client.Client) *client.Client {
	newClient, err := client.New(apiclient.Config{
		Addrs: []string{t.Addr().String()},
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(clt.Config()),
		},
	})
	if err != nil {
		panic(err)
	}
	return newClient
}

// NewClientWithCert creates a new client using given cert and private key
func (t *TLSServer) NewClientWithCert(clientCert tls.Certificate) *client.Client {
	tlsConfig, err := t.Identity.TLSConfig(t.AuthServer.CipherSuites)
	if err != nil {
		panic(err)
	}
	tlsConfig.Time = t.AuthServer.AuthServer.GetClock().Now
	tlsConfig.Certificates = []tls.Certificate{clientCert}
	newClient, err := client.New(apiclient.Config{
		Addrs: []string{t.Addr().String()},
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(tlsConfig),
		},
	})
	if err != nil {
		panic(err)
	}
	return newClient
}

// NewClient returns new client to test server authenticated with identity
func (t *TLSServer) NewClient(identity Identity) (*client.Client, error) {
	tlsConfig, err := t.ClientTLSConfig(identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newClient, err := client.New(apiclient.Config{
		DialInBackground: true,
		Addrs:            []string{t.Addr().String()},
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(tlsConfig),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newClient, nil
}

// Addr returns address of TLS server
func (t *TLSServer) Addr() net.Addr {
	return t.Listener.Addr()
}

// Start starts TLS server on loopback address on the first lisenting socket
func (t *TLSServer) Start() error {
	go t.TLSServer.Serve()
	return nil
}

// Close closes the listener and HTTP server
func (t *TLSServer) Close() error {
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
func (t *TLSServer) Stop() error {
	err := t.TLSServer.Close()
	if t.Listener != nil {
		t.Listener.Close()
	}
	return err
}

// NewServerIdentity generates new server identity, used in tests
func NewServerIdentity(clt *server.Server, hostID string, role teleport.Role) (*server.Identity, error) {
	keys, err := clt.GenerateServerKeys(server.GenerateServerKeysRequest{
		HostID:   hostID,
		NodeName: hostID,
		Roles:    teleport.Roles{teleport.RoleAuth},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return server.ReadIdentityFromKeyPair(keys)
}
