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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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

// sessionStreamPollPeriod defines how frequently web sessions are sent new
// events.
var sessionStreamPollPeriod = 5 * time.Second

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

	// Ticker used to send list of servers to web client periodically.
	tickerCh := time.NewTicker(w.pollPeriod)
	defer tickerCh.Stop()

	defer w.Close()

	c, err := w.ctx.getTerminal(w.sessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	for {
		select {
		// Push the new window size to the web client.
		case wc := <-c.teleportClient.WindowChangeRequests():
			win := wc.TerminalParams.Winsize()

			// Convert the window change request into something that looks like an
			// Audit Event for compatibility and send over the websocket.
			streamEvents := &sessionStreamEvent{
				Events: []events.EventFields{
					events.EventFields{
						events.EventType:      events.ResizeEvent,
						events.SessionEventID: wc.SessionID,
						events.EventTime:      wc.Time.String(),
						events.TerminalSize:   fmt.Sprintf("%d:%d", win.Width, win.Height),
					},
				},
			}
			log.Debugf("Sending window change %v for %v to web client.", win, wc.SessionID)

			err = websocket.JSON.Send(ws, streamEvents)
			if err != nil {
				log.Errorf("Unable to send window change event over websocket: %v", err)
				continue
			}
		// Periodically send list of servers to the web client.
		case <-tickerCh.C:
			servers, err := clt.GetNodes(w.namespace)
			if err != nil {
				log.Errorf("Unable to fetch list of nodes: %v.", err)
				continue
			}

			// Send list of server to the web client.
			streamEvents := &sessionStreamEvent{
				Servers: services.ServersToV1(servers),
			}
			log.Debugf("Sending server list (%v) to web client.", len(servers))

			err = websocket.JSON.Send(ws, streamEvents)
			if err != nil {
				log.Errorf("Unable to send server list event to web client: %v.", err)
				continue
			}
		// Stream is closing.
		case <-w.closeC:
			log.Infof("Exiting event stream to web client.")
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
