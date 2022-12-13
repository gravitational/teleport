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

type signal = chan interface{}

// moveState keeps the state of currect move action
type moveState struct {
	// position is milliseconds that player is seeking
	position int64
	// paused indicates if playback was paused before move
	paused bool
	// initlized tells if move has been initialized
	initialized bool
}

// Player manages the playback of a recorded session.
// It streams events from the audit log to the browser over
// a websocket connection.
type Player struct {
	ws *websocket.Conn

	mu                sync.Mutex
	playState         playbackState
	playSpeed         float32
	eventHandler      eventHandler
	streamer          Streamer
	delayCancelSignal signal
	moveState         moveState
	lastDelay         int64

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
	// handleEvent function should handle received event and optionally return error if events loop needs to be stoped
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
	playStateMove     = playbackState("move")
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

	// actionMove changes the playback position to given value in ms
	actionMove = playbackAction("move")
)

// actionMessage is a message passed from the playback client
// to the server over the websocket connection in order to
// control playback.
type actionMessage struct {
	Action        playbackAction `json:"action"`
	PlaybackSpeed float32        `json:"speed,omitempty"`
	MovePosition  int64          `json:"movePosition,omitempty"`
}

// waitWhilePaused waits idly while the player's state is paused, waiting until:
// - the play state is toggled back to playing
// - the play state is set to finished (the player is closed)
func (pp *Player) waitWhilePaused() {
	for pp.playState == playStatePaused {
		<-pp.delayCancelSignal
	}
}

// togglePlaying toggles the state of the player between playing and paused,
// and wakes up events play goroutine waiting for play state.
func (pp *Player) togglePlaying() {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	switch pp.playState {
	case playStatePlaying:
		pp.playState = playStatePaused
	case playStatePaused:
		pp.playState = playStatePlaying
	}

	pp.cancelDelay()
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
		pp.cancelDelay()
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
			pp.changeActionSpeed(action)
		case actionMove:
			pp.setPlayerToMoveState(action)
		default:
			pp.log.Errorf("received unknown action: %v", action.Action)
			return
		}
	}
}

func (pp *Player) changeActionSpeed(action actionMessage) {
	if action.PlaybackSpeed < minPlaybackSpeed {
		action.PlaybackSpeed = minPlaybackSpeed
	} else if action.PlaybackSpeed > maxPlaybackSpeed {
		action.PlaybackSpeed = maxPlaybackSpeed
	}

	pp.doLocked(func() {
		pp.playSpeed = action.PlaybackSpeed
	})
}

func (pp *Player) setPlayerToMoveState(action actionMessage) {
	paused := pp.playState == playStatePaused

	pp.cancelDelay()

	pp.doLocked(func() {
		pp.moveState = moveState{
			position: action.MovePosition,
			paused:   paused,
		}
		pp.playState = playStateMove
	})
}

// doLocked is just a short hand for doing things inside Mutex Lock/Unlock block
func (pp *Player) doLocked(action func()) {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	action()
}

// marshalAndSend handles all the logic necessary for fast marshaling value and sending it to client via websocket.
func (pp *Player) marshalAndSend(e interface{}) error {
	msg, err := utils.FastMarshal(e)
	if err != nil {
		pp.log.WithError(err).Errorf("failed to marshal %T event into JSON: %v", e, e)
		if _, err := pp.ws.Write([]byte(`{"message":"error","errorText":"server error"}`)); err != nil {
			pp.log.WithError(err).Error("failed to write \"error\" message over websocket")
		}
		return err
	}

	if err := pp.send(msg); err != nil {
		// We expect net.ErrClosed to arise when another goroutine returns before
		// this one or the browser window is closed, both of which cause the websocket to close.
		if !errors.Is(err, net.ErrClosed) {
			pp.log.WithError(err).Error("failed to write %T: %v over websocket", e, e)
		}

		return err
	}

	return nil
}

func (pp *Player) send(msg []byte) error {
	if _, err := pp.ws.Write(msg); err != nil {
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

	pp.lastDelay = 0

	eventsC, errC := pp.streamer.StreamSessionEvents(ctx, session.ID(pp.sID), 0)
	for {
		pp.waitWhilePaused()

		pp.mu.Lock()
		if !pp.moveState.initialized && pp.playState == playStateMove {
			pp.moveState.initialized = true

			// If move position is smaller than last know position then restart events stream
			if pp.moveState.position <= pp.lastDelay {
				eventsC, errC = pp.streamer.StreamSessionEvents(ctx, session.ID(pp.sID), 0)

				if err := pp.send([]byte(`{"event": "reset"}`)); err != nil {
					return
				}
			}
		}
		pp.mu.Unlock()

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
				pp.send([]byte(fmt.Sprintf(`{"message": "error", "errorText": "%v"}`, errorText)))
			}
			return
		case evt := <-eventsC:
			if evt == nil {
				pp.log.Debug("reached end of playback")
				pp.send([]byte(`{"message":"end"}`))

				return
			}

			payload := eventHandlerPayload{
				pp:        pp,
				lastDelay: &pp.lastDelay,
				cancel:    cancel,
				event:     evt,
			}

			if err := pp.eventHandler.handleEvent(ctx, payload); err != nil {
				return
			}
		}
	}
}

// handleMoveState does check if player is in move state and does all neccessary steps to correctly handle it.
// It returns boolean which indicates whenever delay for event should be skipped because of fast-forwarding.
func (pp *Player) handleMoveState(delayMilliseconds int64) bool {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	moveActive := pp.playState == playStateMove
	shouldSkip := moveActive && pp.moveState.position >= delayMilliseconds
	isFirstEventAfterMove := moveActive && pp.moveState.position < delayMilliseconds

	if shouldSkip {
		pp.lastDelay = delayMilliseconds
	}

	if isFirstEventAfterMove {
		pp.lastDelay = pp.moveState.position

		if pp.moveState.paused {
			pp.playState = playStatePaused
		} else {
			pp.playState = playStatePlaying
		}

		pp.send([]byte(fmt.Sprintf(`{"event": "move", "position": %v}`, pp.moveState.position)))
	}

	return shouldSkip
}

// waitForDelay pauses sending the event until given delay is met takes account for pause during delay.
// It scales delay by playback speed.
func (pp *Player) waitForDelay(delayMilliseconds int64) {
	startTime := time.Now()
	for {
		// In case player is seeking position skip delay
		if pp.handleMoveState(delayMilliseconds) {
			return
		}

		pp.waitWhilePaused()

		if pp.playState == playStateMove {
			return
		}

		duration := time.Duration(pp.scaleDelay(delayMilliseconds-pp.lastDelay)) * time.Millisecond

		select {
		case <-time.After(duration):
			pp.lastDelay = delayMilliseconds
			return
		case <-pp.delayCancelSignal:
			sleepDuration := pp.scaleDelay(time.Now().Local().Sub(startTime).Milliseconds())

			if pp.isResetMove(delayMilliseconds) {
				return
			}

			pp.lastDelay += sleepDuration
		}
	}
}

func (pp *Player) cancelDelay() {
	pp.delayCancelSignal <- true
}

func (pp *Player) isResetMove(delayMilliseconds int64) bool {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	return pp.playState == playStateMove && pp.moveState.position < delayMilliseconds
}
