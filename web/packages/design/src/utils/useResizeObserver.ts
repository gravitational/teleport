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

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  type RefCallback,
} from 'react';

/**
 * useResizeObserver creates a ResizeObserver which fires a callback
 * when the provided element is resized. The callback is called with the
 * ResizeObserverEntry. The observer is disconnected when the element is
 * unmounted.
 *
 * Returns a ref callback to attach to the element to be observed – the element
 * must be a HTMLElement but may be conditionally null.
 *
 * `fireOnZeroHeight` determines whether the callback should be fired when the
 * element's height is 0. This defaults to false, to account for a special case in
 * Connect where tabs are hidden using `display: none;`.
 *
 * @example
 * // Basic usage
 * const Component = () => {
 *   const handleResize = (entry: ResizeObserverEntry) => {
 *     console.log({ entryContentRect: entry.contentRect });
 *   };
 *
 *   const ref = useResizeObserver<HTMLDivElement>(handleResize);
 *
 *   return <div ref={ref}>This div is being observed</div>;
 * };
 */
export const useResizeObserver = <T extends HTMLElement = HTMLElement>(
  callback: (entry: ResizeObserverEntry) => void,
  { fireOnZeroHeight = false } = {}
): RefCallback<T> => {
  const callbackRef = useRef(callback);
  const elementRef = useRef<T | null>(null);

  useEffect(() => {
    callbackRef.current = callback;
  }, [callback]);

  const observer = useMemo(
    () =>
      new ResizeObserver(([entry]) => {
        if (!entry || (!fireOnZeroHeight && entry.contentRect.height === 0))
          return;
        callbackRef.current?.(entry);
      }),
    [fireOnZeroHeight]
  );

  useEffect(() => {
    return () => observer.disconnect();
  }, [observer]);

  return useCallback(
    (element: T | null) => {
      if (element) {
        observer.observe(element);
        elementRef.current = element;
      } else {
        elementRef.current && observer.unobserve(elementRef.current);
        elementRef.current = null;
      }
    },
    [observer]
  );
};
