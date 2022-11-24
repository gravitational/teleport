/*
Copyright 2019 Gravitational, Inc.

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

package utils

import (
	"time"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

// HalfJitter is a global jitter instance used for one-off jitters.
// Prefer instantiating a new jitter instance for operations that require
// repeated calls, and use a dedicated sharded jitter instance for
// any usecases that might scale with cluster size or request count.
var HalfJitter = retryutils.NewHalfJitter()

// SeventhJitter is a global jitter instance used for one-off jitters.
// Prefer instantiating a new jitter instance for operations that require
// repeated calls, and use a dedicated sharded jitter instance for
// any usecases that might scale with cluster size or request count.
var SeventhJitter = retryutils.NewSeventhJitter()

// FullJitter is a global jitter instance used for one-off jitters.
// Prefer instantiating a new jitter instance for operations that require
// repeated calls, and use a dedicated sharded jitter instance for
// any usecases that might scale with cluster size or request count.
var FullJitter = retryutils.NewFullJitter()

// NewDefaultLinear creates a linear retry using a half jitter, 10s step, and maxing out
// at 1 minute. These values were selected by reviewing commonly used parameters elsewhere
// in the code base, which (at the time of writing) seem to converge on approximately this
// configuration for "critical but potentially load-inducing" operations like cache watcher
// registration and auth connector setup. It also includes an auto-reset value of 5m. Auto-reset
// is less commonly used, and if used should probably be shorter, but 5m is a reasonable
// safety net to reduce the impact of accidental misuse.
func NewDefaultLinear() *retryutils.Linear {
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:     HalfJitter(time.Second * 5),
		Step:      time.Second * 10,
		Max:       time.Minute,
		Jitter:    retryutils.NewHalfJitter(),
		AutoReset: 5,
	})
	if err != nil {
		panic("default linear retry misconfigured (this is a bug)")
	}
	return retry
}
