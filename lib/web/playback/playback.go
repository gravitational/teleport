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

package playback

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

const (
	minPlaybackSpeed = 0.25
	maxPlaybackSpeed = 16
)

type delayCancelSignal = chan interface{}

// Player manages the playback of a recorded session.
// It streams events from the audit log to the browser over
// a websocket connection.
type Player struct {
	ws *websocket.Conn

	mu                sync.Mutex
	cond              *sync.Cond
	playState         playbackState
	playSpeed         float32
	eventHandler      eventHandler
	streamer          Streamer
	delayCancelSignal delayCancelSignal

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

// eventHandlerPayload has all necessary data to correctly handle event
type eventHandlerPayload struct {
	cancel    context.CancelFunc
	pp        *Player
	event     apievents.AuditEvent
	lastDelay *int64
}

// eventHandler is the interface that provides specific event handling for concreate player
type eventHandler interface {
	handleEvent(ctx context.Context, payload eventHandlerPayload) error
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

	// actionSpeed sets the playback speed
	actionSpeed = playbackAction("speed")
)

// actionMessage is a message passed from the playback client
// to the server over the websocket connection in order to
// control playback.
type actionMessage struct {
	Action        playbackAction `json:"action"`
	PlaybackSpeed float32        `json:"speed,omitempty"`
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
		pp.delayCancelSignal <- playStatePaused
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
		case actionSpeed:
			if action.PlaybackSpeed < minPlaybackSpeed {
				action.PlaybackSpeed = minPlaybackSpeed
			} else if action.PlaybackSpeed > maxPlaybackSpeed {
				action.PlaybackSpeed = maxPlaybackSpeed
			}

			pp.mu.Lock()
			pp.playSpeed = action.PlaybackSpeed
			pp.mu.Unlock()
		default:
			pp.log.Errorf("received unknown action: %v", action.Action)
			return
		}
	}
}

func (pp *Player) marshalAndSendEvent(e interface{}) error {
	msg, err := utils.FastMarshal(e)
	if err != nil {
		pp.log.WithError(err).Errorf("failed to marshal %T event into JSON: %v", e, e)
		if _, err := pp.ws.Write([]byte(`{"message":"error","errorText":"server error"}`)); err != nil {
			pp.log.WithError(err).Error("failed to write \"error\" message over websocket")
		}
		return err
	}

	if _, err := pp.ws.Write(msg); err != nil {
		// We expect net.ErrClosed to arise when another goroutine returns before
		// this one or the browser window is closed, both of which cause the websocket to close.
		if !errors.Is(err, net.ErrClosed) {
			pp.log.WithError(err).Error("failed to write %T event over websocket", e)
		}

		return err
	}

	return nil
}

func (pp *Player) scaleDelay(delay int64) int64 {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	return int64(float32(delay) / pp.playSpeed)
}

func (pp *Player) streamSessionEvents(ctx context.Context, cancel context.CancelFunc) {
	defer pp.log.Debug("playbackPlayer.StreamSessionEvents returned")
	defer pp.close(cancel)

	var lastDelay int64

	eventsC, errC := pp.streamer.StreamSessionEvents(ctx, session.ID(pp.sID), 0)
	for {
		pp.waitWhilePaused()

		select {
		case err := <-errC:
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

			payload := eventHandlerPayload{
				pp:        pp,
				lastDelay: &lastDelay,
				cancel:    cancel,
				event:     evt,
			}

			if err := pp.eventHandler.handleEvent(ctx, payload); err != nil {
				return
			}
		}
	}
}

// waitForDelay pauses sending the event until given delay is met takes account for pause during delay.
// It scales delay by playback speed.
func (pp *Player) waitForDelay(delayMilliseconds int64, lastDelay *int64) {
	startTime := time.Now()
	for {
		pp.waitWhilePaused()
		duration := time.Duration(pp.scaleDelay(delayMilliseconds-*lastDelay)) * time.Millisecond

		select {
		case <-time.After(duration):
			*lastDelay = delayMilliseconds
			return
		case <-pp.delayCancelSignal:
			sleepDuration := pp.scaleDelay(time.Now().Local().Sub(startTime).Milliseconds())
			*lastDelay += sleepDuration
		}
	}
}
