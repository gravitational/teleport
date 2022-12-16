/*
Copyright 2015 Gravitational, Inc.

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

package client

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/client/playback"
	"github.com/gravitational/teleport/lib/client/terminal"
)

// sessionPlayer implements replaying terminal sessions. It runs a playback goroutine
// and allows to control it
type sessionPlayer struct {
	ctx        context.Context
	player     *playback.Player
	term       *terminal.Terminal
	controller *sessionPlayerController
	clock      clockwork.Clock
	position   int64
	mu         sync.Mutex
	log        *logrus.Logger
}

func newSessionPlayer(ctx context.Context, sID string, stream playback.Streamer) (*sessionPlayer, error) {
	term, err := terminal.New(nil, nil, nil)

	if err != nil {
		return nil, err
	}

	if err := term.InitRaw(true); err != nil {
		return nil, err
	}

	controller := &sessionPlayerController{
		term: term,
	}

	log := logrus.New()
	player := playback.NewPlayer(sID, stream, log, controller)
	player.LoggingEnabled = false

	sessionPlayer := &sessionPlayer{
		ctx:        ctx,
		term:       term,
		log:        log,
		controller: controller,
		player:     player,
		clock:      clockwork.NewRealClock(),
	}

	controller.sessionPlayer = sessionPlayer

	return sessionPlayer, nil
}

// keys:
const (
	keyCtrlC = 3
	keyCtrlD = 4
	keySpace = 32
	keyLeft  = 68
	keyRight = 67
	keyUp    = 65
	keyDown  = 66
)

type sessionPlayerController struct {
	term          *terminal.Terminal
	sessionPlayer *sessionPlayer
}

func (c *sessionPlayerController) Error(msg string) error {
	_, err := os.Stderr.WriteString(msg)

	return err
}

func (c *sessionPlayerController) Move(position int64) error {
	c.sessionPlayer.mu.Lock()
	defer c.sessionPlayer.mu.Unlock()

	c.sessionPlayer.position = position

	return nil
}

func (c *sessionPlayerController) Reset() error {
	return c.Send([]byte("\x1bc"))
}

func (c *sessionPlayerController) Close() error {
	return c.term.Close()
}

func (c *sessionPlayerController) Send(msg []byte) error {
	_, err := os.Stdout.Write(msg)

	return err
}

func (c *sessionPlayerController) HandleEvent(ctx context.Context, payload playback.EventHandlerPayload) error {
	switch e := payload.Event.(type) {
	case *apievents.SessionPrint:
		c.sessionPlayer.player.WaitForDelay(e.DelayMilliseconds)
		os.Stdout.Write(e.Data)
		timestampFrame(c.term, e.Metadata.Time.Format(time.RFC3339))
	case *apievents.SessionStart:
		return handleTerminalSize(e.TerminalSize)
	case *apievents.Resize:
		return handleTerminalSize(e.TerminalSize)
	}

	return nil
}

func handleTerminalSize(terminalSize string) error {
	parts := strings.Split(terminalSize, ":")
	if len(parts) != 2 {
		return errors.Errorf("Terminal size should be in format W:H meanwhile got %v", terminalSize)
	}

	w, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}

	h, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write([]byte(fmt.Sprintf("\x1b[8;%v;%vt", h, w)))

	return err
}

func (c *sessionPlayerController) ReceiveAction() (playback.ActionMessage, error) {
	key := make([]byte, 1)
	var action playback.ActionMessage

	// Read keys until valid action is met, rest ignore
	for {
		_, err := c.term.Stdin().Read(key[:])
		if err != nil {
			return action, err
		}

		switch key[0] {
		// Ctrl+C or Ctrl+D
		case keyCtrlC, keyCtrlD:
			action.Action = playback.ActionCancel
			return action, nil
		// Space key
		case keySpace:
			action.Action = playback.ActionPlayPause
			return action, nil
		// <- arrow
		case keyLeft, keyDown:
			action.Action = playback.ActionRewind
			return action, nil
		// -> arrow
		case keyRight, keyUp:
			action.Action = playback.ActionForward
			return action, nil
		}
	}
}

func (p *sessionPlayer) Play() {
	// clear screen between runs:
	os.Stdout.Write([]byte("\x1bc"))

	p.player.Play(p.ctx)
}

// timestampFrame prints 'event timestamp' in the top right corner of the
// terminal after playing every 'print' event
func timestampFrame(term *terminal.Terminal, message string) {
	const (
		saveCursor    = "7"
		restoreCursor = "8"
	)
	width, _, err := term.Size()
	if err != nil {
		return
	}
	esc := func(s string) {
		os.Stdout.Write([]byte("\x1b" + s))
	}
	esc(saveCursor)
	defer esc(restoreCursor)

	// move cursor to -10:0
	// TODO(timothyb89): message length does not account for unicode characters
	// or ANSI sequences.
	esc(fmt.Sprintf("[%d;%df", 0, int(width)-len(message)))
	os.Stdout.WriteString(message)
}
