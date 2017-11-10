/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package srv

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

var ctxID int32

// Server is regular or forwarding SSH server.
type Server interface {
	// ID is the unique ID of the server.
	ID() string

	// GetNamespace returns the namespace the server was created in.
	GetNamespace() string

	// AdvertiseAddr is the publicly addressable address of this server.
	AdvertiseAddr() string

	// Component is the type of server, forwarding or regular.
	Component() string

	// PermitUserEnvironment returns if reading environment variables upon
	// startup is allowed.
	PermitUserEnvironment() bool

	// EmitAuditEvent an Audit Event from this server.
	EmitAuditEvent(string, events.EventFields)

	// GetAuditLog returns the Audit Log for this server.
	GetAuditLog() events.IAuditLog

	// GetAuthService returns an auth.AccessPoint for this server.
	GetAuthService() auth.AccessPoint

	// GetSessionServer returns a session server.
	GetSessionServer() rsession.Service
}

// SessionContext holds session specific context, such as SSH auth agents, PTYs,
// and other resources. SessionContext also holds a ServerContext which can be
// used to access resources on the underlying server. SessionContext can also
// be used to attach resources that should be closed once the session closes.
type ServerContext struct {
	*log.Entry

	sync.RWMutex

	// env is a list of environment variables passed to the session.
	env map[string]string

	// srv is the server that is holding the context.
	srv Server

	// id is the server specific incremental session id.
	id int

	// term holds PTY if it was requested by the session.
	term Terminal

	// agent is a client to remote SSH agent.
	agent agent.Agent

	// agentCh is SSH channel using SSH agent protocol.
	agentChannel ssh.Channel

	// session holds the active session (if there's an active one).
	session *session

	// closers is a list of io.Closer that will be called when session closes
	// this is handy as sometimes client closes session, in this case resources
	// will be properly closed and deallocated, otherwise they could be kept hanging.
	closers []io.Closer

	// Conn is the underlying *ssh.ServerConn.
	Conn *ssh.ServerConn

	// ExecResultCh is a Go channel which will be used to send and receive the
	// result of a "exec" request.
	ExecResultCh chan ExecResult

	// SubsystemResultCh is a Go channel which will be used to send and receive
	// the result of a "subsystem" request.
	SubsystemResultCh chan SubsystemResult

	// TeleportUser is the Teleport user for the current session context.
	TeleportUser string

	// Login is the operating system user for the current session context.
	Login string

	// IsTestStub is set to true by tests.
	IsTestStub bool

	// ExecRequest is the command to be executed within this session context.
	ExecRequest Exec

	// ClusterName is the name of the cluster current user is authenticated with.
	ClusterName string

	// Certificate is the SSH certificate used in this session.
	Certificate string
}

// NewServerContext creates a new *ServerContext which is used to pass and
// manage resources.
func NewServerContext(srv Server, conn *ssh.ServerConn) *ServerContext {
	ctx := &ServerContext{
		id:                int(atomic.AddInt32(&ctxID, int32(1))),
		env:               make(map[string]string),
		srv:               srv,
		Conn:              conn,
		ExecResultCh:      make(chan ExecResult, 10),
		SubsystemResultCh: make(chan SubsystemResult, 10),
		TeleportUser:      conn.Permissions.Extensions[utils.CertTeleportUser],
		ClusterName:       conn.Permissions.Extensions[utils.CertTeleportClusterName],
		Certificate:       conn.Permissions.Extensions[utils.CertTeleportUserCertificate],
		Login:             conn.User(),
	}

	ctx.Entry = log.WithFields(log.Fields{
		trace.Component: srv.Component(),
		trace.ComponentFields: log.Fields{
			"local":        conn.LocalAddr(),
			"remote":       conn.RemoteAddr(),
			"login":        ctx.Login,
			"teleportUser": ctx.TeleportUser,
			"id":           ctx.id,
		},
	})
	return ctx
}

// GetCertificate parses the SSH certificate bytes and returns a *ssh.Certificate.
func (c *ServerContext) GetCertificate() (*ssh.Certificate, error) {
	k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(c.Certificate))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, ok := k.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("not a certificate: %v")
	}

	return cert, nil
}

