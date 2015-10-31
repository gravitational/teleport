package teleagent

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"net"
	"time"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

type TeleAgent struct {
	keys []Key
}

func (a *TeleAgent) Start(agentAddr string) error {
	addr, err := utils.ParseAddr(agentAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	l, err := net.Listen(addr.Network, addr.Addr)
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		for {
			conn, err := l.Accept()
			ag, err := a.GetAgent()
			if err != nil {
				log.Errorf(err.Error())
			} else {
				go func() {
					if err := agent.ServeAgent(ag, conn); err != nil {
						log.Errorf(err.Error())
					}
				}()
			}
		}
	}()

	return nil
}

func (a *TeleAgent) GetAgent() (agent.Agent, error) {
	ag := agent.NewKeyring()

	for _, key := range a.keys {
		k, err := ssh.ParseRawPrivateKey(key.Priv)
		if err != nil {
			log.Errorf("failed to add: %v", err)
			return nil, trace.Wrap(err)
		}
		addedKey := agent.AddedKey{
			PrivateKey:       k,
			Certificate:      key.Cert,
			Comment:          "",
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		}
		if err := ag.Add(addedKey); err != nil {
			log.Errorf("failed to add: %v", err)
			return nil, trace.Wrap(err)
		}
	}

	return ag, nil

}

func (a *TeleAgent) Login(proxyAddr string, user string, pass string,
	hotpToken string, ttl time.Duration) error {
	priv, pub, err := native.New().GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}

	cert, err := web.SSHAgentLogin(proxyAddr, user, pass, hotpToken,
		pub, ttl)
	if err != nil {
		return trace.Wrap(err)
	}

	pcert, _, _, _, err := ssh.ParseAuthorizedKey(cert)
	if err != nil {
		return trace.Wrap(err)
	}

	key := Key{
		Priv: priv,
		Cert: pcert.(*ssh.Certificate),
	}

	a.keys = append(a.keys, key)

	return nil
}

type Key struct {
	Priv []byte
	Cert *ssh.Certificate
}

const (
	DefaultAgentAddress = "unix:///tmp/teleport.agent.sock"
)
