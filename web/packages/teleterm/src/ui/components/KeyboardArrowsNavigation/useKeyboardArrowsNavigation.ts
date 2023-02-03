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
