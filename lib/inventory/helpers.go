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
package inventory

import (
	"github.com/gravitational/teleport/api/utils/retryutils"
)

// we use dedicated global jitters for all the intervals/retries in this
// package. we do this because our jitter usage in this package can scale by
// the number of concurrent connections to auth, making dedicated jitters a
// poor choice (high memory usage for all the rngs).
var (
	seventhJitter = retryutils.NewShardedSeventhJitter()
	halfJitter    = retryutils.NewShardedHalfJitter()
	fullJitter    = retryutils.NewShardedFullJitter()
)
