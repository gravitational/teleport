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

// Agent extends the agent.Agent interface.
// APIs which accept this interface promise to
// call `Close()` when they are done using the
// supplied agent.
type Agent interface {
	agent.Agent
	io.Closer
}

// nopCloser wraps an agent.Agent in the extended
// Agent interface by adding a NOP closer.
type nopCloser struct {
	agent.Agent
}

func (n nopCloser) Close() error { return nil }

// NopCloser wraps an agent.Agent with a NOP closer, allowing it
// to be passed to APIs which expect the extended agent interface.
func NopCloser(std agent.Agent) Agent {
	return nopCloser{std}
}

// Getter is a function used to get an agent instance.
type Getter func() (Agent, error)

// AgentServer is implementation of SSH agent server
type AgentServer struct {
	getAgent Getter
	listener net.Listener
	path     string
}

// NewServer returns new instance of agent server
func NewServer(getter Getter) *AgentServer {
	return &AgentServer{getAgent: getter}
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

		// get an agent instance for serving this conn
		instance, err := a.getAgent()
		if err != nil {
			log.Errorf("Failed to get agent: %v", err)
			return trace.Wrap(err)
		}

		// serve agent protocol against conn in a
		// separate goroutine.
		go func() {
			defer instance.Close()
			if err := agent.ServeAgent(instance, conn); err != nil {
				if err != io.EOF {
					log.Error(err)
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
