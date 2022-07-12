/*
Copyright 2022 Gravitational, Inc.

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

package desktop

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

// Player manages the playback of a recorded desktop session.
// It streams events from the audit log to the browser over
// a websocket connection.
type Player struct {
	ws       *websocket.Conn
	streamer Streamer

	mu        sync.Mutex
	cond      *sync.Cond
	playState playbackState

	log logrus.FieldLogger
	sID string

	closeOnce sync.Once
}

// Streamer is the interface that can provide with a stream of events related to
// a particular session.
type Streamer interface {
	// StreamSessionEvents streams all events from a given session recording. An error is returned on the first
	// channel if one is encountered. Otherwise the event channel is closed when the stream ends.
	// The event channel is not closed on error to prevent race conditions in downstream select statements.
	StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error)
}

// NewPlayer creates a player that streams a desktop session
// over the provided websocket connection.
func NewPlayer(sID string, ws *websocket.Conn, streamer Streamer, log logrus.FieldLogger) *Player {
	p := &Player{
		ws:        ws,
		streamer:  streamer,
		playState: playStatePlaying,
		log:       log,
		sID:       sID,
	}
	p.cond = sync.NewCond(&p.mu)
	return p
}

// Play kicks off goroutines for receiving actions
// and playing back the session over the websocket,
// and then waits for the stream to complete.
func (pp *Player) Play(ctx context.Context) {
	defer pp.log.Debug("playbackPlayer.Play returned")

	pp.ws.PayloadType = websocket.BinaryFrame
	ppCtx, cancel := context.WithCancel(ctx)
	defer pp.close(cancel)

	go pp.receiveActions(cancel)
	go pp.streamSessionEvents(ppCtx, cancel)

	// Wait until the ctx is canceled, either by
	// one of the goroutines above or by the http handler.
	<-ppCtx.Done()
}

type playbackState string

const (
	playStatePlaying  = playbackState("playing")
	playStatePaused   = playbackState("paused")
	playStateFinished = playbackState("finished")
)

// playbackAction identifies a command sent from the
// browser to control playback
type playbackAction string

const (
	// actionPlayPause toggles the playback state
	// between playing and paused
	actionPlayPause = playbackAction("play/pause")
)

// actionMessage is a message passed from the playback client
// to the server over the websocket connection in order to modify
// the playback state.
type actionMessage struct {
	// actionPlayPause toggles the playbackState.playState
	Action playbackAction `json:"action"`
}

// waitWhilePaused waits idly while the player's state is paused, waiting until:
// - the play state is toggled back to playing
// - the play state is set to finished (the player is closed)
func (pp *Player) waitWhilePaused() {
	pp.cond.L.Lock()
	defer pp.cond.L.Unlock()

	for pp.playState == playStatePaused {
		pp.cond.Wait()
	}
}

// togglePlaying toggles the state of the player between playing and paused,
// and wakes up any goroutines waiting in waitWhilePaused.
func (pp *Player) togglePlaying() {
	pp.cond.L.Lock()
	defer pp.cond.L.Unlock()
	switch pp.playState {
	case playStatePlaying:
		pp.playState = playStatePaused
	case playStatePaused:
		pp.playState = playStatePlaying
	}
	pp.cond.Broadcast()
}

// close closes the websocket connection, wakes up any goroutines waiting on the playState condition,
// and cancels the playbackPlayer's context.
//
// It should be deferred by all the goroutines that use playbackPlayer,
// in order to ensure that when one goroutine closes, all the others do too.
func (pp *Player) close(cancel context.CancelFunc) {
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

// receiveActions handles logic for receiving playbackAction jsons
// over the websocket and modifying playbackPlayer's state accordingly.
func (pp *Player) receiveActions(cancel context.CancelFunc) {
	defer pp.log.Debug("playbackPlayer.ReceiveActions returned")
	defer pp.close(cancel)

	for {
		var action actionMessage
		if err := websocket.JSON.Receive(pp.ws, &action); err != nil {
			// We expect net.ErrClosed if the websocket is closed by another
			// goroutine and io.EOF if the websocket is closed by the browser
			// while websocket.JSON.Receive() is hanging.
			if !utils.IsOKNetworkError(err) {
				pp.log.WithError(err).Error("error reading from websocket")
			}
			return
		}
		pp.log.Debugf("received playback action: %+v", action)
		switch action.Action {
		case actionPlayPause:
			pp.togglePlaying()
		default:
			pp.log.Errorf("received unknown action: %v", action.Action)
			return
		}
	}
}

// streamSessionEvents streams the session's events as playback events over the websocket.
func (pp *Player) streamSessionEvents(ctx context.Context, cancel context.CancelFunc) {
	defer pp.log.Debug("playbackPlayer.StreamSessionEvents returned")
	defer pp.close(cancel)

	var lastDelay int64
	eventsC, errC := pp.streamer.StreamSessionEvents(ctx, session.ID(pp.sID), 0)
	for {
		pp.waitWhilePaused()

		select {
		case err := <-errC:
			// TODO(zmb3, isaiah): send some sort of error to the browser,
			// otherwise it just sits at the player UI
			if err != nil && !errors.Is(err, context.Canceled) {
				pp.log.WithError(err).Errorf("streaming session %v", pp.sID)
				var errorText string
				if os.IsNotExist(err) || trace.IsNotFound(err) {
					errorText = "session not found"
				} else {
					errorText = "server error"
				}
				if _, err := pp.ws.Write([]byte(fmt.Sprintf(`{"message": "error", "errorText": "%v"}`, errorText))); err != nil {
					pp.log.WithError(err).Error("failed to write \"error\" message over websocket")
				}
			}
			return
		case evt := <-eventsC:
			if evt == nil {
				pp.log.Debug("reached end of playback")
				if _, err := pp.ws.Write([]byte(`{"message":"end"}`)); err != nil {
					pp.log.WithError(err).Error("failed to write \"end\" message over websocket")
				}
				return
			}
			switch e := evt.(type) {
			case *apievents.DesktopRecording:
				if e.DelayMilliseconds > lastDelay {
					// TODO(zmb3): replace with time.After so we can cancel
					time.Sleep(time.Duration(e.DelayMilliseconds-lastDelay) * time.Millisecond)
					lastDelay = e.DelayMilliseconds
				}
				msg, err := utils.FastMarshal(e)
				if err != nil {
					pp.log.WithError(err).Errorf("failed to marshal DesktopRecording event into JSON: %v", e)
					if _, err := pp.ws.Write([]byte(`{"message":"error","errorText":"server error"}`)); err != nil {
						pp.log.WithError(err).Error("failed to write \"error\" message over websocket")
					}
					return
				}
				if _, err := pp.ws.Write(msg); err != nil {
					// We expect net.ErrClosed to arise when another goroutine returns before
					// this one or the browser window is closed, both of which cause the websocket to close.
					if !errors.Is(err, net.ErrClosed) {
						pp.log.WithError(err).Error("failed to write DesktopRecording event over websocket")
					}
					return
				}
			case *apievents.WindowsDesktopSessionStart, *apievents.WindowsDesktopSessionEnd:
				// these events are part of the stream but never needed for playback
			case *apievents.DesktopClipboardReceive, *apievents.DesktopClipboardSend:
				// these events are not currently needed for playback,
				// but may be useful in the future

			default:
				pp.log.Warnf("session %v contains unexpected event type %T", pp.sID, evt)
			}
		}
	}
}
