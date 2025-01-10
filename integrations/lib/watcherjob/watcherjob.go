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

package watcherjob

import (
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/backoff"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

const (
	DefaultMaxConcurrency   = 128
	DefaultEventFuncTimeout = time.Second * 5
	failFastEnvVarName      = "TELEPORT_PLUGIN_FAIL_FAST"
)

type EventFunc func(context.Context, types.Event) error
type WatchInitFunc func(types.WatchStatus)

type Config struct {
	Watch            types.Watch
	MaxConcurrency   int
	EventFuncTimeout time.Duration
	FailFast         bool
}

type job struct {
	lib.ServiceJob
	config          Config
	eventFunc       EventFunc
	events          types.Events
	eventCh         chan *types.Event
	onWatchInitFunc WatchInitFunc
}

type eventKey struct {
	kind string
	name string
}

func NewJob(client teleport.Client, config Config, fn EventFunc) (lib.ServiceJob, error) {
	return NewJobWithEvents(client, config, fn)
}

// NewJobWithConfirmedWatchKinds returns a new watcherJob and passes confirmed watch kinds
// from the initialisation down confirmedWatchKindsCh.
func NewJobWithConfirmedWatchKinds(events types.Events, config Config, fn EventFunc, watchInitFunc WatchInitFunc) (lib.ServiceJob, error) {
	return newJobWithEvents(events, config, fn, watchInitFunc)
}

func NewJobWithEvents(events types.Events, config Config, fn EventFunc) (lib.ServiceJob, error) {
	return newJobWithEvents(events, config, fn, nil)
}

func newJobWithEvents(events types.Events, config Config, fn EventFunc, watchInitFunc WatchInitFunc) (job, error) {
	if config.MaxConcurrency == 0 {
		config.MaxConcurrency = DefaultMaxConcurrency
	}
	if config.EventFuncTimeout == 0 {
		config.EventFuncTimeout = DefaultEventFuncTimeout
	}
	if flagVar := os.Getenv(failFastEnvVarName); !config.FailFast && flagVar != "" {
		flag, err := strconv.ParseBool(flagVar)
		if err != nil {
			return job{}, trace.WrapWithMessage(err, "failed to parse content '%s' of the %s environment variable", flagVar, failFastEnvVarName)
		}
		config.FailFast = flag
	}
	job := job{
		events:          events,
		config:          config,
		eventFunc:       fn,
		eventCh:         make(chan *types.Event, config.MaxConcurrency),
		onWatchInitFunc: watchInitFunc,
	}
	job.ServiceJob = lib.NewServiceJob(func(ctx context.Context) error {
		process := lib.MustGetProcess(ctx)

		// Run a separate event loop thread which does not depend on streamer context.
		defer close(job.eventCh)
		process.Spawn(job.eventLoop)

		// Create a cancellable ctx for event watcher.
		ctx, cancel := context.WithCancel(ctx)
		process.OnTerminate(func(_ context.Context) error {
			cancel()
			return nil
		})

		bk := backoff.NewDecorr(20*time.Millisecond, 5*time.Second, clockwork.NewRealClock())

		log := logger.Get(ctx)
		for {
			err := job.watchEvents(ctx)
			// We are not supporting liveness/readiness yet, but if we do it would make sense to use job's readiness
			job.SetReady(false)

			// Note: we must always return an error, even if everything went great and we're doing a graceful shutdown.
			// The process library will trigger a complete shutdown only if the critical job exits with an error.
			switch {
			case trace.IsConnectionProblem(err):
				if config.FailFast {
					return trace.WrapWithMessage(err, "Connection problem detected. Exiting as fail fast is on.")
				}
				log.ErrorContext(ctx, "Connection problem detected, attempting to reconnect", "error", err)
			case errors.Is(err, io.EOF):
				if config.FailFast {
					return trace.WrapWithMessage(err, "Watcher stream closed. Exiting as fail fast is on.")
				}
				log.ErrorContext(ctx, "Watcher stream closed attempting to reconnect", "error", err)
			case lib.IsCanceled(err):
				log.DebugContext(ctx, "Watcher context is canceled")
				return trace.Wrap(err)
			default:
				log.ErrorContext(ctx, "Watcher event loop failed", "error", err)
				return trace.Wrap(err)
			}

			// To mitigate a potentially aggressive retry loop, we wait
			if err := bk.Do(ctx); err != nil {
				log.DebugContext(ctx, "Watcher context was canceled while waiting before a reconnection")
				return trace.Wrap(err)
			}
		}
	})
	return job, nil
}

// watchEvents spawns a watcher and reads events from it.
func (job job) watchEvents(ctx context.Context) error {
	watcher, err := job.events.NewWatcher(ctx, job.config.Watch)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			logger.Get(ctx).ErrorContext(ctx, "Failed to close a watcher", "error", err)
		}
	}()

	if err := job.waitInit(ctx, watcher, 15*time.Second); err != nil {
		return trace.Wrap(err)
	}

	logger.Get(ctx).DebugContext(ctx, "Watcher connected")
	job.SetReady(true)

	for {
		select {
		case event := <-watcher.Events():
			job.eventCh <- &event
		case <-watcher.Done(): // When the watcher completes, read the rest of events and quit.
			events := takeEvents(watcher.Events())
			for i := range events {
				select {
				case job.eventCh <- &events[i]:
				case <-ctx.Done():
					return trace.Wrap(ctx.Err())
				}
			}
			if err := watcher.Error(); err != nil {
				return trace.Wrap(err)
			}
			return trace.Errorf("watcher closed without error")
		}
	}
}

