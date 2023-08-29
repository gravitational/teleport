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

import { Attempt } from 'shared/hooks/useAttemptNext';

import { UnifiedResource } from 'teleport/services/agents';

import {
  useKeyBasedPagination,
  Props as PaginationProps,
} from './useKeyBasedPagination';

export type Props<T extends UnifiedResource> = PaginationProps<T>;

/**
 * Fetches a part of resource list whenever the `trigger` element intersects the
 * viewport until the list is exhausted or an error happens. Use
 * [State.forceFetch] to continue after an error.
 */
export function useInfiniteScroll<T extends UnifiedResource>(
  props: Props<T>
): State<T> {
  const observer = useRef<IntersectionObserver | null>(null);
  const trigger = useRef<Element | null>(null);

  const { fetch, forceFetch, attempt, resources, finished } =
    useKeyBasedPagination(props);

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

  useLayoutEffect(() => {
    recreateObserver();
    return () => {
      observer.current?.disconnect();
    };
  }, [fetch]);

  return { setTrigger, forceFetch, attempt, resources, finished };
}

export type State<T> = {
  /**
   * Fetches a new batch of data. Cancels a pending request, if there is one.
   * Disregards whether error has previously occurred.
   */
  forceFetch: () => Promise<void>;

  setTrigger: (el: Element | null) => void;

  attempt: Attempt;
  resources: T[];
  finished: boolean;
};
