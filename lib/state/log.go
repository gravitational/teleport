package state

import (
	"io"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
)

var (
	errNotSupported = trace.Errorf("method not supported")
)

const (
	// CloseLog is a fake (only supported and understood here) event type
	// which signals the Local Audit Log to close its resources
	CloseLog = "x-close-local-log"

	// MaxQueueSize determines how many logging events to queue in-memory
	// before start dropping them (probably because logging server is down)
	MaxQueueSize = 10
)

// LocalAuditLog implements events.IAuditLog on the recording machine (SSH server)
// It captures the local recording and forwards it to the AuditLog network server
type LocalAuditLog struct {
	server events.IAuditLog
	queue  chan msg
}

// msg structure is used to transfer logging calls from the calling thread into
// asynchronous queue
type msg struct {
	eventType string
	fields    events.EventFields
	sid       session.ID
	reader    io.Reader
}

// MakeLocalAuditLog creaets a new & fully initialized instance of the alog
func MakeLocalAuditLog(logServer events.IAuditLog) events.IAuditLog {
	ll := &LocalAuditLog{
		server: logServer,
	}
	// start the queue:
	if logServer != nil {
		ll.queue = make(chan msg, MaxQueueSize+1)
		go ll.run()
	}
	return ll
}

// run thread is picking up logging events and tries to forward them
// to the logging server
func (ll *LocalAuditLog) run() {
	var err error
	for ll.server != nil {
		select {
		case msg := <-ll.queue:
			if msg.fields != nil {
				err = ll.server.EmitAuditEvent(msg.eventType, msg.fields)
			} else if msg.reader != nil {
				err = ll.server.PostSessionChunk(msg.sid, msg.reader)
			}
			if err != nil {
				log.Error(err)
			}
		}
	}
}

func (ll *LocalAuditLog) post(m msg) error {
	if len(ll.queue) < MaxQueueSize {
		ll.queue <- m
	}
	return nil
}

func (ll *LocalAuditLog) Close() error {
	if ll.server != nil {
		ll.server = nil
		close(ll.queue)
	}
	return nil
}

func (ll *LocalAuditLog) EmitAuditEvent(eventType string, fields events.EventFields) error {
	if eventType == CloseLog {
		return ll.Close()
	}
	if ll.server == nil {
		return nil
	}
	return ll.post(msg{eventType: eventType, fields: fields})
}

func (ll *LocalAuditLog) PostSessionChunk(sid session.ID, reader io.Reader) error {
	if ll.server == nil {
		return nil
	}
	return ll.post(msg{sid: sid, reader: reader})
}

func (ll *LocalAuditLog) GetSessionChunk(session.ID, int, int) ([]byte, error) {
	return nil, errNotSupported
}
func (ll *LocalAuditLog) GetSessionEvents(session.ID, int) ([]events.EventFields, error) {
	return nil, errNotSupported
}
func (ll *LocalAuditLog) SearchEvents(time.Time, time.Time, string) ([]events.EventFields, error) {
	return nil, errNotSupported
}
