/*
Copyright 2021 Gravitational, Inc.

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
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

const actionToggle = "toggle"

// playbackAction is a message passed from the playback client
// to the server over the websocket connection in order to modify
// the playback state.
type playbackAction struct {
	// Action is one of actionToggle | TODO: actionMove
	// actionToggle toggles the playbackState.playing
	Action string `json:"action"`
}

// playbackState is a thread-safe struct for managing
// the global state of the playback websocket connection.
type playbackState struct {
	ws        *websocket.Conn
	playing   bool // false means the playback is paused
	wsOpen    bool
	mu        sync.RWMutex
	log       logrus.FieldLogger
	closeOnce sync.Once
}

// GetPlaying returns ps.playing
func (ps *playbackState) GetPlaying() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.playing
}

// TogglePlaying toggles the boolean state of ps.playing
func (ps *playbackState) TogglePlaying() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.playing = !ps.playing
}

// GetWsOpen gets the wsOpen value. It should be called at the top of continuously
// looping goroutines that use playbackState, who should return if it returns false.
func (ps *playbackState) GetWsOpen() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.wsOpen
}

// Close closes the websocket connection and sets wsOpen to false.
// It should be deferred by all the goroutines that use playbackState,
// in order to ensure that when one goroutine closes, all the others do too
// (when used in conjunction with GetWsOpen).
func (ps *playbackState) Close() {
	ps.closeOnce.Do(func() {
		err := ps.ws.Close()
		if err != nil {
			ps.log.WithError(err).Errorf("websocket.Close() failed")
		}
		ps.mu.Lock()
		defer ps.mu.Unlock()
		ps.wsOpen = false
	})
}

func (h *Handler) desktopPlaybackHandle(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	ctx *SessionContext,
) (interface{}, error) {
	sID := p.ByName("sid")
	if sID == "" {
		return nil, trace.BadParameter("missing sid in request URL")
	}

	websocket.Handler(func(ws *websocket.Conn) {
		defer h.log.Debug("playback websocket closed")
		rCtx := r.Context()
		ws.PayloadType = websocket.BinaryFrame
		ps := playbackState{
			ws:      ws,
			wsOpen:  true,
			playing: true, // system always starts in a playing state
		}
		defer ps.Close()

		// Handle incoming playback actions.
		go func() {
			defer h.log.Debug("playback action-recieving goroutine returned")
			defer ps.Close()

			for {
				if !ps.GetWsOpen() {
					return
				}

				action := playbackAction{}
				err := websocket.JSON.Receive(ws, &action)
				if err != nil {
					if errors.Is(err, io.EOF) {
						// io.EOF is only a warning, as we expect it to be returned if the websocket is
						// closed by another goroutine while websocket.JSON.Receive() is hanging.
						h.log.WithError(err).Warn("error reading from websocket")
					} else {
						h.log.WithError(err).Error("error reading from websocket")
					}
					return
				}
				h.log.Debugf("recieved playback action: %+v", action)
				if action.Action == actionToggle {
					ps.TogglePlaying()
				} else {
					h.log.Errorf("received unknown action: %v", action.Action)
					return
				}
			}
		}()

		// Stream session events back to browser.
		go func() {
			defer h.log.Debug("playback event-streaming goroutine returned")
			defer ps.Close()

			var lastDelay int64
			eventsC, errC := ctx.clt.StreamSessionEvents(rCtx, session.ID(sID), 0)
			for {
				if !ps.GetWsOpen() {
					return
				}

				if !ps.GetPlaying() {
					continue
				}

				select {
				case err := <-errC:
					h.log.WithError(err).Errorf("streaming session %v", sID)
					return
				case evt := <-eventsC:
					if evt == nil {
						h.log.Debug("reached end of playback")
						return
					}
					switch e := evt.(type) {
					case *apievents.DesktopRecording:
						if e.DelayMilliseconds > lastDelay {
							time.Sleep(time.Duration(e.DelayMilliseconds-lastDelay) * time.Millisecond)
							lastDelay = e.DelayMilliseconds
						}
						if _, err := ws.Write(e.Message); err != nil {
							if errors.Is(err, net.ErrClosed) {
								// net.ErrClosed is only a warning, as we expect this error to arise
								// in the case where another goroutine returns before this one, causing the
								// websocket to close.
								h.log.WithError(err).Warn("failed to write TDP message over websocket")
							} else {
								h.log.WithError(err).Error("failed to write TDP message over websocket")
							}
							return
						}
					default:
						h.log.Warnf("session %v contains unexpected event type %T", sID, evt)
					}
				default:
					continue
				}
			}
		}()

		// Hang until the request context is cancelled or the websocket is closed by another goroutine.
		for {
			// Return if websocket is closed by another goroutine.
			if !ps.GetWsOpen() {
				return
			}

			// Return if the request context is cancelled.
			select {
			case <-rCtx.Done():
				return
			default:
				continue
			}
		}

	}).ServeHTTP(w, r)
	return nil, nil
}
