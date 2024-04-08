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
  addItem(
    index: number,
    el: HTMLElement,
    onRunActiveItem: RunActiveItemHandler
  ): void;
  removeItem(index: number): void;
}>(null);

type NavigationItem = {
  handler: RunActiveItemHandler;
  el: HTMLElement;
};

export const KeyboardArrowsNavigation: FC<PropsWithChildren> = props => {
  const [items, setItems] = useState<NavigationItem[]>([]);
  const [activeIndex, setActiveIndex] = useState<number>(-1);

  const addItem = useCallback(
    (index: number, el: HTMLElement, onRun: RunActiveItemHandler): void => {
      setItems(prevItems => {
        const newItems = [...prevItems];

        if (newItems[index]?.handler === onRun) {
          throw new Error(
            'Tried to override an index with the same `onRun()` callback.'
          );
        }
        newItems[index] = { handler: onRun, el };
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
    switch (event.key) {
      case 'ArrowDown':
        event.stopPropagation();
        event.preventDefault();

        setActiveIndex(getNextIndex(items, activeIndex));
        break;
      case 'ArrowUp':
        event.stopPropagation();
        event.preventDefault();

        setActiveIndex(getPreviousIndex(items, activeIndex));
        break;
      case 'Enter': {
        const activeEl = items[activeIndex]?.el;

        if (!activeEl) {
          return;
        }

        // TODO: Add comment.
        // Some navigation items might include additional buttons which can be navigated to with
        // Tab. In that case, we want to trigger the handler of the active item only if the element
        // on which Enter was pressed matches the el that represents the active item.

        // The search input is focused at all times when using arrows and pressing enters. However,
        // when the focus is switched with tab, then the focused element is actually the button
        // inside a ListItem. Hence we check if activeEl (ListItem) contains event.target (the
        // button inside ListItem). This returns false if the focused element is the search input.
        if (activeEl.contains(event.target as Node)) {
          console.log('returning because active el contains event.target', {
            target: event.target,
            activeEl,
          });
          return;
        }

        // TODO: The biggest problem is that with how the filterable list + keyboard navigation is
        // currently written, the HTML focus stays on the search input.
        //
        // This means that if you press a down arrow twice and then press tab, the focus changes
        // from the search input to a button next to the first list item. We could fix that by
        // moving the HTML focus to the active item when pressing arrows. But then after you press
        // an arrow you'd no longer be able to type.
        //
        // Ideally, all key down inputs besides up/down arrow and enter should reset the focus back
        // to the search input and forward that keypress to the input.

        event.stopPropagation();
        event.preventDefault();

        items[activeIndex]?.handler();
      }
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

function getNextIndex<Item>(items: Item[], currentIndex: number): number {
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

function getPreviousIndex<Item>(items: Item[], currentIndex: number): number {
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
