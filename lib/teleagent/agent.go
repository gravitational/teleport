package teleagent

import (
	"context"
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

// AgentGetter is a function used to get an agent instance.
type AgentGetter func(context.Context) (agent.Agent, error)

// AgentServer is implementation of SSH agent server
type AgentServer struct {
	agent.Agent
	Getter   AgentGetter
	listener net.Listener
	path     string
}

// NewServer returns new instance of agent server
func NewServer() *AgentServer {
	return &AgentServer{Agent: agent.NewKeyring()}
}

// getAgent gets an agent instance
func (a *AgentServer) getAgent(ctx context.Context) (agent.Agent, error) {
	if a.Agent != nil {
		return a.Agent, nil
	}
	return a.Getter(ctx)
}

// startServe starts serving agent protocol against conn
func (a *AgentServer) startServe(ctx context.Context, conn net.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	instance, err := a.getAgent(ctx)
	if err != nil {
		cancel()
		return trace.Wrap(err)
	}
	go func() {
		defer cancel()
		if err := agent.ServeAgent(instance, conn); err != nil {
			if err != io.EOF {
				log.Errorf(err.Error())
			}
		}
	}()
	return nil
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
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	if a.listener == nil {
		return trace.BadParameter("Serve needs a Listen call first")
	}
	if a.Agent == nil && a.Getter == nil {
		return trace.BadParameter("An agent or agent getter must be supplied.")
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
		if err := a.startServe(ctx, conn); err != nil {
			log.Errorf("Failed to start serving agent: %v", err)
			return trace.Wrap(err)
		}
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
