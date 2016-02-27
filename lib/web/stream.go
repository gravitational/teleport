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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/session"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/net/websocket"
)

func newSessionStreamHandler(sessionID string, ctx *sessionContext, site reversetunnel.RemoteSite, pollPeriod time.Duration) (*sessionStreamHandler, error) {
	return &sessionStreamHandler{
		pollPeriod: pollPeriod,
		sessionID:  sessionID,
		ctx:        ctx,
		site:       site,
		closeC:     make(chan bool),
	}, nil
}

// sessionStreamHandler streams events related to some particular session
// as a stream of JSON encoded event packets
type sessionStreamHandler struct {
	closeOnce  sync.Once
	pollPeriod time.Duration
	ctx        *sessionContext
	site       reversetunnel.RemoteSite
	sessionID  string
	closeC     chan bool
	ws         *websocket.Conn
}

func (w *sessionStreamHandler) Close() error {
	w.ws.Close()
	w.closeOnce.Do(func() {
		close(w.closeC)
	})
	return nil
}

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

	var lastCheckpoint time.Time
	var lastEvent *sessionStreamEvent
	ticker := time.NewTicker(w.pollPeriod)
	defer ticker.Stop()
	defer w.Close()
	for {
		now := time.Now()
		f := events.Filter{
			SessionID: w.sessionID,
			Order:     events.Desc,
			Limit:     20,
			Start:     now,
			End:       lastCheckpoint,
		}
		events, err := clt.GetEvents(f)
		if err != nil {
			if !teleport.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}

		servers, err := clt.GetServers()
		if err != nil {
			if !teleport.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}

		sess, err := clt.GetSession(w.sessionID)
		if err != nil {
			if !teleport.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}

		if sess != nil {
			event := &sessionStreamEvent{
				Session: *sess,
				Nodes:   servers,
				Events:  events,
			}

			newData := w.diffEvents(lastEvent, event)
			lastCheckpoint = now
			lastEvent = event
			if newData {
				log.Infof("about to send %#v", event)
				if err := websocket.JSON.Send(ws, event); err != nil {
					return trace.Wrap(err)
				}
			}
		}

		log.Infof("about to sleep %v", w.pollPeriod)
		select {
		case <-ticker.C:
		case <-w.closeC:
			log.Infof("stream is closed")
			return nil
		}
	}
}

const defaultPollPeriod = time.Second

func (w *sessionStreamHandler) Handler() http.Handler {
	// TODO(klizhentas)
	// we instantiate a server explicitly here instead of using
	// websocket.HandlerFunc to set empty origin checker
	// make sure we check origin when in prod mode
	return &websocket.Server{
		Handler: w.logResult(w.stream),
	}
}

func (w *sessionStreamHandler) logResult(fn func(*websocket.Conn) error) websocket.Handler {
	return func(ws *websocket.Conn) {
		err := fn(ws)
		if err != nil {
			log.WithFields(log.Fields{"sid": w.sessionID}).Infof("handler returned: %#v", err)
		}
	}
}

func (w *sessionStreamHandler) diffEvents(last *sessionStreamEvent, new *sessionStreamEvent) bool {
	// this is first call, ship whatever we have on this session
	if last == nil {
		return true
	}
	// we've got new events
	if len(new.Events) != 0 {
		log.Infof("got new events")
		return true
	}
	// new servers have arrived or disappeared
	if len(last.Nodes) != len(new.Nodes) {
		log.Infof("nodes have changes")
		return true
	}
	// parties have joined or left the scene
	if setsDifferent(partySet(last), partySet(new)) {
		log.Infof("parties have changes")
		return true
	}
	return false
}

func partySet(e *sessionStreamEvent) map[session.Party]bool {
	parties := make(map[session.Party]bool, len(e.Session.Parties))
	for _, party := range e.Session.Parties {
		parties[party] = true
	}
	return parties
}

func setsDifferent(a, b map[session.Party]bool) bool {
	for key := range a {
		if !b[key] {
			return true
		}
	}
	for key := range b {
		if !a[key] {
			return true
		}
	}
	return false
}
