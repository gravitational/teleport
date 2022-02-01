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
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

const actionPlayPause = "play/pause"

const (
	playStatePlaying  = "playing"
	playStatePaused   = "paused"
	playStateFinished = "finished"
)

// playbackAction is a message passed from the playback client
// to the server over the websocket connection in order to modify
// the playback state.
type playbackAction struct {
	// Action is one of actionPlayPause | TODO: actionMove
	// actionPlayPause toggles the playbackState.playState
	Action string `json:"action"`
}

// playbackPlayer is a thread-safe struct for managing
// the global state of the playback websocket connection.
type playbackPlayer struct {
	ws        *websocket.Conn
	playState string // one of playStatePlaying | playStatePaused | playStateFinished
	cond      *sync.Cond
	mu        sync.Mutex
	log       logrus.FieldLogger
	closeOnce sync.Once
	clt       *auth.Client
	sID       string
}

func newPlaybackPlayer(sID string, ws *websocket.Conn, clt *auth.Client, log logrus.FieldLogger) *playbackPlayer {
	ws.PayloadType = websocket.BinaryFrame

	pp := &playbackPlayer{
		ws:        ws,
		playState: playStatePlaying,
		log:       log,
		clt:       clt,
		sID:       sID,
	}
	pp.cond = sync.NewCond(&pp.mu)

	return pp
}

// waitWhilePaused waits while pp.playState = playStatePaused. When waitWhilePaused is called in one goroutine,
// it will only be triggered to check its condition again by another goroutine calling pp.togglePlaying() or pp.close()
// (because those functions call pp.cond.Broadcast()).
func (pp *playbackPlayer) waitWhilePaused() {
	pp.cond.L.Lock()
	defer pp.cond.L.Unlock()

	for pp.playState == playStatePaused {
		pp.cond.Wait()
	}
}

// togglePlaying toggles the state of pp.playState between
// playStatePlaying and playStatePaused.
func (pp *playbackPlayer) togglePlaying() {
	pp.cond.L.Lock()
	defer pp.cond.L.Unlock()
	if pp.playState == playStatePlaying {
		pp.playState = playStatePaused
	} else if pp.playState == playStatePaused {
		pp.playState = playStatePlaying
	}
	pp.cond.Broadcast()
}

// close closes the websocket connection, wakes up any goroutines waiting on the playState condition,
// and cancels the playbackPlayer's context.
//
// It should be deferred by all the goroutines that use playbackPlayer,
// in order to ensure that when one goroutine closes, all the others do too.
func (pp *playbackPlayer) close(cancel context.CancelFunc) {
	pp.closeOnce.Do(func() {
		pp.mu.Lock()
		defer pp.mu.Unlock()

		err := pp.ws.Close()
		if err != nil {
			pp.log.WithError(err).Errorf("websocket.Close() failed")
		}

		pp.playState = playStateFinished
		pp.cond.Broadcast()
		cancel()
	})
}

// receiveActions handles logic for recieving playbackAction jsons
// over the websocket and modifying playbackPlayer's state accordingly.
func (pp *playbackPlayer) receiveActions(cancel context.CancelFunc) {
	defer pp.log.Debug("playbackPlayer.ReceiveActions returned")
	defer pp.close(cancel)

	for {
		action := playbackAction{}
		// Hangs until there is data to be received, or until an error.
		err := websocket.JSON.Receive(pp.ws, &action)
		if err != nil {
			// We expect net.ErrClosed if the websocket is closed by another
			// goroutine and io.EOF if the websocket is closed by the browser
			// while websocket.JSON.Receive() is hanging.
			if !(errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF)) {
				pp.log.WithError(err).Error("error reading from websocket")
			}
			return
		}
		pp.log.Debugf("recieved playback action: %+v", action)
		if action.Action == actionPlayPause {
			pp.togglePlaying()
		} else {
			pp.log.Errorf("received unknown action: %v", action.Action)
			return
		}
	}
}

// streamSessionEvents streams the session's events as playback events over the websocket.
func (pp *playbackPlayer) streamSessionEvents(ctx context.Context, cancel context.CancelFunc) {
	defer pp.log.Debug("playbackPlayer.StreamSessionEvents returned")
	defer pp.close(cancel)

	var lastDelay int64
	eventsC, errC := pp.clt.StreamSessionEvents(ctx, session.ID(pp.sID), 0)
	for {
		pp.waitWhilePaused()

		select {
		case err := <-errC:
			if !errors.Is(err, context.Canceled) {
				pp.log.WithError(err).Errorf("streaming session %v", pp.sID)
			}
			return
		case evt := <-eventsC:
			if evt == nil {
				pp.log.Debug("reached end of playback")
				// deferred ps.Close() will set ps.playState = playStateFinished for us
				return
			}
			switch e := evt.(type) {
			case *apievents.DesktopRecording:
				if e.DelayMilliseconds > lastDelay {
					time.Sleep(time.Duration(e.DelayMilliseconds-lastDelay) * time.Millisecond)
					lastDelay = e.DelayMilliseconds
				}
				msg, err := utils.FastMarshal(e)
				if err != nil {
					pp.log.WithError(err).Errorf("failed to marshal DesktopRecording event into JSON: %v", msg)
				}
				if _, err := pp.ws.Write(msg); err != nil {
					// We expect net.ErrClosed to arise when another goroutine returns before
					// this one or the browser window is closed, both of which cause the websocket to close.
					if !errors.Is(err, net.ErrClosed) {
						pp.log.WithError(err).Error("failed to write TDP message over websocket")
					}
					return
				}
			default:
				pp.log.Warnf("session %v contains unexpected event type %T", pp.sID, evt)
			}
		}
	}
}

// Play kicks off goroutines for receiving actions
// and playing back the session over the websocket,
// and then hangs until the websocket is closed.
func (pp *playbackPlayer) Play(ctx context.Context) {
	defer pp.log.Debug("playbackPlayer.Play returned")
	ppCtx, cancel := context.WithCancel(ctx)
	defer pp.close(cancel)

	go pp.receiveActions(cancel)
	go pp.streamSessionEvents(ppCtx, cancel)

	// Wait until the ctx is cancelled, either by
	// one of the goroutines above or by the http handler.
	<-ppCtx.Done()
	return
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
		defer h.log.Debug("desktopPlaybackHandle websocket handler returned")
		pp := newPlaybackPlayer(sID, ws, ctx.clt, h.log)
		pp.Play(r.Context())
	}).ServeHTTP(w, r)
	return nil, nil
}
