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

package helpers

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/breaker"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/keygen"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
	websession "github.com/gravitational/teleport/lib/web/session"
	"github.com/gravitational/teleport/lib/web/terminal"
)

const (
	Loopback = "127.0.0.1"
	Host     = "localhost"
)

func fatalIf(err error) {
	if err != nil {
		slog.ErrorContext(context.Background(), "Fatal error",
			"stack", string(debug.Stack()),
			"error", err,
		)
		os.Exit(1)
	}
}

type User struct {
	Username      string          `json:"username"`
	AllowedLogins []string        `json:"logins"`
	KeyRing       *client.KeyRing `json:"key"`
	Roles         []types.Role    `json:"-"`
}

type InstanceSecrets struct {
	// instance name (aka "site name")
	SiteName string `json:"site_name"`
	// instance keys+cert (reused for hostCA and userCA)
	// PubKey is instance public key
	PubKey []byte `json:"pub"`
	// PrivKey is instance private key
	PrivKey []byte `json:"priv"`
	// Cert is SSH host certificate
	SSHHostCert []byte `json:"cert"`
	// TLSHostCACert is the certificate of the trusted host certificate authority
	TLSHostCACert []byte `json:"tls_host_ca_cert"`
	// TLSCert is client TLS host X509 certificate
	TLSHostCert []byte `json:"tls_host_cert"`
	// TLSUserCACert is the certificate of the trusted user certificate authority
	TLSUserCACert []byte `json:"tls_user_ca_cert"`
	// TLSUserCert is client TLS user X509 certificate
	TLSUserCert []byte `json:"tls_user_cert"`
	// TunnelAddr is a reverse tunnel listening port, allowing
	// other sites to connect to i instance. Set to empty
	// string if i instance is not allowing incoming tunnels
	TunnelAddr string `json:"tunnel_addr"`
	// list of users i instance trusts (key in the map is username)
	Users map[string]*User `json:"users"`
}

func (s *InstanceSecrets) String() string {
	bytes, _ := json.MarshalIndent(s, "", "\t")
	return string(bytes)
}

// GetRoles returns a list of roles to initiate for this secret
func (s *InstanceSecrets) GetRoles(t *testing.T) []types.Role {
	var roles []types.Role

	cas, err := s.GetCAs()
	require.NoError(t, err)
	for _, ca := range cas {
		if ca.GetType() != types.UserCA {
			continue
		}
		role := services.RoleForCertAuthority(ca)
		role.SetLogins(types.Allow, s.AllowedLogins())
		roles = append(roles, role)
	}
	return roles
}

// GetCAs return an array of CAs stored by the secrets object
func (s *InstanceSecrets) GetCAs() ([]types.CertAuthority, error) {
	hostCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: s.SiteName,
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{
				PrivateKey:     s.PrivKey,
				PrivateKeyType: types.PrivateKeyType_RAW,
				PublicKey:      s.PubKey,
			}},
			TLS: []*types.TLSKeyPair{{
				Key:     s.PrivKey,
				KeyType: types.PrivateKeyType_RAW,
				Cert:    s.TLSHostCACert,
			}},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.UserCA,
		ClusterName: s.SiteName,
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{
				PrivateKey:     s.PrivKey,
				PrivateKeyType: types.PrivateKeyType_RAW,
				PublicKey:      s.PubKey,
			}},
			TLS: []*types.TLSKeyPair{{
				Key:     s.PrivKey,
				KeyType: types.PrivateKeyType_RAW,
				Cert:    s.TLSUserCACert,
			}},
		},
		Roles: []string{services.RoleNameForCertAuthority(s.SiteName)},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.DatabaseCA,
		ClusterName: s.SiteName,
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{
				Key:     s.PrivKey,
				KeyType: types.PrivateKeyType_RAW,
				Cert:    s.TLSHostCACert,
			}},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbClientCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.DatabaseClientCA,
		ClusterName: s.SiteName,
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{
				Key:     s.PrivKey,
				KeyType: types.PrivateKeyType_RAW,
				Cert:    s.TLSHostCACert,
			}},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	osshCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.OpenSSHCA,
		ClusterName: s.SiteName,
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{
				PrivateKey:     s.PrivKey,
				PrivateKeyType: types.PrivateKeyType_RAW,
				PublicKey:      s.PubKey,
			}},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return []types.CertAuthority{hostCA, userCA, dbCA, dbClientCA, osshCA}, nil
}

func (s *InstanceSecrets) AllowedLogins() []string {
	var logins []string
	for i := range s.Users {
		logins = append(logins, s.Users[i].AllowedLogins...)
	}
	return logins
}

func (i *TeleInstance) AsTrustedCluster(token string, roleMap types.RoleMap) types.TrustedCluster {
	return &types.TrustedClusterV2{
		Kind:    types.KindTrustedCluster,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: i.Secrets.SiteName,
		},
		Spec: types.TrustedClusterSpecV2{
			Token:                token,
			Enabled:              true,
			ProxyAddress:         i.Web,
			ReverseTunnelAddress: i.ReverseTunnel,
			RoleMap:              roleMap,
		},
	}
}

func (s *InstanceSecrets) AsSlice() []*InstanceSecrets {
	return []*InstanceSecrets{s}
}

func (s *InstanceSecrets) GetIdentity() *state.Identity {
	i, err := state.ReadIdentityFromKeyPair(s.PrivKey, &clientproto.Certs{
		SSH:        s.SSHHostCert,
		TLS:        s.TLSHostCert,
		TLSCACerts: [][]byte{s.TLSHostCACert},
	})
	fatalIf(err)
	return i
}

// TeleInstance represents an in-memory instance of a teleport
// process for testing
type TeleInstance struct {
	// Secrets holds the keys (pub, priv and derived cert) of i instance
	Secrets InstanceSecrets

	// Hostname is the name of the host where instance is running
	Hostname string

	// Internal stuff...
	Process              *service.TeleportProcess
	Config               *servicecfg.Config
	Tunnel               reversetunnelclient.Server
	RemoteClusterWatcher *reversetunnel.RemoteClusterTunnelManager

	// Nodes is a list of additional nodes
	// started with this instance
	Nodes []*service.TeleportProcess

	// UploadEventsC is a channel for upload events
	UploadEventsC chan events.UploadEvent

	// tempDirs is a list of temporary directories that were created that should
	// be cleaned up after the test has successfully run.
	tempDirs []string

	// Log specifies the instance logger
	Log *slog.Logger
	InstanceListeners
	Fds []*servicecfg.FileDescriptor
	// ProcessProvider creates a Teleport process (OSS or Enterprise)
	ProcessProvider teleportProcProvider
}

type teleportProcProvider interface {
	// NewTeleport Create a teleport process OSS or Enterprise.
	NewTeleport(cfg *servicecfg.Config) (*service.TeleportProcess, error)
}

