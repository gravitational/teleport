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

import React, {
  createContext,
  FC,
  useCallback,
  useMemo,
  useState,
} from 'react';

export type RunActiveItemHandler = () => void;

export const KeyboardArrowsNavigationContext = createContext<{
  activeIndex: number;
  setActiveIndex(index: number): void;
  addItem(index: number, onRunActiveItem: RunActiveItemHandler): void;
  removeItem(index: number): void;
}>(null);

enum KeyboardArrowNavigationKeys {
  ArrowDown = 'ArrowDown',
  ArrowUp = 'ArrowUp',
  Enter = 'Enter',
}

export const KeyboardArrowsNavigation: FC = props => {
  const [items, setItems] = useState<RunActiveItemHandler[]>([]);
  const [activeIndex, setActiveIndex] = useState<number>(-1);

  const addItem = useCallback(
    (index: number, onRun: RunActiveItemHandler): void => {
      setItems(prevItems => {
        const newItems = [...prevItems];
        if (newItems[index] === onRun) {
          throw new Error(
            'Tried to override an index with the same `onRun()` callback.'
          );
        }
        newItems[index] = onRun;
        return newItems;
      });
    },
    [setItems]
  );

  const removeItem = useCallback(
    (index: number): void => {
      setItems(prevItems => {
        const newItems = [...prevItems];
        newItems[index] = undefined;
        return newItems;
      });
    },
    [setItems, setActiveIndex]
  );

  function handleKeyDown(event: React.KeyboardEvent): void {
    if (Object.keys(KeyboardArrowNavigationKeys).includes(event.key)) {
      event.stopPropagation();
      event.preventDefault();
    }

    switch (event.key) {
      case 'ArrowDown':
        setActiveIndex(getNextIndex(items, activeIndex));
        break;
      case 'ArrowUp':
        setActiveIndex(getPreviousIndex(items, activeIndex));
        break;
      case 'Enter':
        items[activeIndex]?.();
    }
  }

  const value = useMemo(
    () => ({
      addItem,
      removeItem,
      activeIndex,
      setActiveIndex,
    }),
    [addItem, removeItem, activeIndex, setActiveIndex]
  );

  return (
    <KeyboardArrowsNavigationContext.Provider value={value}>
      <div onKeyDown={handleKeyDown}>{props.children}</div>
    </KeyboardArrowsNavigationContext.Provider>
  );
};

function getNextIndex(
  items: RunActiveItemHandler[],
  currentIndex: number
): number {
  for (let i = currentIndex + 1; i < items.length; ++i) {
    if (items[i]) {
      return i;
    }
  }

  // if there was no item after the current index, start from the beginning
  for (let i = 0; i < currentIndex; i++) {
    if (items[i]) {
      return i;
    }
  }

  return currentIndex;
}

function getPreviousIndex(
  items: RunActiveItemHandler[],
  currentIndex: number
): number {
  for (let i = currentIndex - 1; i >= 0; --i) {
    if (items[i]) {
      return i;
    }
  }

  // if there was no item before the current index, start from the end
  for (let i = items.length - 1; i > currentIndex; i--) {
    if (items[i]) {
      return i;
    }
  }

  return currentIndex;
}