// waitInit waits for OpInit event be received on a stream.
func (job job) waitInit(ctx context.Context, watcher types.Watcher, timeout time.Duration) error {
	select {
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.ConnectionProblem(nil, "unexpected event type %q", event.Type)
		}
		if watchStatus, ok := event.Resource.(types.WatchStatus); ok {
			if job.onWatchInitFunc != nil {
				job.onWatchInitFunc(watchStatus)
			}
		}
		return nil
	case <-time.After(timeout):
		return trace.ConnectionProblem(nil, "watcher initialization timed out")
	case <-watcher.Done():
		return trace.Wrap(watcher.Error())
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

// eventLoop goes through event stream and spawns the event jobs.
//
// Queue processing algorithm is a bit tricky.
// We want to process events concurrently each in its own job.
// On the other hand, we want to avoid potential race conditions so it seems
// that in some cases it's better to process events sequentially in
// the order they came to the queue.
//
// The algorithm combines two approaches, concurrent and sequential.
// It follows the rules:
// - Events for different resources being processed concurrently.
// - Events for the same resource being processed "sequentially" i.e. in the order they came to the queue.
//
// By "sameness" we mean that Kind and Name fields of one resource object are the same as in the other resource object.
func (job job) eventLoop(ctx context.Context) error {
	var concurrency int
	process := lib.MustGetProcess(ctx)
	log := logger.Get(ctx)
	queues := make(map[eventKey][]types.Event)
	eventDone := make(chan eventKey, job.config.MaxConcurrency)

	for {
		var eventCh <-chan *types.Event
		if concurrency < job.config.MaxConcurrency {
			// If haven't yet reached the limit then we allowed to read from the queue.
			// Otherwise, eventCh would be nil which is a forever-blocking channel.
			eventCh = job.eventCh
		}

		select {
		case eventPtr := <-eventCh:
			if eventPtr == nil { // channel is closed when the parent job is done so just quit normally.
				return nil
			}
			event := *eventPtr
			resource := event.Resource
			if resource == nil {
				log.ErrorContext(ctx, "received an event with empty resource field")
			}
			key := eventKey{kind: resource.GetKind(), name: resource.GetName()}
			if queue, loaded := queues[key]; loaded {
				queues[key] = append(queue, event)
			} else {
				queues[key] = nil
				process.Spawn(job.eventFuncHandler(event, key, eventDone))
			}
			concurrency++

		case key := <-eventDone:
			concurrency--
			queue, ok := queues[key]
			if !ok {
				continue
			}
			if len(queue) > 0 {
				event := queue[0]
				process.Spawn(job.eventFuncHandler(event, key, eventDone))
				queue = queue[1:]
				queues[key] = queue
			}
			if len(queue) == 0 {
				delete(queues, key)
			}

		case <-ctx.Done(): // Stop processing immediately because the context was canceled.
			return trace.Wrap(ctx.Err())
		}
	}
}

// eventFuncHandler returns an event handler ready to spawn.
func (job job) eventFuncHandler(event types.Event, key eventKey, doneCh chan<- eventKey) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		defer func() {
			select {
			case doneCh <- key:
			case <-ctx.Done():
			}
		}()
		eventCtx, cancel := context.WithTimeout(ctx, job.config.EventFuncTimeout)
		defer cancel()
		return job.eventFunc(eventCtx, event)
	}
}

// takeEvents reads all the buffered events from channel.
func takeEvents(events <-chan types.Event) []types.Event {
	var result []types.Event
	for {
		select {
		case event := <-events:
			result = append(result, event)
		default:
			return result
		}
	}
}
