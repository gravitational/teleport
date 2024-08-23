/*
Copyright 2023 Gravitational, Inc.

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

package generic

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/utils"
)

type noncedResource struct {
	types.ResourceHeader
	Nonce uint64 `json:"nonce"`
}

func (r *noncedResource) GetNonce() uint64 {
	return r.Nonce
}

func (r *noncedResource) WithNonce(nonce uint64) any {
	c := *r
	c.Nonce = nonce
	return &c
}

func newNoncedResource(name string, nonce uint64) *noncedResource {
	return &noncedResource{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: name,
			},
		},
		Nonce: nonce,
	}
}

func fastGetResource[T types.Resource](ctx context.Context, bk backend.Backend, key backend.Key) (T, error) {
	var value T

	item, err := bk.Get(ctx, key)
	if err != nil {
		return value, trace.Wrap(err)
	}

	if err := utils.FastUnmarshal(item.Value, &value); err != nil {
		return value, trace.Errorf("failed to unmarshal resource at %q: %v", key, err)
	}

	if item.Expires.IsZero() {
		value.SetExpiry(time.Time{})
	} else {
		value.SetExpiry(item.Expires.UTC())
	}

	return value, nil
}

// TestNonceBasics verifies basic nonce behaviors.
func TestNonceBasics(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	// nonce of 1 is an "update", but resource does not exist yet
	err = FastUpdateNonceProtectedResource(ctx, bk, []byte("k1"), newNoncedResource("r1", 1))
	require.ErrorIs(t, err, ErrNonceViolation)

	// nonce of 0 is a valid "create".
	err = FastUpdateNonceProtectedResource(ctx, bk, []byte("k1"), newNoncedResource("r1", 0))
	require.NoError(t, err)

	// subsequent calls with nonce of 0 fail.
	err = FastUpdateNonceProtectedResource(ctx, bk, []byte("k1"), newNoncedResource("r1", 0))
	require.ErrorIs(t, err, ErrNonceViolation)

	// nonce of 1 is now a valid update
	err = FastUpdateNonceProtectedResource(ctx, bk, []byte("k1"), newNoncedResource("r1", 1))
	require.NoError(t, err)

	// loading and then re-inserting should always work since nonce is incremented internally
	for i := 0; i < 10; i++ {
		rsc, err := fastGetResource[*noncedResource](ctx, bk, []byte("k1"))
		require.NoError(t, err)

		err = FastUpdateNonceProtectedResource(ctx, bk, []byte("k1"), rsc)
		require.NoError(t, err)
	}

	// sanity check: nonce incremented expected number of times
	err = FastUpdateNonceProtectedResource(ctx, bk, []byte("k1"), newNoncedResource("r1", 12))
	require.NoError(t, err)

	// max uint64 "forces" update
	err = FastUpdateNonceProtectedResource(ctx, bk, []byte("k1"), newNoncedResource("r1", math.MaxUint64))
	require.NoError(t, err)

	// forced update correctly conflicts with what would normally be the "next" valid nonce.
	err = FastUpdateNonceProtectedResource(ctx, bk, []byte("k1"), newNoncedResource("r1", 13))
	require.ErrorIs(t, err, ErrNonceViolation)

	// forced update correctly incremented nonce by 1
	err = FastUpdateNonceProtectedResource(ctx, bk, []byte("k1"), newNoncedResource("r1", 14))
	require.NoError(t, err)

	// max uint64 "forces" update for nonexistent resources too
	err = FastUpdateNonceProtectedResource(ctx, bk, []byte("k2"), newNoncedResource("r2", math.MaxUint64))
	require.NoError(t, err)

	// forced update correctly sets new nonce to 1
	err = FastUpdateNonceProtectedResource(ctx, bk, []byte("k2"), newNoncedResource("r2", 1))
	require.NoError(t, err)
}

// TestNonceParallelism verifies expected nonce behavior under high contention.
func TestNonceParallelism(t *testing.T) {
	// note: in theory a higher number of goroutines with a lower number of updates per goroutine
	// would be a better test case. unfortunately, that configuration seems to cause some serious perf degredation
	// on resource-starved test machines. possibly because the mutex goes into starvation mode,
	// which makes it "round robin" across its waiters, which is sub-optimal for operations like
	// compare-and-swap which need to acquire the backend mutex multiple times in quick succession (this
	// is just a guess based on examining tracebacks).
	const routines = 4
	const updates = 512

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	errch := make(chan error, 1)

	fail := func(err error) {
		select {
		case errch <- err:
		default:
		}
	}

	var wg sync.WaitGroup

	var violations atomic.Uint64

	key := "key"
	name := "rsc"

	for r := 0; r < routines; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()

			rem := updates

			for rem > 0 {
				rsc, err := fastGetResource[*noncedResource](ctx, bk, []byte(key))
				if err != nil && !trace.IsNotFound(err) {
					fail(err)
					return
				}

				if rsc == nil {
					// resource does not exist yet, start from 0
					rsc = newNoncedResource(name, 0)
				}

				err = FastUpdateNonceProtectedResource(ctx, bk, []byte(key), rsc)

				if err != nil {
					if errors.Is(err, ErrNonceViolation) {
						violations.Add(1)
						// concurrently modified, try again
						continue
					}
					fail(err)
					return
				}

				rem--
			}
		}(r)
	}

	wg.Wait()

	// verify that none of the writer goroutines hit an unexpected error
	close(errch)
	require.NoError(t, <-errch)

	// load resource and verify that we hit our exact expected number of updates
	rsc, err := fastGetResource[*noncedResource](ctx, bk, []byte(key))
	require.NoError(t, err)
	require.Equal(t, routines*updates, int(rsc.Nonce))

	// sanity-check: test *must* have hit some nonce violations
	require.Greater(t, int(violations.Load()), 0)
}
