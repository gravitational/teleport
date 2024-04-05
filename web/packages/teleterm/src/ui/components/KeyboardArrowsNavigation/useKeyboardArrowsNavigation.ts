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

import { useCallback, useContext, useEffect } from 'react';

import {
  KeyboardArrowsNavigationContext,
  RunActiveItemHandler,
} from './KeyboardArrowsNavigation';

/**
 * onRun must be memoized
 */

export function useKeyboardArrowsNavigation({
  index,
  onRun,
}: {
  index: number;
  onRun: RunActiveItemHandler;
}) {
  const navigationContext = useContext(KeyboardArrowsNavigationContext);

  if (!navigationContext) {
    throw new Error(
      '`useKeyboardArrowsNavigation` must be used in the context of `KeyboardArrowNavigationContext`.'
    );
  }

  useEffect(() => {
    navigationContext.addItem(index, onRun);

    return () => navigationContext.removeItem(index);
  }, [index, onRun, navigationContext.addItem, navigationContext.removeItem]);

  const isActive = index === navigationContext.activeIndex;

  const scrollIntoViewIfActive = useCallback(
    (el: HTMLElement | undefined) => {
      if (!isActive || !el) {
        return;
      }

      // By default, scrollIntoView uses 'start'. This is a problem in two cases:
      //
      // 1. When scrolling from the last to the first element, the top of the scrollable area gets
      // aligned to the top of the first active element, not to the top of the parent container.
      // 2. When scrolling from any other element to the next one, the scrollable area gets aligned
      // to the top of the active element, meaning that the previous element immediately disappears.
      //
      // 'center' fixes both problems, while being closer than 'nearest' to how the browser adjusts
      // the scrollable area when tabbing through focusable elements. It ensures that you see what's
      // after and before the active element.
      //
      // Compared to 'nearest', it also makes sure that the scrollable area is aligned to its bottom
      // when scrolling to the last element – 'nearest' aligns it to the bottom of the active item.
      el.scrollIntoView({ block: 'center' });
    },
    [isActive]
  );

  return {
    isActive,
    scrollIntoViewIfActive,
  };
}

export function useKeyboardArrowsNavigationStateUpdate() {
  const { setActiveIndex } = useContext(KeyboardArrowsNavigationContext);

  return { setActiveIndex };
}
