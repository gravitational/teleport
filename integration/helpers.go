package integration

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
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
	defaults.ReverseTunnelsRefreshPeriod = testVal
	defaults.ReverseTunnelAgentReconnectPeriod = testVal
	defaults.ReverseTunnelAgentHeartbeatPeriod = testVal
	defaults.ServerHeartbeatTTL = testVal
	defaults.AuthServersRefreshPeriod = testVal
	defaults.SessionRefreshPeriod = testVal
}

// TeleInstance represents an in-memory instance of a teleport
// process for testing
type TeleInstance struct {
	// Secrets holds the keys (pub, priv and derived cert) of this instance
	Secrets InstanceSecrets

	// StartPort determines which IP ports this process will start listen
	// on, value 6000 means it will take 6001, 6002, etc...
	StartPort int

	// Internal stuff...
	Process *service.TeleportProcess
	Config  *service.Config

	// Pre-created user which can be used to SSH into this cluster
	UserKey *client.Key
	User    *user.User
}

type InstanceSecrets struct {
	// instance name (aka "site name")
	Name string `json:"name"`
	// instance keys+cert (reused for hostCA and userCA)
	PubKey  []byte `json:"pub"`
	PrivKey []byte `json:"priv"`
	Cert    []byte `json:"cert"`
	// AllowedLogins is a list of OS logins that this instance trusts
	AllowedLogins []string `json:"logins"`
	// ListenPort is a reverse tunnel listening port, allowing
	// other sites to connect to this instance. Set to empty
	// string if this instance is not allowing incoming tunnels
	ListenAddr string `json:"tunnel_addr"`
	// UserKey is the key a user can use to login
}

// NewInstance creates a new Teleport process instance
func NewInstance(name string, startPort int) *TeleInstance {
	u, err := user.Current()
	fatalIf(err)
	// generate instance secrets (keys):
	keygen := native.New()
	priv, pub, _ := keygen.GenerateKeyPair("")
	cert, err := native.New().GenerateHostCert(priv, pub,
		name, name, teleport.RoleAdmin, time.Duration(time.Hour*24))
	fatalIf(err)
	secrets := InstanceSecrets{
		Name:          name,
		PubKey:        pub,
		PrivKey:       priv,
		Cert:          cert,
		AllowedLogins: []string{u.Username},
		ListenAddr:    net.JoinHostPort("127.0.0.1", strconv.Itoa(startPort+4)),
	}
	return &TeleInstance{
		Secrets:   secrets,
		StartPort: startPort,
	}
}

// GetCAs return an array of CAs stored by the secrets object. In this
// case we always return hard-coded userCA + hostCA (and they share keys
// for simplicity)
func (this *InstanceSecrets) GetCAs() []services.CertAuthority {
	return []services.CertAuthority{
		{
			DomainName:    this.Name,
			Type:          services.HostCA,
			SigningKeys:   [][]byte{this.PrivKey},
			CheckingKeys:  [][]byte{this.PubKey},
			AllowedLogins: this.AllowedLogins,
		},
		{
			DomainName:    this.Name,
			Type:          services.UserCA,
			SigningKeys:   [][]byte{this.PrivKey},
			CheckingKeys:  [][]byte{this.PubKey},
			AllowedLogins: this.AllowedLogins,
		},
	}
}

func (this *InstanceSecrets) AsSlice() []*InstanceSecrets {
	return []*InstanceSecrets{this}
}

func (this *InstanceSecrets) GetIdentity() *auth.Identity {
	i, err := auth.ReadIdentityFromKeyPair(this.PrivKey, this.Cert)
	fatalIf(err)
	return i
}

