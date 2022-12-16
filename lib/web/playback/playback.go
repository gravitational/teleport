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
	"fmt"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
	"sigs.k8s.io/kustomize/kyaml/errors"

	clientPlayback "github.com/gravitational/teleport/lib/client/playback"
)

// Player manages the playback of a recorded session.
// It streams events from the audit log to the browser over
// a websocket connection.
type Player struct {
	player *clientPlayback.Player
	ws     *websocket.Conn
}

func NewPlayer(sID string, ws *websocket.Conn, streamer clientPlayback.Streamer, log logrus.FieldLogger, handler eventHandler) *Player {
	controller := playerController{
		eventHandler: handler,
		log:          log,
		ws:           ws,
	}

	player := clientPlayback.NewPlayer(sID, streamer, log, &controller)

	return &Player{
		player: player,
		ws:     ws,
	}
}

// Play kicks off goroutines for receiving actions
// and playing back the session over the websocket,
// and then waits for the stream to complete.
func (pp *Player) Play(ctx context.Context) {
	pp.ws.PayloadType = websocket.BinaryFrame
	pp.player.Play(ctx)
}

// eventHandler is the interface that provides specific event handling for concreate player
type eventHandler interface {
	// handleEvent function should handle received event and optionally return error if events loop needs to be stoped
	handleEvent(ctx context.Context, payload clientPlayback.EventHandlerPayload) error
}

type playerController struct {
	ws           *websocket.Conn
	eventHandler eventHandler
	log          logrus.FieldLogger
}

func (pc *playerController) Error(msg string) error {
	return pc.Send([]byte(fmt.Sprintf(`{"message":"error", "errorText":"%v"}`, msg)))
}

func (pc *playerController) Move(position int64) error {
	return pc.Send([]byte(fmt.Sprintf(`{"event": "move", "position": %v}`, position)))
}

func (pc *playerController) Reset() error {
	return pc.Send([]byte(`{"event": "reset"}`))
}

func (pc *playerController) HandleEvent(ctx context.Context, payload clientPlayback.EventHandlerPayload) error {
	return pc.eventHandler.handleEvent(ctx, payload)
}

func (pc *playerController) Close() error {
	endErr := pc.Send([]byte(`{"message":"end"}`))
	closeErr := pc.ws.Close()

	if endErr != nil || closeErr != nil {
		return errors.Errorf("Cloud not close player controller: %v, %v", endErr, closeErr)
	}

	return nil
}

func (pc *playerController) ReceiveAction() (clientPlayback.ActionMessage, error) {
	var action clientPlayback.ActionMessage
	err := websocket.JSON.Receive(pc.ws, &action)

	return action, err
}

func (pc *playerController) Send(msg []byte) error {
	if _, err := pc.ws.Write(msg); err != nil {
		pc.log.Debugf("Failed to send %v over websocket", msg)
		return err
	}

	return nil
}
