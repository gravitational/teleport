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

package firestore

import (
	"context"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/gravitational/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
)

func (b *Backend) AtomicWrite(ctx context.Context, condacts []backend.ConditionalAction) (revision string, err error) {
	if err := backend.ValidateAtomicWrite(condacts); err != nil {
		return "", trace.Wrap(err)
	}

	revision = createRevisionV2()
	var includesPut bool
	var n int

	err = b.svc.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		n++

		// perform all condition evaluations first
		for _, ca := range condacts {
			docRef := b.svc.Collection(b.CollectionName).Doc(b.keyToDocumentID(ca.Key))
			switch ca.Condition.Kind {
			case backend.KindWhatever:
				// no comparison to assert
			case backend.KindExists:
				// verify that document exists
				_, err := tx.Get(docRef)
				if err != nil {
					if status.Code(err) == codes.NotFound {
						return trace.Wrap(backend.ErrConditionFailed)
					}
					return trace.Wrap(ConvertGRPCError(err))
				}
			case backend.KindNotExists:
				// verify that document does not exist
				_, err := tx.Get(docRef)
				if err == nil {
					return trace.Wrap(backend.ErrConditionFailed)
				}
				if status.Code(err) != codes.NotFound {
					return trace.Wrap(ConvertGRPCError(err))
				}
			case backend.KindRevision:
				// verfiy that document exposes exact expected revision
				docSnap, err := tx.Get(docRef)
				if err != nil {
					if status.Code(err) == codes.NotFound {
						return trace.Wrap(backend.ErrConditionFailed)
					}
					return trace.Wrap(ConvertGRPCError(err))
				}
				if isRevisionV2(ca.Condition.Revision) {
					existingRec, err := newRecordFromDoc(docSnap)
					if err != nil {
						return trace.Wrap(err)
					}

					if existingRec.RevisionV2 != ca.Condition.Revision {
						return trace.Wrap(backend.ErrConditionFailed)
					}
				} else {
					expectedRev, err := fromRevisionV1(ca.Condition.Revision)
					if err != nil {
						return trace.Wrap(backend.ErrConditionFailed)
					}

					if !docSnap.UpdateTime.Equal(expectedRev) {
						return trace.Wrap(backend.ErrConditionFailed)
					}
				}
			default:
				return trace.BadParameter("unexpected condition kind %v in conditional action against key %q", ca.Condition.Kind, ca.Key)
			}
		}

		// if we get this far, all conditions hold for this txn context. perform all writes.
		for _, ca := range condacts {
			docRef := b.svc.Collection(b.CollectionName).Doc(b.keyToDocumentID(ca.Key))

			switch ca.Action.Kind {
			case backend.KindNop:
				// no action to be taken
			case backend.KindPut:
				includesPut = true
				// create shallow copy of item to avoid mutating condacts
				item := ca.Action.Item
				item.Key = ca.Key
				item.Revision = revision
				newRec := newRecord(item, b.clock)
				if err := tx.Set(docRef, newRec); err != nil {
					return trace.Wrap(ConvertGRPCError(err))
				}
			case backend.KindDelete:
				if err := tx.Delete(docRef); err != nil {
					return trace.Wrap(ConvertGRPCError(err))
				}
			default:
				return trace.BadParameter("unexpected action kind %v in conditional action against key %q", ca.Action.Kind, ca.Key)
			}
		}

		return nil
	}, firestore.MaxAttempts(maxTxnAttempts))

	if err != nil {
		if status.Code(err) == codes.Aborted {
			var keys []string
			for _, ca := range condacts {
				keys = append(keys, ca.Key.String())
			}
			b.logger.ErrorContext(ctx, "AtomicWrite failed, firestore experienced too many txn rollbacks.", "keys", strings.Join(keys, ","))
			// RunTransaction does not officially document what error is returned if MaxAttempts is exceeded,
			// but as currently implemented it should simply bubble up the Aborted error from the most recent
			// failed commit attempt.
			return "", trace.Errorf("too many attempts during firestore txn for AtomicWrite")
		}

		return "", trace.Wrap(ConvertGRPCError(err))
	}

	if n > 1 {
		backend.AtomicWriteContention.WithLabelValues(teleport.ComponentFirestore).Add(float64(n - 1))
	}

	if n > 2 {
		// if we retried more than once, txn experienced non-trivial contention and we should warn about it. Infrequent warnings of this kind
		// are nothing to be concerned about, but high volumes may indicate than an automatic process is creating excessive conflicts.
		b.logger.WarnContext(ctx, "AtomicWrite retried due to firestore txn rollbacks. Some rollbacks are expected, but persistent rollback warnings may indicate an unhealthy state.", "retry_attempts", n)
	}

	// atomic writes don't have a meaningful concept of revision outside of put operations
	if !includesPut {
		return "", nil
	}

	return revision, nil
}
