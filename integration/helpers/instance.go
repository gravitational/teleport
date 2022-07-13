// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helpers

import (
	"context"
	"crypto/rsa"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

const (
	Loopback = "127.0.0.1"
	Host     = "localhost"
)

func fatalIf(err error) {
	if err != nil {
		log.Fatalf("%v at %v", string(debug.Stack()), err)
	}
}

type User struct {
	Username      string       `json:"username"`
	AllowedLogins []string     `json:"logins"`
	Key           *client.Key  `json:"key"`
	Roles         []types.Role `json:"-"`
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
	Cert []byte `json:"cert"`
	// TLSCACert is the certificate of the trusted certificate authority
	TLSCACert []byte `json:"tls_ca_cert"`
	// TLSCert is client TLS X509 certificate
	TLSCert []byte `json:"tls_cert"`
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

// GetCAs return an array of CAs stored by the secrets object. In i
// case we always return hard-coded userCA + hostCA (and they share keys
// for simplicity)
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
				Cert:    s.TLSCACert,
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
				Cert:    s.TLSCACert,
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
				Cert:    s.TLSCACert,
			}},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return []types.CertAuthority{hostCA, userCA, dbCA}, nil
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
			ProxyAddress:         i.GetWebAddr(),
			ReverseTunnelAddress: i.GetReverseTunnelAddr(),
			RoleMap:              roleMap,
		},
	}
}

func (s *InstanceSecrets) AsSlice() []*InstanceSecrets {
	return []*InstanceSecrets{s}
}

func (s *InstanceSecrets) GetIdentity() *auth.Identity {
	i, err := auth.ReadIdentityFromKeyPair(s.PrivKey, &proto.Certs{
		SSH:        s.Cert,
		TLS:        s.TLSCert,
		TLSCACerts: [][]byte{s.TLSCACert},
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
	Config               *service.Config
	Tunnel               reversetunnel.Server
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
	Log utils.Logger
	InstancePorts
}

// InstanceConfig is an instance configuration
type InstanceConfig struct {
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
	// Log specifies the logger
	Log utils.Logger
	// Ports is a collection of instance ports.
	Ports *InstancePorts
}

// NewInstance creates a new Teleport process instance.
//
// The caller is responsible for calling StopAll on the returned instance to
// clean up spawned processes.
func NewInstance(cfg InstanceConfig) *TeleInstance {
	var err error
	if cfg.NodeName == "" {
		cfg.NodeName, err = os.Hostname()
		fatalIf(err)
	}

	if cfg.Ports == nil {
		cfg.Ports = StandardPortSetup()
	}
	if cfg.Ports.Host == "" {
		cfg.Ports.Host = cfg.NodeName
	}

	// generate instance secrets (keys):
	keygen := native.New(context.TODO())
	if cfg.Priv == nil || cfg.Pub == nil {
		cfg.Priv, cfg.Pub, _ = keygen.GenerateKeyPair()
	}
	rsaKey, err := ssh.ParseRawPrivateKey(cfg.Priv)
	fatalIf(err)

	tlsCACert, err := tlsca.GenerateSelfSignedCAWithSigner(rsaKey.(*rsa.PrivateKey), pkix.Name{
		CommonName:   cfg.ClusterName,
		Organization: []string{cfg.ClusterName},
	}, nil, defaults.CATTL)
	fatalIf(err)

	signer, err := ssh.ParsePrivateKey(cfg.Priv)
	fatalIf(err)

	cert, err := keygen.GenerateHostCert(services.HostCertParams{
		CASigner:      signer,
		PublicHostKey: cfg.Pub,
		HostID:        cfg.HostID,
		NodeName:      cfg.NodeName,
		ClusterName:   cfg.ClusterName,
		Role:          types.RoleAdmin,
		TTL:           24 * time.Hour,
	})
	fatalIf(err)
	tlsCA, err := tlsca.FromKeys(tlsCACert, cfg.Priv)
	fatalIf(err)
	cryptoPubKey, err := sshutils.CryptoPublicKey(cfg.Pub)
	fatalIf(err)
	identity := tlsca.Identity{
		Username: fmt.Sprintf("%v.%v", cfg.HostID, cfg.ClusterName),
		Groups:   []string{string(types.RoleAdmin)},
	}
	clock := clockwork.NewRealClock()
	subject, err := identity.Subject()
	fatalIf(err)
	tlsCert, err := tlsCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: cryptoPubKey,
		Subject:   subject,
		NotAfter:  clock.Now().UTC().Add(time.Hour * 24),
	})
	fatalIf(err)

	i := &TeleInstance{
		Hostname:      cfg.NodeName,
		UploadEventsC: make(chan events.UploadEvent, 100),
		Log:           cfg.Log,
		InstancePorts: *cfg.Ports,
	}

	secrets := InstanceSecrets{
		SiteName:   cfg.ClusterName,
		PrivKey:    cfg.Priv,
		PubKey:     cfg.Pub,
		Cert:       cert,
		TLSCACert:  tlsCACert,
		TLSCert:    tlsCert,
		TunnelAddr: net.JoinHostPort(cfg.NodeName, i.GetPortReverseTunnel()),
		Users:      make(map[string]*User),
	}

	i.Secrets = secrets
	return i
}

