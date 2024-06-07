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

package lite

import (
	"context"
	"database/sql"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/backend"
)

func (l *Backend) AtomicWrite(ctx context.Context, condacts []backend.ConditionalAction) (revision string, err error) {
	if err := backend.ValidateAtomicWrite(condacts); err != nil {
		return "", trace.Wrap(err)
	}

	revision = backend.CreateRevision()
	var includesPut bool

	err = l.inTransaction(ctx, func(tx *sql.Tx) error {
		for _, ca := range condacts {
			switch ca.Condition.Kind {
			case backend.KindWhatever:
				// no comparison to assert
			case backend.KindExists:
				var item backend.Item
				if err := l.getInTransaction(ctx, ca.Key, tx, &item); err != nil {
					if trace.IsNotFound(err) {
						return trace.Wrap(backend.ErrConditionFailed)
					}
					return trace.Wrap(err)
				}
			case backend.KindNotExists:
				var item backend.Item
				err := l.getInTransaction(ctx, ca.Key, tx, &item)
				if !trace.IsNotFound(err) {
					if err == nil {
						return trace.Wrap(backend.ErrConditionFailed)
					}
					return trace.Wrap(err)
				}
			case backend.KindRevision:
				var item backend.Item
				if err := l.getInTransaction(ctx, ca.Key, tx, &item); err != nil {
					if trace.IsNotFound(err) {
						return trace.Wrap(backend.ErrConditionFailed)
					}
					return trace.Wrap(err)
				}
				if item.Revision != ca.Condition.Revision {
					return trace.Wrap(backend.ErrConditionFailed)
				}
			default:
				return trace.BadParameter("unexpected condition kind %v in conditional action against key %q", ca.Condition.Kind, ca.Key)
			}
		}

		for _, ca := range condacts {
			switch ca.Action.Kind {
			case backend.KindNop:
				// no action to be taken
			case backend.KindPut:
				includesPut = true
				// modify a shallow copy of item to avoid mutating condacts.
				item := ca.Action.Item
				item.Key = ca.Key
				item.Revision = revision
				if err := l.putInTransaction(ctx, item, tx); err != nil {
					return trace.Wrap(err)
				}
			case backend.KindDelete:
				if err := l.deleteInTransaction(ctx, ca.Key, tx); err != nil && !trace.IsNotFound(err) {
					return trace.Wrap(err)
				}
			default:
				return trace.BadParameter("unexpected action kind %v in conditional action against key %q", ca.Action.Kind, ca.Key)
			}
		}

		return nil
	})

	if err != nil {
		return "", trace.Wrap(err)
	}

	if !includesPut {
		return "", nil
	}

	return revision, nil
}
