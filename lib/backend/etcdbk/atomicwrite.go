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

// Package etcdbk implements Etcd powered backend
package etcdbk

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/gravitational/trace"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/gravitational/teleport/lib/backend"
)

func (b *EtcdBackend) AtomicWrite(ctx context.Context, condacts []backend.ConditionalAction) (revision string, err error) {
	if err := backend.ValidateAtomicWrite(condacts); err != nil {
		return "", trace.Wrap(err)
	}

	var cmps []clientv3.Cmp
	var ops []clientv3.Op
	var includesPut bool

	for _, ca := range condacts {
		key := b.prependPrefix(ca.Key)

		switch ca.Condition.Kind {
		case backend.KindWhatever:
			// no comparison to assert
		case backend.KindExists:
			cmps = append(cmps, clientv3.Compare(clientv3.CreateRevision(key), "!=", 0))
		case backend.KindNotExists:
			cmps = append(cmps, clientv3.Compare(clientv3.CreateRevision(key), "=", 0))
		case backend.KindRevision:
			rev, err := fromBackendRevision(ca.Condition.Revision)
			if err != nil {
				// malformed revisions are considered a kind of failed condition since they indicate that
				// the supplied revision did not originate from a preceding read from this backend.
				return "", trace.Wrap(backend.ErrConditionFailed)
			}

			cmps = append(cmps, clientv3.Compare(clientv3.CreateRevision(key), "!=", 0))
			cmps = append(cmps, clientv3.Compare(clientv3.ModRevision(key), "=", rev))
		default:
			return "", trace.BadParameter("unexpected condition kind %v in conditional action against key %q", ca.Condition.Kind, ca.Key)
		}

		switch ca.Action.Kind {
		case backend.KindNop:
			// no action to be taken
		case backend.KindPut:
			includesPut = true
			var opts []clientv3.OpOption
			var lease backend.Lease
			if !ca.Action.Item.Expires.IsZero() {
				if err := b.setupLease(ctx, ca.Action.Item, &lease, &opts); err != nil {
					return "", trace.Wrap(err)
				}
			}

			ops = append(ops, clientv3.OpPut(key, base64.StdEncoding.EncodeToString(ca.Action.Item.Value), opts...))
		case backend.KindDelete:
			ops = append(ops, clientv3.OpDelete(key))
		default:
			return "", trace.BadParameter("unexpected action kind %v in conditional action against key %q", ca.Action.Kind, ca.Key)
		}
	}

	start := b.clock.Now()
	re, err := b.clients.Next().Txn(ctx).
		If(cmps...).
		Then(ops...).
		Commit()
	txLatencies.Observe(time.Since(start).Seconds())
	txRequests.Inc()
	if err != nil {
		return "", trace.Wrap(convertErr(err))
	}

	if !re.Succeeded {
		return "", trace.Wrap(backend.ErrConditionFailed)
	}

	// all etcd writes have a corresponding revision, but most other backends only
	// have a concept of a put revision. in the interest of consistency, we explicitly
	// omit returning revision from atomic writes that did not contain any puts.
	if !includesPut {
		return "", nil
	}

	return toBackendRevision(re.Header.Revision), nil
}
