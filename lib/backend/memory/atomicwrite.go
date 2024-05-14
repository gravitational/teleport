/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package memory

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

// AtomicWrite executes a batch of conditional actions atomically s.t. all actions happen if all
// conditions are met, but no actions happen if any condition fails to hold.
func (m *Memory) AtomicWrite(ctx context.Context, condacts []backend.ConditionalAction) (revision string, err error) {
	if err := backend.ValidateAtomicWrite(condacts); err != nil {
		return "", trace.Wrap(err)
	}

	m.Lock()
	defer m.Unlock()

	m.removeExpired()

	revision = backend.CreateRevision()
	var includesPut bool
	var events []backend.Event

	for _, ca := range condacts {
		switch ca.Condition.Kind {
		case backend.KindWhatever:
			// no comparison to assert
		case backend.KindExists:
			if !m.tree.Has(&btreeItem{Item: backend.Item{Key: ca.Key}}) {
				return "", trace.Wrap(backend.ErrConditionFailed)
			}
		case backend.KindNotExists:
			if m.tree.Has(&btreeItem{Item: backend.Item{Key: ca.Key}}) {
				return "", trace.Wrap(backend.ErrConditionFailed)
			}
		case backend.KindRevision:
			item, found := m.tree.Get(&btreeItem{Item: backend.Item{
				Key: ca.Key,
			}})
			if !found || item.Item.Revision != ca.Condition.Revision {
				return "", trace.Wrap(backend.ErrConditionFailed)
			}
		default:
			return "", trace.BadParameter("unexpected condition kind %v in conditional action against key %q", ca.Condition.Kind, ca.Key)
		}

		switch ca.Action.Kind {
		case backend.KindNop:
			// no action to be taken
		case backend.KindPut:
			includesPut = true
			event := backend.Event{
				Type: types.OpPut,
				Item: ca.Action.Item,
			}
			event.Item.Key = ca.Key
			event.Item.ID = m.generateID()
			event.Item.Revision = revision
			events = append(events, event)
		case backend.KindDelete:
			if m.tree.Has(&btreeItem{Item: backend.Item{Key: ca.Key}}) {
				// only bother creating delete event if there is actually an
				// item to delete.
				event := backend.Event{
					Type: types.OpDelete,
					Item: backend.Item{
						Key: ca.Key,
					},
				}
				events = append(events, event)
			}
		default:
			return "", trace.BadParameter("unexpected action kind %v in conditional action against key %q", ca.Action.Kind, ca.Key)
		}
	}

	for _, event := range events {
		m.processEvent(event)
		if !m.EventsOff {
			m.buf.Emit(event)
		}
	}

	if !includesPut {
		return "", nil
	}

	return revision, nil
}
