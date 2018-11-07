/*
Copyright 2018 Gravitational, Inc.

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

package local

import (
	"bytes"
	"context"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// EventsService provides events
type EventsService struct {
	*logrus.Entry
	backend backend.Backend
}

// NewEventsService returns new events service instance
func NewEventsService(b backend.Backend) *EventsService {
	return &EventsService{
		Entry:   logrus.WithFields(logrus.Fields{trace.Component: "Events"}),
		backend: b,
	}
}

// NewWatcher returns a new event watcher
func (e *EventsService) NewWatcher(ctx context.Context, watch services.Watch) (services.Watcher, error) {
	if len(watch.Kinds) == 0 {
		return nil, trace.BadParameter("global watches are not supported yet")
	}
	if len(watch.Kinds) > 1 {
		return nil, trace.BadParameter("watches on multiple objects are not supported yet")
	}
	switch watch.Kinds[0] {
	case services.KindCertAuthority:
		prefix := []byte(backend.Key(authoritiesPrefix))
		w, err := e.backend.NewWatcher(ctx, backend.Watch{
			Prefix: prefix,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return newWatcher(w, e.Entry, []parser{{prefix: prefix, parser: parseCertAuthority}}), nil
	default:
		return nil, trace.BadParameter("watcher on object kind %q is not supported", watch.Kinds[0])
	}
}

func newWatcher(backendWatcher backend.Watcher, l *logrus.Entry, parsers []parser) *watcher {
	w := &watcher{
		backendWatcher: backendWatcher,
		Entry:          l,
		parsers:        parsers,
		eventsC:        make(chan services.Event),
	}
	go w.forwardEvents()
	return w
}

type parser struct {
	prefix []byte
	parser parserFunc
}

type watcher struct {
	*logrus.Entry
	parsers        []parser
	backendWatcher backend.Watcher
	eventsC        chan services.Event
}

func (w *watcher) Error() error {
	return nil
}

func (w *watcher) parseEvent(e backend.Event) (*services.Event, error) {
	for _, p := range w.parsers {
		if bytes.HasPrefix(e.Item.Key, p.prefix) {
			resource, err := p.parser(e)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &services.Event{Type: e.Type, Resource: resource}, nil
		}
	}
	return nil, trace.NotFound("no match found for %v", e.Type)
}

func (w *watcher) forwardEvents() {
	for {
		select {
		case <-w.backendWatcher.Done():
			return
		case event := <-w.backendWatcher.Events():
			converted, err := w.parseEvent(event)
			if err != nil {
				w.Warning(err)
				continue
			}
			select {
			case w.eventsC <- *converted:
			case <-w.backendWatcher.Done():
				return
			}
		}
	}
}

// Events returns channel with events
func (w *watcher) Events() <-chan services.Event {
	return w.eventsC
}

// Done returns the channel signalling the closure
func (w *watcher) Done() <-chan struct{} {
	return w.backendWatcher.Done()
}

// Close closes the watcher and releases
// all associated resources
func (w *watcher) Close() error {
	return w.backendWatcher.Close()
}

func parseCertAuthority(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		name, err := base(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &services.ResourceHeader{
			Kind:    services.KindCertAuthority,
			Version: services.V3,
			Metadata: services.Metadata{
				Name:      string(name),
				Namespace: defaults.Namespace,
			},
		}, nil
	case backend.OpPut:
		ca, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(event.Item.Value, services.WithResourceID(event.Item.ID))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// never send private signing keys over event stream
		setSigningKeys(ca, false)
		return ca, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

// base returns last element delimited by separator
func base(key []byte) ([]byte, error) {
	parts := bytes.Split(key, []byte{backend.Separator})
	if len(parts) == 0 {
		return nil, trace.NotFound("failed parsing %v", string(key))
	}
	return parts[len(parts)-1], nil
}

type parserFunc func(i backend.Event) (services.Resource, error)
