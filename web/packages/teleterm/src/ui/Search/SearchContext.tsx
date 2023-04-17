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
  onInputValueChange(value: string): void;
  changeActivePicker(picker: SearchPicker): void;
  isOpen: boolean;
  open(fromElement?: Element): void;
  lockOpen(action: Promise<any>): Promise<void>;
  close(): void;
  closeAndResetInput(): void;
  resetInput(): void;
  setFilter(filter: SearchFilter): void;
  removeFilter(filter: SearchFilter): void;
}

const SearchContext = createContext<SearchContext>(null);

export const SearchContextProvider: FC = props => {
  const previouslyActive = useRef<Element>();
  const inputRef = useRef<HTMLInputElement>();
  const [isOpen, setIsOpen] = useState(false);
  const [isLockedOpen, setIsLockedOpen] = useState(false);
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
    if (isLockedOpen) {
      return;
    }

    setIsOpen(false);
    setActivePicker(actionPicker);
    if (previouslyActive.current instanceof HTMLElement) {
      previouslyActive.current.focus();
    }
  }, [isLockedOpen]);

  const closeAndResetInput = useCallback(() => {
    if (isLockedOpen) {
      return;
    }

    close();
    setInputValue('');
  }, [isLockedOpen, close]);

  const resetInput = useCallback(() => {
    setInputValue('');
  }, []);

  function open(fromElement?: Element): void {
    if (isOpen) {
      // Even if the search bar is already open, we want to focus on the input anyway. The search
      // input might lose focus due to user interaction while the search bar stays open. Focusing
      // here again makes it possible to use the shortcut to grant the focus to the input again.
      inputRef.current?.focus();
      return;
    }

    // In case `open` was called without `fromElement` (e.g. when using the keyboard shortcut), we
    // must read `document.activeElement` before we focus the input.
    previouslyActive.current = fromElement || document.activeElement;
    inputRef.current?.focus();
    setIsOpen(true);
  }

  /**
   * lockOpen forces the search bar to stay open for the duration of the action. It also restores
   * focus on the search input after the action is done.
   *
   * This is useful in situations where want the search bar to not close when the user interacts
   * with modals shown from the search bar.
   */
  async function lockOpen(action: Promise<any>): Promise<void> {
    setIsLockedOpen(true);

    try {
      await action;
    } finally {
      // By the time the action passes, the user might have caused the focus to be lost on the
      // search input, so let's bring it back.
      //
      // focus needs to be executed before the state update, otherwise the search bar will close for
      // some reason.
      inputRef.current?.focus();
      setIsLockedOpen(false);
    }
  }

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
        onInputValueChange: setInputValue,
        changeActivePicker,
        activePicker,
        filters,
        setFilter,
        removeFilter,
        resetInput,
        isOpen,
        open,
        lockOpen,
        close,
        closeAndResetInput,
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
