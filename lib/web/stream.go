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

	"golang.org/x/net/websocket"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

func newSessionStreamHandler(namespace string, sessionID session.ID, ctx *SessionContext, site reversetunnel.RemoteSite) (*sessionStreamHandler, error) {
	return &sessionStreamHandler{
		sessionID: sessionID,
		ctx:       ctx,
		site:      site,
		closeC:    make(chan bool),
		namespace: namespace,
	}, nil
}

// sessionStreamHandler streams events related to some particular session
// as a stream of JSON encoded event packets
type sessionStreamHandler struct {
	closeOnce sync.Once
	ctx       *SessionContext
	site      reversetunnel.RemoteSite
	namespace string
	sessionID session.ID
	closeC    chan bool
	ws        *websocket.Conn
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

func (w *sessionStreamHandler) pollEvents(authClient auth.ClientI, cursor int) ([]events.EventFields, int, error) {
	// Poll for events since the last call (cursor location).
	sessionEvents, err := authClient.GetSessionEvents(w.namespace, w.sessionID, cursor+1, false)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, 0, trace.Wrap(err)
		}
		return nil, 0, trace.NotFound("no events from cursor: %v", cursor)
	}

	// Get the batch size to see if any events were returned.
	batchLen := len(sessionEvents)
	if batchLen == 0 {
		return nil, 0, trace.NotFound("no events from cursor: %v", cursor)
	}

	// Advance the cursor.
	newCursor := sessionEvents[batchLen-1].GetInt(events.EventCursor)

	// Filter out any resize events as we get them over push notifications.
	var filteredEvents []events.EventFields
	for _, event := range sessionEvents {
		if event.GetType() == events.ResizeEvent ||
			event.GetType() == events.SessionJoinEvent {
			continue
		}
		filteredEvents = append(filteredEvents, event)
	}

	return filteredEvents, newCursor, nil
}

// stream returns a stream of events that occur during this session.
func (w *sessionStreamHandler) stream(ws *websocket.Conn) error {
	w.ws = ws

	// Spin up a goroutine to detect closed socket by reading from it.
	go func() {
		defer w.Close()
		io.Copy(ioutil.Discard, ws)
	}()
	defer w.Close()

	// Extract a Teleport client from the terminal.
	c, err := w.ctx.getTerminal(w.sessionID)
	if err != nil {
		return trace.Wrap(err)
	}
	teleportClient := c.teleportClient

	// Get access to Auth Client to fetch sessions from the backend. An Auth
	// Client and a cursor are used to keep track of where we are in the event
	// stream. This is to find "session.end" events.
	authClient, err := w.site.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}
	var cursor int = -1

	tickerCh := time.NewTicker(defaults.SessionRefreshPeriod)
	defer tickerCh.Stop()

	for {
		select {
		// Send push events that come over the events channel to the web client.
		case event := <-teleportClient.EventsChannel():
			streamEvents := &sessionStreamEvent{
				Events: []events.EventFields{event},
			}
			log.Debugf("Sending %v event to web client.", event.GetType())

			err = websocket.JSON.Send(ws, streamEvents)
			if err != nil {
				log.Errorf("Unable to %v event to web client: %v.", event.GetType(), err)
				continue
			}
		// Poll for events to send to the web client. This is for events that can
		// not be sent over the events channel (like "session.end" which lingers for
		// a while after all party members have left).
		case <-tickerCh.C:
			// Fetch all session events from the backend.
			sessionEvents, cur, err := w.pollEvents(authClient, cursor)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.Errorf("Unable to poll for events: %v.", err)
					continue
				}
				continue
			}

			// Update the cursor location.
			cursor = cur

			// Send all events to the web client.
			err = websocket.JSON.Send(ws, &sessionStreamEvent{
				Events: sessionEvents,
			})
			if err != nil {
				log.Warnf("Unable to send %v events to web client: %v.", len(sessionEvents), err)
				continue
			}
		// Close the stream, the session is over.
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
