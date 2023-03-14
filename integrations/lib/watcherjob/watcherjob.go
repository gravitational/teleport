// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package watcherjob

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

const DefaultMaxConcurrency = 128
const DefaultEventFuncTimeout = time.Second * 5

type EventFunc func(context.Context, types.Event) error

type Config struct {
	Watch            types.Watch
	MaxConcurrency   int
	EventFuncTimeout time.Duration
}

type job struct {
	lib.ServiceJob
	config    Config
	eventFunc EventFunc
	events    types.Events
	eventCh   chan *types.Event
}

type eventKey struct {
	kind string
	name string
}

func NewJob(client teleport.Client, config Config, fn EventFunc) lib.ServiceJob {
	return NewJobWithEvents(client, config, fn)
}

func NewJobWithEvents(events types.Events, config Config, fn EventFunc) lib.ServiceJob {
	if config.MaxConcurrency == 0 {
		config.MaxConcurrency = DefaultMaxConcurrency
	}
	if config.EventFuncTimeout == 0 {
		config.EventFuncTimeout = DefaultEventFuncTimeout
	}
	job := job{
		events:    events,
		config:    config,
		eventFunc: fn,
		eventCh:   make(chan *types.Event, config.MaxConcurrency),
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

		log := logger.Get(ctx)
		for {
			err := job.watchEvents(ctx)
			switch {
			case trace.IsConnectionProblem(err):
				log.WithError(err).Error("Failed to connect to Teleport Auth server. Reconnecting...")
			case trace.IsEOF(err):
				log.WithError(err).Error("Watcher stream closed. Reconnecting...")
			case lib.IsCanceled(err):
				log.Debug("Watcher context is canceled")
				// Context cancellation is not an error
				return nil
			default:
				log.WithError(err).Error("Watcher event loop failed")
				return trace.Wrap(err)
			}
		}
	})
	return job
}

// watchEvents spawns a watcher and reads events from it.
func (job job) watchEvents(ctx context.Context) error {
	watcher, err := job.events.NewWatcher(ctx, job.config.Watch)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			logger.Get(ctx).WithError(err).Error("Failed to close a watcher")
		}
	}()

	if err := job.waitInit(ctx, watcher, 15*time.Second); err != nil {
		return trace.Wrap(err)
	}

	logger.Get(ctx).Debug("Watcher connected")
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
			return trace.Wrap(watcher.Error())
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
				log.Error("received an event with empty resource field")
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
