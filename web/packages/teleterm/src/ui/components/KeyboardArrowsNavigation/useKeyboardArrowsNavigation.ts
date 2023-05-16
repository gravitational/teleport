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
