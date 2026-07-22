/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { useCallback, useLayoutEffect, useRef } from 'react';

/**
 * Calls fetch function whenever the `trigger` element intersects the
 * viewport.
 * It also triggers the initial request when the `trigger` element
 * is rendered for the first time.
 *
 * Callers must set the `trigger` element by passing the [`State.setTrigger`] function
 * as the `ref` prop of the element they want to use as the trigger.
 */
export function useInfiniteScroll({
  fetch,
}: {
  fetch: () => Promise<void>;
}): InfiniteScroll {
  const observer = useRef<IntersectionObserver | null>(null);
  const trigger = useRef<Element | null>(null);

  const recreateObserver = useCallback(() => {
    observer.current?.disconnect();
    if (trigger.current) {
      // multiple entries can be received even from a single target, so we loop
      // through each entry to look for intersection:
      // https://developer.mozilla.org/en-US/docs/Web/API/Intersection_Observer_API#intersection_change_callbacks
      observer.current = new IntersectionObserver(entries => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            fetch();
            break;
          }
        }
      });
      observer.current.observe(trigger.current);
    }
  }, [fetch]);

  const setTrigger = (el: Element | null) => {
    trigger.current = el;
    recreateObserver();
  };

  // Using layout effect instead of a regular one helps prevent sneaky race
  // conditions. If we used a regular effect, the observer may be recreated
  // after the current one (which, by now, may be tied to a stale state)
  // triggers a fetch. Thus, the fetch would use stale state and may ultimately
  // cause us to display incorrect data. (This issue can be reproduced by
  // switching this to `useEffect` and rapidly changing filtering data on the
  // resources list page).
  useLayoutEffect(() => {
    // triggers the initial request
    recreateObserver();
    return () => {
      observer.current?.disconnect();
    };
  }, [recreateObserver]);

  return { setTrigger };
}

type InfiniteScroll = {
  /**
   * Sets an element that will be observed and will trigger a fetch once it
   * becomes visible. The element doesn't need to become fully visible; a single
   * pixel will be enough to trigger.
   */
  setTrigger: (el: Element | null) => void;
};
