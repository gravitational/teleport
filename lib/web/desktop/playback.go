/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/events"
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

	// actionSeek moves to a different position in the recording
	actionSeek = playbackAction("seek")
)

// UnmarshalJSON implements custom json unmarshalling for playbackAction.
// It ensures that actions conform to the expected set of values.
func (a *playbackAction) UnmarshalJSON(data []byte) error {
	var action string
	if err := json.Unmarshal(data, &action); err != nil {
		return err
	}

	switch action {
	case string(actionPlayPause), string(actionSpeed), string(actionSeek):
		*a = playbackAction(action)
		return nil
	default:
		return trace.BadParameter("invalid desktop action")
	}
}

type playbackSpeed float64

// UnmarshalJSON implements custom json unmarshalling for playbackSpeed.
// It coerces the received value into the range [minPlaybackSpeed, maxPlaybackSpeed].
func (p *playbackSpeed) UnmarshalJSON(data []byte) error {
	var speed float64
	if err := json.Unmarshal(data, &speed); err != nil {
		return err
	}

	speed = max(speed, minPlaybackSpeed)
	speed = min(speed, maxPlaybackSpeed)
	*p = playbackSpeed(speed)
	return nil
}

// actionMessage is a message passed from the playback client
// to the server over the websocket connection in order to
// control playback.
type actionMessage struct {
	Action        playbackAction `json:"action"`
	PlaybackSpeed playbackSpeed  `json:"speed,omitempty"`
	Pos           int64          `json:"pos"`
}

// JSONReader is used to read JSON messages containing
// commands for the session player.
type JSONReader interface {
	ReadJSON(v any) error
}

// PlayerController models the session player.
type PlayerController interface {
	SetPos(time.Duration) error
	SetSpeed(float64) error
	Pause() error
	Play() error
}

// ReceivePlaybackActions handles logic for receiving playbackAction messages
// over the websocket and updating the player state accordingly.
func ReceivePlaybackActions(
	ctx context.Context,
	logger *slog.Logger,
	reader JSONReader,
	player PlayerController) error {
	// playback always starts in a playing state
	playing := true

	for {
		var action actionMessage
		err := reader.ReadJSON(&action)
		if err != nil {
			return trace.Wrap(err)
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
			player.SetSpeed(float64(action.PlaybackSpeed))
		case actionSeek:
			player.SetPos(time.Duration(action.Pos) * time.Millisecond)
		default:
			slog.WarnContext(ctx, "invalid desktop playback action", "action", action.Action)
			return trace.BadParameter("invalid desktop action")
		}
	}
}

// RecordingPlayer models the read actions of the session player.
type RecordingPlayer interface {
	C() <-chan events.AuditEvent
	Err() error
}

// PlayRecording feeds recorded events from a player
// over a websocket.
func PlayRecording(
	ctx context.Context,
	log *slog.Logger,
	ws *websocket.Conn,
	player RecordingPlayer) {
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
						log.WarnContext(ctx, "failed to marshal player error message", "error", err)
						msg = []byte(`"internal server error"`)
					}
					//lint:ignore QF1012 this write needs to happen in a single operation
					bytes := fmt.Appendf(nil, `{"message":"error", "errorText":%s}`, string(msg))
					if err := ws.WriteMessage(websocket.BinaryMessage, bytes); err != nil {
						log.ErrorContext(ctx, "failed to write error message", "error", err)
					}
					return
				}
				if err := ws.WriteMessage(websocket.BinaryMessage, []byte(`{"message":"end"}`)); err != nil {
					log.ErrorContext(ctx, "failed to write end message", "error", err)
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
				log.ErrorContext(ctx, "failed to marshal desktop event", "error", err)
				ws.WriteMessage(websocket.BinaryMessage, []byte(`{"message":"error","errorText":"server error"}`))
				return
			}
			if err := ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				// Connection close errors are expected if the user closes the tab.
				// Only log unexpected errors to avoid cluttering the logs.
				if !utils.IsOKNetworkError(err) {
					log.WarnContext(ctx, "websocket write error", "error", err)
				}
				return
			}
		}
	}
}
