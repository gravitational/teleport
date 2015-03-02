package srv

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/codahale/lunk"
	"github.com/mailgun/log"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent" // ctxID is a incremental context ID used for debugging and logging purposes
)

var ctxID int32

// closeMsg is a close request
type closeMsg struct {
	reason string
}

// ctx holds session specific context, such as SSH auth agents
// PTYs, and other resources. ctx can be used to attach resources
// that should be closed once the session closes.
type ctx struct {
	// srv is a pointer to the server holding the context
	srv *Server

	// this event id will be associated with all events emitted with this context
	eid lunk.EventID

	// server specific incremental session id
	id int
	// info about connection for debugging purposes
	info ssh.ConnMetadata

	sync.RWMutex
	// term holds PTY if it was requested by the session
	term *term

	// agent is a client to remote SSH agent
	agent agent.Agent
	// agentCh is SSH channel using SSH agent protocol
	agentCh ssh.Channel

	// result channel will be used by remote executions
	// that are processed in separate process, once the result is collected
	// they would send the result to this channel
	result chan execResult

	// close used by channel operations asking to close the session
	close chan closeMsg

	// closers is a list of io.Closer that will be called when session closes
	// this is handy as sometimes client closes session, in this case resources
	// will be properly closed and deallocated, otherwise they could be kept hanging
	closers []io.Closer
}

// emit emits event
func (c *ctx) emit(e lunk.Event) {
	c.srv.elog.Log(c.eid, e)
}

// addCloser adds any closer in ctx that will be called
// whenever server closes session channel
func (c *ctx) addCloser(closer io.Closer) {
	c.Lock()
	defer c.Unlock()
	c.closers = append(c.closers, closer)
}

func (c *ctx) getAgent() agent.Agent {
	c.RLock()
	defer c.RUnlock()
	return c.agent
}

func (c *ctx) setAgent(a agent.Agent, ch ssh.Channel) {
	c.Lock()
	defer c.Unlock()
	if c.agentCh != nil {
		log.Infof("closing previous agent channel")
		c.agentCh.Close()
	}
	c.agentCh = ch
	c.agent = a
}

func (c *ctx) getTerm() *term {
	c.RLock()
	defer c.RUnlock()
	return c.term
}

func (c *ctx) setTerm(t *term) {
	c.Lock()
	defer c.Unlock()
	c.term = t
}

// takeClosers returns all resources that should be closed and sets the properties to null
// we do this to avoid calling Close() under lock to avoid potential deadlocks
func (c *ctx) takeClosers() []io.Closer {
	// this is done to avoid any operation holding the lock for too long
	c.Lock()
	defer c.Unlock()
	closers := []io.Closer{}
	if c.term != nil {
		closers = append(closers, c.term)
		c.term = nil
	}
	if c.agentCh != nil {
		closers = append(closers, c.agentCh)
		c.agentCh = nil
	}
	closers = append(closers, c.closers...)
	c.closers = nil
	return closers
}

func (c *ctx) Close() error {
	log.Infof("%v ctx.Close()", c)
	return closeAll(c.takeClosers()...)
}

func (c *ctx) sendResult(r execResult) {
	log.Infof("%v sendResult(%v)", c, r)
	select {
	case c.result <- r:
	default:
		log.Infof("blocked on sending exec result %v", r)
	}
}

func (c *ctx) requestClose(msg string) {
	select {
	case c.close <- closeMsg{reason: msg}:
	default:
		log.Infof("blocked on sending close request %v", msg)
	}
}

func (c *ctx) resolver() resolver {
	return c.srv.rr
}

func (c *ctx) String() string {
	return fmt.Sprintf("sess(%v->%v, user=%v, id=%v)", c.info.RemoteAddr(), c.info.LocalAddr(), c.info.User(), c.id)
}

func newCtx(srv *Server, info ssh.ConnMetadata) *ctx {
	return &ctx{
		eid:    lunk.NewRootEventID(),
		info:   info,
		id:     int(atomic.AddInt32(&ctxID, int32(1))),
		result: make(chan execResult, 10),
		close:  make(chan closeMsg, 10),
		srv:    srv,
	}
}