// GetSiteAPI is a helper which returns an API endpoint to a site with
// a given name. i endpoint implements HTTP-over-SSH access to the
// site's auth server.
func (i *TeleInstance) GetSiteAPI(siteName string) auth.ClientI {
	siteTunnel, err := i.Tunnel.GetSite(siteName)
	if err != nil {
		log.Warn(err)
		return nil
	}
	siteAPI, err := siteTunnel.GetClient()
	if err != nil {
		log.Warn(err)
		return nil
	}
	return siteAPI
}

// Create creates a new instance of Teleport which trusts a list of other clusters (other
// instances)
func (i *TeleInstance) Create(t *testing.T, trustedSecrets []*InstanceSecrets, enableSSH bool, console io.Writer) error {
	tconf := service.MakeDefaultConfig()
	tconf.SSH.Enabled = enableSSH
	tconf.Console = console
	tconf.Log = i.Log
	tconf.Proxy.DisableWebService = true
	tconf.Proxy.DisableWebInterface = true
	tconf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	return i.CreateEx(t, trustedSecrets, tconf)
}

// GenerateConfig generates instance config
func (i *TeleInstance) GenerateConfig(t *testing.T, trustedSecrets []*InstanceSecrets, tconf *service.Config) (*service.Config, error) {
	var err error
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	if tconf == nil {
		tconf = service.MakeDefaultConfig()
	}
	tconf.Log = i.Log
	tconf.DataDir = dataDir
	tconf.UploadEventsC = i.UploadEventsC
	tconf.CachePolicy.Enabled = true
	tconf.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: i.Secrets.SiteName,
	})
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
	tconf.SSH.Addr.Addr = net.JoinHostPort(i.Hostname, i.GetPortSSH())
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
	tconf.Auth.ListenAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortAuth())
	tconf.Auth.PublicAddrs = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        i.Hostname,
		},
	}
	tconf.Proxy.PublicAddrs = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        i.Hostname,
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

	if i.isSinglePortSetup {
		tconf.Proxy.WebAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortWeb())
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
		tconf.Proxy.SSHAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortProxy())
		tconf.Proxy.WebAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortWeb())
		tconf.Proxy.MySQLAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortMySQL())
		if i.Postgres != nil {
			// Postgres proxy port was configured on a separate listener.
			tconf.Proxy.PostgresAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortPostgres())
		}
		if i.Mongo != nil {
			// Mongo proxy port was configured on a separate listener.
			tconf.Proxy.MongoAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortMongo())
		}
	}
	tconf.AuthServers = append(tconf.AuthServers, tconf.Auth.ListenAddr)
	tconf.Auth.StorageConfig = backend.Config{
		Type:   lite.GetName(),
		Params: backend.Params{"path": dataDir + string(os.PathListSeparator) + defaults.BackendDir, "poll_stream_period": 50 * time.Millisecond},
	}

	tconf.Kube.CheckImpersonationPermissions = nullImpersonationCheck

	tconf.Keygen = testauthority.New()
	tconf.MaxRetryPeriod = defaults.HighResPollingPeriod
	tconf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	i.Config = tconf
	return tconf, nil
}

