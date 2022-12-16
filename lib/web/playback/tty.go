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

	apievents "github.com/gravitational/teleport/api/types/events"
	clientPlayback "github.com/gravitational/teleport/lib/client/playback"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

type ttyEventHandler struct{}

// NewPlayer creates a player that streams a tty session
// over the provided websocket connection.
func NewTtyPlayer(sID string, ws *websocket.Conn, streamer clientPlayback.Streamer, log logrus.FieldLogger) *Player {
	return NewPlayer(sID, ws, streamer, log, &ttyEventHandler{})
}

func (e ttyEventHandler) handleEvent(ctx context.Context, payload clientPlayback.EventHandlerPayload) error {
	evt, pp := payload.Event, payload.Pp

	switch e := evt.(type) {
	case *apievents.SessionPrint:
		pp.WaitForDelay(e.DelayMilliseconds)

		if err := pp.MarshalAndSend(e); err != nil {
			return err
		}
	case *apievents.Resize, *apievents.SessionStart:
		if err := pp.MarshalAndSend(e); err != nil {
			return err
		}
	default:
		pp.Log.Warnf("session %v contains unexpected event type %T", pp.SID, evt)
	}

	return nil
}
