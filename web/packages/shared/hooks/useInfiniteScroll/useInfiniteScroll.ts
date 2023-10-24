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

/**
 * Calls fetch function whenever the `trigger` element intersects the
 * viewport until the list is exhausted or an error happens.
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

  const recreateObserver = () => {
    observer.current?.disconnect();
    if (trigger.current) {
      observer.current = new IntersectionObserver(entries => {
        if (entries[0]?.isIntersecting) {
          fetch();
        }
      });
      observer.current.observe(trigger.current);
    }
  };

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
    recreateObserver();
    return () => {
      observer.current?.disconnect();
    };
  }, [fetch]);

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