// CreateEx creates a new instance of Teleport which trusts a list of other clusters (other
// instances)
//
// Unlike Create() it allows for greater customization because it accepts
// a full Teleport config structure
func (i *TeleInstance) CreateEx(t *testing.T, trustedSecrets []*InstanceSecrets, tconf *service.Config) error {
	ctx := context.TODO()
	tconf, err := i.GenerateConfig(t, trustedSecrets, tconf)
	if err != nil {
		return trace.Wrap(err)
	}
	i.Config = tconf
	i.Process, err = service.NewTeleport(tconf, service.WithIMDSClient(&DisabledIMDSClient{}))
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
			role.SetOptions(roleOptions)

			err = auth.UpsertRole(ctx, role)
			if err != nil {
				return trace.Wrap(err)
			}
			teleUser.AddRole(role.GetMetadata().Name)
		} else {
			for _, role := range user.Roles {
				err := auth.UpsertRole(ctx, role)
				if err != nil {
					return trace.Wrap(err)
				}
				teleUser.AddRole(role.GetName())
			}
		}
		err = auth.UpsertUser(teleUser)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// StartNode starts a SSH node and connects it to the cluster.
func (i *TeleInstance) StartNode(tconf *service.Config) (*service.TeleportProcess, error) {
	return i.startNode(tconf, i.GetPortAuth())
}

// StartReverseTunnelNode starts a SSH node and connects it to the cluster via reverse tunnel.
func (i *TeleInstance) StartReverseTunnelNode(tconf *service.Config) (*service.TeleportProcess, error) {
	return i.startNode(tconf, i.GetPortWeb())
}

// startNode starts a node and connects it to the cluster.
func (i *TeleInstance) startNode(tconf *service.Config, authPort string) (*service.TeleportProcess, error) {
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	tconf.DataDir = dataDir

	authServer := utils.MustParseAddr(net.JoinHostPort(i.Hostname, authPort))
	tconf.AuthServers = append(tconf.AuthServers, *authServer)
	tconf.Token = "token"
	tconf.UploadEventsC = i.UploadEventsC
	tconf.CachePolicy = service.CachePolicy{
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

	// Create a new Teleport process and add it to the list of nodes that
	// compose this "cluster".
	process, err := service.NewTeleport(tconf, service.WithIMDSClient(&DisabledIMDSClient{}))
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

	log.Debugf("Teleport node (in instance %v) started: %v/%v events received.",
		i.Secrets.SiteName, len(expectedEvents), len(receivedEvents))
	return process, nil
}

func (i *TeleInstance) StartApp(conf *service.Config) (*service.TeleportProcess, error) {
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	conf.DataDir = dataDir
	conf.AuthServers = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        net.JoinHostPort(Loopback, i.GetPortWeb()),
		},
	}
	conf.Token = "token"
	conf.UploadEventsC = i.UploadEventsC
	conf.Auth.Enabled = false
	conf.Proxy.Enabled = false

	// Create a new Teleport process and add it to the list of nodes that
	// compose this "cluster".
	process, err := service.NewTeleport(conf, service.WithIMDSClient(&DisabledIMDSClient{}))
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

	log.Debugf("Teleport Application Server (in instance %v) started: %v/%v events received.",
		i.Secrets.SiteName, len(expectedEvents), len(receivedEvents))
	return process, nil
}