// CreateOrJoinSession will look in the SessionRegistry for the session ID. If
// no session is found, a new one is created. If one is found, it is returned.
func (c *ServerContext) CreateOrJoinSession(reg *SessionRegistry) error {
	// As SSH conversation progresses, at some point a session will be created and
	// its ID will be added to the environment
	ssid, found := c.GetEnv(sshutils.SessionEnvVar)
	if !found {
		return nil
	}
	// make sure whatever session is requested is a valid session
	_, err := rsession.ParseID(ssid)
	if err != nil {
		return trace.BadParameter("invalid session id")
	}

	findSession := func() (*session, bool) {
		reg.Lock()
		defer reg.Unlock()
		return reg.findSession(rsession.ID(ssid))
	}

	// update ctx with a session ID
	c.session, _ = findSession()
	if c.session == nil {
		log.Debugf("[SSH] will create new session for SSH connection %v", c.Conn.RemoteAddr())
	} else {
		log.Debugf("[SSH] will join session %v for SSH connection %v", c.session, c.Conn.RemoteAddr())
	}

	return nil
}

// AddCloser adds any closer in ctx that will be called
// whenever server closes session channel
func (c *ServerContext) AddCloser(closer io.Closer) {
	c.Lock()
	defer c.Unlock()
	c.closers = append(c.closers, closer)
}

// GetAgent returns a agent.Agent which represents the capabilities of an SSH agent.
func (c *ServerContext) GetAgent() agent.Agent {
	c.RLock()
	defer c.RUnlock()
	return c.agent
}

// GetAgentChannel returns the channel over which communication with the agent occurs.
func (c *ServerContext) GetAgentChannel() ssh.Channel {
	c.RLock()
	defer c.RUnlock()
	return c.agentChannel
}

// SetAgent sets the agent and channel over which communication with the agent occurs.
func (c *ServerContext) SetAgent(a agent.Agent, channel ssh.Channel) {
	c.Lock()
	defer c.Unlock()
	if c.agentChannel != nil {
		c.Infof("closing previous agent channel")
		c.agentChannel.Close()
	}
	c.agentChannel = channel
	c.agent = a
}

// GetTerm returns a Terminal.
func (c *ServerContext) GetTerm() Terminal {
	c.RLock()
	defer c.RUnlock()

	return c.term
}

// SetTerm set a Terminal.
func (c *ServerContext) SetTerm(t Terminal) {
	c.Lock()
	defer c.Unlock()

	c.term = t
}

// SetEnv sets a environment variable within this context.
func (c *ServerContext) SetEnv(key, val string) {
	c.env[key] = val
}

// GetEnv returns a environment variable within this context.
func (c *ServerContext) GetEnv(key string) (string, bool) {
	val, ok := c.env[key]
	return val, ok
}

// takeClosers returns all resources that should be closed and sets the properties to null
// we do this to avoid calling Close() under lock to avoid potential deadlocks
func (c *ServerContext) takeClosers() []io.Closer {
	// this is done to avoid any operation holding the lock for too long
	c.Lock()
	defer c.Unlock()
	closers := []io.Closer{}
	if c.term != nil {
		closers = append(closers, c.term)
		c.term = nil
	}
	if c.agentChannel != nil {
		closers = append(closers, c.agentChannel)
		c.agentChannel = nil
	}
	closers = append(closers, c.closers...)
	c.closers = nil
	return closers
}

func (c *ServerContext) Close() error {
	return closeAll(c.takeClosers()...)
}

// SendExecResult sends the result of execution of the "exec" command over the
// ExecResultCh.
func (c *ServerContext) SendExecResult(r ExecResult) {
	select {
	case c.ExecResultCh <- r:
	default:
		log.Infof("blocked on sending exec result %v", r)
	}
}

// SendSubsystemResult sends the result of running the subsystem over the
// SubsystemResultCh.
func (c *ServerContext) SendSubsystemResult(r SubsystemResult) {
	select {
	case c.SubsystemResultCh <- r:
	default:
		c.Infof("blocked on sending subsystem result")
	}
}

func (c *ServerContext) String() string {
	return fmt.Sprintf("ServerContext(%v->%v, user=%v, id=%v)", c.Conn.RemoteAddr(), c.Conn.LocalAddr(), c.Conn.User(), c.id)
}

func closeAll(closers ...io.Closer) error {
	var err error
	for _, cl := range closers {
		if cl == nil {
			continue
		}
		if e := cl.Close(); e != nil {
			err = e
		}
	}
	return err
}

type closerFunc func() error

func (f closerFunc) Close() error {
	return f()
}
