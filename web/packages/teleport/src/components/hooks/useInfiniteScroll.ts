/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useLayoutEffect, useRef } from 'react';

/** Calls `callback` whenever the `trigger` element intersects the viewport.
 *
 * This hook is intended to be used in tandem with the `useKeyBasedPagination`
 * hook. Example usage:
 *
 * ```ts
 * const scrollDetector = useRef(null);
 * const { fetch, resources } = useKeyBasedPagination({
 *   fetchFunc: api.fetchItems,
 *   filter,
 * });
 * useInfiniteScroll(scrollDetector.current, fetch);
 *
 * return (<>
 *   {items.map(renderItem)}
 *   <div ref={scrollDetector} />
 * </>);
 * ```
 */
export function useInfiniteScroll(trigger: Element, callback: () => void) {
  const observer = useRef<IntersectionObserver | null>(null);

  useLayoutEffect(() => {
    if (observer.current) {
      observer.current.disconnect();
    }
    observer.current = new IntersectionObserver(entries => {
      if (entries[0]?.isIntersecting) {
        callback();
      }
    });
    if (trigger) {
      observer.current.observe(trigger);
    }
  }, [trigger, callback]);
}
