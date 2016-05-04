package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
)

// SetTestTimeouts affects global timeouts inside Teleport, making connections
// work faster but consuming more CPU (useful for integration testing)
func SetTestTimeouts(ms int) {
	if ms == 0 {
		ms = 10
	}
	testVal := time.Duration(time.Millisecond * time.Duration(ms))
	defaults.ReverseTunnelAgentHeartbeatPeriod = testVal
	defaults.ServerHeartbeatTTL = testVal
	defaults.SessionRefreshPeriod = testVal
}

// TeleInstance represents an in-memory instance of a teleport
// process for testing
type TeleInstance struct {
	// Secrets holds the keys (pub, priv and derived cert) of this instance
	Secrets InstanceSecrets

	// Slice of TCP ports used by Teleport services
	Ports []int

	// Hostname is the name of the host where this isnstance is running
	Hostname string

	// Internal stuff...
	Process *service.TeleportProcess
	Config  *service.Config
	Tunnel  reversetunnel.Server
}

type User struct {
	Username      string      `json:"username"`
	AllowedLogins []string    `json:"logins"`
	Key           *client.Key `json:"key"`
}

type InstanceSecrets struct {
	// instance name (aka "site name")
	SiteName string `json:"site_name"`
	// instance keys+cert (reused for hostCA and userCA)
	PubKey  []byte `json:"pub"`
	PrivKey []byte `json:"priv"`
	Cert    []byte `json:"cert"`
	// ListenPort is a reverse tunnel listening port, allowing
	// other sites to connect to this instance. Set to empty
	// string if this instance is not allowing incoming tunnels
	ListenAddr string `json:"tunnel_addr"`
	// list of users this instance trusts (key in the map is username)
	Users map[string]*User
}

func (s *InstanceSecrets) String() string {
	bytes, _ := json.MarshalIndent(s, "", "\t")
	return string(bytes)
}

// NewInstance creates a new Teleport process instance
func NewInstance(siteName string, hostName string, ports []int, priv, pub []byte) *TeleInstance {
	var err error
	if len(ports) < 5 {
		fatalIf(fmt.Errorf("not enough free ports given: %v", ports))
	}
	if hostName == "" {
		hostName, err = os.Hostname()
		fatalIf(err)
	}
	// generate instance secrets (keys):
	keygen := native.New()
	if priv == nil || pub == nil {
		priv, pub, _ = keygen.GenerateKeyPair("")
	}
	cert, err := keygen.GenerateHostCert(priv, pub,
		hostName, siteName, teleport.RoleAdmin, time.Duration(time.Hour*24))
	fatalIf(err)
	secrets := InstanceSecrets{
		SiteName:   siteName,
		PrivKey:    priv,
		PubKey:     pub,
		Cert:       cert,
		ListenAddr: net.JoinHostPort(hostName, strconv.Itoa(ports[4])),
		Users:      make(map[string]*User),
	}
	return &TeleInstance{
		Secrets:  secrets,
		Ports:    ports,
		Hostname: hostName,
	}
}

// GetCAs return an array of CAs stored by the secrets object. In this
// case we always return hard-coded userCA + hostCA (and they share keys
// for simplicity)
func (this *InstanceSecrets) GetCAs() []services.CertAuthority {
	return []services.CertAuthority{
		{
			DomainName:    this.SiteName,
			Type:          services.HostCA,
			SigningKeys:   [][]byte{this.PrivKey},
			CheckingKeys:  [][]byte{this.PubKey},
			AllowedLogins: this.AllowedLogins(),
		},
		{
			DomainName:    this.SiteName,
			Type:          services.UserCA,
			SigningKeys:   [][]byte{this.PrivKey},
			CheckingKeys:  [][]byte{this.PubKey},
			AllowedLogins: this.AllowedLogins(),
		},
	}
}

func (this *InstanceSecrets) AllowedLogins() []string {
	logins := make([]string, len(this.Users))
	for i := range this.Users {
		logins = append(logins, this.Users[i].AllowedLogins...)
	}
	return logins
}

