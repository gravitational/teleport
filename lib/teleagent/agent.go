package teleagent

import (
	"io"
	"net"
	"time"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// AgentServer is implementation of SSH agent server
type AgentServer struct {
	agent.Agent
}

// NewServer returns new instance of agent server
func NewServer() *AgentServer {
	return &AgentServer{agent.NewKeyring()}
}

// ListenAndServe is similar http.ListenAndServe
func (a *AgentServer) ListenAndServe(addr utils.NetAddr) error {
	l, err := net.Listen(addr.AddrNetwork, addr.Addr)
	if err != nil {
		return trace.Wrap(err)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Errorf(err.Error())
			continue
		}
		go func() {
			if err := agent.ServeAgent(a.Agent, conn); err != nil {
				if err != io.EOF {
					log.Errorf(err.Error())
				}
			}
		}()
	}
}

// Client is a client connection to SSH agent
type Client struct {
	agent.Agent
}

// NewClient returns a new client connected to remote agent
func NewClient(addr utils.NetAddr) (*Client, error) {
	conn, err := net.Dial(addr.AddrNetwork, addr.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Client{agent.NewClient(conn)}, nil
}

// Login logins with remote proxy and adds the certificate to it
func (a *Client) Login(proxyAddr string,
	user string, pass string, hotpToken string,
	ttl time.Duration, insecure bool) error {

	priv, pub, err := native.New().GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}

	login, err := client.SSHAgentLogin(proxyAddr, user, pass, hotpToken,
		pub, ttl, insecure, nil)
	if err != nil {
		return trace.Wrap(err)
	}

	pcert, _, _, _, err := ssh.ParseAuthorizedKey(login.Cert)
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
	if err := a.Agent.Add(addedKey); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
