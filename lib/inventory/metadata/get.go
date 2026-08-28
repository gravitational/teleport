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

// metadata is a cache of all instance metadata.
var metadata *Metadata

// metadataReady is a channel that is closed when the metadata has been fetched.
var metadataReady = make(chan struct{})

// fetched is used to ensure that the instance metadata is fetched at most once.
var fetched atomic.Bool

// Get fetches the instance metadata. The first call can take some time as all
// metadata will be retrieved. The resulting metadata is cached, so subsequent
// calls will be fast. The return value of Get might be shared between callers
// and should not be modified. If the cached metadata is ready, it will be
// returned successfully even if the context is done.
func Get(ctx context.Context) (*Metadata, error) {
	if !fetched.Swap(true) {
		// Spawn a goroutine responsible for fetching the metadata if we're the
		// first Get caller.
		go func() {
			defaultFetcher := &fetchConfig{}
			defaultFetcher.setDefaults()
			metadata = defaultFetcher.fetch(context.Background())

			// Signal that the metadata is ready.
			close(metadataReady)
		}()
	}

	// if the metadata is ready we don't care if the context is already done
	select {
	case <-metadataReady:
		return metadata, nil
	default:
	}

	select {
	case <-metadataReady:
		return metadata, nil
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err())
	}
}
