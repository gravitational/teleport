import React, { useEffect } from 'react';
import {
  KeyboardArrowsNavigationContext,
  RunActiveItemHandler,
} from './KeyboardArrowsNavigation';

/**
 * onRunActiveItem must be memoized
 */

export function useKeyboardArrowsNavigation({
  index,
  onRunActiveItem,
}: {
  index: number;
  onRunActiveItem: RunActiveItemHandler;
}) {
  const navigationContext = React.useContext(KeyboardArrowsNavigationContext);

  if (!navigationContext) {
    throw new Error(
      '`useKeyboardArrowsNavigation` must be used in the context of `KeyboardArrowNavigationContext`.'
    );
  }

  useEffect(() => {
    navigationContext.addItem(index, onRunActiveItem);

    return () => navigationContext.removeItem(index);
  }, [index, onRunActiveItem]);

  return {
    isActive: index === navigationContext.activeIndex,
  };
}
