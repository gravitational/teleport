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

package state

import (
	"io"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
)

var (
	errNotSupported = trace.BadParameter("method not supported")
)

const (
	// MaxQueueSize determines how many logging events to queue in-memory
	// before start dropping them (probably because logging server is down)
	MaxQueueSize = 10
)

// CachingAuditLog implements events.IAuditLog on the recording machine (SSH server)
// It captures the local recording and forwards it to the AuditLog network server
type CachingAuditLog struct {
	server    events.IAuditLog
	queue     chan msg
	closeC    chan int
	closeOnce sync.Once
}

// msg structure is used to transfer logging calls from the calling thread into
// asynchronous queue
type msg struct {
	eventType string
	fields    events.EventFields
	sid       session.ID
	namespace string
	reader    io.Reader
}

// MakeCachingAuditLog creaets a new & fully initialized instance of the alog
func MakeCachingAuditLog(logServer events.IAuditLog) *CachingAuditLog {
	ll := &CachingAuditLog{
		server: logServer,
		closeC: make(chan int),
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
func (ll *CachingAuditLog) run() {
	var err error
	for ll.server != nil {
		select {
		case <-ll.closeC:
			return
		case msg := <-ll.queue:
			if msg.fields != nil {
				err = ll.server.EmitAuditEvent(msg.eventType, msg.fields)
			} else if msg.reader != nil {
				err = ll.server.PostSessionChunk(msg.namespace, msg.sid, msg.reader)
			}
			if err != nil {
				log.Error(err)
			}
		}
	}
}

func (ll *CachingAuditLog) post(m msg) error {
	select {
	case ll.queue <- m:
	default:
		log.Warnf("Audit log cannot keep up. Dropping event '%v'", m.eventType)
	}
	return nil

}

func (ll *CachingAuditLog) Close() error {
	ll.closeOnce.Do(func() {
		close(ll.closeC)
	})
	return nil
}

func (ll *CachingAuditLog) EmitAuditEvent(eventType string, fields events.EventFields) error {
	return ll.post(msg{eventType: eventType, fields: fields})
}

func (ll *CachingAuditLog) PostSessionChunk(namespace string, sid session.ID, reader io.Reader) error {
	return ll.post(msg{sid: sid, reader: reader, namespace: namespace})
}

func (ll *CachingAuditLog) GetSessionChunk(string, session.ID, int, int) ([]byte, error) {
	return nil, errNotSupported
}
func (ll *CachingAuditLog) GetSessionEvents(string, session.ID, int) ([]events.EventFields, error) {
	return nil, errNotSupported
}
func (ll *CachingAuditLog) SearchEvents(time.Time, time.Time, string) ([]events.EventFields, error) {
	return nil, errNotSupported
}
