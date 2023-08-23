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

package cache

import (
	"fmt"
	"time"
)

// cachedError wraps an error before storing it into cache. This adds more
// context into the original error by clearly indicating the error have been
// cached and for how long.
type cachedError struct {
	err   error
	until time.Time
}

func (e cachedError) Error() string {
	return fmt.Sprintf("error cached until '%s': %s", e.until, e.err)
}

// OrigError returns the original error. This implements trace.ErrorWrapper
// and allows to be unwrapped by trace.Unwrap().
func (e cachedError) OrigError() error {
	return e.err
}

// Unwrap returns the original error.
func (e cachedError) Unwrap() error {
	return e.err
}

// newCachedError takes an error and wraps it into a cachedError. If there is no
// error, it returns nothing.
func newCachedError(err error, until time.Time) error {
	if err == nil {
		return nil
	}
	return cachedError{err, until}
}
