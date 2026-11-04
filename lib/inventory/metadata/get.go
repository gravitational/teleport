/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package metadata

import (
	"context"
	"sync/atomic"

	"github.com/gravitational/trace"
)

// Getter fetches the instance metadata. See [Fetcher.Get].
type Getter func(ctx context.Context) (*Metadata, error)

// FetcherConfig configures a [Fetcher].
type FetcherConfig struct {
	// CloudEnvironmentDiscovery controls which cloud environment checks are
	// performed. The zero value enables every check.
	CloudEnvironmentDiscovery CloudEnvironmentDiscoveryConfig
}

// CheckAndSetDefaults validates the configuration.
func (c *FetcherConfig) CheckAndSetDefaults() error {
	return trace.Wrap(c.CloudEnvironmentDiscovery.CheckAndSetDefaults())
}

// Fetcher fetches and caches the instance metadata. The metadata is fetched
// at most once per Fetcher, on the first call to [Fetcher.Get].
type Fetcher struct {
	cfg FetcherConfig

	// metadata is the cached instance metadata, valid once ready is closed.
	metadata *Metadata
	// ready is closed when the metadata has been fetched.
	ready chan struct{}
	// fetched is used to ensure that the metadata is fetched at most once.
	fetched atomic.Bool
}

// NewFetcher creates a [Fetcher] with the given configuration.
func NewFetcher(cfg FetcherConfig) (*Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return newFetcher(cfg), nil
}

func newFetcher(cfg FetcherConfig) *Fetcher {
	return &Fetcher{
		cfg:   cfg,
		ready: make(chan struct{}),
	}
}

// Get fetches the instance metadata. The first call can take some time as all
// metadata will be retrieved. The resulting metadata is cached, so subsequent
// calls will be fast. The return value of Get might be shared between callers
// and should not be modified. If the cached metadata is ready, it will be
// returned successfully even if the context is done.
func (f *Fetcher) Get(ctx context.Context) (*Metadata, error) {
	if !f.fetched.Swap(true) {
		// Spawn a goroutine responsible for fetching the metadata if we're the
		// first Get caller.
		go func() {
			fetcher := &fetchConfig{
				cloudDiscovery: &f.cfg.CloudEnvironmentDiscovery,
			}
			fetcher.setDefaults()
			f.metadata = fetcher.fetch(context.Background())

			// Signal that the metadata is ready.
			close(f.ready)
		}()
	}

	// if the metadata is ready we don't care if the context is already done
	select {
	case <-f.ready:
		return f.metadata, nil
	default:
	}

	select {
	case <-f.ready:
		return f.metadata, nil
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err())
	}
}

// defaultFetcher is the process-wide [Fetcher] used by [Get]. It performs
// every cloud environment check.
var defaultFetcher = newFetcher(FetcherConfig{})

// Get fetches the instance metadata using a default [Fetcher] that performs
// every cloud environment check. Components that must honor the instance's
// configuration should be given a [Getter] from a [Fetcher] built with
// [NewFetcher] instead. See [Fetcher.Get] for the caching semantics.
func Get(ctx context.Context) (*Metadata, error) {
	return defaultFetcher.Get(ctx)
}