// InstanceConfig is an instance configuration
type InstanceConfig struct {
	// Clock is an optional clock to use
	Clock clockwork.Clock
	// ClusterName is a cluster name of the instance
	ClusterName string
	// HostID is a host id of the instance
	HostID string
	// NodeName is a node name of the instance
	NodeName string
	// Priv is SSH private key of the instance
	Priv []byte
	// Pub is SSH public key of the instance
	Pub []byte
	// Logger specifies the logger
	Logger *slog.Logger
	// Ports is a collection of instance ports.
	Listeners *InstanceListeners

	Fds []*servicecfg.FileDescriptor
}

// NewInstance creates a new Teleport process instance.
//
// The caller is responsible for calling StopAll on the returned instance to
// clean up spawned processes.
func NewInstance(t *testing.T, cfg InstanceConfig) *TeleInstance {
	var err error
	if cfg.NodeName == "" {
		cfg.NodeName, err = os.Hostname()
		fatalIf(err)
	}

	if cfg.Listeners == nil {
		cfg.Listeners = StandardListenerSetup(t, &cfg.Fds)
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}

	// generate instance secrets (keys):
	if cfg.Priv == nil || cfg.Pub == nil {
		privateKey, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
		fatalIf(err)
		cfg.Priv = privateKey.PrivateKeyPEM()
		cfg.Pub = privateKey.MarshalSSHPublicKey()
	}
	key, err := keys.ParsePrivateKey(cfg.Priv)
	fatalIf(err)

	sshSigner, err := ssh.NewSignerFromSigner(key)
	fatalIf(err)

	keygen := keygen.New(context.TODO())
	hostCert, err := keygen.GenerateHostCert(sshca.HostCertificateRequest{
		CASigner:      sshSigner,
		PublicHostKey: cfg.Pub,
		HostID:        cfg.HostID,
		NodeName:      cfg.NodeName,
		TTL:           24 * time.Hour,
		Identity: sshca.Identity{
			ClusterName: cfg.ClusterName,
			SystemRole:  types.RoleAdmin,
		},
	})
	fatalIf(err)

	clock := cfg.Clock
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	identity := tlsca.Identity{
		Username: fmt.Sprintf("%v.%v", cfg.HostID, cfg.ClusterName),
		Groups:   []string{string(types.RoleAdmin)},
	}
	subject, err := identity.Subject()
	fatalIf(err)

	tlsCAHostCert, err := tlsca.GenerateSelfSignedCAWithSigner(key, pkix.Name{
		CommonName:   cfg.ClusterName,
		Organization: []string{cfg.ClusterName},
	}, nil, defaults.CATTL)
	fatalIf(err)
	tlsHostCA, err := tlsca.FromKeys(tlsCAHostCert, cfg.Priv)
	fatalIf(err)
	hostCryptoPubKey, err := sshutils.CryptoPublicKey(cfg.Pub)
	fatalIf(err)
	tlsHostCert, err := tlsHostCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: hostCryptoPubKey,
		Subject:   subject,
		NotAfter:  clock.Now().UTC().Add(time.Hour * 24),
	})
	fatalIf(err)

	tlsCAUserCert, err := tlsca.GenerateSelfSignedCAWithSigner(key, pkix.Name{
		CommonName:   cfg.ClusterName,
		Organization: []string{cfg.ClusterName},
	}, nil, defaults.CATTL)
	fatalIf(err)
	tlsUserCA, err := tlsca.FromKeys(tlsCAHostCert, cfg.Priv)
	fatalIf(err)
	userCryptoPubKey, err := sshutils.CryptoPublicKey(cfg.Pub)
	fatalIf(err)
	tlsUserCert, err := tlsUserCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: userCryptoPubKey,
		Subject:   subject,
		NotAfter:  clock.Now().UTC().Add(time.Hour * 24),
	})
	fatalIf(err)

	i := &TeleInstance{
		Hostname:          cfg.NodeName,
		UploadEventsC:     make(chan events.UploadEvent, 100),
		Log:               cfg.Logger,
		InstanceListeners: *cfg.Listeners,
		Fds:               cfg.Fds,
	}

	secrets := InstanceSecrets{
		SiteName:      cfg.ClusterName,
		PrivKey:       cfg.Priv,
		PubKey:        cfg.Pub,
		SSHHostCert:   hostCert,
		TLSHostCACert: tlsCAHostCert,
		TLSHostCert:   tlsHostCert,
		TLSUserCACert: tlsCAUserCert,
		TLSUserCert:   tlsUserCert,
		TunnelAddr:    i.ReverseTunnel,
		Users:         make(map[string]*User),
	}

	i.Secrets = secrets
	return i
}

// GetSiteAPI is a helper which returns an API endpoint to a site with
// a given name. i endpoint implements HTTP-over-SSH access to the
// site's auth server.
func (i *TeleInstance) GetSiteAPI(siteName string) authclient.ClientI {
	siteTunnel, err := i.Tunnel.GetSite(siteName)
	if err != nil {
		i.Log.WarnContext(context.Background(), "failed to get site", "error", err, "siter", siteName)
		return nil
	}
	siteAPI, err := siteTunnel.GetClient()
	if err != nil {
		i.Log.WarnContext(context.Background(), "failed to get site client", "error", err, "site", siteName)
		return nil
	}
	return siteAPI
}

// Create creates a new instance of Teleport which trusts a list of other clusters (other
// instances)
func (i *TeleInstance) Create(t *testing.T, trustedSecrets []*InstanceSecrets, enableSSH bool) error {
	tconf := servicecfg.MakeDefaultConfig()
	tconf.SSH.Enabled = enableSSH
	tconf.Logger = i.Log
	tconf.Proxy.DisableWebService = true
	tconf.Proxy.DisableWebInterface = true
	tconf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	tconf.InstanceMetadataClient = imds.NewDisabledIMDSClient()
	tconf.DebugService.Enabled = false
	return i.CreateEx(t, trustedSecrets, tconf)
}

