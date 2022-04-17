/*
Copyright 2022 Gravitational, Inc.

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

package sqlbk

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestRetryTx(t *testing.T) {
	ctx := context.Background()
	b := &Backend{Config: &Config{RetryTimeout: time.Minute}}
	t.Run("Return without calling txFn when begin() returns a failed Tx", func(t *testing.T) {
		begin := func(context.Context) Tx {
			return &testTx{err: errFailedTx}
		}
		err := b.retryTx(ctx, begin, nil)
		require.ErrorIs(t, err, errFailedTx)
	})
	t.Run("Commit when txFn returns nil", func(t *testing.T) {
		var tx testTx
		begin := func(context.Context) Tx {
			return &tx
		}
		txFn := func(Tx) error {
			return nil
		}
		err := b.retryTx(ctx, begin, txFn)
		require.Nil(t, err)
		require.Equal(t, tx.committed, 1)
	})
	t.Run("Rollback when txFn returns an error", func(t *testing.T) {
		var tx testTx
		begin := func(context.Context) Tx {
			return &tx
		}
		txFn := func(Tx) error {
			return errFailedTx
		}
		err := b.retryTx(ctx, begin, txFn)
		require.ErrorIs(t, err, errFailedTx)
		require.ErrorIs(t, tx.rollbackErr, errFailedTx)
	})
	t.Run("Return Tx error when not ErrRetry", func(t *testing.T) {
		var tx testTx
		begin := func(context.Context) Tx {
			return &tx
		}
		txFn := func(Tx) error {
			tx.err = errFailedTx
			return nil
		}
		err := b.retryTx(ctx, begin, txFn)
		require.ErrorIs(t, err, errFailedTx)
		require.Nil(t, tx.rollbackErr)
	})
	t.Run("Rollback when context is canceled during delay", func(t *testing.T) {
		var tx testTx
		ctx, cancel := context.WithCancel(ctx)
		begin := func(context.Context) Tx {
			return &tx
		}
		txFn := func(Tx) error {
			tx.err = ErrRetry
			cancel()
			return nil
		}
		b.Config.RetryDelayPeriod = time.Minute
		err := b.retryTx(ctx, begin, txFn)
		require.ErrorIs(t, err, context.Canceled)
	})
	t.Run("fnTx is retried", func(t *testing.T) {
		var i int
		var txns [2]testTx
		begin := func(context.Context) Tx {
			return &txns[i]
		}
		txFn := func(Tx) error {
			if i == 0 {
				txns[i].err = ErrRetry
			}
			i++
			return nil
		}
		b.Config.RetryDelayPeriod = time.Millisecond
		err := b.retryTx(ctx, begin, txFn)
		require.Nil(t, err)
		require.Equal(t, i, 2)
		require.Equal(t, ErrRetry, txns[0].err)
		require.Nil(t, txns[1].rollbackErr)
		require.Equal(t, txns[0].committed, 0)
		require.Equal(t, txns[1].committed, 1)
	})
}

var errFailedTx = trace.BadParameter("failedTx")

// testTx is a Tx that exposes the transaction err
// and tracks calls to Commit and Rollback.
type testTx struct {
	Tx
	err         error // Transaction error
	committed   int   // Incremented each time Commit is called.
	rollbackErr error // Set with err passed to Rollback.
}

func (tx *testTx) Err() error {
	return tx.err
}

func (tx *testTx) Commit() error {
	if tx.err == nil {
		tx.committed++
	}
	return tx.err
}

func (tx *testTx) Rollback(err error) error {
	tx.rollbackErr = err
	return err
}

var _ backend.Backend = (*Backend)(nil)
