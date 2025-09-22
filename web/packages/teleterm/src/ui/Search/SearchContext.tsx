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

import {
  createContext,
  FC,
  MutableRefObject,
  PropsWithChildren,
  useCallback,
  useContext,
  useRef,
  useState,
} from 'react';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { SearchFilter } from 'teleterm/ui/Search/searchResult';
import {
  Document,
  DocumentClusterQueryParams,
  useWorkspaceServiceState,
} from 'teleterm/ui/services/workspacesService';

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
  advancedSearchEnabled: boolean;
  toggleAdvancedSearch(): void;
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

export const SearchContextProvider: FC<PropsWithChildren> = props => {
  const appContext = useAppContext();
  // The type of the ref is Element to adhere to the type of document.activeElement.
  const previouslyActive = useRef<Element>(undefined);
  const inputRef = useRef<HTMLInputElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [inputValue, setInputValue] = useState('');
  const [activePicker, setActivePicker] = useState(actionPicker);
  const [filters, setFilters] = useState<SearchFilter[]>([]);
  const [advancedSearchEnabled, setAdvancedSearchEnabled] = useState(false);

  function toggleAdvancedSearch(): void {
    setAdvancedSearchEnabled(prevState => !prevState);
  }

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

  function resetState(): void {
    setInputValue('');
    setFilters([]);
    setAdvancedSearchEnabled(false);
  }

  function updateStateFromQueryParams(
    queryParams: DocumentClusterQueryParams
  ): void {
    setActivePicker(actionPicker);
    setInputValue(queryParams.search);
    setAdvancedSearchEnabled(queryParams.advancedSearchEnabled);
    setFilters(
      queryParams.resourceKinds.map(resourceType => ({
        filter: 'resource-type',
        resourceType,
      }))
    );
  }

  useWorkspaceServiceState();
  const activeDocument = appContext.workspacesService
    .getActiveWorkspaceDocumentService()
    ?.getActive();

  const [previousActiveDocument, setPreviousActiveDocument] =
    useState<Document>(activeDocument);

  // update the state when the cluster document becomes active
  if (previousActiveDocument !== activeDocument) {
    if (activeDocument?.kind === 'doc.cluster') {
      updateStateFromQueryParams(activeDocument.queryParams);
      // clear it when a non-cluster document is activated
    } else if (previousActiveDocument?.kind === 'doc.cluster') {
      resetState();
    }
    setPreviousActiveDocument(activeDocument);
  }

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
        advancedSearchEnabled,
        toggleAdvancedSearch,
      }}
      children={props.children}
    />
  );
};

export const useSearchContext = () => {
  const context = useContext(SearchContext);

  if (!context) {
    throw new Error(
      'useSearchContext must be used within a SearchContextProvider'
    );
  }

  return context;
};