// GenerateConfig generates instance config
func (i *TeleInstance) GenerateConfig(t *testing.T, trustedSecrets []*InstanceSecrets, tconf *servicecfg.Config) (*servicecfg.Config, error) {
	var err error
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	if tconf == nil {
		tconf = servicecfg.MakeDefaultConfig()
	}
	if tconf.InstanceMetadataClient == nil {
		tconf.InstanceMetadataClient = imds.NewDisabledIMDSClient()
	}
	tconf.Logger = i.Log
	tconf.DataDir = dataDir
	tconf.Testing.UploadEventsC = i.UploadEventsC
	tconf.CachePolicy.Enabled = true
	tconf.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: i.Secrets.SiteName,
	})
	tconf.DebugService.Enabled = false
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tconf.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Roles: []types.SystemRole{
					types.RoleNode,
					types.RoleProxy,
					types.RoleTrustedCluster,
					types.RoleApp,
					types.RoleDatabase,
					types.RoleKube,
				},
				Token: "token",
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rootCAs, err := i.Secrets.GetCAs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tconf.Auth.Authorities = append(tconf.Auth.Authorities, rootCAs...)

	tconf.Identities = append(tconf.Identities, i.Secrets.GetIdentity())

	for _, trusted := range trustedSecrets {
		leafCAs, err := trusted.GetCAs()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tconf.Auth.Authorities = append(tconf.Auth.Authorities, leafCAs...)

		tconf.Auth.Roles = append(tconf.Auth.Roles, trusted.GetRoles(t)...)
		tconf.Identities = append(tconf.Identities, trusted.GetIdentity())
		if trusted.TunnelAddr != "" {
			rt, err := types.NewReverseTunnel(trusted.SiteName, []string{trusted.TunnelAddr})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			tconf.ReverseTunnels = []types.ReverseTunnel{rt}
		}
	}
	tconf.HostUUID = i.Secrets.GetIdentity().ID.HostUUID
	tconf.SSH.Addr.Addr = i.SSH
	tconf.SSH.PublicAddrs = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        Loopback,
		},
		{
			AddrNetwork: "tcp",
			Addr:        Host,
		},
	}
	tconf.SSH.AllowFileCopying = true
	tconf.Auth.ListenAddr.Addr = i.Auth
	tconf.Auth.PublicAddrs = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        i.Hostname,
		},
	}
	tconf.Proxy.PublicAddrs = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        i.Web,
		},
		{
			AddrNetwork: "tcp",
			Addr:        Loopback,
		},
		{
			AddrNetwork: "tcp",
			Addr:        Host,
		},
	}

	if i.IsSinglePortSetup {
		tconf.Proxy.WebAddr.Addr = i.Web
		// Reset other addresses to ensure that teleport instance will expose only web port listener.
		tconf.Proxy.ReverseTunnelListenAddr = utils.NetAddr{}
		tconf.Proxy.MySQLAddr = utils.NetAddr{}
		tconf.Proxy.SSHAddr = utils.NetAddr{}
	} else {
		tunAddr, err := utils.ParseAddr(i.Secrets.TunnelAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tconf.Proxy.ReverseTunnelListenAddr = *tunAddr
		tconf.Proxy.SSHAddr.Addr = i.SSHProxy
		tconf.Proxy.WebAddr.Addr = i.Web
		tconf.Proxy.MySQLAddr.Addr = i.MySQL
		if i.Postgres != "" {
			// Postgres proxy port was configured on a separate listener.
			tconf.Proxy.PostgresAddr.Addr = i.Postgres
		}
		if i.Mongo != "" {
			// Mongo proxy port was configured on a separate listener.
			tconf.Proxy.MongoAddr.Addr = i.Mongo
		}
	}
	tconf.SetAuthServerAddress(tconf.Auth.ListenAddr)
	tconf.Auth.StorageConfig = backend.Config{
		Type:   lite.GetName(),
		Params: backend.Params{"path": dataDir + string(os.PathListSeparator) + defaults.BackendDir, "poll_stream_period": 50 * time.Millisecond},
	}

	tconf.Kube.CheckImpersonationPermissions = nullImpersonationCheck

	tconf.Keygen = testauthority.New()
	tconf.MaxRetryPeriod = defaults.HighResPollingPeriod
	tconf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	tconf.FileDescriptors = append(tconf.FileDescriptors, i.Fds...)

	i.Config = tconf
	return tconf, nil
}

// CreateEx creates a new instance of Teleport which trusts a list of other clusters (other
// instances)
//
// Unlike Create() it allows for greater customization because it accepts
// a full Teleport config structure
func (i *TeleInstance) CreateEx(t *testing.T, trustedSecrets []*InstanceSecrets, tconf *servicecfg.Config) error {
	tconf, err := i.GenerateConfig(t, trustedSecrets, tconf)
	if err != nil {
		return trace.Wrap(err)
	}

	return i.CreateWithConf(t, tconf)
}

