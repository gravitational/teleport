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
	"net/http"
	"time"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/reversetunnel"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/net/websocket"
)

func newSessionStreamHandler(sessionID string, ctx *sessionContext, site reversetunnel.RemoteSite, pollPeriod time.Duration) (*sessionStreamHandler, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = clt.GetSession(sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &sessionStreamHandler{
		pollPeriod: pollPeriod,
		sessionID:  sessionID,
		ctx:        ctx,
		site:       site,
	}, nil
}

// sessionStreamHandler streams events related to some particular session
// as a stream of JSON encoded event packets
type sessionStreamHandler struct {
	pollPeriod time.Duration
	ctx        *sessionContext
	site       reversetunnel.RemoteSite
	sessionID  string
}

func (w *sessionStreamHandler) Close() error {
	return nil
}

func (w *sessionStreamHandler) stream(ws *websocket.Conn) error {
	clt, err := w.ctx.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}
	var lastCheckpoint time.Time
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
			return trace.Wrap(err)
		}

		servers, err := clt.GetServers()
		if err != nil {
			return trace.Wrap(err)
		}

		sess, err := clt.GetSession(w.sessionID)
		if err != nil {
			return trace.Wrap(err)
		}

		event := sessionStreamEvent{
			Session: *sess,
			Nodes:   servers,
			Events:  events,
		}

		if err := websocket.JSON.Send(ws, event); err != nil {
			return trace.Wrap(err)
		}

		time.Sleep(w.pollPeriod)
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
			log.WithFields(log.Fields{"sid": w.sessionID}).Infof("handler returned: %v", err)
		}
	}
}
