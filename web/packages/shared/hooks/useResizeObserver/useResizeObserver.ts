/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useCallback, useMemo, type RefCallback } from 'react';

/**
 * useResizeObserver creates a ResizeObserver which fires a callback
 * when the provided element is resized. The callback is called with the
 * ResizeObserverEntry. The observer is disconnected when the element is
 * unmounted.
 *
 * Returns a ref callback to attach to the element to be observed – the element
 * must be a HTMLElement but may be conditionally null.
 */
export const useResizeObserver = <T extends HTMLElement = HTMLElement>(
  callback: (entry: ResizeObserverEntry) => void,
  { fireOnZeroHeight = true } = {}
): RefCallback<T> => {
  const observer = useMemo(
    () =>
      new ResizeObserver(([entry]) => {
        if (!entry || (!fireOnZeroHeight && entry.contentRect.height === 0))
          return;
        callback(entry);
      }),
    [callback, fireOnZeroHeight]
  );

  return useCallback(
    (element: T) => {
      if (!element) return;
      observer.observe(element);
      return () => observer.disconnect();
    },
    [observer]
  );
};
