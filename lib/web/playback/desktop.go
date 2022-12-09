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
	"net"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/utils"
)

type desktopEventHandler struct {
}

// NewPlayer creates a player that streams a desktop session
// over the provided websocket connection.
func NewDesktopPlayer(sID string, ws *websocket.Conn, streamer Streamer, log logrus.FieldLogger) *Player {
	p := &Player{
		ws:                ws,
		streamer:          streamer,
		eventHandler:      desktopEventHandler{},
		playState:         playStatePlaying,
		log:               log,
		sID:               sID,
		playSpeed:         1.0,
		delayCancelSignal: make(chan interface{}, 1),
	}
	p.cond = sync.NewCond(&p.mu)
	return p
}

// StreamSessionEvents streams the session's events as playback events over the websocket.
func (e desktopEventHandler) handleEvent(ctx context.Context, payload eventHandlerPayload) error {
	evt, pp := payload.event, payload.pp

	switch e := evt.(type) {
	case *apievents.DesktopRecording:
		pp.waitForDelay(e.DelayMilliseconds, payload.lastDelay)

		msg, err := utils.FastMarshal(e)
		if err != nil {
			pp.log.WithError(err).Errorf("failed to marshal DesktopRecording event into JSON: %v", e)
			if _, err := pp.ws.Write([]byte(`{"message":"error","errorText":"server error"}`)); err != nil {
				pp.log.WithError(err).Error("failed to write \"error\" message over websocket")
			}
			return err
		}
		if _, err := pp.ws.Write(msg); err != nil {
			// We expect net.ErrClosed to arise when another goroutine returns before
			// this one or the browser window is closed, both of which cause the websocket to close.
			if !errors.Is(err, net.ErrClosed) {
				pp.log.WithError(err).Error("failed to write DesktopRecording event over websocket")
			}
			return err
		}
	case *apievents.WindowsDesktopSessionStart, *apievents.WindowsDesktopSessionEnd:
		// these events are part of the stream but never needed for playback
	case *apievents.DesktopClipboardReceive, *apievents.DesktopClipboardSend:
		// these events are not currently needed for playback,
		// but may be useful in the future

	default:
		pp.log.Warnf("session %v contains unexpected event type %T", pp.sID, evt)
	}

	return nil
}
