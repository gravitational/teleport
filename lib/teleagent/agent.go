package teleagent

import (
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"io"
	"net"
	"time"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

type TeleAgent struct {
	agent agent.Agent
}

func NewTeleAgent() *TeleAgent {
	ta := TeleAgent{
		agent: agent.NewKeyring(),
	}
	return &ta
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
			if err != nil {
				log.Errorf(err.Error())
				continue
			}
			go func() {
				if err := agent.ServeAgent(a.agent, conn); err != nil {
					if err != io.EOF {
						log.Errorf(err.Error())
					}
				}
			}()
		}
	}()

	return nil
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

	pk, err := ssh.ParseRawPrivateKey(priv)
	if err != nil {
		return trace.Wrap(err)
	}
	addedKey := agent.AddedKey{
		PrivateKey:       pk,
		Certificate:      pcert.(*ssh.Certificate),
		Comment:          "",
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}
	if err := a.agent.Add(addedKey); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

const (
	DefaultAgentAddress = "unix:///tmp/teleport.agent.sock"
)
