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

// Package player includes an API to play back recorded sessions.
package player

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"maps"
	"math"
	"slices"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/player/db"
	"github.com/gravitational/teleport/lib/session"
)

// Player is used to stream recorded sessions over a channel.
type Player struct {
	// read only config fields
	clock        clockwork.Clock
	log          *slog.Logger
	sessionID    session.ID
	streamer     Streamer
	skipIdleTime bool

	speed      atomic.Value // playback speed (1.0 for normal speed)
	lastPlayed atomic.Int64 // timestamp of most recently played event

	// advanceTo is used to implement fast-forward and rewind.
	// During normal operation, it is set to [normalPlayback].
	//
	// When set to a positive value the player is seeking forward
	// in time (and plays events as quickly as possible).
	//
	// When set to a negative value, the player needs to "rewind"
	// by starting the stream over from the beginning and then
	// seeking forward to the rewind point.
	advanceTo atomic.Int64

	emit chan events.AuditEvent
	wake chan time.Duration
	done chan struct{}

	// playPause holds a channel to be closed when
	// the player transitions from paused to playing,
	// or nil if the player is already playing.
	//
	// This approach mimics a "select-able" condition variable
	// and is inspired by "Rethinking Classical Concurrency Patterns"
	// by Bryan C. Mills (GopherCon 2018): https://www.youtube.com/watch?v=5zXAHh5tJqQ
	playPause chan chan struct{}

	// err holds the error (if any) encountered during playback
	err error

	// translator is the current SessionPrintTranslator used.
	translator sessionPrintTranslator
}

const (
	normalPlayback = time.Duration(0)
	// MaxIdleTime defines the max idle time when skipping idle
	// periods on the recording.
	MaxIdleTime = 500 * time.Millisecond
)

// Streamer is the underlying streamer that provides
// access to recorded session events.
type Streamer interface {
	StreamSessionEvents(
		ctx context.Context,
		sessionID session.ID,
		startIndex int64,
	) (chan events.AuditEvent, chan error)
}

// newSessionPrintTranslatorFunc defines a SessionPrintTranslator constructor.
type newSessionPrintTranslatorFunc func() sessionPrintTranslator

// sessionPrintTranslator provides a way to transform detailed protocol-specific
// audit events into a textual representation.
type sessionPrintTranslator interface {
	// TranslateEvent takes an audit event and converts it into a print event.
	// The function might return `nil` in cases where there is no textual
	// representation for the provided event.
	TranslateEvent(events.AuditEvent) *events.SessionPrint
}

// Config configures a session player.
type Config struct {
	Clock        clockwork.Clock
	Log          *slog.Logger
	SessionID    session.ID
	Streamer     Streamer
	SkipIdleTime bool
	Context      context.Context
}

func New(cfg *Config) (*Player, error) {
	if cfg.Streamer == nil {
		return nil, trace.BadParameter("missing Streamer")
	}

	if cfg.SessionID == "" {
		return nil, trace.BadParameter("missing SessionID")
	}

	clk := cfg.Clock
	if clk == nil {
		clk = clockwork.NewRealClock()
	}

	log := cmp.Or(
		cfg.Log,
		slog.With(teleport.ComponentKey, "player"),
	)

	ctx := context.Background()
	if cfg.Context != nil {
		ctx = cfg.Context
	}

	p := &Player{
		clock:        clk,
		log:          log,
		sessionID:    cfg.SessionID,
		streamer:     cfg.Streamer,
		emit:         make(chan events.AuditEvent, 1024),
		playPause:    make(chan chan struct{}, 1),
		wake:         make(chan time.Duration),
		done:         make(chan struct{}),
		skipIdleTime: cfg.SkipIdleTime,
	}

	p.speed.Store(float64(defaultPlaybackSpeed))
	p.advanceTo.Store(int64(normalPlayback))

	// start in a paused state
	p.playPause <- make(chan struct{})

	go p.stream(ctx)

	return p, nil
}

// errClosed is an internal error that is used to signal
// that the player has been closed
var errClosed = errors.New("player closed")

const (
	minPlaybackSpeed     = 0.25
	defaultPlaybackSpeed = 1.0
	maxPlaybackSpeed     = 16
)

// SetSpeed adjusts the playback speed of the player.
// It can be called at any time (the player can be in a playing
// or paused state). A speed of 1.0 plays back at regular speed,
// while a speed of 2.0 plays back twice as fast as originally
// recorded. Valid speeds range from 0.25 to 16.0.
func (p *Player) SetSpeed(s float64) error {
	if s < minPlaybackSpeed || s > maxPlaybackSpeed {
		return trace.BadParameter("speed %v is out of range", s)
	}
	p.speed.Store(s)
	return nil
}