func (i *TeleInstance) createTeleportProcess(tconf *servicecfg.Config) (*service.TeleportProcess, error) {
	if i.ProcessProvider == nil {
		p, err := service.NewTeleport(tconf)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return p, nil
	}
	p, err := i.ProcessProvider.NewTeleport(tconf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return p, nil
}

// CreateWithConf creates a new instance of Teleport using the supplied config
func (i *TeleInstance) CreateWithConf(_ *testing.T, tconf *servicecfg.Config) error {
	i.Config = tconf
	var err error
	i.Process, err = i.createTeleportProcess(tconf)
	if err != nil {
		return trace.Wrap(err)
	}

	// if the auth server is not enabled, nothing more to do be done
	if !tconf.Auth.Enabled {
		return nil
	}

	// if this instance contains an auth server, configure the auth server as well.
	// create users and roles if they don't exist, or sign their keys if they're
	// already present
	auth := i.Process.GetAuthServer()
	ctx := context.TODO()

	for _, user := range i.Secrets.Users {
		teleUser, err := types.NewUser(user.Username)
		if err != nil {
			return trace.Wrap(err)
		}
		// set hardcode traits to trigger new style certificates
		teleUser.SetTraits(map[string][]string{"testing": {"integration"}})
		if len(user.Roles) == 0 {
			role := services.RoleForUser(teleUser)
			role.SetLogins(types.Allow, user.AllowedLogins)

			// allow tests to forward agent, still needs to be passed in client
			roleOptions := role.GetOptions()
			roleOptions.ForwardAgent = types.NewBool(true)
			roleOptions.PermitX11Forwarding = types.NewBool(true)
			role.SetOptions(roleOptions)

			role, err = auth.UpsertRole(ctx, role)
			if err != nil {
				return trace.Wrap(err)
			}
			teleUser.AddRole(role.GetMetadata().Name)
		} else {
			for _, role := range user.Roles {
				role, err := auth.UpsertRole(ctx, role)
				if err != nil {
					return trace.Wrap(err)
				}
				teleUser.AddRole(role.GetName())
			}
		}
		_, err = auth.UpsertUser(ctx, teleUser)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// StartNode starts a SSH node and connects it to the cluster.
func (i *TeleInstance) StartNode(tconf *servicecfg.Config) (*service.TeleportProcess, error) {
	_, port, err := net.SplitHostPort(i.Auth)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return i.StartNodeWithTargetPort(tconf, port)
}

// StartReverseTunnelNode starts a SSH node and connects it to the cluster via reverse tunnel.
func (i *TeleInstance) StartReverseTunnelNode(tconf *servicecfg.Config) (*service.TeleportProcess, error) {
	_, port, err := net.SplitHostPort(i.Web)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return i.StartNodeWithTargetPort(tconf, port)
}

// StartNodeWithTargetPort starts a node and connects it to the cluster via a specified port.
func (i *TeleInstance) StartNodeWithTargetPort(tconf *servicecfg.Config, authPort string) (*service.TeleportProcess, error) {
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	tconf.DataDir = dataDir

	if tconf.Version == defaults.TeleportConfigVersionV3 {
		if tconf.ProxyServer.IsEmpty() {
			authServer := utils.MustParseAddr(net.JoinHostPort(i.Hostname, authPort))
			tconf.SetAuthServerAddress(*authServer)
		}
	} else {
		authServer := utils.MustParseAddr(net.JoinHostPort(i.Hostname, authPort))
		if err := tconf.SetAuthServerAddresses(append(tconf.AuthServerAddresses(), *authServer)); err != nil {
			return nil, err
		}
	}

	tconf.SetToken("token")
	tconf.Testing.UploadEventsC = i.UploadEventsC
	tconf.CachePolicy = servicecfg.CachePolicy{
		Enabled: true,
	}
	tconf.SSH.PublicAddrs = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        Loopback,
		},
		{
			AddrNetwork: "tcp",
			Addr:        Host,
		},
	}
	tconf.Auth.Enabled = false
	tconf.Proxy.Enabled = false
	tconf.DebugService.Enabled = false

	// Create a new Teleport process and add it to the list of nodes that
	// compose this "cluster".
	process, err := service.NewTeleport(tconf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.Nodes = append(i.Nodes, process)

	// Build a list of expected events to wait for before unblocking based off
	// the configuration passed in.
	expectedEvents := []string{
		service.NodeSSHReady,
	}

	// Start the process and block until the expected events have arrived.
	receivedEvents, err := StartAndWait(process, expectedEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	i.Log.DebugContext(context.Background(), "Teleport node started",
		"node_name", process.Config.Hostname,
		"instance", i.Secrets.SiteName,
		"expected_events_count", len(expectedEvents),
		"received_events_count", len(receivedEvents),
	)
	return process, nil
}

func (i *TeleInstance) StartApp(conf *servicecfg.Config) (*service.TeleportProcess, error) {
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	conf.DataDir = dataDir
	conf.SetAuthServerAddress(utils.NetAddr{
		AddrNetwork: "tcp",
		Addr:        i.Web,
	})
	conf.SetToken("token")
	conf.Testing.UploadEventsC = i.UploadEventsC
	conf.Auth.Enabled = false
	conf.Proxy.Enabled = false
	conf.DebugService.Enabled = false

	// Create a new Teleport process and add it to the list of nodes that
	// compose this "cluster".
	process, err := service.NewTeleport(conf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.Nodes = append(i.Nodes, process)

	// Build a list of expected events to wait for before unblocking based off
	// the configuration passed in.
	expectedEvents := []string{
		service.AppsReady,
	}

	// Start the process and block until the expected events have arrived.
	receivedEvents, err := StartAndWait(process, expectedEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	i.Log.DebugContext(context.Background(), "Teleport Application Server started",
		"instance", i.Secrets.SiteName,
		"expected_events_count", len(expectedEvents),
		"received_events_count", len(receivedEvents),
	)
	return process, nil
}

func (i *TeleInstance) StartApps(configs []*servicecfg.Config) ([]*service.TeleportProcess, error) {
	type result struct {
		process *service.TeleportProcess
		tmpDir  string
		err     error
	}

	results := make(chan result, len(configs))
	for _, conf := range configs {
		go func(cfg *servicecfg.Config) {
			dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
			if err != nil {
				results <- result{err: err}
			}

			cfg.DataDir = dataDir
			cfg.SetAuthServerAddress(utils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        i.Web,
			})
			cfg.SetToken("token")
			cfg.Testing.UploadEventsC = i.UploadEventsC
			cfg.Auth.Enabled = false
			cfg.Proxy.Enabled = false
			cfg.DebugService.Enabled = false

			// Create a new Teleport process and add it to the list of nodes that
			// compose this "cluster".
			process, err := service.NewTeleport(cfg)
			if err != nil {
				results <- result{err: err, tmpDir: dataDir}
			}

			// Build a list of expected events to wait for before unblocking based off
			// the configuration passed in.
			expectedEvents := []string{
				service.AppsReady,
			}

			// Start the process and block until the expected events have arrived.
			receivedEvents, err := StartAndWait(process, expectedEvents)
			if err != nil {
				results <- result{err: err, tmpDir: dataDir}
			}

			i.Log.DebugContext(context.Background(), "Teleport Application Server started",
				"instance", i.Secrets.SiteName,
				"expected_events_count", len(expectedEvents),
				"received_events_count", len(receivedEvents),
			)

			results <- result{err: err, tmpDir: dataDir, process: process}
		}(conf)
	}

	processes := make([]*service.TeleportProcess, 0, len(configs))
	for j := 0; j < len(configs); j++ {
		result := <-results
		if result.tmpDir != "" {
			i.tempDirs = append(i.tempDirs, result.tmpDir)
		}

		if result.err != nil {
			return nil, trace.Wrap(result.err)
		}

		i.Nodes = append(i.Nodes, result.process)
		processes = append(processes, result.process)
	}

	return processes, nil
}

// StartDatabase starts the database access service with the provided config.
func (i *TeleInstance) StartDatabase(conf *servicecfg.Config) (*service.TeleportProcess, *authclient.Client, error) {
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	conf.DataDir = dataDir
	conf.SetAuthServerAddress(utils.NetAddr{
		AddrNetwork: "tcp",
		Addr:        i.Web,
	})
	conf.SetToken("token")
	conf.Testing.UploadEventsC = i.UploadEventsC
	conf.Databases.Enabled = true
	conf.Auth.Enabled = false
	conf.Proxy.Enabled = false
	conf.Apps.Enabled = false
	conf.SSH.Enabled = false
	conf.DebugService.Enabled = false

	// Create a new Teleport process and add it to the list of nodes that
	// compose this "cluster".
	process, err := service.NewTeleport(conf)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	i.Nodes = append(i.Nodes, process)

	// Build a list of expected events to wait for before unblocking based off
	// the configuration passed in.
	expectedEvents := []string{
		service.DatabasesIdentityEvent,
		service.DatabasesReady,
		service.TeleportReadyEvent,
	}

	// Start the process and block until the expected events have arrived.
	receivedEvents, err := StartAndWait(process, expectedEvents)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Retrieve auth server connector.
	var client *authclient.Client
	for _, event := range receivedEvents {
		if event.Name == service.DatabasesIdentityEvent {
			conn, ok := (event.Payload).(*service.Connector)
			if !ok {
				return nil, nil, trace.BadParameter("unsupported event payload type %q", event.Payload)
			}
			client = conn.Client
		}
	}
	if client == nil {
		return nil, nil, trace.BadParameter("failed to retrieve auth client")
	}

	i.Log.DebugContext(context.Background(), "Teleport Database Server started",
		"instance", i.Secrets.SiteName,
		"expected_events_count", len(expectedEvents),
		"received_events_count", len(receivedEvents),
	)

	return process, client, nil
}

func (i *TeleInstance) StartKube(t *testing.T, conf *servicecfg.Config, clusterName string) (*service.TeleportProcess, error) {
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	conf.DataDir = dataDir
	conf.SetAuthServerAddress(utils.NetAddr{
		AddrNetwork: "tcp",
		Addr:        i.Web,
	})
	conf.SetToken("token")
	conf.Testing.UploadEventsC = i.UploadEventsC
	conf.Auth.Enabled = false
	conf.Proxy.Enabled = false
	conf.Apps.Enabled = false
	conf.SSH.Enabled = false
	conf.Databases.Enabled = false
	conf.DebugService.Enabled = false

	conf.Kube.KubeconfigPath = filepath.Join(dataDir, "kube_config")
	if err := EnableKube(t, conf, clusterName); err != nil {
		return nil, trace.Wrap(err)
	}
	conf.Kube.ListenAddr = nil

	process, err := service.NewTeleport(conf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.Nodes = append(i.Nodes, process)

	expectedEvents := []string{
		service.KubeIdentityEvent,
		service.KubernetesReady,
		service.TeleportReadyEvent,
	}

	receivedEvents, err := StartAndWait(process, expectedEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	i.Log.DebugContext(context.Background(), "Teleport Kube Server started",
		"instance", i.Secrets.SiteName,
		"expected_events_count", len(expectedEvents),
		"received_events_count", len(receivedEvents),
	)

	return process, nil
}

// StartNodeAndProxy starts a SSH node and a Proxy Server and connects it to
// the cluster.
func (i *TeleInstance) StartNodeAndProxy(t *testing.T, name string) (sshPort, webProxyPort, sshProxyPort int) {
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
	require.NoError(t, err)

	i.tempDirs = append(i.tempDirs, dataDir)

	tconf := servicecfg.MakeDefaultConfig()

	tconf.Logger = i.Log
	authServer := utils.MustParseAddr(i.Auth)
	tconf.SetAuthServerAddress(*authServer)
	tconf.SetToken("token")
	tconf.HostUUID = name
	tconf.Hostname = name
	tconf.Testing.UploadEventsC = i.UploadEventsC
	tconf.DataDir = dataDir
	tconf.CachePolicy = servicecfg.CachePolicy{
		Enabled: true,
	}

	tconf.Auth.Enabled = false

	tconf.Proxy.Enabled = true
	tconf.Proxy.SSHAddr.Addr = NewListenerOn(t, i.Hostname, service.ListenerProxySSH, &tconf.FileDescriptors)
	sshProxyPort = Port(t, tconf.Proxy.SSHAddr.Addr)
	tconf.Proxy.WebAddr.Addr = NewListenerOn(t, i.Hostname, service.ListenerProxyWeb, &tconf.FileDescriptors)
	webProxyPort = Port(t, tconf.Proxy.WebAddr.Addr)
	tconf.Proxy.DisableReverseTunnel = true
	tconf.Proxy.DisableWebService = true

	tconf.SSH.Enabled = true
	tconf.SSH.Addr.Addr = NewListenerOn(t, i.Hostname, service.ListenerNodeSSH, &tconf.FileDescriptors)
	sshPort = Port(t, tconf.SSH.Addr.Addr)
	tconf.SSH.PublicAddrs = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        Loopback,
		},
		{
			AddrNetwork: "tcp",
			Addr:        Host,
		},
	}
	tconf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	tconf.InstanceMetadataClient = imds.NewDisabledIMDSClient()
	tconf.DebugService.Enabled = false

	// Create a new Teleport process and add it to the list of nodes that
	// compose this "cluster".
	process, err := service.NewTeleport(tconf)
	require.NoError(t, err)
	i.Nodes = append(i.Nodes, process)

	// Build a list of expected events to wait for before unblocking based off
	// the configuration passed in.
	expectedEvents := []string{
		service.ProxySSHReady,
		service.NodeSSHReady,
	}

	// Start the process and block until the expected events have arrived.
	receivedEvents, err := StartAndWait(process, expectedEvents)
	require.NoError(t, err)

	i.Log.DebugContext(context.Background(), "Teleport node and proxy started",
		"instance", i.Secrets.SiteName,
		"expected_events_count", len(expectedEvents),
		"received_events_count", len(receivedEvents),
	)

	return
}

// ProxyConfig is a set of configuration parameters for Proxy
// TODO(tcsc): Add file descriptor slice to inject FDs into proxy process
type ProxyConfig struct {
	// Name is a proxy name
	Name string
	// SSHAddr the address the node ssh service should listen on
	SSHAddr string
	// WebAddr the address the web service should listen on
	WebAddr string
	// KubeAddr is the kube proxy address.
	KubeAddr string
	// ReverseTunnelAddr the address the reverse proxy service should listen on
	ReverseTunnelAddr string
	// Disable the web service
	DisableWebService bool
	// Disable the web ui
	DisableWebInterface bool
	// Disable ALPN routing
	DisableALPNSNIListener bool
	// FileDescriptors holds FDs to be injected into the Teleport process
	FileDescriptors []*servicecfg.FileDescriptor
}

// StartProxy starts another Proxy Server and connects it to the cluster.
func (i *TeleInstance) StartProxy(cfg ProxyConfig, opts ...Option) (reversetunnelclient.Server, *service.TeleportProcess, error) {
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName+"-"+cfg.Name)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	tconf := servicecfg.MakeDefaultConfig()
	tconf.Logger = i.Log
	authServer := utils.MustParseAddr(i.Auth)
	tconf.SetAuthServerAddress(*authServer)
	tconf.CachePolicy = servicecfg.CachePolicy{Enabled: true}
	tconf.DataDir = dataDir
	tconf.Testing.UploadEventsC = i.UploadEventsC
	tconf.HostUUID = cfg.Name
	tconf.Hostname = cfg.Name
	tconf.SetToken("token")

	tconf.Auth.Enabled = false

	tconf.SSH.Enabled = false

	tconf.Proxy.Enabled = true
	tconf.Proxy.SSHAddr.Addr = cfg.SSHAddr
	tconf.Proxy.PublicAddrs = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        Loopback,
		},
		{
			AddrNetwork: "tcp",
			Addr:        Host,
		},
	}
	tconf.Proxy.ReverseTunnelListenAddr.Addr = cfg.ReverseTunnelAddr
	tconf.Proxy.WebAddr.Addr = cfg.WebAddr
	tconf.Proxy.Kube.Enabled = cfg.KubeAddr != ""
	tconf.Proxy.Kube.ListenAddr.Addr = cfg.KubeAddr
	tconf.Proxy.DisableReverseTunnel = false
	tconf.Proxy.DisableWebService = cfg.DisableWebService
	tconf.Proxy.DisableWebInterface = cfg.DisableWebInterface
	tconf.Proxy.DisableALPNSNIListener = cfg.DisableALPNSNIListener
	tconf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	tconf.InstanceMetadataClient = imds.NewDisabledIMDSClient()
	tconf.DebugService.Enabled = false
	tconf.FileDescriptors = cfg.FileDescriptors
	// apply options
	for _, o := range opts {
		o(tconf)
	}
	// Create a new Teleport process and add it to the list of nodes that
	// compose this "cluster".
	process, err := service.NewTeleport(tconf)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	i.Nodes = append(i.Nodes, process)

	// Build a list of expected events to wait for before unblocking based off
	// the configuration passed in.
	expectedEvents := []string{
		service.ProxyReverseTunnelReady,
		service.ProxySSHReady,
	}

	// Start the process and block until the expected events have arrived.
	receivedEvents, err := StartAndWait(process, expectedEvents)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	i.Log.DebugContext(context.Background(), "Teleport proxy started",
		"instance", i.Secrets.SiteName,
		"expected_events_count", len(expectedEvents),
		"received_events_count", len(receivedEvents),
	)

	// Extract and set reversetunnelclient.Server and reversetunnel.AgentPool upon
	// receipt of a ProxyReverseTunnelReady event
	for _, re := range receivedEvents {
		switch re.Name {
		case service.ProxyReverseTunnelReady:
			ts, ok := re.Payload.(reversetunnelclient.Server)
			if ok {
				return ts, process, nil
			}
		}
	}

	// If we get to here then something has gone seriously wrong as we can't find
	// the event in `receivedEvents` that we explicitly asserted was there
	// in `StartAndWait()`.
	return nil, nil, trace.Errorf("Missing expected %v event in %v",
		service.ProxyReverseTunnelReady, receivedEvents)
}

// Option is a functional option for configuring a ProxyConfig
type Option func(*servicecfg.Config)

// WithLegacyKubeProxy enables the legacy kube proxy.
func WithLegacyKubeProxy(kubeconfig string) Option {
	return func(tconf *servicecfg.Config) {
		tconf.Proxy.Kube.Enabled = true
		tconf.Proxy.Kube.KubeconfigPath = kubeconfig
		tconf.Proxy.Kube.LegacyKubeProxy = true
	}
}

// Reset re-creates the teleport instance based on the same configuration
// This is needed if you want to stop the instance, reset it and start again
func (i *TeleInstance) Reset() (err error) {
	if i.Process != nil {
		if err := i.Process.Close(); err != nil {
			return trace.Wrap(err)
		}
		if err := i.Process.Wait(); err != nil {
			return trace.Wrap(err)
		}
	}
	i.Process, err = service.NewTeleport(i.Config)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// AddUserUserWithRole adds user with one or many assigned roles
func (i *TeleInstance) AddUserWithRole(username string, roles ...types.Role) *User {
	user := &User{
		Username: username,
		Roles:    make([]types.Role, len(roles)),
	}
	copy(user.Roles, roles)
	i.Secrets.Users[username] = user
	return user
}

// Adds a new user into i Teleport instance. 'mappings' is a comma-separated
// list of OS users
func (i *TeleInstance) AddUser(username string, mappings []string) *User {
	i.Log.InfoContext(context.Background(), "Adding user to teleInstance", "user", username, "mappings", mappings)
	if mappings == nil {
		mappings = make([]string, 0)
	}
	user := &User{
		Username:      username,
		AllowedLogins: mappings,
	}
	i.Secrets.Users[username] = user
	return user
}

// Start will start the TeleInstance and then block until it is ready to
// process requests based off the passed in configuration.
func (i *TeleInstance) Start() error {
	// Build a list of expected events to wait for before unblocking based off
	// the configuration passed in.
	var expectedEvents []string
	if i.Config.Auth.Enabled {
		expectedEvents = append(expectedEvents, service.AuthTLSReady)
	}
	if i.Config.Proxy.Enabled {
		expectedEvents = append(expectedEvents, service.ProxyReverseTunnelReady)
		expectedEvents = append(expectedEvents, service.ProxySSHReady)
		expectedEvents = append(expectedEvents, service.ProxyAgentPoolReady)
		if !i.Config.Proxy.DisableWebService {
			expectedEvents = append(expectedEvents, service.ProxyWebServerReady)
		}
	}
	if i.Config.SSH.Enabled {
		expectedEvents = append(expectedEvents, service.NodeSSHReady)
	}
	if i.Config.Apps.Enabled {
		expectedEvents = append(expectedEvents, service.AppsReady)
	}
	if i.Config.Databases.Enabled {
		expectedEvents = append(expectedEvents, service.DatabasesReady)
	}
	if i.Config.Kube.Enabled {
		expectedEvents = append(expectedEvents, service.KubernetesReady)
	}

	if i.Config.Discovery.Enabled {
		expectedEvents = append(expectedEvents, service.DiscoveryReady)
	}

	expectedEvents = append(expectedEvents, service.InstanceReady)

	// Start the process and block until the expected events have arrived.
	receivedEvents, err := StartAndWait(i.Process, expectedEvents)
	if err != nil {
		return trace.Wrap(err)
	}

	// Extract and set reversetunnelclient.Server and reversetunnel.AgentPool upon
	// receipt of a ProxyReverseTunnelReady and ProxyAgentPoolReady respectively.
	for _, re := range receivedEvents {
		switch re.Name {
		case service.ProxyReverseTunnelReady:
			ts, ok := re.Payload.(reversetunnelclient.Server)
			if ok {
				i.Tunnel = ts
			}
		case service.ProxyAgentPoolReady:
			w, ok := re.Payload.(*reversetunnel.RemoteClusterTunnelManager)
			if ok {
				i.RemoteClusterWatcher = w
			}
		}
	}

	i.Log.DebugContext(context.Background(), "Teleport instance started",
		"instance", i.Secrets.SiteName,
		"expected_events_count", len(expectedEvents),
		"received_events_count", len(receivedEvents),
	)

	return nil
}

// ClientConfig is a client configuration
type ClientConfig struct {
	// TeleportUser is Teleport username
	TeleportUser string
	// Login is SSH login name
	Login string
	// Cluster is a cluster name to connect to
	Cluster string
	// Host string is a target host to connect to
	Host string
	// Port is a target port to connect to
	Port int
	// Proxy is an optional alternative proxy to use
	Proxy *ProxyConfig
	// ForwardAgent controls if the client requests it's agent be forwarded to
	// the server.
	ForwardAgent bool
	// JumpHost turns on jump host mode
	JumpHost bool
	// Labels represents host labels
	Labels map[string]string
	// Interactive launches with the terminal attached if true
	Interactive bool
	// Source IP to used in generated SSH cert
	SourceIP string
	// EnableEscapeSequences will scan Stdin for SSH escape sequences during command/shell execution.
	EnableEscapeSequences bool
	// Password to use when creating a web session
	Password string
	// Stdin overrides standard input for the session
	Stdin io.Reader
	// Stderr overrides standard error for the session
	Stderr io.Writer
	// Stdout overrides standard output for the session
	Stdout io.Writer
	// ALBAddr is the address to a local server that simulates a layer 7 load balancer.
	ALBAddr string
	// DisableSSHResumption disables SSH connection resumption.
	DisableSSHResumption bool
}

// NewClientWithCreds creates client with credentials
func (i *TeleInstance) NewClientWithCreds(cfg ClientConfig, creds UserCreds) (tc *client.TeleportClient, err error) {
	clt, err := i.NewUnauthenticatedClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = SetupUserCreds(clt, i.Config.Proxy.SSHAddr.Addr, creds)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt, nil
}

// NewUnauthenticatedClient returns a fully configured and un-authenticated client
func (i *TeleInstance) NewUnauthenticatedClient(cfg ClientConfig) (tc *client.TeleportClient, err error) {
	keyDir, err := os.MkdirTemp(i.Config.DataDir, "tsh")
	if err != nil {
		return nil, err
	}

	var webProxyAddr string
	var sshProxyAddr string
	var kubeProxyAddr string

	switch {
	case cfg.Proxy != nil:
		webProxyAddr = cfg.Proxy.WebAddr
		sshProxyAddr = cfg.Proxy.SSHAddr
		kubeProxyAddr = cfg.Proxy.KubeAddr
	case cfg.ALBAddr != "":
		if i.IsSinglePortSetup {
			webProxyAddr = cfg.ALBAddr
			sshProxyAddr = cfg.ALBAddr
			kubeProxyAddr = cfg.ALBAddr
		} else {
			webProxyAddr = cfg.ALBAddr
			sshProxyAddr = i.SSHProxy
			kubeProxyAddr = i.Config.Proxy.Kube.ListenAddr.Addr
		}
	default:
		webProxyAddr = i.Web
		sshProxyAddr = i.SSHProxy
		kubeProxyAddr = i.Config.Proxy.Kube.ListenAddr.Addr
	}

	fwdAgentMode := client.ForwardAgentNo
	if cfg.ForwardAgent {
		fwdAgentMode = client.ForwardAgentYes
	}

	if cfg.TeleportUser == "" {
		cfg.TeleportUser = cfg.Login
	}

	cconf := &client.Config{
		Username:                      cfg.TeleportUser,
		Host:                          cfg.Host,
		HostPort:                      cfg.Port,
		HostLogin:                     cfg.Login,
		InsecureSkipVerify:            true,
		ClientStore:                   client.NewFSClientStore(keyDir),
		SiteName:                      cfg.Cluster,
		ForwardAgent:                  fwdAgentMode,
		Labels:                        cfg.Labels,
		WebProxyAddr:                  webProxyAddr,
		SSHProxyAddr:                  sshProxyAddr,
		KubeProxyAddr:                 kubeProxyAddr,
		InteractiveCommand:            cfg.Interactive,
		TLSRoutingEnabled:             i.IsSinglePortSetup,
		TLSRoutingConnUpgradeRequired: cfg.ALBAddr != "",
		Tracer:                        tracing.NoopProvider().Tracer("test"),
		DisableEscapeSequences:        !cfg.EnableEscapeSequences,
		Stderr:                        cfg.Stderr,
		Stdin:                         cfg.Stdin,
		Stdout:                        cfg.Stdout,
		NonInteractive:                true,
		DisableSSHResumption:          cfg.DisableSSHResumption,
	}

	// JumpHost turns on jump host mode
	if cfg.JumpHost {
		cconf.JumpHosts = []utils.JumpHost{{
			Username: cfg.Login,
			Addr:     *utils.MustParseAddr(sshProxyAddr),
		}}
	}

	return client.NewClient(cconf)
}

// NewClient returns a fully configured and pre-authenticated client
// (pre-authenticated with server CAs and signed session key).
func (i *TeleInstance) NewClient(cfg ClientConfig) (*client.TeleportClient, error) {
	tc, err := i.NewUnauthenticatedClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return i.AddClientCredentials(tc, cfg)
}

// CreateWebUser creates a user with the provided password which can be
// used to create a web session.
func (i *TeleInstance) CreateWebUser(t *testing.T, username, password string) {
	user, err := types.NewUser(username)
	require.NoError(t, err)

	role := services.RoleForUser(user)
	role.SetLogins(types.Allow, []string{username})
	role, err = i.Process.GetAuthServer().UpsertRole(context.Background(), role)
	require.NoError(t, err)

	user.AddRole(role.GetName())
	_, err = i.Process.GetAuthServer().CreateUser(context.Background(), user)
	require.NoError(t, err)

	err = i.Process.GetAuthServer().UpsertPassword(user.GetName(), []byte(password))
	require.NoError(t, err)
}

// WebClient allows web sessions to be created as
// if they were from the UI.
type WebClient struct {
	tc      *client.TeleportClient
	i       *TeleInstance
	token   string
	cookies []*http.Cookie
}

// NewWebClient returns a fully configured and authenticated client
func (i *TeleInstance) NewWebClient(cfg ClientConfig) (*WebClient, error) {
	resp, cookies, err := CreateWebSession(i.Web, cfg.Login, cfg.Password)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract session cookie and bearer token.
	if len(cookies) != 1 {
		return nil, trace.BadParameter("unexpected number of cookies returned; got %d, want %d", len(cookies), 1)
	}
	cookie := cookies[0]
	if cookie.Name != websession.CookieName {
		return nil, trace.BadParameter("unexpected session cookies returned; got %s, want %s", cookie.Name, websession.CookieName)
	}

	tc, err := i.NewUnauthenticatedClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tc, err = i.AddClientCredentials(tc, cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	return &WebClient{
		tc:      tc,
		i:       i,
		token:   resp.Token,
		cookies: cookies,
	}, nil
}

// CreateWebSession establishes a web session in the same manner that the web UI
// does. There is no MFA performed, the session will only successfully be created
// if second factor configuration is `off`. The [web.CreateSessionResponse.Token] and
// cookies can be used to interact with any authenticated web api endpoints.
func CreateWebSession(proxyHost, user, password string) (*web.CreateSessionResponse, []*http.Cookie, error) {
	csReq, err := json.Marshal(web.CreateSessionReq{
		User: user,
		Pass: password,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Create POST request to create session.
	u := url.URL{
		Scheme: "https",
		Host:   proxyHost,
		Path:   "/v1/webapi/sessions/web",
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(csReq))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// Issue request.
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	defer resp.Body.Close()
	if http.StatusOK != resp.StatusCode {
		return nil, nil, trace.ConnectionProblem(nil, "received unexpected status code: %d", resp.StatusCode)
	}

	// Read in response.
	var csResp *web.CreateSessionResponse
	err = json.NewDecoder(resp.Body).Decode(&csResp)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return csResp, resp.Cookies(), nil
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

// SSH establishes an SSH connection via the web api in the same manner that
// the web UI does. The returned [web.TerminalStream] should be used as stdin/stdout
// for the session.
func (w *WebClient) SSH(termReq web.TerminalRequest) (*terminal.Stream, error) {
	u := url.URL{
		Host:   w.i.Web,
		Scheme: client.WSS,
		Path:   fmt.Sprintf("/v1/webapi/sites/%v/connect/ws", w.tc.SiteName),
	}
	data, err := json.Marshal(termReq)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("params", string(data))
	u.RawQuery = q.Encode()

	header := http.Header{}
	header.Add("Origin", "http://localhost")
	for _, cookie := range w.cookies {
		header.Add("Cookie", cookie.String())
	}

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	ws, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := makeAuthReqOverWS(ws, w.token); err != nil {
		return nil, trace.Wrap(err)
	}

	defer resp.Body.Close()
	return terminal.NewStream(context.Background(), terminal.StreamConfig{WS: ws}), nil
}

func (w *WebClient) JoinKubernetesSession(id string, mode types.SessionParticipantMode) (*terminal.Stream, error) {
	u := url.URL{
		Host:   w.i.Web,
		Scheme: client.WSS,
		Path:   fmt.Sprintf("/v1/webapi/sites/%v/kube/exec/ws", w.tc.SiteName),
	}

	params := struct {
		// Term is the initial PTY size.
		Term session.TerminalParams `json:"term"`
		// SessionID is a Teleport session ID to join as.
		SessionID session.ID `json:"sid"`
		// ParticipantMode is the mode that determines what you can do when you join an active session.
		ParticipantMode types.SessionParticipantMode `json:"mode"`
	}{
		SessionID:       session.ID(id),
		ParticipantMode: mode,
	}

	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("params", string(data))
	u.RawQuery = q.Encode()

	header := http.Header{}
	header.Add("Origin", "http://localhost")
	for _, cookie := range w.cookies {
		header.Add("Cookie", cookie.String())
	}

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	ws, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := makeAuthReqOverWS(ws, w.token); err != nil {
		return nil, trace.Wrap(err)
	}

	defer resp.Body.Close()
	return terminal.NewStream(context.Background(), terminal.StreamConfig{WS: ws}), nil
}

// AddClientCredentials adds authenticated credentials to a client.
// (server CAs and signed session key).
func (i *TeleInstance) AddClientCredentials(tc *client.TeleportClient, cfg ClientConfig) (*client.TeleportClient, error) {
	login := cfg.Login
	if cfg.TeleportUser != "" {
		login = cfg.TeleportUser
	}

	// Generate certificates for the user simulating login.
	creds, err := GenerateUserCreds(UserCredsRequest{
		Process:  i.Process,
		Username: login,
		SourceIP: cfg.SourceIP,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Add key to client and update CAs that will be trusted (equivalent to
	// updating "known hosts" with OpenSSH.
	err = tc.AddKeyRing(&creds.KeyRing)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cas, err := i.Secrets.GetCAs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, ca := range cas {
		err = tc.AddTrustedCA(context.Background(), ca)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return tc, nil
}

// StopProxy loops over the extra nodes in a TeleInstance and stops all
// nodes where the proxy server is enabled.
func (i *TeleInstance) StopProxy() error {
	var errors []error

	for _, p := range i.Nodes {
		if p.Config.Proxy.Enabled {
			if err := p.Close(); err != nil {
				errors = append(errors, err)
				i.Log.ErrorContext(context.Background(), "Failed closing extra proxy", "error", err)
			}
			if err := p.Wait(); err != nil {
				errors = append(errors, err)
				i.Log.ErrorContext(context.Background(), "Failed to stop extra proxy", "error", err)
			}
		}
	}

	return trace.NewAggregate(errors...)
}

// StopNodes stops additional nodes
func (i *TeleInstance) StopNodes() error {
	var errors []error
	for _, node := range i.Nodes {
		if err := node.Close(); err != nil {
			errors = append(errors, err)
			i.Log.ErrorContext(context.Background(), "Failed closing extra node", "error", err)
		}
		if err := node.Wait(); err != nil {
			errors = append(errors, err)
			i.Log.ErrorContext(context.Background(), "Failed stopping extra node", "error", err)
		}
	}
	return trace.NewAggregate(errors...)
}

// RestartAuth stops and then starts the auth service.
func (i *TeleInstance) RestartAuth() error {
	if i.Process == nil {
		return nil
	}

	i.Log.InfoContext(context.Background(), "Asking Teleport instance to stop", "instance", i.Secrets.SiteName)
	err := i.Process.Close()
	if err != nil {
		i.Log.ErrorContext(context.Background(), "Failed closing the teleport process", "error", err)
		return trace.Wrap(err)
	}
	i.Log.InfoContext(context.Background(), "Teleport instance stopped", "instance", i.Secrets.SiteName)

	if err := i.Process.Wait(); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(i.Process.Start())
}

// StopAuth stops the auth server process. If removeData is true, the data
// directory is also cleaned up.
func (i *TeleInstance) StopAuth(removeData bool) error {
	defer func() {
		if i.Config != nil && removeData {
			i.Log.InfoContext(context.Background(), "Removing data dir", "data_dir", i.Config.DataDir)
			if err := os.RemoveAll(i.Config.DataDir); err != nil {
				i.Log.ErrorContext(context.Background(), "Failed removing temporary local Teleport directory", "error", err)
			}
		}
		i.Process = nil
	}()

	if i.Process == nil {
		return nil
	}
	i.Log.InfoContext(context.Background(), "Asking Teleport instance to stop", "instance", i.Secrets.SiteName)
	err := i.Process.Close()
	if err != nil {
		i.Log.ErrorContext(context.Background(), "Failed closing the teleport process", "error", err)
		return trace.Wrap(err)
	}
	defer func() {
		i.Log.InfoContext(context.Background(), "Teleport instance stopped", "instance", i.Secrets.SiteName)
	}()
	return i.Process.Wait()
}

// StopAll stops all spawned processes (auth server, nodes, proxies). StopAll
// should always be called at the end of TeleInstance's usage.
func (i *TeleInstance) StopAll() error {
	var errors []error

	// Stop all processes within this instance.
	errors = append(errors, i.StopNodes())
	errors = append(errors, i.StopProxy())
	errors = append(errors, i.StopAuth(true))

	// Remove temporary data directories that were created.
	for _, dir := range i.tempDirs {
		errors = append(errors, os.RemoveAll(dir))
	}

	i.Log.InfoContext(context.Background(), "Stopped all teleport services for site", "instance", i.Secrets.SiteName)
	return trace.NewAggregate(errors...)
}

// WaitForNodeCount waits for a certain number of nodes in the provided cluster
// to be visible to the Proxy. This should be called prior to any client dialing
// of nodes to be sure that the node is registered and routable.
func (i *TeleInstance) WaitForNodeCount(ctx context.Context, cluster string, count int) error {
	const (
		deadline     = time.Second * 30
		iterWaitTime = time.Second
	)

	err := retryutils.RetryStaticFor(deadline, iterWaitTime, func() error {
		site, err := i.Tunnel.GetSite(cluster)
		if err != nil {
			return trace.Wrap(err)
		}

		// Validate that the site cache contains the expected count.
		accessPoint, err := site.CachingAccessPoint()
		if err != nil {
			return trace.Wrap(err)
		}

		nodes, err := accessPoint.GetNodes(ctx, apidefaults.Namespace)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(nodes) != count {
			return trace.BadParameter("cache contained %v nodes, but wanted to find %v nodes", len(nodes), count)
		}

		// Validate that the site watcher contains the expected count.
		watcher, err := site.NodeWatcher()
		if err != nil {
			return trace.Wrap(err)
		}

		if watcher.ResourceCount() != count {
			return trace.BadParameter("node watcher contained %v nodes, but wanted to find %v nodes", watcher.ResourceCount(), count)
		}

		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