func (i *TeleInstance) StartApps(configs []*service.Config) ([]*service.TeleportProcess, error) {
	type result struct {
		process *service.TeleportProcess
		tmpDir  string
		err     error
	}

	results := make(chan result, len(configs))
	for _, conf := range configs {
		go func(cfg *service.Config) {
			dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
			if err != nil {
				results <- result{err: err}
			}

			cfg.DataDir = dataDir
			cfg.AuthServers = []utils.NetAddr{
				{
					AddrNetwork: "tcp",
					Addr:        net.JoinHostPort(Loopback, i.GetPortWeb()),
				},
			}
			cfg.Token = "token"
			cfg.UploadEventsC = i.UploadEventsC
			cfg.Auth.Enabled = false
			cfg.Proxy.Enabled = false

			// Create a new Teleport process and add it to the list of nodes that
			// compose this "cluster".
			process, err := service.NewTeleport(cfg, service.WithIMDSClient(&DisabledIMDSClient{}))
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

			log.Debugf("Teleport Application Server (in instance %v) started: %v/%v events received.",
				i.Secrets.SiteName, len(expectedEvents), len(receivedEvents))

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
func (i *TeleInstance) StartDatabase(conf *service.Config) (*service.TeleportProcess, *auth.Client, error) {
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	conf.DataDir = dataDir
	conf.AuthServers = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        net.JoinHostPort(Loopback, i.GetPortWeb()),
		},
	}
	conf.Token = "token"
	conf.UploadEventsC = i.UploadEventsC
	conf.Auth.Enabled = false
	conf.Proxy.Enabled = false
	conf.Apps.Enabled = false
	conf.SSH.Enabled = false

	// Create a new Teleport process and add it to the list of nodes that
	// compose this "cluster".
	process, err := service.NewTeleport(conf, service.WithIMDSClient(&DisabledIMDSClient{}))
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
	var client *auth.Client
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

	log.Debugf("Teleport Database Server (in instance %v) started: %v/%v events received.",
		i.Secrets.SiteName, len(expectedEvents), len(receivedEvents))
	return process, client, nil
}

func (i *TeleInstance) StartKube(conf *service.Config, clusterName string) (*service.TeleportProcess, error) {
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	conf.DataDir = dataDir
	conf.AuthServers = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        net.JoinHostPort(Loopback, i.GetPortWeb()),
		},
	}
	conf.Token = "token"
	conf.UploadEventsC = i.UploadEventsC
	conf.Auth.Enabled = false
	conf.Proxy.Enabled = false
	conf.Apps.Enabled = false
	conf.SSH.Enabled = false
	conf.Databases.Enabled = false

	conf.Kube.KubeconfigPath = filepath.Join(dataDir, "kube_config")
	if err := EnableKube(conf, clusterName); err != nil {
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
	log.Debugf("Teleport Kube Server (in instance %v) started: %v/%v events received.",
		i.Secrets.SiteName, len(expectedEvents), len(receivedEvents))
	return process, nil
}

// StartNodeAndProxy starts a SSH node and a Proxy Server and connects it to
// the cluster.
func (i *TeleInstance) StartNodeAndProxy(name string, sshPort, proxyWebPort, proxySSHPort int) error {
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	tconf := service.MakeDefaultConfig()

	tconf.Log = i.Log
	authServer := utils.MustParseAddr(net.JoinHostPort(i.Hostname, i.GetPortAuth()))
	tconf.AuthServers = append(tconf.AuthServers, *authServer)
	tconf.Token = "token"
	tconf.HostUUID = name
	tconf.Hostname = name
	tconf.UploadEventsC = i.UploadEventsC
	tconf.DataDir = dataDir
	tconf.CachePolicy = service.CachePolicy{
		Enabled: true,
	}

	tconf.Auth.Enabled = false

	tconf.Proxy.Enabled = true
	tconf.Proxy.SSHAddr.Addr = net.JoinHostPort(i.Hostname, fmt.Sprintf("%v", proxySSHPort))
	tconf.Proxy.WebAddr.Addr = net.JoinHostPort(i.Hostname, fmt.Sprintf("%v", proxyWebPort))
	tconf.Proxy.DisableReverseTunnel = true
	tconf.Proxy.DisableWebService = true

	tconf.SSH.Enabled = true
	tconf.SSH.Addr.Addr = net.JoinHostPort(i.Hostname, fmt.Sprintf("%v", sshPort))
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

	// Create a new Teleport process and add it to the list of nodes that
	// compose this "cluster".
	process, err := service.NewTeleport(tconf, service.WithIMDSClient(&DisabledIMDSClient{}))
	if err != nil {
		return trace.Wrap(err)
	}
	i.Nodes = append(i.Nodes, process)

	// Build a list of expected events to wait for before unblocking based off
	// the configuration passed in.
	expectedEvents := []string{
		service.ProxySSHReady,
		service.NodeSSHReady,
	}

	// Start the process and block until the expected events have arrived.
	receivedEvents, err := StartAndWait(process, expectedEvents)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Debugf("Teleport node and proxy (in instance %v) started: %v/%v events received.",
		i.Secrets.SiteName, len(expectedEvents), len(receivedEvents))
	return nil
}