func (p *Player) stream(baseContext context.Context) {
	ctx, cancel := context.WithCancel(baseContext)
	defer cancel()

	eventsC, errC := p.streamer.StreamSessionEvents(metadata.WithSessionRecordingFormatContext(ctx, teleport.PTY), p.sessionID, 0)
	var lastDelay time.Duration
	for {
		select {
		case <-p.done:
			close(p.emit)
			return
		case err := <-errC:
			p.log.WarnContext(ctx, "Event streamer encountered error", "error", err)
			p.err = err
			close(p.emit)
			return
		case evt := <-eventsC:
			if evt == nil {
				p.log.DebugContext(ctx, "Reached end of playback for session", "session_id", p.sessionID)
				close(p.emit)
				return
			}

			if err := p.waitWhilePaused(); err != nil {
				p.log.WarnContext(ctx, "Encountered error in pause state", "error", err)
				close(p.emit)
				return
			}

			var skip bool
			evt, skip = p.translateEvent(evt)
			if skip {
				continue
			}

			currentDelay := getDelay(evt)
			if currentDelay > 0 && currentDelay >= lastDelay {
				switch adv := time.Duration(p.advanceTo.Load()); {
				case adv >= currentDelay:
					// no timing delay necessary, we are fast forwarding
					break
				case adv < 0 && adv != normalPlayback:
					// any negative value other than normalPlayback means
					// we rewind (by restarting the stream and seeking forward
					// to the rewind point)
					p.advanceTo.Store(int64(adv) * -1)
					go p.stream(baseContext)
					return
				default:
					if adv != normalPlayback {
						p.advanceTo.Store(int64(normalPlayback))

						// we're catching back up to real time, so the delay
						// is calculated not from the last event but from the
						// time we were advanced to
						lastDelay = adv
					}

					switch err := p.applyDelay(lastDelay, currentDelay); {
					case errors.Is(err, errSeekWhilePaused):
						p.log.DebugContext(ctx, "Seeked during pause, will restart stream")
						go p.stream(baseContext)
						return
					case err != nil:
						close(p.emit)
						return
					}
				}

				lastDelay = currentDelay
			}

			// if the receiver can't keep up, let the channel throttle us
			// (it's better for playback to be a little slower than realtime
			// than to drop events)
			//
			// TODO: consider a select with a timeout to detect blocked readers?
			p.emit <- evt
			p.lastPlayed.Store(int64(currentDelay))
		}
	}
}

// Close shuts down the player and cancels any streams that are
// in progress.
func (p *Player) Close() error {
	close(p.done)
	return nil
}

// C returns a read only channel of recorded session events.
// The player manages the timing of events and writes them to the channel
// when they should be rendered. The channel is closed when the player
// has reached the end of playback.
func (p *Player) C() <-chan events.AuditEvent {
	return p.emit
}

// Err returns the error (if any) that occurred during playback.
// It should only be called after the channel returned by [C] is
// closed.
func (p *Player) Err() error {
	return p.err
}

// Pause temporarily stops the player from emitting events.
// It is a no-op if playback is currently paused.
func (p *Player) Pause() error {
	p.setPlaying(false)
	return nil
}

// Play starts emitting events. It is used to start playback
// for the first time and to resume playing after the player
// is paused.
func (p *Player) Play() error {
	p.setPlaying(true)
	return nil
}

// SetPos sets playback to a specific time, expressed as a duration
// from the beginning of the session. A duration of 0 restarts playback
// from the beginning. A duration greater than the length of the session
// will cause playback to rapidly advance to the end of the recording.
func (p *Player) SetPos(d time.Duration) error {
	// we use a negative value to indicate rewinding, which means we can't
	// rewind to position 0 (there is no negative 0)
	if d == 0 {
		d = 1 * time.Millisecond
	}
	if d < time.Duration(p.lastPlayed.Load()) {
		d = -1 * d
	}
	p.advanceTo.Store(int64(d))

	// try to wake up the player if it's waiting to emit an event
	select {
	case p.wake <- d:
	default:
	}

	return nil
}

