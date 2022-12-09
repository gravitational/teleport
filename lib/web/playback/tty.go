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
	"sync"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

type ttyEventHandler struct{}

// NewPlayer creates a player that streams a tty session
// over the provided websocket connection.
func NewTtyPlayer(sID string, ws *websocket.Conn, streamer Streamer, log logrus.FieldLogger) *Player {
	p := &Player{
		ws:                ws,
		streamer:          streamer,
		eventHandler:      ttyEventHandler{},
		playState:         playStatePlaying,
		log:               log,
		sID:               sID,
		playSpeed:         1.0,
		delayCancelSignal: make(chan interface{}, 1),
	}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (e ttyEventHandler) handleEvent(ctx context.Context, payload eventHandlerPayload) error {
	evt, pp := payload.event, payload.pp

	switch e := evt.(type) {
	case *apievents.SessionPrint:
		pp.waitForDelay(e.DelayMilliseconds, payload.lastDelay)

		if err := pp.marshalAndSendEvent(e); err != nil {
			return err
		}
	case *apievents.Resize, *apievents.SessionStart:
		if err := pp.marshalAndSendEvent(e); err != nil {
			return err
		}
	default:
		pp.log.Warnf("session %v contains unexpected event type %T", pp.sID, evt)
	}

	return nil
}
