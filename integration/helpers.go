/*
Copyright 2018 Gravitational, Inc.

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

package integration

import (
	"context"
	"crypto/rsa"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

const (
	Loopback = "127.0.0.1"
	Host     = "localhost"
)

// SetTestTimeouts affects global timeouts inside Teleport, making connections
// work faster but consuming more CPU (useful for integration testing)
func SetTestTimeouts(t time.Duration) {
	defaults.KeepAliveInterval = t
	defaults.ResyncInterval = t
	defaults.ServerKeepAliveTTL = t
	defaults.SessionRefreshPeriod = t
	defaults.HeartbeatCheckPeriod = t
	defaults.CachePollPeriod = t
}

// TeleInstance represents an in-memory instance of a teleport
// process for testing
type TeleInstance struct {
	// Secrets holds the keys (pub, priv and derived cert) of i instance
	Secrets InstanceSecrets

	// Slice of TCP ports used by Teleport services
	Ports []int

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

	// log specifies the instance logger
	log utils.Logger
}

type User struct {
	Username      string          `json:"username"`
	AllowedLogins []string        `json:"logins"`
	Key           *client.Key     `json:"key"`
	Roles         []services.Role `json:"-"`
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
	// ListenAddr is a reverse tunnel listening port, allowing
	// other sites to connect to i instance. Set to empty
	// string if i instance is not allowing incoming tunnels
	ListenAddr string `json:"tunnel_addr"`
	// WebProxyAddr is address for web proxy
	WebProxyAddr string `json:"web_proxy_addr"`
	// MySQLProxyAddr is the address of MySQL proxy.
	MySQLProxyAddr string `json:"mysql_proxy_addr"`
	// list of users i instance trusts (key in the map is username)
	Users map[string]*User `json:"users"`
}

func (s *InstanceSecrets) String() string {
	bytes, _ := json.MarshalIndent(s, "", "\t")
	return string(bytes)
}

// InstanceConfig is an instance configuration
type InstanceConfig struct {
	// ClusterName is a cluster name of the instance
	ClusterName string
	// HostID is a host id of the instance
	HostID string
	// NodeName is a node name of the instance
	NodeName string
	// Ports is a list of assigned ports to use
	Ports []int
	// Priv is SSH private key of the instance
	Priv []byte
	// Pub is SSH public key of the instance
	Pub []byte
	// MultiplexProxy uses the same port for web and SSH reverse tunnel proxy
	MultiplexProxy bool

	// log specifies the logger
	log utils.Logger
}

// NewInstance creates a new Teleport process instance.
//
// The caller is responsible for calling StopAll on the returned instance to
// clean up spawned processes.
func NewInstance(cfg InstanceConfig) *TeleInstance {
	var err error
	if len(cfg.Ports) < 5 {
		fatalIf(fmt.Errorf("not enough free ports given: %v", cfg.Ports))
	}
	if cfg.NodeName == "" {
		cfg.NodeName, err = os.Hostname()
		fatalIf(err)
	}
	// generate instance secrets (keys):
	keygen := native.New(context.TODO(), native.PrecomputeKeys(0))
	if cfg.Priv == nil || cfg.Pub == nil {
		cfg.Priv, cfg.Pub, _ = keygen.GenerateKeyPair("")
	}
	rsaKey, err := ssh.ParseRawPrivateKey(cfg.Priv)
	fatalIf(err)

	tlsCAKey, tlsCACert, err := tlsca.GenerateSelfSignedCAWithPrivateKey(rsaKey.(*rsa.PrivateKey), pkix.Name{
		CommonName:   cfg.ClusterName,
		Organization: []string{cfg.ClusterName},
	}, nil, defaults.CATTL)
	fatalIf(err)

	cert, err := keygen.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: cfg.Priv,
		CASigningAlg:        defaults.CASignatureAlgorithm,
		PublicHostKey:       cfg.Pub,
		HostID:              cfg.HostID,
		NodeName:            cfg.NodeName,
		ClusterName:         cfg.ClusterName,
		Roles:               teleport.Roles{teleport.RoleAdmin},
		TTL:                 24 * time.Hour,
	})
	fatalIf(err)
	tlsCA, err := tlsca.FromKeys(tlsCACert, tlsCAKey)
	fatalIf(err)
	cryptoPubKey, err := sshutils.CryptoPublicKey(cfg.Pub)
	fatalIf(err)
	identity := tlsca.Identity{
		Username: fmt.Sprintf("%v.%v", cfg.HostID, cfg.ClusterName),
		Groups:   []string{string(teleport.RoleAdmin)},
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
		Ports:         cfg.Ports,
		Hostname:      cfg.NodeName,
		UploadEventsC: make(chan events.UploadEvent, 100),
		log:           cfg.log,
	}
	secrets := InstanceSecrets{
		SiteName:       cfg.ClusterName,
		PrivKey:        cfg.Priv,
		PubKey:         cfg.Pub,
		Cert:           cert,
		TLSCACert:      tlsCACert,
		TLSCert:        tlsCert,
		ListenAddr:     net.JoinHostPort(cfg.NodeName, i.GetPortReverseTunnel()),
		WebProxyAddr:   net.JoinHostPort(cfg.NodeName, i.GetPortWeb()),
		MySQLProxyAddr: net.JoinHostPort(cfg.NodeName, i.GetPortMySQL()),
		Users:          make(map[string]*User),
	}
	if cfg.MultiplexProxy {
		secrets.ListenAddr = secrets.WebProxyAddr
	}
	i.Secrets = secrets
	return i
}

// GetRoles returns a list of roles to initiate for this secret
func (s *InstanceSecrets) GetRoles() []services.Role {
	var roles []services.Role
	for _, ca := range s.GetCAs() {
		if ca.GetType() != services.UserCA {
			continue
		}
		role := services.RoleForCertAuthority(ca)
		role.SetLogins(services.Allow, s.AllowedLogins())
		roles = append(roles, role)
	}
	return roles
}

// GetCAs return an array of CAs stored by the secrets object. In i
// case we always return hard-coded userCA + hostCA (and they share keys
// for simplicity)
func (s *InstanceSecrets) GetCAs() []services.CertAuthority {
	hostCA := types.NewCertAuthority(services.CertAuthoritySpecV2{
		Type:         services.HostCA,
		ClusterName:  s.SiteName,
		SigningKeys:  [][]byte{s.PrivKey},
		CheckingKeys: [][]byte{s.PubKey},
		Roles:        []string{},
		SigningAlg:   services.CertAuthoritySpecV2_RSA_SHA2_512,
	})
	hostCA.SetTLSKeyPairs([]services.TLSKeyPair{{Cert: s.TLSCACert, Key: s.PrivKey}})

	userCA := types.NewCertAuthority(services.CertAuthoritySpecV2{
		Type:         services.UserCA,
		ClusterName:  s.SiteName,
		SigningKeys:  [][]byte{s.PrivKey},
		CheckingKeys: [][]byte{s.PubKey},
		Roles:        []string{services.RoleNameForCertAuthority(s.SiteName)},
		SigningAlg:   services.CertAuthoritySpecV2_RSA_SHA2_512,
	})
	userCA.SetTLSKeyPairs([]services.TLSKeyPair{{Cert: s.TLSCACert, Key: s.PrivKey}})

	return []services.CertAuthority{hostCA, userCA}
}

func (s *InstanceSecrets) AllowedLogins() []string {
	var logins []string
	for i := range s.Users {
		logins = append(logins, s.Users[i].AllowedLogins...)
	}
	return logins
}

func (s *InstanceSecrets) AsTrustedCluster(token string, roleMap services.RoleMap) services.TrustedCluster {
	return &services.TrustedClusterV2{
		Kind:    services.KindTrustedCluster,
		Version: services.V2,
		Metadata: services.Metadata{
			Name: s.SiteName,
		},
		Spec: services.TrustedClusterSpecV2{
			Token:                token,
			Enabled:              true,
			ProxyAddress:         s.WebProxyAddr,
			ReverseTunnelAddress: s.ListenAddr,
			RoleMap:              roleMap,
		},
	}
}

func (s *InstanceSecrets) AsSlice() []*InstanceSecrets {
	return []*InstanceSecrets{s}
}

func (s *InstanceSecrets) GetIdentity() *auth.Identity {
	i, err := auth.ReadIdentityFromKeyPair(&auth.PackedKeys{
		Key:        s.PrivKey,
		Cert:       s.Cert,
		TLSCert:    s.TLSCert,
		TLSCACerts: [][]byte{s.TLSCACert},
	})
	fatalIf(err)
	return i
}

func (i *TeleInstance) GetPortSSHInt() int {
	return i.Ports[0]
}

func (i *TeleInstance) GetPortSSH() string {
	return strconv.Itoa(i.GetPortSSHInt())
}

func (i *TeleInstance) GetPortAuth() string {
	return strconv.Itoa(i.Ports[1])
}

func (i *TeleInstance) GetPortProxy() string {
	return strconv.Itoa(i.Ports[2])
}

func (i *TeleInstance) GetPortWeb() string {
	return strconv.Itoa(i.Ports[3])
}

func (i *TeleInstance) GetPortReverseTunnel() string {
	return strconv.Itoa(i.Ports[4])
}

func (i *TeleInstance) GetPortMySQL() string {
	return strconv.Itoa(i.Ports[5])
}

// GetSiteAPI() is a helper which returns an API endpoint to a site with
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

// Create creates a new instance of Teleport which trusts a lsit of other clusters (other
// instances)
func (i *TeleInstance) Create(trustedSecrets []*InstanceSecrets, enableSSH bool, console io.Writer) error {
	tconf := service.MakeDefaultConfig()
	tconf.SSH.Enabled = enableSSH
	tconf.Console = console
	tconf.Log = i.log
	tconf.Proxy.DisableWebService = true
	tconf.Proxy.DisableWebInterface = true
	return i.CreateEx(trustedSecrets, tconf)
}

// UserCreds holds user client credentials
type UserCreds struct {
	// Key is user client key and certificate
	Key client.Key
	// HostCA is a trusted host certificate authority
	HostCA services.CertAuthority
}

// SetupUserCreds sets up user credentials for client
func SetupUserCreds(tc *client.TeleportClient, proxyHost string, creds UserCreds) error {
	_, err := tc.AddKey(&creds.Key)
	if err != nil {
		return trace.Wrap(err)
	}
	err = tc.AddTrustedCA(creds.HostCA)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SetupUser sets up user in the cluster
func SetupUser(process *service.TeleportProcess, username string, roles []services.Role) error {
	ctx := context.TODO()
	auth := process.GetAuthServer()
	teleUser, err := services.NewUser(username)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(roles) == 0 {
		role := services.RoleForUser(teleUser)
		role.SetLogins(services.Allow, []string{username})

		// allow tests to forward agent, still needs to be passed in client
		roleOptions := role.GetOptions()
		roleOptions.ForwardAgent = services.NewBool(true)
		role.SetOptions(roleOptions)

		err = auth.UpsertRole(ctx, role)
		if err != nil {
			return trace.Wrap(err)
		}
		teleUser.AddRole(role.GetMetadata().Name)
	} else {
		for _, role := range roles {
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
	return nil
}

// UserCredsRequest is a request to generate user creds
type UserCredsRequest struct {
	// Process is a teleport process
	Process *service.TeleportProcess
	// Username is a user to generate certs for
	Username string
	// RouteToCluster is an optional cluster to route creds to
	RouteToCluster string
}

// GenerateUserCreds generates key to be used by client
func GenerateUserCreds(req UserCredsRequest) (*UserCreds, error) {
	priv, pub, err := testauthority.New().GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a := req.Process.GetAuthServer()
	sshCert, x509Cert, err := a.GenerateUserTestCerts(
		pub, req.Username, time.Hour, teleport.CertificateFormatStandard, req.RouteToCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := a.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &UserCreds{
		HostCA: ca,
		Key: client.Key{
			Priv:    priv,
			Pub:     pub,
			Cert:    sshCert,
			TLSCert: x509Cert,
		},
	}, nil
}

// GenerateConfig generates instance config
func (i *TeleInstance) GenerateConfig(trustedSecrets []*InstanceSecrets, tconf *service.Config) (*service.Config, error) {
	var err error
	dataDir, err := ioutil.TempDir("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	if tconf == nil {
		tconf = service.MakeDefaultConfig()
	}
	tconf.Log = i.log
	tconf.DataDir = dataDir
	tconf.UploadEventsC = i.UploadEventsC
	tconf.CachePolicy.Enabled = true
	tconf.Auth.ClusterName, err = services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: i.Secrets.SiteName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tconf.Auth.StaticTokens, err = services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionTokenV1{
			{
				Roles: []teleport.Role{
					teleport.RoleNode,
					teleport.RoleProxy,
					teleport.RoleTrustedCluster,
					teleport.RoleApp,
					teleport.RoleDatabase,
				},
				Token: "token",
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tconf.Auth.Authorities = append(tconf.Auth.Authorities, i.Secrets.GetCAs()...)
	tconf.Identities = append(tconf.Identities, i.Secrets.GetIdentity())
	for _, trusted := range trustedSecrets {
		tconf.Auth.Authorities = append(tconf.Auth.Authorities, trusted.GetCAs()...)
		tconf.Auth.Roles = append(tconf.Auth.Roles, trusted.GetRoles()...)
		tconf.Identities = append(tconf.Identities, trusted.GetIdentity())
		if trusted.ListenAddr != "" {
			tconf.ReverseTunnels = []services.ReverseTunnel{
				services.NewReverseTunnel(trusted.SiteName, []string{trusted.ListenAddr}),
			}
		}
	}
	tconf.Proxy.ReverseTunnelListenAddr.Addr = i.Secrets.ListenAddr
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
	tconf.Auth.SSHAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortAuth())
	tconf.Auth.PublicAddrs = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        i.Hostname,
		},
	}
	tconf.Proxy.SSHAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortProxy())
	tconf.Proxy.WebAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortWeb())
	tconf.Proxy.MySQLAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortMySQL())
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
	tconf.AuthServers = append(tconf.AuthServers, tconf.Auth.SSHAddr)
	tconf.Auth.StorageConfig = backend.Config{
		Type:   lite.GetName(),
		Params: backend.Params{"path": dataDir + string(os.PathListSeparator) + defaults.BackendDir, "poll_stream_period": 50 * time.Millisecond},
	}

	tconf.Keygen = testauthority.New()
	i.Config = tconf
	return tconf, nil
}

// CreateEx creates a new instance of Teleport which trusts a list of other clusters (other
// instances)
//
// Unlike Create() it allows for greater customization because it accepts
// a full Teleport config structure
func (i *TeleInstance) CreateEx(trustedSecrets []*InstanceSecrets, tconf *service.Config) error {
	ctx := context.TODO()
	tconf, err := i.GenerateConfig(trustedSecrets, tconf)
	if err != nil {
		return trace.Wrap(err)
	}
	i.Config = tconf
	i.Process, err = service.NewTeleport(tconf)
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
		teleUser, err := services.NewUser(user.Username)
		if err != nil {
			return trace.Wrap(err)
		}
		// set hardcode traits to trigger new style certificates
		teleUser.SetTraits(map[string][]string{"testing": {"integration"}})
		if len(user.Roles) == 0 {
			role := services.RoleForUser(teleUser)
			role.SetLogins(services.Allow, user.AllowedLogins)

			// allow tests to forward agent, still needs to be passed in client
			roleOptions := role.GetOptions()
			roleOptions.ForwardAgent = services.NewBool(true)
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
		// if user keys are not present, auto-geneate keys:
		if user.Key == nil || len(user.Key.Pub) == 0 {
			priv, pub, _ := tconf.Keygen.GenerateKeyPair("")
			user.Key = &client.Key{
				Priv: priv,
				Pub:  pub,
			}
		}
		// sign user's keys:
		ttl := 24 * time.Hour
		user.Key.Cert, user.Key.TLSCert, err = auth.GenerateUserTestCerts(user.Key.Pub, teleUser.GetName(), ttl, teleport.CertificateFormatStandard, "")
		if err != nil {
			return err
		}
	}
	return nil
}

// StartNode starts a SSH node and connects it to the cluster.
func (i *TeleInstance) StartNode(tconf *service.Config) (*service.TeleportProcess, error) {
	return i.startNode(tconf, false)
}

// StartReverseTunnelNode starts a SSH node and connects it to the cluster via reverse tunnel.
func (i *TeleInstance) StartReverseTunnelNode(tconf *service.Config) (*service.TeleportProcess, error) {
	return i.startNode(tconf, true)
}

// startNode starts a node and connects it to the cluster.
func (i *TeleInstance) startNode(tconf *service.Config, reverseTunnel bool) (*service.TeleportProcess, error) {
	dataDir, err := ioutil.TempDir("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	tconf.DataDir = dataDir

	authPort := i.GetPortAuth()
	if reverseTunnel {
		authPort = i.GetPortWeb()
	}

	authServer := utils.MustParseAddr(net.JoinHostPort(i.Hostname, authPort))
	tconf.AuthServers = append(tconf.AuthServers, *authServer)
	tconf.Token = "token"
	tconf.UploadEventsC = i.UploadEventsC
	var ttl time.Duration
	tconf.CachePolicy = service.CachePolicy{
		Enabled:   true,
		RecentTTL: &ttl,
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
	receivedEvents, err := startAndWait(process, expectedEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debugf("Teleport node (in instance %v) started: %v/%v events received.",
		i.Secrets.SiteName, len(expectedEvents), len(receivedEvents))
	return process, nil
}

func (i *TeleInstance) StartApp(conf *service.Config) (*service.TeleportProcess, error) {
	dataDir, err := ioutil.TempDir("", "cluster-"+i.Secrets.SiteName)
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
	receivedEvents, err := startAndWait(process, expectedEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debugf("Teleport Application Server (in instance %v) started: %v/%v events received.",
		i.Secrets.SiteName, len(expectedEvents), len(receivedEvents))
	return process, nil
}

// StartDatabase starts the database access service with the provided config.
func (i *TeleInstance) StartDatabase(conf *service.Config) (*service.TeleportProcess, *auth.Client, error) {
	dataDir, err := ioutil.TempDir("", "cluster-"+i.Secrets.SiteName)
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
	}

	// Start the process and block until the expected events have arrived.
	receivedEvents, err := startAndWait(process, expectedEvents)
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

// StartNodeAndProxy starts a SSH node and a Proxy Server and connects it to
// the cluster.
func (i *TeleInstance) StartNodeAndProxy(name string, sshPort, proxyWebPort, proxySSHPort int) error {
	dataDir, err := ioutil.TempDir("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	tconf := service.MakeDefaultConfig()

	tconf.Log = i.log
	authServer := utils.MustParseAddr(net.JoinHostPort(i.Hostname, i.GetPortAuth()))
	tconf.AuthServers = append(tconf.AuthServers, *authServer)
	tconf.Token = "token"
	tconf.HostUUID = name
	tconf.Hostname = name
	tconf.UploadEventsC = i.UploadEventsC
	tconf.DataDir = dataDir
	var ttl time.Duration
	tconf.CachePolicy = service.CachePolicy{
		Enabled:   true,
		RecentTTL: &ttl,
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

	// Create a new Teleport process and add it to the list of nodes that
	// compose this "cluster".
	process, err := service.NewTeleport(tconf)
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
	receivedEvents, err := startAndWait(process, expectedEvents)
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
}

// StartProxy starts another Proxy Server and connects it to the cluster.
func (i *TeleInstance) StartProxy(cfg ProxyConfig) (reversetunnel.Server, error) {
	dataDir, err := ioutil.TempDir("", "cluster-"+i.Secrets.SiteName+"-"+cfg.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.tempDirs = append(i.tempDirs, dataDir)

	tconf := service.MakeDefaultConfig()
	tconf.Console = nil
	tconf.Log = i.log
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
	tconf.Proxy.DisableWebService = true

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
		service.ProxyReverseTunnelReady,
		service.ProxySSHReady,
	}

	// Start the process and block until the expected events have arrived.
	receivedEvents, err := startAndWait(process, expectedEvents)
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
	}
	i.Process, err = service.NewTeleport(i.Config)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// AddUserUserWithRole adds user with one or many assigned roles
func (i *TeleInstance) AddUserWithRole(username string, roles ...services.Role) *User {
	user := &User{
		Username: username,
		Roles:    make([]services.Role, len(roles)),
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

	// Start the process and block until the expected events have arrived.
	receivedEvents, err := startAndWait(i.Process, expectedEvents)
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
	keyDir, err := ioutil.TempDir(i.Config.DataDir, "tsh")
	if err != nil {
		return nil, err
	}

	proxyConf := &i.Config.Proxy
	proxyHost, _, err := net.SplitHostPort(proxyConf.SSHAddr.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var webProxyAddr string
	var sshProxyAddr string

	if cfg.Proxy == nil {
		webProxyAddr = proxyConf.WebAddr.Addr
		sshProxyAddr = proxyConf.SSHAddr.Addr
	} else {
		webProxyAddr = net.JoinHostPort(proxyHost, strconv.Itoa(cfg.Proxy.WebPort))
		sshProxyAddr = net.JoinHostPort(proxyHost, strconv.Itoa(cfg.Proxy.SSHPort))
	}

	cconf := &client.Config{
		Username:           cfg.Login,
		Host:               cfg.Host,
		HostPort:           cfg.Port,
		HostLogin:          cfg.Login,
		InsecureSkipVerify: true,
		KeysDir:            keyDir,
		SiteName:           cfg.Cluster,
		ForwardAgent:       cfg.ForwardAgent,
		Labels:             cfg.Labels,
		WebProxyAddr:       webProxyAddr,
		SSHProxyAddr:       sshProxyAddr,
		Interactive:        cfg.Interactive,
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
// (pre-authenticated with server CAs and signed session key)
func (i *TeleInstance) NewClient(cfg ClientConfig) (*client.TeleportClient, error) {
	tc, err := i.NewUnauthenticatedClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// configures the client authenticate using the keys from 'secrets':
	user, ok := i.Secrets.Users[cfg.Login]
	if !ok {
		return nil, trace.BadParameter("unknown login %q", cfg.Login)
	}
	if user.Key == nil {
		return nil, trace.BadParameter("user %q has no key", cfg.Login)
	}
	_, err = tc.AddKey(user.Key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// tell the client to trust given CAs (from secrets). this is the
	// equivalent of 'known hosts' in openssh
	cas := i.Secrets.GetCAs()
	for i := range cas {
		err = tc.AddTrustedCA(cas[i])
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
				i.log.Errorf("Failed closing extra proxy: %v.", err)
			}
			if err := p.Wait(); err != nil {
				errors = append(errors, err)
				i.log.Errorf("Failed to stop extra proxy: %v.", err)
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
			i.log.Errorf("Failed closing extra node %v", err)
		}
		if err := node.Wait(); err != nil {
			errors = append(errors, err)
			i.log.Errorf("Failed stopping extra node %v", err)
		}
	}
	return trace.NewAggregate(errors...)
}

// StopAuth stops the auth server process. If removeData is true, the data
// directory is also cleaned up.
func (i *TeleInstance) StopAuth(removeData bool) error {
	if i.Config != nil && removeData {
		err := os.RemoveAll(i.Config.DataDir)
		if err != nil {
			i.log.WithError(err).Error("Failed removing temporary local Teleport directory.")
		}
	}
	if i.Process == nil {
		return nil
	}
	i.log.Infof("Asking Teleport to stop")
	err := i.Process.Close()
	if err != nil {
		i.log.WithError(err).Error("Failed closing the teleport process.")
		return trace.Wrap(err)
	}
	defer func() {
		i.log.Infof("Teleport instance %q stopped!", i.Secrets.SiteName)
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

	i.log.Info("Stopped all teleport services.")
	return trace.NewAggregate(errors...)
}

func startAndWait(process *service.TeleportProcess, expectedEvents []string) ([]service.Event, error) {
	// register to listen for all ready events on the broadcast channel
	broadcastCh := make(chan service.Event)
	for _, eventName := range expectedEvents {
		process.WaitForEvent(context.TODO(), eventName, broadcastCh)
	}

	// start the process
	err := process.Start()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// wait for all events to arrive or a timeout. if all the expected events
	// from above are not received, this instance will not start
	receivedEvents := []service.Event{}
	timeoutCh := time.After(10 * time.Second)

	for idx := 0; idx < len(expectedEvents); idx++ {
		select {
		case e := <-broadcastCh:
			receivedEvents = append(receivedEvents, e)
		case <-timeoutCh:
			return nil, trace.BadParameter("timed out, only %v/%v events received. received: %v, expected: %v",
				len(receivedEvents), len(expectedEvents), receivedEvents, expectedEvents)
		}
	}

	// Not all services follow a non-blocking Start/Wait pattern. This means a
	// *Ready event may be emit slightly before the service actually starts for
	// blocking services. Long term those services should be re-factored, until
	// then sleep for 250ms to handle this situation.
	time.Sleep(250 * time.Millisecond)

	return receivedEvents, nil
}

type proxyServer struct {
	sync.Mutex
	count int
}

// ServeHTTP only accepts the CONNECT verb and will tunnel your connection to
// the specified host. Also tracks the number of connections that it proxies for
// debugging purposes.
func (p *proxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Validate http connect parameters.
	if r.Method != http.MethodConnect {
		trace.WriteError(w, trace.BadParameter("%v not supported", r.Method))
		return
	}
	if r.Host == "" {
		trace.WriteError(w, trace.BadParameter("host not set"))
		return
	}

	// Dial to the target host, this is done before hijacking the connection to
	// ensure the target host is accessible.
	dconn, err := net.Dial("tcp", r.Host)
	if err != nil {
		trace.WriteError(w, err)
		return
	}
	defer dconn.Close()

	// Once the client receives 200 OK, the rest of the data will no longer be
	// http, but whatever protocol is being tunneled.
	w.WriteHeader(http.StatusOK)

	// Hijack request so we can get underlying connection.
	hj, ok := w.(http.Hijacker)
	if !ok {
		trace.WriteError(w, trace.AccessDenied("unable to hijack connection"))
		return
	}
	sconn, _, err := hj.Hijack()
	if err != nil {
		trace.WriteError(w, err)
		return
	}
	defer sconn.Close()

	// Success, we're proxying data now.
	p.Lock()
	p.count = p.count + 1
	p.Unlock()

	// Copy from src to dst and dst to src.
	errc := make(chan error, 2)
	replicate := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		errc <- err
	}
	go replicate(sconn, dconn)
	go replicate(dconn, sconn)

	// Wait until done, error, or 10 second.
	select {
	case <-time.After(10 * time.Second):
	case <-errc:
	}
}

// Count returns the number of connections that have been proxied.
func (p *proxyServer) Count() int {
	p.Lock()
	defer p.Unlock()
	return p.count
}

// discardServer is a SSH server that discards SSH exec requests and starts
// with the passed in host signer.
type discardServer struct {
	sshServer *sshutils.Server
}

func newDiscardServer(host string, port int, hostSigner ssh.Signer) (*discardServer, error) {
	ds := &discardServer{}

	// create underlying ssh server
	sshServer, err := sshutils.NewServer(
		"integration-discard-server",
		utils.NetAddr{AddrNetwork: "tcp", Addr: fmt.Sprintf("%v:%v", host, port)},
		ds,
		[]ssh.Signer{hostSigner},
		sshutils.AuthMethods{
			PublicKey: ds.userKeyAuth,
		},
		sshutils.SetInsecureSkipHostValidation(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ds.sshServer = sshServer

	return ds, nil
}

func (s *discardServer) userKeyAuth(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
	return nil, nil
}

func (s *discardServer) Start() error {
	return s.sshServer.Start()
}

func (s *discardServer) Stop() {
	s.sshServer.Close()
}

func (s *discardServer) HandleNewChan(_ context.Context, ccx *sshutils.ConnectionContext, newChannel ssh.NewChannel) {
	channel, reqs, err := newChannel.Accept()
	if err != nil {
		ccx.ServerConn.Close()
		ccx.NetConn.Close()
		return
	}

	go s.handleChannel(channel, reqs)
}

func (s *discardServer) handleChannel(channel ssh.Channel, reqs <-chan *ssh.Request) {
	defer channel.Close()

	for req := range reqs {
		if req.Type == "exec" {
			successPayload := ssh.Marshal(struct{ C uint32 }{C: uint32(0)})
			channel.SendRequest("exit-status", false, successPayload)
			if req.WantReply {
				req.Reply(true, nil)
			}
			return
		}
		if req.WantReply {
			req.Reply(true, nil)
		}
	}
}

// commandOptions controls how the SSH command is built.
type commandOptions struct {
	forwardAgent bool
	forcePTY     bool
	controlPath  string
	socketPath   string
	proxyPort    string
	nodePort     string
	command      string
}

// externalSSHCommand runs an external SSH command (if an external ssh binary
// exists) with the passed in parameters.
func externalSSHCommand(o commandOptions) (*exec.Cmd, error) {
	var execArgs []string

	// Don't check the host certificate as part of the testing an external SSH
	// client, this is done elsewhere.
	execArgs = append(execArgs, "-oStrictHostKeyChecking=no")
	execArgs = append(execArgs, "-oUserKnownHostsFile=/dev/null")

	// ControlMaster is often used by applications like Ansible.
	if o.controlPath != "" {
		execArgs = append(execArgs, "-oControlMaster=auto")
		execArgs = append(execArgs, "-oControlPersist=1s")
		execArgs = append(execArgs, "-oConnectTimeout=2")
		execArgs = append(execArgs, fmt.Sprintf("-oControlPath=%v", o.controlPath))
	}

	// The -tt flag is used to force PTY allocation. It's often used by
	// applications like Ansible.
	if o.forcePTY {
		execArgs = append(execArgs, "-tt")
	}

	// Connect to node on the passed in port.
	execArgs = append(execArgs, fmt.Sprintf("-p %v", o.nodePort))

	// Build proxy command.
	proxyCommand := []string{"ssh"}
	proxyCommand = append(proxyCommand, "-oStrictHostKeyChecking=no")
	proxyCommand = append(proxyCommand, "-oUserKnownHostsFile=/dev/null")
	if o.forwardAgent {
		proxyCommand = append(proxyCommand, "-oForwardAgent=yes")
	}
	proxyCommand = append(proxyCommand, fmt.Sprintf("-p %v", o.proxyPort))
	proxyCommand = append(proxyCommand, `%r@localhost -s proxy:%h:%p`)

	// Add in ProxyCommand option, needed for all Teleport connections.
	execArgs = append(execArgs, fmt.Sprintf("-oProxyCommand=%v", strings.Join(proxyCommand, " ")))

	// Add in the host to connect to and the command to run when connected.
	execArgs = append(execArgs, Host)
	execArgs = append(execArgs, o.command)

	// Find the OpenSSH binary.
	sshpath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create an exec.Command and tell it where to find the SSH agent.
	cmd, err := exec.Command(sshpath, execArgs...), nil
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cmd.Env = []string{fmt.Sprintf("SSH_AUTH_SOCK=%v", o.socketPath)}

	return cmd, nil
}

// createAgent creates a SSH agent with the passed in private key and
// certificate that can be used in tests. This is useful so tests don't
// clobber your system agent.
func createAgent(me *user.User, privateKeyByte []byte, certificateBytes []byte) (*teleagent.AgentServer, string, string, error) {
	// create a path to the unix socket
	sockDir, err := ioutil.TempDir("", "int-test")
	if err != nil {
		return nil, "", "", trace.Wrap(err)
	}
	sockPath := filepath.Join(sockDir, "agent.sock")

	uid, err := strconv.Atoi(me.Uid)
	if err != nil {
		return nil, "", "", trace.Wrap(err)
	}
	gid, err := strconv.Atoi(me.Gid)
	if err != nil {
		return nil, "", "", trace.Wrap(err)
	}

	// transform the key and certificate bytes into something the agent can understand
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(certificateBytes)
	if err != nil {
		return nil, "", "", trace.Wrap(err)
	}
	privateKey, err := ssh.ParseRawPrivateKey(privateKeyByte)
	if err != nil {
		return nil, "", "", trace.Wrap(err)
	}
	agentKey := agent.AddedKey{
		PrivateKey:       privateKey,
		Certificate:      publicKey.(*ssh.Certificate),
		Comment:          "",
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}

	// create a (unstarted) agent and add the key to it
	keyring := agent.NewKeyring()
	if err := keyring.Add(agentKey); err != nil {
		return nil, "", "", trace.Wrap(err)
	}
	teleAgent := teleagent.NewServer(func() (teleagent.Agent, error) {
		return teleagent.NopCloser(keyring), nil
	})

	// start the SSH agent
	err = teleAgent.ListenUnixSocket(sockPath, uid, gid, 0600)
	if err != nil {
		return nil, "", "", trace.Wrap(err)
	}
	go teleAgent.Serve()

	return teleAgent, sockDir, sockPath, nil
}

func closeAgent(teleAgent *teleagent.AgentServer, socketDirPath string) error {
	err := teleAgent.Close()
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.RemoveAll(socketDirPath)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func fatalIf(err error) {
	if err != nil {
		log.Fatalf("%v at %v", string(debug.Stack()), err)
	}
}
