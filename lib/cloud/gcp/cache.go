// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package gcp

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
)

type clientConstructor[T any] func(context.Context) (T, error)

// clientCache is a struct that holds a cloud client that will only be
// initialized once.
type clientCache[T any] struct {
	makeClient clientConstructor[T]
	client     T
	err        error
	once       sync.Once
}

// newClientCache creates a new client cache.
func newClientCache[T any](makeClient clientConstructor[T]) *clientCache[T] {
	return &clientCache[T]{makeClient: makeClient}
}

// GetClient gets the client, initializing it if necessary.
func (c *clientCache[T]) GetClient(ctx context.Context) (T, error) {
	c.once.Do(func() {
		c.client, c.err = c.makeClient(ctx)
	})
	return c.client, trace.Wrap(c.err)
}