// ProxyConfig is a set of configuration parameters for Proxy
type ProxyConfig struct {
	// Name is a proxy name
	Name string
	// SSHPort is SSH proxy port
	SSHPort int
	// WebPort is web proxy port
	WebPort int
	// ReverseTunnelPort is a port for reverse tunnel addresses
	ReverseTunnelPort int
	// Disable the web service
	DisableWebService bool
	// Disable the web ui
	DisableWebInterface bool
	// Disable ALPN routing
	DisableALPNSNIListener bool
}

// StartProxy starts another Proxy Server and connects it to the cluster.
func (i *TeleInstance) StartProxy(cfg ProxyConfig) (reversetunnel.Server, error) {
	dataDir, err := os.MkdirTemp("", "cluster-"+i.Secrets.SiteName+"-"+cfg.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	tconf := service.MakeDefaultConfig()
	tconf.Console = nil
	tconf.Log = i.Log
	authServer := utils.MustParseAddr(net.JoinHostPort(i.Hostname, i.GetPortAuth()))
	tconf.AuthServers = append(tconf.AuthServers, *authServer)
	tconf.CachePolicy = service.CachePolicy{Enabled: true}
	tconf.DataDir = dataDir
	tconf.UploadEventsC = i.UploadEventsC
	tconf.HostUUID = cfg.Name
	tconf.Hostname = cfg.Name
	tconf.Token = "token"

	tconf.Auth.Enabled = false

	tconf.SSH.Enabled = false

	tconf.Proxy.Enabled = true
	tconf.Proxy.SSHAddr.Addr = net.JoinHostPort(i.Hostname, fmt.Sprintf("%v", cfg.SSHPort))
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
	tconf.Proxy.ReverseTunnelListenAddr.Addr = net.JoinHostPort(i.Hostname, fmt.Sprintf("%v", cfg.ReverseTunnelPort))
	tconf.Proxy.WebAddr.Addr = net.JoinHostPort(i.Hostname, fmt.Sprintf("%v", cfg.WebPort))
	tconf.Proxy.DisableReverseTunnel = false
	tconf.Proxy.DisableWebService = cfg.DisableWebService
	tconf.Proxy.DisableWebInterface = cfg.DisableWebInterface
	tconf.Proxy.DisableALPNSNIListener = cfg.DisableALPNSNIListener
	tconf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	// Create a new Teleport process and add it to the list of nodes that
	// compose this "cluster".
	process, err := service.NewTeleport(tconf, service.WithIMDSClient(&DisabledIMDSClient{}))
	if err != nil {
		return nil, trace.Wrap(err)
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
		return nil, trace.Wrap(err)
	}

	// Extract and set reversetunnel.Server and reversetunnel.AgentPool upon
	// receipt of a ProxyReverseTunnelReady event
	var tunnel reversetunnel.Server
	for _, re := range receivedEvents {
		switch re.Name {
		case service.ProxyReverseTunnelReady:
			ts, ok := re.Payload.(reversetunnel.Server)
			if ok {
				tunnel = ts
				break
			}
		}
	}

	log.Debugf("Teleport proxy (in instance %v) started: %v/%v events received.",
		i.Secrets.SiteName, len(expectedEvents), len(receivedEvents))
	return tunnel, nil
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
	i.Process, err = service.NewTeleport(i.Config, service.WithIMDSClient(&DisabledIMDSClient{}))
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
	log.Infof("teleInstance.AddUser(%v) mapped to %v", username, mappings)
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
	expectedEvents := []string{}
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

	// Start the process and block until the expected events have arrived.
	receivedEvents, err := StartAndWait(i.Process, expectedEvents)
	if err != nil {
		return trace.Wrap(err)
	}

	// Extract and set reversetunnel.Server and reversetunnel.AgentPool upon
	// receipt of a ProxyReverseTunnelReady and ProxyAgentPoolReady respectively.
	for _, re := range receivedEvents {
		switch re.Name {
		case service.ProxyReverseTunnelReady:
			ts, ok := re.Payload.(reversetunnel.Server)
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

	log.Debugf("Teleport instance %v started: %v/%v events received.",
		i.Secrets.SiteName, len(receivedEvents), len(expectedEvents))
	return nil
}

// ClientConfig is a client configuration
type ClientConfig struct {
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

// NewUnauthenticatedClient returns a fully configured and pre-authenticated client
// (pre-authenticated with server CAs and signed session key)
func (i *TeleInstance) NewUnauthenticatedClient(cfg ClientConfig) (tc *client.TeleportClient, err error) {
	keyDir, err := os.MkdirTemp(i.Config.DataDir, "tsh")
	if err != nil {
		return nil, err
	}

	proxyConf := &i.Config.Proxy
	var proxyHost string
	if !proxyConf.SSHAddr.IsEmpty() {
		proxyHost, _, err = net.SplitHostPort(proxyConf.SSHAddr.Addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var webProxyAddr string
	var sshProxyAddr string

	if cfg.Proxy == nil {
		webProxyAddr = i.GetWebAddr()
		sshProxyAddr = i.GetProxyAddr()
	} else {
		webProxyAddr = net.JoinHostPort(proxyHost, strconv.Itoa(cfg.Proxy.WebPort))
		sshProxyAddr = net.JoinHostPort(proxyHost, strconv.Itoa(cfg.Proxy.SSHPort))
	}

	fwdAgentMode := client.ForwardAgentNo
	if cfg.ForwardAgent {
		fwdAgentMode = client.ForwardAgentYes
	}

	cconf := &client.Config{
		Username:              cfg.Login,
		Host:                  cfg.Host,
		HostPort:              cfg.Port,
		HostLogin:             cfg.Login,
		InsecureSkipVerify:    true,
		KeysDir:               keyDir,
		SiteName:              cfg.Cluster,
		ForwardAgent:          fwdAgentMode,
		Labels:                cfg.Labels,
		WebProxyAddr:          webProxyAddr,
		SSHProxyAddr:          sshProxyAddr,
		Interactive:           cfg.Interactive,
		TLSRoutingEnabled:     i.isSinglePortSetup,
		Tracer:                tracing.NoopProvider().Tracer("test"),
		EnableEscapeSequences: cfg.EnableEscapeSequences,
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

	// Generate certificates for the user simulating login.
	creds, err := GenerateUserCreds(UserCredsRequest{
		Process:  i.Process,
		Username: cfg.Login,
		SourceIP: cfg.SourceIP,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Add key to client and update CAs that will be trusted (equivalent to
	// updating "known hosts" with OpenSSH.
	_, err = tc.AddKey(&creds.Key)
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
				i.Log.Errorf("Failed closing extra proxy: %v.", err)
			}
			if err := p.Wait(); err != nil {
				errors = append(errors, err)
				i.Log.Errorf("Failed to stop extra proxy: %v.", err)
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
			i.Log.Errorf("Failed closing extra node %v", err)
		}
		if err := node.Wait(); err != nil {
			errors = append(errors, err)
			i.Log.Errorf("Failed stopping extra node %v", err)
		}
	}
	return trace.NewAggregate(errors...)
}

// StopAuth stops the auth server process. If removeData is true, the data
// directory is also cleaned up.
func (i *TeleInstance) StopAuth(removeData bool) error {
	defer func() {
		if i.Config != nil && removeData {
			i.Log.Infoln("Removing data dir", i.Config.DataDir)
			if err := os.RemoveAll(i.Config.DataDir); err != nil {
				i.Log.WithError(err).Error("Failed removing temporary local Teleport directory.")
			}
		}
		i.Process = nil
	}()

	if i.Process == nil {
		return nil
	}
	i.Log.Infof("Asking Teleport instance %q to stop", i.Secrets.SiteName)
	err := i.Process.Close()
	if err != nil {
		i.Log.WithError(err).Error("Failed closing the teleport process.")
		return trace.Wrap(err)
	}
	defer func() {
		i.Log.Infof("Teleport instance %q stopped!", i.Secrets.SiteName)
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

	i.Log.Infof("Stopped all teleport services for site %q", i.Secrets.SiteName)
	return trace.NewAggregate(errors...)
}
