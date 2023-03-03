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
	"sync"
)

// metadata is a cache of all instance metadata.
var metadata *Metadata

// fetchOnce ensures that the instance metadata is fetched at most once.
var fetchOnce sync.Once

// Get fetches the instance metadata.
// The first call can take some time as all metadata will be retrieved.
// The resulting metadata is cached, so subsequent calls will be fast.
// Note that the context used to retrieve the metadata is the one passed in to
// the first `Get` call.
func Get(ctx context.Context) *Metadata {
	fetchOnce.Do(func() {
		defaultFetcher := &fetchConfig{context: ctx}
		defaultFetcher.setDefaults()
		metadata = defaultFetcher.fetch()
	})
	return metadata
}
