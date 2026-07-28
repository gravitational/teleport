// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package subcav1

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

type resourceWatcher struct {
	logger  *slog.Logger
	source  WatcherSource
	retry   *retryutils.RetryV2
	ctx     context.Context
	onInit  func(e *types.Event)
	onEvent func(op types.OpType, e *types.Event)
	spec    types.Watch

	current types.Watcher
}

func (s *Service) newResourceWatcher(
	ctx context.Context,
	spec types.Watch,
	onInit func(e *types.Event),
	onEvent func(op types.OpType, e *types.Event),
) (*resourceWatcher, error) {
	retry, err := s.newWatcherRetrier()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &resourceWatcher{
		logger:  s.logger.With("watcher", spec.Name),
		source:  s.watcherSource,
		retry:   retry,
		ctx:     ctx,
		onInit:  onInit,
		onEvent: onEvent,
		spec:    spec,
	}, nil
}

func (s *resourceWatcher) run() {
	var exitErr error
	defer func() {
		if errors.Is(exitErr, context.Canceled) {
			s.logger.DebugContext(s.ctx, "Watcher exited (context canceled)")
		} else {
			s.logger.DebugContext(s.ctx, "Watcher exited", "error", exitErr)
		}
	}()

	for {
		switch err, abort := s.init(); {
		case abort:
			exitErr = trace.Wrap(err)
			return
		case err != nil:
			s.logger.DebugContext(s.ctx, "Watcher init errored, re-creating", "error", err)
			continue
		}

	Receive:
		for {
			switch err, abort := s.receive(); {
			case abort:
				exitErr = trace.Wrap(err)
				return
			case err != nil:
				s.logger.DebugContext(s.ctx, "Watcher receive errored, re-creating", "error", err)
				break Receive
			}
		}
	}
}

func (s *resourceWatcher) init() (_ error, abort bool) {
	select {
	case <-s.ctx.Done():
		return trace.Wrap(s.ctx.Err()), true
	case <-s.retry.After():
	}
	s.retry.Inc()

	var err error
	s.current, err = s.source.NewWatcher(s.ctx, s.spec)
	if err != nil {
		return trace.Wrap(err), false
	}
	s.logger.DebugContext(s.ctx, "Watcher created")
	s.retry.Reset()

	e, err, abort := s.receiveOnce()
	if err != nil {
		return trace.Wrap(err), abort
	}

	if e.Type != types.OpInit {
		const silent = false
		s.closeCurrent(silent)
		s.logger.WarnContext(s.ctx, "Initial watcher event.Type is not OpInit", "event_type", e.Type)
		return trace.Wrap(errors.New("initial watcher event.Type is not OpInit")), false
	}

	s.onInit(e)
	return nil, false
}

func (s *resourceWatcher) receive() (_ error, abort bool) {
	e, err, abort := s.receiveOnce()
	if err != nil {
		return trace.Wrap(err), abort
	}
	if e.Type == types.OpInit {
		const silent = false
		s.closeCurrent(silent)
		s.logger.WarnContext(s.ctx, "Received unexpected OpInit event")
		return trace.Wrap(errors.New("received unexpected OpInit event")), false
	}

	s.onEvent(e.Type, e)
	return nil, false
}

func (s *resourceWatcher) receiveOnce() (_ *types.Event, err error, abort bool) {
	silent := false
	defer func() {
		if err != nil {
			s.closeCurrent(silent)
		}
	}()

	select {
	case <-s.ctx.Done():
		return nil, s.ctx.Err(), true

	case <-s.current.Done():
		// Make sure the error is always non-nil here.
		err = cmp.Or(s.current.Error(), errors.New("watcher closed"))
		// Likely already closed.
		silent = true
		return nil, trace.Wrap(err), false

	case e, ok := <-s.current.Events():
		switch {
		case !ok:
			s.logger.DebugContext(s.ctx, "Watcher events channel closed, re-connecting")
			return nil, trace.Wrap(errors.New("events channel closed")), false
		case e.Type != types.OpInit && e.Type != types.OpPut && e.Type != types.OpDelete:
			s.logger.WarnContext(s.ctx, "Watcher event.Type unknown or disallowed, re-connecting", "event_type", e.Type)
			return nil, trace.Wrap(fmt.Errorf("event.Type invalid: %v", e.Type)), false
		case e.Type != types.OpInit && e.Resource == nil:
			s.logger.WarnContext(s.ctx, "Watcher event has nil Resource, re-connecting")
			return nil, trace.Wrap(errors.New("event has nil Resource")), false
		}

		logger := s.logger
		if e.Resource != nil {
			logger = logger.With(
				"kind", e.Resource.GetKind(),
				"sub_kind", e.Resource.GetSubKind(),
				"name", e.Resource.GetName(),
				"revision", e.Resource.GetRevision(),
			)
		}
		logger.DebugContext(s.ctx, "Received watcher event", "event_type", e.Type)
		return &e, nil, false
	}
}

func (s *resourceWatcher) closeCurrent(silent bool) {
	if err := s.current.Close(); err != nil && !silent {
		s.logger.DebugContext(s.ctx, "Error closing watcher", "error", err)
	}
}
