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

import {
  RefCallback,
  RefObject,
  useCallback,
  useLayoutEffect,
  useMemo,
} from 'react';

/**
 * useResizeObserver sets up a ResizeObserver for ref and calls callback on each resize.
 *
 * It does not fire if ref.current.contentRect.height is zero, to account for a special case in
 * Connect where tabs are hidden using `display: none;`.
 *
 * Uses a layout effect underneath. If ref is conditionally rendered, set enabled to false when ref
 * is null.
 */
export function useResizeObserver(
  callback: (entry: ResizeObserverEntry) => void
): RefCallback<HTMLElement> {
  const observer = useMemo(() => {
    const uuid = crypto.randomUUID();
    console.log(`ResizeObserver creating ${uuid}`);
    return new ResizeObserver(entries => {
      console.log(`ResizeObserver changes ${uuid}`);
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

  const uuid = crypto.randomUUID();
  return useCallback(
    node => {
      if (node) {
        console.log(`ref callback ${uuid} with node`);
        observer.observe(node);
      } else {
        console.log(`ref callback ${uuid} cleanup`);
        observer.disconnect();
      }
    },
    [observer]
  );
}
