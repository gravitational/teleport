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

import { useContext, useEffect } from 'react';

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

  return {
    isActive: index === navigationContext.activeIndex,
  };
}

export function useKeyboardArrowsNavigationStateUpdate() {
  const { setActiveIndex } = useContext(KeyboardArrowsNavigationContext);

  return { setActiveIndex };
}