// applyDelay applies the timing delay between the last emitted event
// (lastDelay) and the event that will be emitted next (currentDelay).
//
// The delay can be interrupted by:
// 1. The player being closed.
// 2. The user pausing playback.
// 3. The user seeking to a new position in the playback (SetPos)
//
// A nil return value indicates that the delay has elapsed and that
// the next even can be emitted.
func (p *Player) applyDelay(lastDelay, currentDelay time.Duration) error {
loop:
	for {
		// TODO(zmb3): changing play speed during a long sleep
		// will not apply until after the sleep completes
		speed := p.speed.Load().(float64)
		scaled := time.Duration(float64(currentDelay-lastDelay) / speed)
		if p.skipIdleTime {
			scaled = min(scaled, MaxIdleTime)
		}

		timer := p.clock.NewTimer(scaled)
		defer timer.Stop()

		start := time.Now()

		select {
		case <-p.done:
			return errClosed
		case newPos := <-p.wake:
			// the sleep was interrupted due to the user changing playback controls
			switch {
			case newPos == interruptForPause:
				// the user paused playback while we were waiting to emit the next event:
				// 1) figure out much of the sleep we completed
				dur := time.Duration(float64(time.Since(start)) * speed)

				// 2) wait here until the user resumes playback
				if err := p.waitWhilePaused(); errors.Is(err, errSeekWhilePaused) {
					// the user changed the playback position, so consider the delay
					// applied and let the player pick up from the new position
					return errSeekWhilePaused
				}

				// now that we're playing again, update our delay to account
				// for the portion that was already satisfied and apply the
				// remaining delay
				lastDelay += dur
				timer.Stop()
				continue loop
			case newPos > currentDelay:
				// the user scrubbed forward in time past the current event,
				// so we can return as if the delay has elapsed naturally
				return nil
			case newPos < 0:
				// the user has rewinded playback, which means we need to restart
				// the stream and can consider this delay as having elapsed naturally
				return nil
			case newPos < currentDelay:
				// the user has scrubbed forward in time, but not enough to
				// emit the next event - we need to delay more
				lastDelay = newPos
				timer.Stop()
				continue loop
			default:
				return nil
			}

		case <-timer.Chan():
			return nil
		}
	}
}

// interruptForPause is a special value used to interrupt the player's
// sleep due to the user pausing playback.
const interruptForPause = math.MaxInt64

func (p *Player) setPlaying(play bool) {
	ch := <-p.playPause
	alreadyPlaying := ch == nil

	if alreadyPlaying && !play {
		ch = make(chan struct{})

		// try to wake up the player if it's waiting to emit an event
		select {
		case p.wake <- interruptForPause:
		default:
		}

	} else if !alreadyPlaying && play {
		// signal waiters who are paused that it's time to resume playing
		close(ch)
		ch = nil
	}

	p.playPause <- ch
}

var errSeekWhilePaused = errors.New("player seeked during pause")

// waitWhilePaused blocks while the player is in a paused state.
// It returns immediately if the player is currently playing.
func (p *Player) waitWhilePaused() error {
	seeked := false
	for {
		ch := <-p.playPause
		p.playPause <- ch

		if alreadyPlaying := ch == nil; !alreadyPlaying {
			select {
			case <-p.done:
				return errClosed
			case <-p.wake:
				// seek while paused, this can happen an unlimited number of times,
				// we just keep waiting until we're unpaused
				seeked = true
				continue
			case <-ch:
				// we have been unpaused
			}
		}
		if seeked {
			return errSeekWhilePaused
		}
		return nil
	}
}

// LastPlayed returns the time of the last played event,
// expressed as milliseconds since the start of the session.
func (p *Player) LastPlayed() time.Duration {
	return time.Duration(p.lastPlayed.Load())
}

// translateEvent translates events if applicable and return if they should be
// skipped.
func (p *Player) translateEvent(evt events.AuditEvent) (translatedEvent events.AuditEvent, shouldSkip bool) {
	// We can only define the translator when the first event arrives.
	switch e := evt.(type) {
	case *events.DatabaseSessionStart:
		if newTranslatorFunc, ok := databaseTranslators[e.DatabaseProtocol]; ok {
			p.translator = newTranslatorFunc()
		}
	}

	if p.translator == nil {
		return evt, false
	}

	if translatedEvt := p.translator.TranslateEvent(evt); translatedEvt != nil {
		return translatedEvt, false
	}

	// Always skip if the translator returns an nil event.
	return nil, true
}

// databaseTranslators maps database protocol event translators.
var databaseTranslators = map[string]newSessionPrintTranslatorFunc{
	defaults.ProtocolPostgres: func() sessionPrintTranslator { return db.NewPostgresTranslator() },
}

// SupportedDatabaseProtocols a list of database protocols supported by the
// player.
var SupportedDatabaseProtocols = slices.Collect(maps.Keys(databaseTranslators))

func getDelay(e events.AuditEvent) time.Duration {
	switch x := e.(type) {
	case *events.DesktopRecording:
		return time.Duration(x.DelayMilliseconds) * time.Millisecond
	case *events.SessionPrint:
		return time.Duration(x.DelayMilliseconds) * time.Millisecond
	default:
		return time.Duration(0)
	}
}
