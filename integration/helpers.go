package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

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
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

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
	PubKey  []byte `json:"pub"`
	PrivKey []byte `json:"priv"`
	Cert    []byte `json:"cert"`
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

// NewInstance creates a new Teleport process instance
func NewInstance(clusterName string, hostID string, nodeName string, ports []int, priv, pub []byte) *TeleInstance {
	var err error
	if len(ports) < 5 {
		fatalIf(fmt.Errorf("not enough free ports given: %v", ports))
	}
	if nodeName == "" {
		nodeName, err = os.Hostname()
		fatalIf(err)
	}
	// generate instance secrets (keys):
	keygen := native.New()
	if priv == nil || pub == nil {
		priv, pub, _ = keygen.GenerateKeyPair("")
	}
	cert, err := keygen.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: priv,
		PublicHostKey:       pub,
		HostID:              hostID,
		NodeName:            nodeName,
		ClusterName:         clusterName,
		Roles:               teleport.Roles{teleport.RoleAdmin},
		TTL:                 time.Duration(time.Hour * 24),
	})
	fatalIf(err)
	i := &TeleInstance{
		Ports:    ports,
		Hostname: nodeName,
	}
	secrets := InstanceSecrets{
		SiteName:     clusterName,
		PrivKey:      priv,
		PubKey:       pub,
		Cert:         cert,
		ListenAddr:   net.JoinHostPort(nodeName, strconv.Itoa(ports[4])),
		WebProxyAddr: net.JoinHostPort(nodeName, i.GetPortWeb()),
		Users:        make(map[string]*User),
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
	return []services.CertAuthority{
		services.NewCertAuthority(services.HostCA, s.SiteName, [][]byte{s.PrivKey}, [][]byte{s.PubKey}, []string{}),
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
	i, err := auth.ReadIdentityFromKeyPair(s.PrivKey, s.Cert)
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
	tconf.Auth.ClusterConfig, err = services.NewClusterConfig(services.ClusterConfigSpecV3{
		SessionRecording: services.RecordAtNode,
	})
	if err != nil {
		return trace.Wrap(err)
	}
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
	// create users and roles if they don't exist, or sign their keys if they're already present
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
		logins, err := services.RoleSet(roles).CheckLoginDuration(ttl)
		if err != nil {
			return trace.Wrap(err)
		}
		user.Key.Cert, err = auth.GenerateUserCert(user.Key.Pub, teleUser, logins, ttl, true, teleport.CompatibilityNone)
		if err != nil {
			return err
		}
	}
	return nil
}

// StartNode starts SSH node and connects it to the cluster
func (i *TeleInstance) StartNode(name string, sshPort, proxyWebPort, proxySSHPort int) error {
	dataDir, err := ioutil.TempDir("", "cluster-"+i.Secrets.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}
	tconf := service.MakeDefaultConfig()
	tconf.HostUUID = name
	tconf.Hostname = name
	tconf.DataDir = dataDir
	tconf.Auth.Enabled = false
	tconf.Proxy.Enabled = true
	tconf.SSH.Enabled = true
	tconf.SSH.Addr.Addr = net.JoinHostPort(i.Hostname, fmt.Sprintf("%v", sshPort))
	authServer := utils.MustParseAddr(net.JoinHostPort(i.Hostname, i.GetPortAuth()))
	tconf.AuthServers = append(tconf.AuthServers, *authServer)
	tconf.Token = "token"
	tconf.Proxy.Enabled = true
	tconf.Proxy.SSHAddr.Addr = net.JoinHostPort(i.Hostname, fmt.Sprintf("%v", proxySSHPort))
	tconf.Proxy.WebAddr.Addr = net.JoinHostPort(i.Hostname, fmt.Sprintf("%v", proxyWebPort))
	tconf.Proxy.DisableReverseTunnel = true
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

func (i *TeleInstance) Start() (err error) {
	proxyReady := make(chan service.Event)
	sshReady := make(chan service.Event)
	tunnelReady := make(chan service.Event)
	allReady := make(chan interface{})

	i.Process.WaitForEvent(service.ProxyIdentityEvent, proxyReady, make(chan struct{}))
	i.Process.WaitForEvent(service.SSHIdentityEvent, sshReady, make(chan struct{}))
	i.Process.WaitForEvent(service.ProxyReverseTunnelServerEvent, tunnelReady, make(chan struct{}))

	if err = i.Process.Start(); err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		close(sshReady)
		close(proxyReady)
	}()

	go func() {
		if i.Config.SSH.Enabled {
			<-sshReady
		}
		<-proxyReady
		te := <-tunnelReady
		ts, ok := te.Payload.(reversetunnel.Server)
		if !ok {
			err = fmt.Errorf("Global event '%v' did not deliver reverseTunenl server pointer as a payload", service.ProxyReverseTunnelServerEvent)
			log.Error(err)
		}
		i.Tunnel = ts
		close(allReady)
	}()

	timeoutTicker := time.NewTicker(time.Second * 5)
	defer timeoutTicker.Stop()

	select {
	case <-allReady:
		time.Sleep(time.Millisecond * 100)
		break
	case <-timeoutTicker.C:
		return fmt.Errorf("failed to start local Teleport instance: timeout")
	}
	log.Infof("Teleport instance '%v' started!", i.Secrets.SiteName)
	return err
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
		err = tc.AddTrustedCA(cas[i].V1())
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

func fatalIf(err error) {
	if err != nil {
		log.Fatal("", err)
	}
}

func makeKey() (priv, pub []byte) {
	priv, pub, _ = native.New().GenerateKeyPair("")
	return priv, pub
}
