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
