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

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils/log"
)

type caOverrideWatcher struct {
	logger  *slog.Logger
	source  WatcherSource
	retry   *retryutils.RetryV2
	ctx     context.Context
	onInit  func(e *types.Event)
	onEvent func(op types.OpType, caOverride *subcav1.CertAuthorityOverride)
	spec    types.Watch

	current types.Watcher
}

func (s *Service) newCAOverrideWatcher(
	ctx context.Context,
	name string,
	onInit func(e *types.Event),
	onEvent func(op types.OpType, caOverride *subcav1.CertAuthorityOverride),
) (*caOverrideWatcher, error) {
	retry, err := s.newWatcherRetrier()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	spec := types.Watch{
		Name: name,
		Kinds: []types.WatchKind{
			{
				Kind: types.KindCertAuthorityOverride,
			},
		},
	}
	return &caOverrideWatcher{
		logger:  s.logger.With("watcher", name),
		source:  s.watcherSource,
		retry:   retry,
		ctx:     ctx,
		onInit:  onInit,
		onEvent: onEvent,
		spec:    spec,
	}, nil
}

func (w *caOverrideWatcher) run() {
	var exitErr error
	defer func() {
		if errors.Is(exitErr, context.Canceled) {
			w.logger.DebugContext(w.ctx, "Watcher exited (context canceled)")
		} else {
			w.logger.DebugContext(w.ctx, "Watcher exited", "error", exitErr)
		}
	}()

	for {
		switch err, abort := w.init(); {
		case abort:
			exitErr = trace.Wrap(err)
			return
		case err != nil:
			w.logger.DebugContext(w.ctx, "Watcher init errored, re-creating", "error", err)
			continue
		}

	Receive:
		for {
			switch err, abort := w.receive(); {
			case abort:
				exitErr = trace.Wrap(err)
				return
			case err != nil:
				w.logger.DebugContext(w.ctx, "Watcher receive errored, re-creating", "error", err)
				break Receive
			}
		}
	}
}

func (w *caOverrideWatcher) init() (_ error, abort bool) {
	select {
	case <-w.ctx.Done():
		return trace.Wrap(w.ctx.Err()), true
	case <-w.retry.After():
	}
	w.retry.Inc()

	var err error
	w.current, err = w.source.NewWatcher(w.ctx, w.spec)
	if err != nil {
		return trace.Wrap(err), false
	}
	w.logger.DebugContext(w.ctx, "Watcher created")
	w.retry.Reset()

	e, err, abort := w.receiveOnce()
	if err != nil {
		return trace.Wrap(err), abort
	}

	if e.Type != types.OpInit {
		const silent = false
		w.closeCurrent(silent)
		w.logger.WarnContext(w.ctx, "Initial watcher event.Type is not OpInit", "event_type", e.Type)
		return trace.Wrap(errors.New("initial watcher event.Type is not OpInit")), false
	}

	w.onInit(e)
	return nil, false
}

func (w *caOverrideWatcher) receive() (_ error, abort bool) {
	e, err, abort := w.receiveOnce()
	if err != nil {
		return trace.Wrap(err), abort
	}
	if e.Type == types.OpInit {
		const silent = false
		w.closeCurrent(silent)
		w.logger.WarnContext(w.ctx, "Received unexpected OpInit event")
		return trace.Wrap(errors.New("received unexpected OpInit event")), false
	}

	caOverride, ok := w.caOverrideFromEvent(e)
	if ok {
		w.onEvent(e.Type, caOverride)
	}
	return nil, false
}

func (w *caOverrideWatcher) caOverrideFromEvent(e *types.Event) (_ *subcav1.CertAuthorityOverride, ok bool) {
	rw, ok := e.Resource.(types.Resource153UnwrapperT[*subcav1.CertAuthorityOverride])
	if !ok {
		w.logger.WarnContext(w.ctx, "Received non-types.Resource153UnwrapperT resource",
			"resource_type", log.TypeAttr(e.Resource),
			"kind", e.Resource.GetKind(),
			"sub_kind", e.Resource.GetSubKind(),
			"name", e.Resource.GetName(),
		)
		return nil, ok
	}
	caOverride := rw.UnwrapT()
	if caOverride == nil {
		w.logger.WarnContext(w.ctx, "Received nil CA override resource",
			"kind", e.Resource.GetKind(),
			"sub_kind", e.Resource.GetSubKind(),
			"name", e.Resource.GetName(),
		)
		return nil, false
	}
	return caOverride, true
}

func (w *caOverrideWatcher) receiveOnce() (_ *types.Event, err error, abort bool) {
	silent := false
	defer func() {
		if err != nil {
			w.closeCurrent(silent)
		}
	}()

	select {
	case <-w.ctx.Done():
		return nil, w.ctx.Err(), true

	case <-w.current.Done():
		// Make sure the error is always non-nil here.
		err = cmp.Or(w.current.Error(), errors.New("watcher closed"))
		// Likely already closed.
		silent = true
		return nil, trace.Wrap(err), false

	case e, ok := <-w.current.Events():
		switch {
		case !ok:
			w.logger.DebugContext(w.ctx, "Watcher events channel closed, re-connecting")
			return nil, trace.Wrap(errors.New("events channel closed")), false
		case e.Type != types.OpInit && e.Type != types.OpPut && e.Type != types.OpDelete:
			w.logger.WarnContext(w.ctx, "Watcher event.Type unknown or disallowed, re-connecting", "event_type", e.Type)
			return nil, trace.Wrap(fmt.Errorf("event.Type invalid: %v", e.Type)), false
		case e.Type != types.OpInit && e.Resource == nil:
			w.logger.WarnContext(w.ctx, "Watcher event has nil Resource, re-connecting")
			return nil, trace.Wrap(errors.New("event has nil Resource")), false
		}

		logger := w.logger
		if e.Resource != nil {
			logger = logger.With(
				"kind", e.Resource.GetKind(),
				"sub_kind", e.Resource.GetSubKind(),
				"name", e.Resource.GetName(),
				"revision", e.Resource.GetRevision(),
			)
		}
		logger.DebugContext(w.ctx, "Received watcher event", "event_type", e.Type)
		return &e, nil, false
	}
}

func (w *caOverrideWatcher) closeCurrent(silent bool) {
	if err := w.current.Close(); err != nil && !silent {
		w.logger.DebugContext(w.ctx, "Error closing watcher", "error", err)
	}
}
