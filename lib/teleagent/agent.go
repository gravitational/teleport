package teleagent

import (
	"io"
	"net"

	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
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
