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
	cond      *sync.Cond
	wsOpen    bool
	mu        *sync.RWMutex
	log       logrus.FieldLogger
	closeOnce sync.Once
}

func newPlaybackState(ws *websocket.Conn, playing bool, wsOpen bool, log logrus.FieldLogger) playbackState {
	var mu sync.RWMutex
	cond := sync.NewCond(&mu)
	return playbackState{
		ws:      ws,
		playing: playing,
		cond:    cond,
		wsOpen:  wsOpen,
		mu:      &mu,
		log:     log,
	}
}

// HangIfPlaybackPausedAndWsOpen hangs while the playback state is paused and the websocket remains open,
// and returns the state of the websocket connection once either of those conditions are no longer met
// (true if the websocket is open and playback state is unpaused, false is websocket is closed).
// After it's first called in one goroutine, it can only be triggered to check the conditions again upon
// another goroutine calling ps.TogglePlaying() or ps.Close() (because those functions call ps.cond.Broadcast()).
func (ps *playbackState) HangIfPlaybackPausedAndWsOpen() bool {
	ps.cond.L.Lock()
	defer ps.cond.L.Unlock()
	for !ps.playing && ps.wsOpen {
		ps.cond.Wait()
	}
	return ps.wsOpen
}

// IsPlaying returns ps.playing
func (ps *playbackState) IsPlaying() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.playing
}

// TogglePlaying toggles the boolean state of ps.playing
func (ps *playbackState) TogglePlaying() {
	ps.cond.L.Lock()
	defer ps.cond.L.Unlock()
	ps.playing = !ps.playing
	ps.cond.Broadcast()
}

// IsWsOpen gets the wsOpen value. It should be called at the top of continuously
// looping goroutines that use playbackState, who should return if it returns false.
func (ps *playbackState) IsWsOpen() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.wsOpen
}

// Close closes the websocket connection and sets IsWsOpen() to false, and wakes
// up any goroutines that were hanging on HangIfPaused().
// It should be deferred by all the goroutines that use playbackState,
// in order to ensure that when one goroutine closes, all the others do too
// (when used in conjunction with IsWsOpen).
func (ps *playbackState) Close() {
	ps.closeOnce.Do(func() {
		err := ps.ws.Close()
		if err != nil {
			ps.log.WithError(err).Errorf("websocket.Close() failed")
		}

		ps.mu.Lock()
		defer ps.mu.Unlock()
		ps.wsOpen = false

		ps.cond.Broadcast()
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
		ps := newPlaybackState(ws, true, true, h.log)
		defer ps.Close()

		// Handle incoming playback actions.
		go func() {
			defer h.log.Debug("playback action-recieving goroutine returned")
			defer ps.Close()

			for {
				action := playbackAction{}
				// Hangs until there is data to be received,
				// or returns io.EOF if websocket connection is closed.
				err := websocket.JSON.Receive(ws, &action)
				if err != nil {
					if errors.Is(err, io.EOF) {
						// Only a warning as we expect this case if the websocket is
						// closed by another goroutine or by the browser while
						// websocket.JSON.Receive() is hanging.
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
				if ps.HangIfPlaybackPausedAndWsOpen() == false {
					// Websocket closed by browser or another goroutine.
					return
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
								// Only a warning as we expect this case to arise when another
								// goroutine returns before this one or the browser window is closed,
								// both of which cause the websocket to close.
								h.log.WithError(err).Warn("failed to write TDP message over websocket")
							} else {
								h.log.WithError(err).Error("failed to write TDP message over websocket")
							}
							return
						}
					default:
						h.log.Warnf("session %v contains unexpected event type %T", sID, evt)
					}
				}
			}
		}()

		// Hang until the request context is cancelled or the websocket is closed by another goroutine.
		for {
			// Return if websocket is closed by another goroutine.
			if !ps.IsWsOpen() {
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
