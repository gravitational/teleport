/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, {
  useContext,
  useState,
  FC,
  useCallback,
  createContext,
  useRef,
  MutableRefObject,
} from 'react';

import { SearchFilter } from 'teleterm/ui/Search/searchResult';

import { actionPicker, SearchPicker } from './pickers/pickers';

export interface SearchContext {
  inputRef: MutableRefObject<HTMLInputElement>;
  inputValue: string;
  filters: SearchFilter[];
  activePicker: SearchPicker;
  setInputValue(value: string): void;
  changeActivePicker(picker: SearchPicker): void;
  isOpen: boolean;
  open(fromElement?: Element): void;
  close(): void;
  closeWithoutRestoringFocus(): void;
  resetInput(): void;
  setFilter(filter: SearchFilter): void;
  removeFilter(filter: SearchFilter): void;
  pauseUserInteraction(action: () => Promise<any>): Promise<void>;
  addWindowEventListener: AddWindowEventListener;
  makeEventListener: <EventListener>(
    eventListener: EventListener
  ) => EventListener | undefined;
}

export type AddWindowEventListener = (
  ...args: Parameters<typeof window.addEventListener>
) => {
  cleanup: () => void;
};

const SearchContext = createContext<SearchContext>(null);

export const SearchContextProvider: FC = props => {
  // The type of the ref is Element to adhere to the type of document.activeElement.
  const previouslyActive = useRef<Element>();
  const inputRef = useRef<HTMLInputElement>();
  const [isOpen, setIsOpen] = useState(false);
  const [inputValue, setInputValue] = useState('');
  const [activePicker, setActivePicker] = useState(actionPicker);
  // TODO(ravicious): Consider using another data structure for search filters as we know that we
  // always have only two specific filters: one for clusters and one for resource type.
  //
  // This could probably be represented by an object instead plus an array for letting the user
  // provide those filters in any order they want. The array would be used in the UI that renders
  // the filters while code that uses the search filters, such as ResourcesService.searchResources,
  // could operate on the object instead.
  const [filters, setFilters] = useState<SearchFilter[]>([]);

  function changeActivePicker(picker: SearchPicker): void {
    setActivePicker(picker);
    setInputValue('');
  }

  const close = useCallback(() => {
    setIsOpen(false);
    setActivePicker(actionPicker);
    if (
      // The Element type is not guaranteed to have the focus function so we're forced to manually
      // perform the type check.
      previouslyActive.current
    ) {
      // TODO(ravicious): Revert to a regular `focus()` call (#25186@4f9077eb7) once #25683 gets in.
      previouslyActive.current['focus']?.();
    }
  }, []);

  const closeWithoutRestoringFocus = useCallback(() => {
    previouslyActive.current = undefined;
    close();
  }, [close]);

  const resetInput = useCallback(() => {
    setInputValue('');
  }, []);

  function open(fromElement?: HTMLElement): void {
    if (isOpen) {
      // Even if the search bar is already open, we want to focus on the input anyway. The search
      // input might lose focus due to user interaction while the search bar stays open. Focusing
      // here again makes it possible to use the shortcut to grant the focus to the input again.
      //
      // Also note that SearchBar renders two distinct input elements, one when the search bar is
      // closed and another when its open. During the initial call to this function,
      // inputRef.current is equal to the element from when the search bar was closed.
      inputRef.current?.focus();
      return;
    }

    previouslyActive.current = fromElement || document.activeElement;
    setIsOpen(true);
  }

  const [isUserInteractionPaused, setIsUserInteractionPaused] = useState(false);
  /**
   * pauseUserInteraction temporarily causes addWindowEventListener not to add listeners for the
   * duration of the action. It also restores focus on the search input after the action is done.
   *
   * This is useful in situations where want the search bar to show some other element the user can
   * interact with, for example a modal. When the user interacts with the modal, we don't want the
   * search bar listeners to intercept those interactions, which could for example cause the search
   * bar to close or make the user unable to press Enter in the modal as the search bar would
   * swallow that event.
   */
  const pauseUserInteraction = useCallback(
    async (action: () => Promise<any>): Promise<void> => {
      setIsUserInteractionPaused(true);

      try {
        await action();
      } finally {
        // By the time the action passes, the user might have caused the focus to be lost on the
        // search input, so let's bring it back.
        //
        // focus needs to be executed before the state update, otherwise the search bar will close
        // for some reason.
        inputRef.current?.focus();
        setIsUserInteractionPaused(false);
      }
    },
    []
  );

  /**
   * addWindowEventListener is meant to be used in useEffect calls which register event listeners
   * related to the search bar. It automatically removes the listener when the user interaction gets
   * paused.
   *
   * pauseUserInteraction is supposed to be called in situations where the search bar is obstructed
   * by another element (such as a modal) and we don't want interactions with that other element to
   * have any effect on the search bar.
   */
  const addWindowEventListener = useCallback(
    (...args: Parameters<typeof window.addEventListener>) => {
      if (isUserInteractionPaused) {
        return { cleanup: undefined };
      }

      window.addEventListener(...args);

      return {
        cleanup: () => {
          window.removeEventListener(...args);
        },
      };
    },
    [isUserInteractionPaused]
  );

  /**
   * makeEventListener is similar to addWindowEventListener but meant for situations where you want
   * to add a listener to an element directly. By wrapping the listener in makeEventListener, you
   * make sure that the listener will be removed when the interaction with the search bar is paused.
   */
  const makeEventListener = useCallback(
    eventListener => {
      if (isUserInteractionPaused) {
        return;
      }

      return eventListener;
    },
    [isUserInteractionPaused]
  );

  function setFilter(filter: SearchFilter) {
    // UI prevents adding more than one filter of the same type
    setFilters(prevState => [...prevState, filter]);
    inputRef.current?.focus();
  }

  function removeFilter(filter: SearchFilter) {
    setFilters(prevState => {
      const index = prevState.indexOf(filter);
      if (index >= 0) {
        const copied = [...prevState];
        copied.splice(index, 1);
        return copied;
      }
      return prevState;
    });
    inputRef.current?.focus();
  }

  return (
    <SearchContext.Provider
      value={{
        inputRef,
        inputValue,
        setInputValue,
        changeActivePicker,
        activePicker,
        filters,
        setFilter,
        removeFilter,
        resetInput,
        isOpen,
        open,
        close,
        closeWithoutRestoringFocus,
        pauseUserInteraction,
        addWindowEventListener,
        makeEventListener,
      }}
      children={props.children}
    />
  );
};

export const useSearchContext = () => {
  const context = useContext(SearchContext);

  if (!context) {
    throw new Error('SearchContext requires SearchContextProvider context.');
  }

  return context;
};
