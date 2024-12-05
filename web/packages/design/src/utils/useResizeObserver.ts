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

import { RefObject, useCallback, useLayoutEffect } from 'react';

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
  ref: RefObject<HTMLElement>,
  callback: (entry: ResizeObserverEntry) => void,
  { enabled = true }
): void {
  const effect = useCallback(() => {
    if (!ref.current || !enabled) {
      return;
    }

    const observer = new ResizeObserver(entries => {
      const entry = entries[0];

      // In Connect, when a tab becomes active, its outermost DOM element switches from `display:
      // none` to `display: flex`. This callback is then fired with the height reported as zero.
      // To avoid unnecessary calls to callback, return early here.
      if (entry.contentRect.height === 0) {
        return;
      }

      callback(entry);
    });

    observer.observe(ref.current);

    return () => {
      observer.disconnect();
    };
  }, [callback, ref, enabled]);

  useLayoutEffect(effect, [effect]);
}
