/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { RefCallback, useCallback, useMemo } from 'react';

// TODO: Add back the old implementation and use it only in Transition. Check if the new one works
// as expected with normal code.

/**
 * useResizeObserverRef returns a ref callback. After assigning it to a React node, the ref callback
 * sets up a ResizeObserver and calls callback on each resize.
 *
 * It does not fire if node's contentRect.height is zero, to account for a special case in
 * Connect where tabs are hidden using `display: none;`.
 */
export function useResizeObserverRef(
  callback: (entry: ResizeObserverEntry) => void
): RefCallback<HTMLElement> {
  const observer = useMemo(() => {
    console.log('Initializing a ResizeObserver');
    return new ResizeObserver(entries => {
      const entry = entries[0];

      // In Connect, when a tab becomes active, its outermost DOM element switches from `display:
      // none` to `display: flex`. This callback is then fired with the height reported as zero.
      // To avoid unnecessary calls to callback, return early here.
      if (entry.contentRect.height === 0) {
        return;
      }

      callback(entry);
    });
  }, [callback]);

  return useCallback(
    node => {
      if (node) {
        // TODO: Why doesn't it get called again once we close MenuLogin for the first time?
        console.log('Observing a node with ResizeObserver');
        observer.observe(node);
      } else {
        console.log('Disconnecting ResizeObserver from a node');
        observer.disconnect();
      }
    },
    [observer]
  );
}
