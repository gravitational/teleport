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

package web

import (
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/net/websocket"
)

func newSessionStreamHandler(namespace string, sessionID session.ID, ctx *SessionContext, site reversetunnel.RemoteSite, pollPeriod time.Duration) (*sessionStreamHandler, error) {
	return &sessionStreamHandler{
		pollPeriod: pollPeriod,
		sessionID:  sessionID,
		ctx:        ctx,
		site:       site,
		closeC:     make(chan bool),
		namespace:  namespace,
	}, nil
}

// sessionStreamHandler streams events related to some particular session
// as a stream of JSON encoded event packets
type sessionStreamHandler struct {
	closeOnce  sync.Once
	pollPeriod time.Duration
	ctx        *SessionContext
	site       reversetunnel.RemoteSite
	namespace  string
	sessionID  session.ID
	closeC     chan bool
	ws         *websocket.Conn
}

func (w *sessionStreamHandler) Close() error {
	if w.ws != nil {
		w.ws.Close()
	}
	w.closeOnce.Do(func() {
		close(w.closeC)
	})
	return nil
}

// sessionStreamPollPeriod defines how frequently web sessions are
// sent new events
var sessionStreamPollPeriod = time.Second

// stream runs in a loop generating "something changed" events for a
// given active WebSession
//
// The events are fed to a web client via the websocket
func (w *sessionStreamHandler) stream(ws *websocket.Conn) error {
	w.ws = ws
	clt, err := w.site.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}
	// spin up a goroutine to detect closed socket by reading
	// from it
	go func() {
		defer w.Close()
		io.Copy(ioutil.Discard, ws)
	}()

	eventsCursor := -1
	emptyEventList := make([]events.EventFields, 0)

	pollEvents := func() []events.EventFields {
		// ask for any events than happened since the last call:
		re, err := clt.GetSessionEvents(w.namespace, w.sessionID, eventsCursor+1)
		if err != nil {
			log.Error(err)
			return emptyEventList
		}
		batchLen := len(re)
		if batchLen == 0 {
			return emptyEventList
		}
		// advance the cursor, so next time we'll ask for the latest:
		eventsCursor = re[batchLen-1].GetInt(events.EventCursor)
		return re
	}

	ticker := time.NewTicker(w.pollPeriod)
	defer ticker.Stop()
	defer w.Close()

	// keep polling in a loop:
	for {
		// wait for next timer tick or a signal to abort:
		select {
		case <-ticker.C:
		case <-w.closeC:
			log.Infof("[web] session.stream() exited")
			return nil
		}

		newEvents := pollEvents()
		sess, err := clt.GetSession(w.namespace, w.sessionID)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			log.Error(err)
		}
		if sess == nil {
			log.Warningf("invalid session ID: %v", w.sessionID)
			continue
		}
		servers, err := clt.GetNodes(w.namespace)
		if err != nil {
			log.Error(err)
		}
		if len(newEvents) > 0 {
			log.Infof("[WEB] streaming for %v. Events: %v, Nodes: %v, Parties: %v",
				w.sessionID, len(newEvents), len(servers), len(sess.Parties))
		}

		// push events to the web client
		event := &sessionStreamEvent{
			Events:  newEvents,
			Session: sess,
			Servers: services.ServersToV1(servers),
		}
		if err := websocket.JSON.Send(ws, event); err != nil {
			log.Error(err)
		}
	}
}

func (w *sessionStreamHandler) Handler() http.Handler {
	// TODO(klizhentas)
	// we instantiate a server explicitly here instead of using
	// websocket.HandlerFunc to set empty origin checker
	// make sure we check origin when in prod mode
	return &websocket.Server{
		Handler: func(ws *websocket.Conn) {
			if err := w.stream(ws); err != nil {
				log.WithFields(log.Fields{"sid": w.sessionID}).Infof("handler returned: %#v", err)
			}
		},
	}
}
