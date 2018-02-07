package integration

import (
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
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
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

// SetTestTimeouts affects global timeouts inside Teleport, making connections
// work faster but consuming more CPU (useful for integration testing)
func SetTestTimeouts(t time.Duration) {
	defaults.ReverseTunnelAgentHeartbeatPeriod = t
	defaults.ServerHeartbeatTTL = t
	defaults.SessionRefreshPeriod = t
}

// TeleInstance represents an in-memory instance of a teleport
// process for testing
type TeleInstance struct {
	// Secrets holds the keys (pub, priv and derived cert) of i instance
	Secrets InstanceSecrets

	// Slice of TCP ports used by Teleport services
	Ports []int

	// Hostname is the name of the host where i isnstance is running
	Hostname string

	// Internal stuff...
	Process *service.TeleportProcess
	Config  *service.Config
	Tunnel  reversetunnel.Server

	// Nodes is a list of additional nodes
	// started with this instance
	Nodes []*service.TeleportProcess
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
}

// NewInstance creates a new Teleport process instance
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
	keygen := native.New()
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
		PublicHostKey:       cfg.Pub,
		HostID:              cfg.HostID,
		NodeName:            cfg.NodeName,
		ClusterName:         cfg.ClusterName,
		Roles:               teleport.Roles{teleport.RoleAdmin},
		TTL:                 time.Duration(time.Hour * 24),
	})
	fatalIf(err)
	tlsCA, err := tlsca.New(tlsCACert, tlsCAKey)
	fatalIf(err)
	cryptoPubKey, err := sshutils.CryptoPublicKey(cfg.Pub)
	identity := tlsca.Identity{
		Username: fmt.Sprintf("%v.%v", cfg.HostID, cfg.ClusterName),
		Groups:   []string{string(teleport.RoleAdmin)},
	}
	clock := clockwork.NewRealClock()
	tlsCert, err := tlsCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: cryptoPubKey,
		Subject:   identity.Subject(),
		NotAfter:  clock.Now().UTC().Add(time.Hour * 24),
	})
	fatalIf(err)

	i := &TeleInstance{
		Ports:    cfg.Ports,
		Hostname: cfg.NodeName,
	}
	secrets := InstanceSecrets{
		SiteName:     cfg.ClusterName,
		PrivKey:      cfg.Priv,
		PubKey:       cfg.Pub,
		Cert:         cert,
		TLSCACert:    tlsCACert,
		TLSCert:      tlsCert,
		ListenAddr:   net.JoinHostPort(cfg.NodeName, i.GetPortReverseTunnel()),
		WebProxyAddr: net.JoinHostPort(cfg.NodeName, i.GetPortWeb()),
		Users:        make(map[string]*User),
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
	hostCA := services.NewCertAuthority(services.HostCA, s.SiteName, [][]byte{s.PrivKey}, [][]byte{s.PubKey}, []string{})
	hostCA.SetTLSKeyPairs([]services.TLSKeyPair{{Cert: s.TLSCACert, Key: s.PrivKey}})
	return []services.CertAuthority{
		hostCA,
		services.NewCertAuthority(services.UserCA, s.SiteName, [][]byte{s.PrivKey}, [][]byte{s.PubKey}, []string{services.RoleNameForCertAuthority(s.SiteName)}),
	}
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
	i, err := auth.ReadIdentityFromKeyPair(s.PrivKey, s.Cert, s.TLSCert, s.TLSCACert)
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
	tconf.Proxy.DisableWebService = true
	tconf.Proxy.DisableWebInterface = true
	return i.CreateEx(trustedSecrets, tconf)
}

