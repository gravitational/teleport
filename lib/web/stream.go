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

	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/session"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/net/websocket"
)

func newSessionStreamHandler(sessionID session.ID, ctx *SessionContext, site reversetunnel.RemoteSite, pollPeriod time.Duration) (*sessionStreamHandler, error) {
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
	ctx        *SessionContext
	site       reversetunnel.RemoteSite
	sessionID  session.ID
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

	ticker := time.NewTicker(w.pollPeriod)
	defer ticker.Stop()
	defer w.Close()
	for {
		// TODO: do we need to return nodes to the client here?
		servers, err := clt.GetNodes()
		if err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
		sess, err := clt.GetSession(w.sessionID)
		if err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
		if sess != nil {
			event := &sessionStreamEvent{
				Session: *sess,
				Nodes:   servers,
			}
			if err := websocket.JSON.Send(ws, event); err != nil {
				return trace.Wrap(err)
			}
		}

		select {
		case <-ticker.C:
		case <-w.closeC:
			log.Infof("[web] session.stream() exited")
			return nil
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
