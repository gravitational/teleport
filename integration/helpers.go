package integration

import (
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
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/pborman/uuid"
)

// TeleInstance represents an in-memory instance of a teleport
// process for testing
type TeleInstance struct {
	// Name is a: "domain name" AKA "site name" AKA "cluster name" AKA "instance name"
	Name string
	ID   *auth.Identity

	// List of CAs to pre-create for this instance
	CAs []services.CertAuthority

	// StartPort determines which IP ports this process will start listen
	// on, value 6000 means it will take 6001, 6002, etc...
	StartPort int

	// Reverse tunnel listen address (AKA "cluster address")
	ListenAddr string

	// Internal stuff...
	Process *service.TeleportProcess
	Config  *service.Config

	// Pre-created user which can be used to SSH into this cluster
	UserKey *client.Key
	User    *user.User
}

func NewInstance(name string, startPort int) *TeleInstance {
	u, err := user.Current()
	fatalIf(err)

	id := makeAuthIdentity(name, teleport.RoleAdmin)
	cas := []services.CertAuthority{
		{
			DomainName:    id.AuthorityDomain,
			Type:          services.HostCA,
			SigningKeys:   [][]byte{id.KeyBytes},
			CheckingKeys:  [][]byte{id.CertBytes},
			AllowedLogins: []string{u.Username},
		},
		{
			DomainName:    id.AuthorityDomain,
			Type:          services.UserCA,
			SigningKeys:   [][]byte{id.KeyBytes},
			CheckingKeys:  [][]byte{id.CertBytes},
			AllowedLogins: []string{u.Username},
		},
	}
	return &TeleInstance{
		Name:       name,
		ID:         id,
		CAs:        cas,
		StartPort:  startPort,
		ListenAddr: net.JoinHostPort("127.0.0.1", strconv.Itoa(startPort+4)),
	}
}

func (this *TeleInstance) Create(tunnelTo *TeleInstance, enableSSH bool) error {
	dataDir, err := ioutil.TempDir("", "telecast-"+this.Name)
	if err != nil {
		return err
	}
	// create teleport process:
	tconf := service.MakeDefaultConfig()
	tconf.Auth.Authorities = append(tconf.Auth.Authorities, this.CAs...)
	tconf.Identities = append(tconf.Identities, this.ID)
	if tunnelTo != nil {
		tconf.Auth.Authorities = append(tconf.Auth.Authorities, tunnelTo.CAs...)
		tconf.Identities = append(tconf.Identities, tunnelTo.ID)
		if tunnelTo.ListenAddr != "" {
			tconf.ReverseTunnels = []services.ReverseTunnel{
				{
					DomainName: tunnelTo.Name,
					DialAddrs:  []string{tunnelTo.ListenAddr},
				},
			}
		}
	}
	tconf.Proxy.ReverseTunnelListenAddr.Addr = this.ListenAddr
	tconf.HostUUID = this.ID.ID.HostUUID
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
	log.Infof("Teleport instance '%v' started!", this.Name)
	return nil
}

func (this *TeleInstance) SSH(command []string, host string, port int) error {
	tc, err := client.NewClient(&client.Config{
		Login:              this.User.Username,
		ProxyHost:          this.Config.Proxy.SSHAddr.Addr,
		Host:               host,
		HostPort:           port,
		HostLogin:          this.User.Username,
		InsecureSkipVerify: true,
		KeysDir:            this.Config.DataDir,
		SiteName:           "client",
	})
	if err != nil {
		return err
	}
	err = tc.SaveKey(this.UserKey)
	if err != nil {
		return err
	}
	for i := range this.CAs {
		err = tc.AddTrustedCA(&this.CAs[i])
		if err != nil {
			return err
		}
	}
	return tc.SSH(command, false)
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
		log.Infof("Teleport instance '%v' stopped!", this.Name)
	}()
	return this.Process.Wait()
}

func makeAuthIdentity(authDomain string, role teleport.Role) *auth.Identity {
	native.PrecalculatedKeysNum = 0

	keygen := native.New()
	defer keygen.Close()

	priv, pub := makeKey()

	_ = uuid.New()
	cert, err := keygen.GenerateHostCert(priv, pub,
		authDomain, authDomain, role, time.Duration(time.Hour*24))
	fatalIf(err)

	i, err := auth.ReadIdentityFromKeyPair(priv, cert)
	fatalIf(err)

	// ok, it's not a cert, just the public key, but that's what we need
	// down the road
	i.CertBytes = pub
	return i
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