// CreateEx creates a new instance of Teleport which trusts a list of other clusters (other
// instances)
//
// Unlike Create() it allows for greater customization because it accepts
// a full Teleport config structure
func (i *TeleInstance) CreateEx(trustedSecrets []*InstanceSecrets, tconf *service.Config) error {
	var err error
	dataDir, err := ioutil.TempDir("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}
	if tconf == nil {
		tconf = service.MakeDefaultConfig()
	}
	tconf.DataDir = dataDir
	tconf.Auth.ClusterName, err = services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: i.Secrets.SiteName,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	tconf.Auth.StaticTokens, err = services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionToken{
			{
				Roles: []teleport.Role{teleport.RoleNode, teleport.RoleProxy, teleport.RoleTrustedCluster},
				Token: "token",
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
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
	tconf.Auth.SSHAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortAuth())
	tconf.Proxy.SSHAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortProxy())
	tconf.Proxy.WebAddr.Addr = net.JoinHostPort(i.Hostname, i.GetPortWeb())
	tconf.AuthServers = append(tconf.AuthServers, tconf.Auth.SSHAddr)
	tconf.Auth.StorageConfig = backend.Config{
		Type:   boltbk.GetName(),
		Params: backend.Params{"path": dataDir},
	}

	tconf.Keygen = testauthority.New()

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
		var roles []services.Role
		if len(user.Roles) == 0 {
			role := services.RoleForUser(teleUser)
			role.SetLogins(services.Allow, user.AllowedLogins)

			// allow tests to forward agent, still needs to be passed in client
			roleOptions := role.GetOptions()
			roleOptions.Set(services.ForwardAgent, true)
			role.SetOptions(roleOptions)

			err = auth.UpsertRole(role, backend.Forever)
			if err != nil {
				return trace.Wrap(err)
			}
			teleUser.AddRole(role.GetMetadata().Name)
			roles = append(roles, role)
		} else {
			roles = user.Roles
			for _, role := range user.Roles {
				err := auth.UpsertRole(role, backend.Forever)
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
		user.Key.Cert, user.Key.TLSCert, err = auth.GenerateUserCerts(user.Key.Pub, teleUser.GetName(), ttl, teleport.CertificateFormatStandard)
		if err != nil {
			return err
		}
	}
	return nil
}

// StartNode starts SSH node and connects it to the cluster.
func (i *TeleInstance) StartNode(name string, sshPort int) (*service.TeleportProcess, error) {
	dataDir, err := ioutil.TempDir("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tconf := service.MakeDefaultConfig()

	authServer := utils.MustParseAddr(net.JoinHostPort(i.Hostname, i.GetPortAuth()))
	tconf.AuthServers = append(tconf.AuthServers, *authServer)
	tconf.Token = "token"
	tconf.HostUUID = name
	tconf.Hostname = name
	tconf.DataDir = dataDir
	var ttl time.Duration
	tconf.CachePolicy = service.CachePolicy{
		Enabled:   true,
		RecentTTL: &ttl,
	}

	tconf.Auth.Enabled = false

	tconf.Proxy.Enabled = false

	tconf.SSH.Enabled = true
	tconf.SSH.Addr.Addr = net.JoinHostPort(i.Hostname, fmt.Sprintf("%v", sshPort))

	process, err := service.NewTeleport(tconf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	i.Nodes = append(i.Nodes, process)

	err = process.Start()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return process, nil
}

// StartNodeAndProxy starts SSH node and proxy and connects it to the cluster.
func (i *TeleInstance) StartNodeAndProxy(name string, sshPort, proxyWebPort, proxySSHPort int) error {
	dataDir, err := ioutil.TempDir("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}

	tconf := service.MakeDefaultConfig()

	authServer := utils.MustParseAddr(net.JoinHostPort(i.Hostname, i.GetPortAuth()))
	tconf.AuthServers = append(tconf.AuthServers, *authServer)
	tconf.Token = "token"
	tconf.HostUUID = name
	tconf.Hostname = name
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

	process, err := service.NewTeleport(tconf)
	if err != nil {
		return trace.Wrap(err)
	}
	i.Nodes = append(i.Nodes, process)
	return process.Start()
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

// StartProxy starts proxy server and adds it to the cluster
func (i *TeleInstance) StartProxy(cfg ProxyConfig) error {
	dataDir, err := ioutil.TempDir("", "cluster-"+i.Secrets.SiteName+"-"+cfg.Name)
	if err != nil {
		return trace.Wrap(err)
	}
	tconf := service.MakeDefaultConfig()
	tconf.HostUUID = cfg.Name
	tconf.Hostname = cfg.Name
	tconf.DataDir = dataDir
	tconf.Auth.Enabled = false
	tconf.Proxy.Enabled = true
	tconf.SSH.Enabled = false
	authServer := utils.MustParseAddr(net.JoinHostPort(i.Hostname, i.GetPortAuth()))
	tconf.AuthServers = append(tconf.AuthServers, *authServer)
	tconf.Token = "token"
	tconf.Proxy.Enabled = true
	tconf.Proxy.SSHAddr.Addr = net.JoinHostPort(i.Hostname, fmt.Sprintf("%v", cfg.SSHPort))
	tconf.Proxy.ReverseTunnelListenAddr.Addr = net.JoinHostPort(i.Hostname, fmt.Sprintf("%v", cfg.ReverseTunnelPort))
	tconf.Proxy.WebAddr.Addr = net.JoinHostPort(i.Hostname, fmt.Sprintf("%v", cfg.WebPort))
	tconf.Proxy.DisableReverseTunnel = false
	tconf.Proxy.DisableWebService = true
	// Enable caching
	tconf.CachePolicy = service.CachePolicy{Enabled: true}

	process, err := service.NewTeleport(tconf)
	if err != nil {
		return trace.Wrap(err)
	}
	i.Nodes = append(i.Nodes, process)
	return process.Start()
}

// Reset re-creates the teleport instance based on the same configuration
// This is needed if you want to stop the instance, reset it and start again
func (i *TeleInstance) Reset() (err error) {
	i.Process, err = service.NewTeleport(i.Config)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// AddUserUserWithRole adds user with assigned role
func (i *TeleInstance) AddUserWithRole(username string, role services.Role) *User {
	user := &User{
		Username: username,
		Roles:    []services.Role{role},
	}
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

func (i *TeleInstance) Start() error {
	// build a list of expected events based off the configuration passed in
	expectedEvents := []string{}
	if i.Config.Auth.Enabled {
		expectedEvents = append(expectedEvents, service.AuthSSHReady)
		expectedEvents = append(expectedEvents, service.AuthTLSReady)
	}
	if i.Config.Proxy.Enabled {
		expectedEvents = append(expectedEvents, service.ProxyReverseTunnelReady)
		expectedEvents = append(expectedEvents, service.ProxySSHReady)
		if !i.Config.Proxy.DisableWebService {
			expectedEvents = append(expectedEvents, service.ProxyWebServerReady)
		}
	}
	if i.Config.SSH.Enabled {
		expectedEvents = append(expectedEvents, service.NodeSSHReady)
	}

	// register to listen for all ready events on the broadcast channel
	broadcastCh := make(chan service.Event)
	for _, eventName := range expectedEvents {
		i.Process.WaitForEvent(eventName, broadcastCh, make(chan struct{}))
	}

	// start the teleport process
	err := i.Process.Start()
	if err != nil {
		return trace.Wrap(err)
	}

	// wait for all events to arrive or a timeout. if all the expected events
	// from above are not received, this instance will not start
	receivedEvents := []string{}
	timeoutCh := time.After(5 * time.Second)

	for idx := 0; idx < len(expectedEvents); idx++ {
		select {
		case e := <-broadcastCh:
			// if it's a reverse tunnel, save the server in the instance
			ts, ok := e.Payload.(reversetunnel.Server)
			if ok {
				i.Tunnel = ts
			}

			// update list of events that have been received
			receivedEvents = append(receivedEvents, e.Name)
		case <-timeoutCh:
			return trace.BadParameter("timed out, only %v/%v services started: %v", len(receivedEvents), len(expectedEvents), receivedEvents)
		}
	}

	// wait a little bit longer because for some processes, we emit the event
	// right before the service has started
	time.Sleep(250 * time.Millisecond)
	log.Debugf("Teleport instance %v started: %v/%v services started.", i.Secrets.SiteName, len(receivedEvents), len(expectedEvents))

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
}

// NewClient returns a fully configured and pre-authenticated client
// (pre-authenticated with server CAs and signed session key)
func (i *TeleInstance) NewClient(cfg ClientConfig) (tc *client.TeleportClient, err error) {
	keyDir, err := ioutil.TempDir(i.Config.DataDir, "tsh")
	if err != nil {
		return nil, err
	}

	// break down proxy address into host, ssh_port and web_port:
	proxyConf := &i.Config.Proxy
	proxyHost, sp, err := net.SplitHostPort(proxyConf.SSHAddr.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// use alternative proxy if necessary
	var proxySSHPort, proxyWebPort int
	if cfg.Proxy == nil {
		proxySSHPort, err = strconv.Atoi(sp)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		_, sp, err = net.SplitHostPort(proxyConf.WebAddr.Addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		proxyWebPort, err = strconv.Atoi(sp)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		proxySSHPort, proxyWebPort = cfg.Proxy.SSHPort, cfg.Proxy.WebPort
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
	}
	cconf.SetProxy(proxyHost, proxyWebPort, proxySSHPort)

	tc, err = client.NewClient(cconf)
	if err != nil {
		return nil, err
	}
	// confnigures the client authenticate using the keys from 'secrets':
	user, ok := i.Secrets.Users[cfg.Login]
	if !ok {
		return nil, trace.BadParameter("unknown login %q", cfg.Login)
	}
	if user.Key == nil {
		return nil, trace.BadParameter("user %q has no key", cfg.Login)
	}
	_, err = tc.AddKey(cfg.Host, user.Key)
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

// StopNodes stops additional nodes
func (i *TeleInstance) StopNodes() error {
	var errors []error
	for _, node := range i.Nodes {
		if err := node.Close(); err != nil {
			errors = append(errors, err)
			log.Errorf("failed closing extra node %v", err)
		}
		if err := node.Wait(); err != nil {
			errors = append(errors, err)
			log.Errorf("failed stopping extra node %v", err)
		}
	}
	return trace.NewAggregate(errors...)
}

func (i *TeleInstance) Stop(removeData bool) error {
	if i.Config != nil && removeData {
		err := os.RemoveAll(i.Config.DataDir)
		if err != nil {
			log.Error("failed removing temporary local Teleport directory", err)
		}
	}

	log.Infof("Asking Teleport to stop")
	err := i.Process.Close()
	if err != nil {
		log.Error(err)
		return trace.Wrap(err)
	}
	defer func() {
		log.Infof("Teleport instance '%v' stopped!", i.Secrets.SiteName)
	}()
	return i.Process.Wait()
}

type proxyServer struct {
	sync.Mutex
	count int
}

// ServeHTTP only accepts the CONNECT verb and will tunnel your connection to
// the specified host. Also tracks the number of connections that it proxies for
// debugging purposes.
func (p *proxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// validate http connect parameters
	if r.Method != http.MethodConnect {
		trace.WriteError(w, trace.BadParameter("%v not supported", r.Method))
		return
	}
	if r.Host == "" {
		trace.WriteError(w, trace.BadParameter("host not set"))
		return
	}

	// hijack request so we can get underlying connection
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

	// dial to host we want to proxy connection to
	dconn, err := net.Dial("tcp", r.Host)
	if err != nil {
		trace.WriteError(w, err)
		return
	}
	defer dconn.Close()

	// write 200 OK to the source, but don't close the connection
	resp := &http.Response{
		Status:     "OK",
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 0,
	}
	err = resp.Write(sconn)
	if err != nil {
		trace.WriteError(w, err)
		return
	}

	// success, we're proxying data now
	p.Lock()
	p.count = p.count + 1
	p.Unlock()

	// copy from src to dst and dst to src
	errc := make(chan error, 2)
	replicate := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		errc <- err
	}
	go replicate(sconn, dconn)
	go replicate(dconn, sconn)

	// wait until done, error, or 10 second
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

func (s *discardServer) HandleNewChan(conn net.Conn, sconn *ssh.ServerConn, newChannel ssh.NewChannel) {
	channel, reqs, err := newChannel.Accept()
	if err != nil {
		sconn.Close()
		conn.Close()
		return
	}

	go s.handleChannel(channel, reqs)
}

func (s *discardServer) handleChannel(channel ssh.Channel, reqs <-chan *ssh.Request) {
	defer channel.Close()

	for {
		select {
		case req := <-reqs:
			if req == nil {
				return
			}
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
}

// externalSSHCommand runs an external SSH command (if an external ssh binary
// exists) with the passed in parameters.
func externalSSHCommand(forwardAgent bool, socketPath string, proxyPort string, nodePort string, command string) (*exec.Cmd, error) {
	var execArgs []string

	// don't check the host certificate during tests
	execArgs = append(execArgs, "-oStrictHostKeyChecking=no")
	execArgs = append(execArgs, "-oUserKnownHostsFile=/dev/null")

	// connect to node on the passed in port
	execArgs = append(execArgs, "-p")
	execArgs = append(execArgs, nodePort)

	// build proxy command
	var proxyCommand string
	switch forwardAgent {
	case true:
		proxyCommand = fmt.Sprintf("ProxyCommand ssh -oStrictHostKeyChecking=no -oUserKnownHostsFile=/dev/null -oForwardAgent=yes -p %v %%r@localhost -s proxy:%%h:%%p", proxyPort)
	case false:
		proxyCommand = fmt.Sprintf("ProxyCommand ssh -oStrictHostKeyChecking=no -oUserKnownHostsFile=/dev/null -p %v %%r@localhost -s proxy:%%h:%%p", proxyPort)
	}
	execArgs = append(execArgs, "-o")
	execArgs = append(execArgs, proxyCommand)

	// add in the host the connect to and the command to run when connected
	execArgs = append(execArgs, Host)
	execArgs = append(execArgs, command)

	// find the ssh binary
	sshpath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create exec command and tell it where to find the ssh agent
	cmd, err := exec.Command(sshpath, execArgs...), nil
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cmd.Env = []string{fmt.Sprintf("SSH_AUTH_SOCK=%v", socketPath)}

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
	teleAgent := teleagent.NewServer()
	teleAgent.Add(agentKey)

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

func makeKey() (priv, pub []byte) {
	priv, pub, _ = native.New().GenerateKeyPair("")
	return priv, pub
}
