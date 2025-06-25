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
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
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

	var txnItems []types.TransactWriteItem
	var includesPut bool

	for _, ca := range condacts {
		var condExpr *string
		var exprAttrValues map[string]types.AttributeValue

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
				exprAttrValues = map[string]types.AttributeValue{
					":rev": &types.AttributeValueMemberS{Value: ca.Condition.Revision},
				}
			}
		default:
			return "", trace.BadParameter("unexpected condition kind %v in conditional action against key %q", ca.Condition.Kind, ca.Key)
		}

		fullPath := prependPrefix(ca.Key)

		var txnItem types.TransactWriteItem

		switch ca.Action.Kind {
		case backend.KindNop:
			av, err := attributevalue.MarshalMap(keyLookup{
				HashKey:  hashKey,
				FullPath: fullPath,
			})
			if err != nil {
				return "", trace.Wrap(err)
			}

			txnItem.ConditionCheck = &types.ConditionCheck{
				ConditionExpression:       condExpr,
				ExpressionAttributeValues: exprAttrValues,
				Key:                       av,
				TableName:                 tableName,
			}

		case backend.KindPut:
			includesPut = true
			r := record{
				HashKey:   hashKey,
				FullPath:  fullPath,
				Value:     ca.Action.Item.Value,
				Timestamp: time.Now().UTC().Unix(),
				Revision:  revision,
			}
			if !ca.Action.Item.Expires.IsZero() {
				r.Expires = aws.Int64(ca.Action.Item.Expires.UTC().Unix())
			}

			av, err := attributevalue.MarshalMap(r)
			if err != nil {
				return "", trace.Wrap(err)
			}

			txnItem.Put = &types.Put{
				ConditionExpression:       condExpr,
				ExpressionAttributeValues: exprAttrValues,
				Item:                      av,
				TableName:                 tableName,
			}
		case backend.KindDelete:
			av, err := attributevalue.MarshalMap(keyLookup{
				HashKey:  hashKey,
				FullPath: fullPath,
			})
			if err != nil {
				return "", trace.Wrap(err)
			}

			txnItem.Delete = &types.Delete{
				ConditionExpression:       condExpr,
				ExpressionAttributeValues: exprAttrValues,
				Key:                       av,
				TableName:                 tableName,
			}

		default:
			return "", trace.BadParameter("unexpected action kind %v in conditional action against key %q", ca.Action.Kind, ca.Key)
		}

		txnItems = append(txnItems, txnItem)
	}

	// dynamo cancels overlapping transactions without evaluating their conditions. the AtomicWrite API is expected to only fail
	// if one or more conditions fail to hold when (barring unrelated errors like network interruptions). we therefore perform a
	// fairly large number of internal retry attempts if cancellation occurs due to conflict.

	// retry is lazily initialized as-needed.
	var retry *retryutils.RetryV2
TxnLoop:
	for i := range maxTxnAttempts {
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
					Jitter: retryutils.FullJitter,
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
		_, err = b.svc.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
			TransactItems: txnItems,
		})
		if err != nil {
			var txnErr *types.TransactionCanceledException
			if !errors.As(err, &txnErr) {
				if s := err.Error(); strings.Contains(s, "AccessDenied") && strings.Contains(s, "dynamodb:ConditionCheckItem") {
					b.logger.WarnContext(ctx, "AtomicWrite failed with error that may indicate dynamodb is missing the required dynamodb:ConditionCheckItem permission (this permission is now required for teleport v16 and later). Consider updating your IAM policy to include this permission.", "error", err)
					return "", trace.Errorf("teleport is missing required AWS permission dynamodb:ConditionCheckItem, please contact your administrator to update permissions")
				}
				return "", trace.Errorf("unexpected error during atomic write: %v", err)
			}

			// cancellation reasons are reported as an ordered list. for our purposese,
			// a condition failure for any key should result in ErrConditionFailed, and
			// a conflict should result in an internal retry. All other possible errors
			// are unexpected and should be bubbled up to the caller.
			var conditionFailed bool
			var txnConflict bool
			for _, reason := range txnErr.CancellationReasons {
				code := aws.ToString(reason.Code)
				switch types.BatchStatementErrorCodeEnum(code) {
				case types.BatchStatementErrorCodeEnumConditionalCheckFailed:
					conditionFailed = true
				case types.BatchStatementErrorCodeEnumTransactionConflict:
					txnConflict = true
				case "":
					continue
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
			b.logger.WarnContext(ctx, "AtomicWrite retried due to dynamodb transaction conflicts. Some conflict is expected, but persistent conflict warnings may indicate an unhealthy state.", "retry_attempts", n)
		}

		if !includesPut {
			// revision is only meaningful in the context of put operations
			return "", nil
		}

		return revision, nil
	}

	var keys []string
	for _, ca := range condacts {
		keys = append(keys, ca.Key.String())
	}

	b.logger.ErrorContext(ctx, "AtomicWrite failed, dynamodb transaction experienced too many conflicts", "keys", strings.Join(keys, ","))

	return "", trace.Errorf("dynamodb transaction experienced too many conflicts")
}
