package teleagent

import (
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/agent"
)

// AgentServer is implementation of SSH agent server
type AgentServer struct {
	agent.Agent
	listener net.Listener
	path     string
}

// NewServer returns new instance of agent server
func NewServer() *AgentServer {
	return &AgentServer{Agent: agent.NewKeyring()}
}

// ListenUnixSocket starts listening and serving agent assuming that
func (a *AgentServer) ListenUnixSocket(path string, uid, gid int, mode os.FileMode) error {
	l, err := net.Listen("unix", path)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := os.Chown(path, uid, gid); err != nil {
		l.Close()
		return trace.ConvertSystemError(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		l.Close()
		return trace.ConvertSystemError(err)
	}
	a.listener = l
	a.path = path
	return nil
}

// Serve starts serving on the listener, assumes that Listen was called before
func (a *AgentServer) Serve() error {
	if a.listener == nil {
		return trace.BadParameter("Serve needs a Listen call first")
	}
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		conn, err := a.listener.Accept()
		if err != nil {
			neterr, ok := err.(net.Error)
			if !ok {
				return trace.Wrap(err, "unknown error")
			}
			if !neterr.Temporary() {
				if !strings.Contains(neterr.Error(), "use of closed network connection") {
					log.Errorf("got permanent error: %v", err)
				}
				return err
			}
			if tempDelay == 0 {
				tempDelay = 5 * time.Millisecond
			} else {
				tempDelay *= 2
			}
			if max := 1 * time.Second; tempDelay > max {
				tempDelay = max
			}
			log.Errorf("got temp error: %v, will sleep %v", err, tempDelay)
			time.Sleep(tempDelay)
			continue
		}
		tempDelay = 0
		go func() {
			if err := agent.ServeAgent(a.Agent, conn); err != nil {
				if err != io.EOF {
					log.Errorf(err.Error())
				}
			}
		}()
	}
}

// ListenAndServe is similar http.ListenAndServe
func (a *AgentServer) ListenAndServe(addr utils.NetAddr) error {
	l, err := net.Listen(addr.AddrNetwork, addr.Addr)
	if err != nil {
		return trace.Wrap(err)
	}
	a.listener = l
	return a.Serve()
}

// Close closes listener and stops serving agent
func (a *AgentServer) Close() error {
	var errors []error
	if a.listener != nil {
		log.Debugf("AgentServer(%v) is closing", a.listener.Addr())
		if err := a.listener.Close(); err != nil {
			errors = append(errors, trace.ConvertSystemError(err))
		}
	}
	if a.path != "" {
		if err := os.Remove(a.path); err != nil {
			errors = append(errors, trace.ConvertSystemError(err))
		}
	}
	return trace.NewAggregate(errors...)
}
