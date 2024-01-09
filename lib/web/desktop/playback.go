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
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	minPlaybackSpeed = 0.25
	maxPlaybackSpeed = 16
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
	PlaybackSpeed float64        `json:"speed,omitempty"`
}

// ReceivePlaybackActions handles logic for receiving playbackAction messages
// over the websocket and updating the player state accordingly.
func ReceivePlaybackActions(
	log logrus.FieldLogger,
	ws *websocket.Conn,
	player *player.Player) {
	// playback always starts in a playing state
	playing := true

	for {
		var action actionMessage

		if err := ws.ReadJSON(&action); err != nil {
			// Connection close errors are expected if the user closes the tab.
			// Only log unexpected errors to avoid cluttering the logs.
			if !utils.IsOKNetworkError(err) {
				log.Warnf("websocket read error: %v", err)
			}
			return
		}

		switch action.Action {
		case actionPlayPause:
			if playing {
				player.Pause()
			} else {
				player.Play()
			}
			playing = !playing
		case actionSpeed:
			action.PlaybackSpeed = max(action.PlaybackSpeed, minPlaybackSpeed)
			action.PlaybackSpeed = min(action.PlaybackSpeed, maxPlaybackSpeed)
			player.SetSpeed(action.PlaybackSpeed)
		default:
			log.Warnf("invalid desktop playback action: %v", action.Action)
			return
		}
	}
}

// PlayRecording feeds recorded events from a player
// over a websocket.
func PlayRecording(
	ctx context.Context,
	log logrus.FieldLogger,
	ws *websocket.Conn,
	player *player.Player) {
	player.Play()
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-player.C():
			if !ok {
				if playerErr := player.Err(); playerErr != nil {
					// Attempt to JSONify the error (escaping any quotes)
					msg, err := json.Marshal(playerErr.Error())
					if err != nil {
						log.Warnf("failed to marshal player error message: %v", err)
						msg = []byte(`"internal server error"`)
					}
					//lint:ignore QF1012 this write needs to happen in a single operation
					bytes := []byte(fmt.Sprintf(`{"message":"error", "errorText":%s}`, string(msg)))
					if err := ws.WriteMessage(websocket.BinaryMessage, bytes); err != nil {
						log.Errorf("failed to write error message: %v", err)
					}
					return
				}
				if err := ws.WriteMessage(websocket.BinaryMessage, []byte(`{"message":"end"}`)); err != nil {
					log.Errorf("failed to write end message: %v", err)
				}
				return
			}

			// some events are part of the stream but not currently
			// needed during playback (session start/end, clipboard use, etc)
			if _, ok := evt.(*events.DesktopRecording); !ok {
				continue
			}
			msg, err := utils.FastMarshal(evt)
			if err != nil {
				log.Errorf("failed to marshal desktop event: %v", err)
				ws.WriteMessage(websocket.BinaryMessage, []byte(`{"message":"error","errorText":"server error"}`))
				return
			}
			if err := ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				// Connection close errors are expected if the user closes the tab.
				// Only log unexpected errors to avoid cluttering the logs.
				if !utils.IsOKNetworkError(err) {
					log.Warnf("websocket write error: %v", err)
				}
				return
			}
		}
	}
}