func (this *InstanceSecrets) AsSlice() []*InstanceSecrets {
	return []*InstanceSecrets{this}
}

func (this *InstanceSecrets) GetIdentity() *auth.Identity {
	i, err := auth.ReadIdentityFromKeyPair(this.PrivKey, this.Cert)
	fatalIf(err)
	return i
}

func (this *TeleInstance) GetPortSSHInt() int {
	return this.Ports[0]
}

func (this *TeleInstance) GetPortSSH() string {
	return strconv.Itoa(this.GetPortSSHInt())
}

func (this *TeleInstance) GetPortAuth() string {
	return strconv.Itoa(this.Ports[1])
}

func (this *TeleInstance) GetPortProxy() string {
	return strconv.Itoa(this.Ports[2])
}

func (this *TeleInstance) GetPortWeb() string {
	return strconv.Itoa(this.Ports[3])
}

// GetSiteAPI() is a helper which returns an API endpoint to a site with
// a given name. This endpoint implements HTTP-over-SSH access to the
// site's auth server.
func (this *TeleInstance) GetSiteAPI(siteName string) auth.ClientI {
	siteTunnel, err := this.Tunnel.GetSite(siteName)
	if siteTunnel == nil || err != nil {
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
func (this *TeleInstance) Create(trustedSecrets []*InstanceSecrets, enableSSH bool, console io.Writer) error {
	dataDir, err := ioutil.TempDir("", "cluster-"+this.Secrets.SiteName)
	if err != nil {
		return err
	}
	tconf := service.MakeDefaultConfig()
	tconf.Console = console
	tconf.Auth.DomainName = this.Secrets.SiteName
	tconf.Auth.Authorities = append(tconf.Auth.Authorities, this.Secrets.GetCAs()...)
	tconf.Identities = append(tconf.Identities, this.Secrets.GetIdentity())
	for _, trusted := range trustedSecrets {
		tconf.Auth.Authorities = append(tconf.Auth.Authorities, trusted.GetCAs()...)
		tconf.Identities = append(tconf.Identities, trusted.GetIdentity())
		if trusted.ListenAddr != "" {
			tconf.ReverseTunnels = []services.ReverseTunnel{
				{
					DomainName: trusted.SiteName,
					DialAddrs:  []string{trusted.ListenAddr},
				},
			}
		}
	}
	tconf.Proxy.ReverseTunnelListenAddr.Addr = this.Secrets.ListenAddr
	tconf.HostUUID = this.Secrets.GetIdentity().ID.HostUUID
	tconf.SSH.Enabled = enableSSH
	tconf.SSH.Addr.Addr = net.JoinHostPort(this.Hostname, this.GetPortSSH())
	tconf.Auth.SSHAddr.Addr = net.JoinHostPort(this.Hostname, this.GetPortAuth())
	tconf.Proxy.SSHAddr.Addr = net.JoinHostPort(this.Hostname, this.GetPortProxy())
	tconf.Proxy.WebAddr.Addr = net.JoinHostPort(this.Hostname, this.GetPortWeb())
	tconf.Proxy.DisableWebUI = true
	tconf.AuthServers[0].Addr = tconf.Auth.SSHAddr.Addr
	tconf.ConfigureBolt(dataDir)
	tconf.DataDir = dataDir
	tconf.Keygen = testauthority.New()
	this.Config = tconf
	this.Process, err = service.NewTeleport(tconf)
	if err != nil {
		return err
	}
	// create users:
	auth := this.Process.GetAuthServer()
	for _, user := range this.Secrets.Users {
		err := auth.UpsertUser(&services.TeleportUser{
			Name:          user.Username,
			AllowedLogins: user.AllowedLogins,
		})
		if err != nil {
			return err
		}
		priv, pub, _ := tconf.Keygen.GenerateKeyPair("")
		//priv, pub := makeKey()
		ttl := time.Duration(time.Hour * 24)
		cert, err := auth.GenerateUserCert(pub, user.Username, ttl)
		if err != nil {
			return err
		}
		user.Key = &client.Key{
			Priv: priv,
			Pub:  pub,
			Cert: cert,
		}
	}
	return nil
}

// Adds a new user into this Teleport instance. 'mappings' is a comma-separated
// list of OS users
func (this *TeleInstance) AddUser(username string, mappings []string) {
	log.Infof("teleInstance.AddUser(%v) mapped to %v", username, mappings)
	if mappings == nil {
		mappings = make([]string, 0)
	}
	this.Secrets.Users[username] = &User{
		Username:      username,
		AllowedLogins: mappings,
	}
}

func (this *TeleInstance) Start() (err error) {
	proxyReady := make(chan service.Event)
	sshReady := make(chan service.Event)
	tunnelReady := make(chan service.Event)
	allReady := make(chan interface{})

	this.Process.WaitForEvent(service.ProxyIdentityEvent, proxyReady, make(chan struct{}))
	this.Process.WaitForEvent(service.SSHIdentityEvent, sshReady, make(chan struct{}))
	this.Process.WaitForEvent(service.ProxyReverseTunnelServerEvent, tunnelReady, make(chan struct{}))

	if err = this.Process.Start(); err != nil {
		fatalIf(err)
	}

	defer func() {
		close(sshReady)
		close(proxyReady)
	}()

	go func() {
		if this.Config.SSH.Enabled {
			<-sshReady
		}
		<-proxyReady
		te := <-tunnelReady
		ts, ok := te.Payload.(reversetunnel.Server)
		if !ok {
			err = fmt.Errorf("Global event '%v' did not deliver reverseTunenl server pointer as a payload", service.ProxyReverseTunnelServerEvent)
			log.Error(err)
		}
		this.Tunnel = ts
		close(allReady)
	}()

	timeoutTicker := time.NewTicker(time.Second * 5)

	select {
	case <-allReady:
		time.Sleep(time.Millisecond * 100)
		break
	case <-timeoutTicker.C:
		return fmt.Errorf("failed to start local Teleport instance: timeout")
	}
	log.Infof("Teleport instance '%v' started!", this.Secrets.SiteName)
	return err
}

// NewClient returns a fully configured client (with server CAs and user keys)
func (this *TeleInstance) NewClient(login string, site string, host string, port int) (tc *client.TeleportClient, err error) {
	keyDir, err := ioutil.TempDir(this.Config.DataDir, "tsh")
	if err != nil {
		return nil, err
	}
	tc, err = client.NewClient(&client.Config{
		Login:              login,
		ProxyHost:          this.Config.Proxy.SSHAddr.Addr,
		Host:               host,
		HostPort:           port,
		HostLogin:          login,
		InsecureSkipVerify: true,
		KeysDir:            keyDir,
		SiteName:           site,
	})
	if err != nil {
		return nil, err
	}
	// tells the client to use user keys from 'secrets':
	user, ok := this.Secrets.Users[login]
	if !ok {
		return nil, fmt.Errorf("unknown login '%v'", login)
	}
	if user.Key == nil {
		return nil, fmt.Errorf("user %v has no key", login)
	}
	err = tc.AddKey(host, user.Key)
	if err != nil {
		return nil, err
	}
	// tell the client to trust given CAs (from secrets)
	cas := this.Secrets.GetCAs()
	for i := range cas {
		err = tc.AddTrustedCA(&cas[i])
		if err != nil {
			return nil, err
		}
	}
	return tc, nil
}

func (this *TeleInstance) Stop() error {
	if this.Config != nil {
		err := os.RemoveAll(this.Config.DataDir)
		if err != nil {
			log.Error("failed removing temporary local Teleport directory", err)
		}
	}
	log.Infof("Asking Teleport to stop")
	err := this.Process.Close()
	if err != nil {
		log.Error(err)
		return err
	}
	defer func() {
		log.Infof("Teleport instance '%v' stopped!", this.Secrets.SiteName)
	}()
	return this.Process.Wait()
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
