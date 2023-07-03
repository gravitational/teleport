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

// Get fetches the instance metadata.
// The first call can take some time as all metadata will be retrieved.
// The resulting metadata is cached, so subsequent calls will be fast.
// The return value of Get might be shared between callers and should not be
// modified.
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

	select {
	case <-metadataReady:
		return metadata, nil
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err())
	}
}