// Create creates a new instance of Teleport which trusts a lsit of other clusters (other
// instances)
func (this *TeleInstance) Create(trustedSecrets []*InstanceSecrets, enableSSH bool) error {
	dataDir, err := ioutil.TempDir("", "telecast-"+this.Secrets.Name)
	if err != nil {
		return err
	}
	tconf := service.MakeDefaultConfig()
	tconf.Auth.Authorities = append(tconf.Auth.Authorities, this.Secrets.GetCAs()...)
	tconf.Identities = append(tconf.Identities, this.Secrets.GetIdentity())
	for _, trusted := range trustedSecrets {
		tconf.Auth.Authorities = append(tconf.Auth.Authorities, trusted.GetCAs()...)
		tconf.Identities = append(tconf.Identities, trusted.GetIdentity())
		if trusted.ListenAddr != "" {
			tconf.ReverseTunnels = []services.ReverseTunnel{
				{
					DomainName: trusted.Name,
					DialAddrs:  []string{trusted.ListenAddr},
				},
			}
		}
	}
	tconf.Proxy.ReverseTunnelListenAddr.Addr = this.Secrets.ListenAddr
	tconf.HostUUID = this.Secrets.GetIdentity().ID.HostUUID
	tconf.SSH.Enabled = enableSSH
	tconf.SSH.Addr.Addr = net.JoinHostPort("127.0.0.1", strconv.Itoa(this.StartPort+0))
	tconf.Auth.SSHAddr.Addr = net.JoinHostPort("127.0.0.1", strconv.Itoa(this.StartPort+1))
	tconf.Proxy.SSHAddr.Addr = net.JoinHostPort("127.0.0.1", strconv.Itoa(this.StartPort+2))
	tconf.Proxy.WebAddr.Addr = net.JoinHostPort("127.0.0.1", strconv.Itoa(this.StartPort+3))
	tconf.Proxy.DisableWebUI = true
	tconf.AuthServers[0].Addr = tconf.Auth.SSHAddr.Addr
	tconf.ConfigureBolt(dataDir)
	tconf.DataDir = dataDir
	this.Config = tconf
	this.Process, err = service.NewTeleport(tconf)
	if err != nil {
		return err
	}

	auth := this.Process.GetAuthServer()
	this.User, err = user.Current()
	fatalIf(err)

	// create user:
	err = auth.UpsertUser(&services.TeleportUser{
		Name:          this.User.Username,
		AllowedLogins: []string{this.User.Username},
	})
	if err != nil {
		return err
	}
	priv, pub := makeKey()
	ttl := time.Duration(time.Hour * 24)
	cert, err := auth.GenerateUserCert(pub, this.User.Username, ttl)
	if err != nil {
		return err
	}
	this.UserKey = &client.Key{
		Priv:     priv,
		Pub:      pub,
		Cert:     cert,
		Deadline: time.Now().Add(ttl),
	}
	return nil
}

func (this *TeleInstance) Start() error {
	proxyReady := make(chan service.Event)
	sshReady := make(chan service.Event)
	allReady := make(chan interface{})

	this.Process.WaitForEvent(service.ProxyIdentityEvent, proxyReady, make(chan struct{}))
	this.Process.WaitForEvent(service.SSHIdentityEvent, sshReady, make(chan struct{}))

	if err := this.Process.Start(); err != nil {
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
		close(allReady)
	}()

	timeoutTicker := time.NewTicker(time.Second * 10)

	select {
	case <-allReady:
		time.Sleep(time.Millisecond * 100)
		break
	case <-timeoutTicker.C:
		return fmt.Errorf("failed to start local Teleport instance: timeout")
	}
	log.Infof("Teleport instance '%v' started!", this.Secrets.Name)
	return nil
}

// SSH executes SSH command on a remote node behind a given site
func (this *TeleInstance) SSH(command []string, site string, host string, port int) (output []byte, err error) {
	tc, err := client.NewClient(&client.Config{
		Login:              this.User.Username,
		ProxyHost:          this.Config.Proxy.SSHAddr.Addr,
		Host:               host,
		HostPort:           port,
		HostLogin:          this.User.Username,
		InsecureSkipVerify: true,
		KeysDir:            this.Config.DataDir,
		SiteName:           site,
	})
	if err != nil {
		return nil, err
	}
	var buff bytes.Buffer
	tc.Output = &buff
	err = tc.SaveKey(this.UserKey)
	if err != nil {
		return nil, err
	}
	cas := this.Secrets.GetCAs()
	for i := range cas {
		err = tc.AddTrustedCA(&cas[i])
		if err != nil {
			return nil, err
		}
	}
	err = tc.SSH(command, false)
	return buff.Bytes(), err
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
		log.Infof("Teleport instance '%v' stopped!", this.Secrets.Name)
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
