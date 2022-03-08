import React, { createContext, FC, useEffect, useState } from 'react';

export type RunActiveItemHandler = () => void;

export const KeyboardArrowsNavigationContext = createContext<{
  activeIndex: number;
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
  const [activeIndex, setActiveIndex] = useState<number>(0);

  function addItem(index: number, onRunActiveItem: RunActiveItemHandler): void {
    setItems(prevItems => {
      const newItems = [...prevItems];
      if (newItems[index] === onRunActiveItem) {
        throw new Error(
          'Tried to override an index with the same `onRunActiveItem()` callback.'
        );
      }
      newItems[index] = onRunActiveItem;
      return newItems;
    });
  }

  function removeItem(index: number): void {
    setItems(prevItems => {
      const newItems = [...prevItems];
      newItems[index] = undefined;
      return newItems;
    });
    setActiveIndex(0);
  }

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
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
          items[activeIndex]();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [items, setActiveIndex, activeIndex]);

  return (
    <KeyboardArrowsNavigationContext.Provider
      value={{ addItem, removeItem, activeIndex }}
    >
      {props.children}
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
