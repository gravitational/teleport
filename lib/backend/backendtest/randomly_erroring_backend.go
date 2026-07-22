// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backendtest

import (
	"context"
	"errors"
	"iter"
	"math/rand/v2"

	"github.com/gravitational/teleport/lib/backend"
)

var ErrRandomBackend = errors.New("RandomlyErroringBackend error")

// RandomlyErroringBackend wraps Backend reading methods making them fail 50% of the time with
// [ErrRandomBackend].
type RandomlyErroringBackend struct {
	backend.Backend
}

// NewRandomlyErroringBackend creates new instance of RandomlyErroringBackend for a given backend.
func NewRandomlyErroringBackend(b backend.Backend) *RandomlyErroringBackend {
	return &RandomlyErroringBackend{
		Backend: b,
	}
}

func (b *RandomlyErroringBackend) Get(ctx context.Context, key backend.Key) (*backend.Item, error) {
	if rand.IntN(2) == 0 {
		return nil, ErrRandomBackend
	}
	return b.Backend.Get(ctx, key)
}

func (b *RandomlyErroringBackend) Items(ctx context.Context, params backend.ItemsParams) iter.Seq2[backend.Item, error] {
	return func(yield func(backend.Item, error) bool) {
		for item, err := range b.Backend.Items(ctx, params) {
			var ok bool
			if rand.IntN(2) == 0 {
				ok = yield(backend.Item{}, ErrRandomBackend)
			} else {
				ok = yield(item, err)
			}
			if !ok {
				return
			}
		}
	}
}

func (b *RandomlyErroringBackend) GetRange(ctx context.Context, startKey, endKey backend.Key, limit int) (*backend.GetResult, error) {
	if rand.IntN(2) == 0 {
		return nil, ErrRandomBackend
	}
	return b.Backend.GetRange(ctx, startKey, endKey, limit)
}
