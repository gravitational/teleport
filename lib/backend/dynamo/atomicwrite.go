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

package dynamo

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	maxTxnAttempts        = 32
	txnAttemptLogInterval = 8
)

var (
	existsExpr          = "attribute_exists(FullPath)"
	notExistsExpr       = "attribute_not_exists(FullPath)"
	revisionExpr        = "Revision = :rev AND attribute_exists(FullPath)"
	missingRevisionExpr = "attribute_not_exists(Revision) AND attribute_exists(FullPath)"
)

func (b *Backend) AtomicWrite(ctx context.Context, condacts []backend.ConditionalAction) (revision string, err error) {
	if err := backend.ValidateAtomicWrite(condacts); err != nil {
		return "", trace.Wrap(err)
	}

	revision = backend.CreateRevision()

	tableName := aws.String(b.TableName)

	var txnItems []*dynamodb.TransactWriteItem
	var includesPut bool

	for _, ca := range condacts {
		var condExpr *string
		var exprAttrValues map[string]*dynamodb.AttributeValue

		switch ca.Condition.Kind {
		case backend.KindWhatever:
			// no comparison to assert
		case backend.KindExists:
			condExpr = &existsExpr
		case backend.KindNotExists:
			condExpr = &notExistsExpr
		case backend.KindRevision:
			switch ca.Condition.Revision {
			case "":
				// dynamo backend doesn't support empty revision values, caller is working with outdated state.
				return "", trace.Wrap(backend.ErrConditionFailed)
			case backend.BlankRevision:
				// item has not been modified since the introduction of the revision attr
				condExpr = &missingRevisionExpr
			default:
				// revision is expected to be present and well-defined
				condExpr = &revisionExpr
				exprAttrValues = map[string]*dynamodb.AttributeValue{
					":rev": {S: aws.String(ca.Condition.Revision)},
				}
			}
		default:
			return "", trace.BadParameter("unexpected condition kind %v in conditional action against key %q", ca.Condition.Kind, ca.Key)
		}

		fullPath := prependPrefix(ca.Key)

		var txnItem dynamodb.TransactWriteItem

		switch ca.Action.Kind {
		case backend.KindNop:
			av, err := dynamodbattribute.MarshalMap(keyLookup{
				HashKey:  hashKey,
				FullPath: fullPath,
			})
			if err != nil {
				return "", trace.Wrap(err)
			}

			// morally this should be a dynamodb.ConditionCheck transaction
			// item, but condition checks use the dynamodb:ConditionCheckItem
			// permission, which wasn't documented in v15 and earlier; an update
			// operation in a TransactWrite costs the same in terms of write
			// capacity as a condition check, so we update the potentially
			// non-existent item by removing a top-level field that we know
			// doesn't exist anyway
			txnItem.Update = &dynamodb.Update{
				ConditionExpression:       condExpr,
				ExpressionAttributeValues: exprAttrValues,
				Key:                       av,
				TableName:                 tableName,

				UpdateExpression: aws.String("REMOVE NotAField"),
			}

		case backend.KindPut:
			includesPut = true
			r := record{
				HashKey:   hashKey,
				FullPath:  fullPath,
				Value:     ca.Action.Item.Value,
				Timestamp: time.Now().UTC().Unix(),
				ID:        time.Now().UTC().UnixNano(),
				Revision:  revision,
			}
			if !ca.Action.Item.Expires.IsZero() {
				r.Expires = aws.Int64(ca.Action.Item.Expires.UTC().Unix())
			}

			av, err := dynamodbattribute.MarshalMap(r)
			if err != nil {
				return "", trace.Wrap(err)
			}

			txnItem.Put = &dynamodb.Put{
				ConditionExpression:       condExpr,
				ExpressionAttributeValues: exprAttrValues,
				Item:                      av,
				TableName:                 tableName,
			}
		case backend.KindDelete:
			av, err := dynamodbattribute.MarshalMap(keyLookup{
				HashKey:  hashKey,
				FullPath: fullPath,
			})
			if err != nil {
				return "", trace.Wrap(err)
			}

			txnItem.Delete = &dynamodb.Delete{
				ConditionExpression:       condExpr,
				ExpressionAttributeValues: exprAttrValues,
				Key:                       av,
				TableName:                 tableName,
			}

		default:
			return "", trace.BadParameter("unexpected action kind %v in conditional action against key %q", ca.Action.Kind, ca.Key)
		}

		txnItems = append(txnItems, &txnItem)
	}

	// dynamo cancels overlapping transactions without evaluating their conditions. the AtomicWrite API is expected to only fail
	// if one or more conditions fail to hold when (barring unrelated errors like network interruptions). we therefore perform a
	// fairly large number of internal retry attempts if cancellation occurs due to conflict.

	// retry is lazily initialized as-needed.
	var retry *retryutils.RetryV2
TxnLoop:
	for i := 0; i < maxTxnAttempts; i++ {
		if i != 0 {
			if retry == nil {
				// ideally we want one of the concurrently canceled transactions to retry immediately, with the rest holding back. since we
				// can't control wether that happens, the next best thing is to configure our backoff to use exponential scaling + full jitter,
				// which strikes a nice balance between retrying quickly when under low contention, and rapidly spreading out retries when under
				// high contention.
				retry, err = retryutils.NewRetryV2(retryutils.RetryV2Config{
					First:  time.Millisecond * 16,
					Driver: retryutils.NewExponentialDriver(time.Millisecond * 16),
					Max:    time.Millisecond * 1024,
					Jitter: utils.FullJitter,
				})

				if err != nil {
					return "", trace.Errorf("failed to setup retry for atomic write: %v (this is a bug)", err)
				}
			}
			retry.Inc()
			select {
			case <-retry.After():
			case <-ctx.Done():
				return "", trace.Wrap(ctx.Err())
			}
		}

		// execute the transaction
		_, err = b.svc.TransactWriteItemsWithContext(ctx, &dynamodb.TransactWriteItemsInput{
			TransactItems: txnItems,
		})
		if err != nil {
			txnErr := &dynamodb.TransactionCanceledException{}
			if !errors.As(err, &txnErr) {
				return "", trace.Errorf("unexpected error during atomic write: %v", err)
			}

			// cancellation reasons are reported as an ordered list. for our purposese,
			// a condition failure for any key should result in ErrConditionFailed, and
			// a conflict should result in an internal retry. All other possible errors
			// are unexpected and should be bubbled up to the caller.
			var conditionFailed bool
			var txnConflict bool
			for _, reason := range txnErr.CancellationReasons {
				if reason.Code == nil {
					continue
				}

				switch *reason.Code {
				case dynamodb.BatchStatementErrorCodeEnumConditionalCheckFailed:
					conditionFailed = true
				case dynamodb.BatchStatementErrorCodeEnumTransactionConflict:
					txnConflict = true
				}
			}

			switch {
			case conditionFailed:
				return "", trace.Wrap(backend.ErrConditionFailed)
			case txnConflict:
				// dynamodb cancels transactions that overlap even if their conditions/actions don't conflict, so we need to retry
				// in order to determine if our conditions actually hold or not.
				continue TxnLoop
			}

			// if we get here, the error was a transaction cancellation, but not for any reason we expect to
			// see during normal healthy operation. Extract the reason code and bubble it up to the caller.
			var codes []string
			for _, reason := range txnErr.CancellationReasons {
				if reason.Code == nil {
					continue
				}

				codes = append(codes, *reason.Code)
			}

			slices.Sort(codes)
			codes = slices.Compact(codes)

			return "", trace.Errorf("unexpected error during atomic write: %v (reason(s): %s)", err, strings.Join(codes, ","))
		}

		if i > 0 {
			backend.AtomicWriteContention.WithLabelValues(teleport.ComponentDynamoDB).Add(float64(i))
		}

		if n := i + 1; n > 2 {
			// if we retried more than once, txn experienced non-trivial conflict and we should warn about it. Infrequent warnings of this kind
			// are nothing to be concerned about, but high volumes may indicate that an automatic process is creating excessive conflicts.
			b.Warnf("AtomicWrite retried %d times due to dynamodb transaction conflicts. Some conflict is expected, but persistent conflict warnings may indicate an unhealthy state.", n)
		}

		if !includesPut {
			// revision is only meaningful in the context of put operations
			return "", nil
		}

		return revision, nil
	}

	var keys [][]byte
	for _, ca := range condacts {
		keys = append(keys, ca.Key)
	}

	b.Errorf("AtomicWrite failed, dynamodb transaction experienced too many conflicts. keys=%s", bytes.Join(keys, []byte(",")))

	return "", trace.Errorf("dynamodb transaction experienced too many conflicts")
}
