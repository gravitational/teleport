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

import React, {
  createContext,
  FC,
  PropsWithChildren,
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

export const KeyboardArrowsNavigation: FC<PropsWithChildren> = props => {
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

export const NullKeyboardArrowsNavigation: FC<PropsWithChildren> = props => {
  const value = useMemo(
    () => ({
      addItem: () => {},
      removeItem: () => {},
      activeIndex: -1,
      setActiveIndex: () => {},
    }),
    []
  );

  return (
    <KeyboardArrowsNavigationContext.Provider value={value}>
      <div>{props.children}</div>
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
