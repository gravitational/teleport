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

package utils

import (
	"context"
	"iter"
)

// BatchReadChannel returns a single-use sequence that waits for a value on the
// supplied channel, and then pulls out as many items as it can until the read
// would block again, up to the supplied maximum number of items.
//
// The yielded values are a (T, bool) pair, with the boolean value indicating
// that the supplied T value is good (true), or if the drain was canceled and
// the T value is the type zero value.
//
// The BatchReadChannel iterator will stop reading from the channel when:
//   - the supplied context is canceled
//   - a read from the channel (other than the initial read) would block
//   - the function has read `max` items out of the channel
//   - the callback function returns an error
func BatchReadChannel[T any](ctx context.Context, ch <-chan T, max int) iter.Seq2[T, bool] {
	return func(yield func(T, bool) bool) {
		count := 0

		// Wait of the first item indefinitely (or at least as long as the supplied
		// context will let us)
		select {
		case <-ctx.Done():
			var t T
			yield(t, false)
			return

		case item, ok := <-ch:
			if !yield(item, ok) || !ok {
				return
			}
		}

		// Pull remaining items out of the channel until the specified maximum
		// is reached, or the read would block
		for count < max {
			select {
			case item, ok := <-ch:
				if !yield(item, ok) || !ok {
					return
				}
				count++

			case <-ctx.Done():
				var t T
				yield(t, false)
				return

			default:
				return
			}
		}
	}
}
