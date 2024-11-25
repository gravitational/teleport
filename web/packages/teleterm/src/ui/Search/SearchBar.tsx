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

import React, { useCallback, useEffect, useRef } from 'react';
import styled from 'styled-components';

import { Box, Flex } from 'design';

import { KeyboardShortcutAction } from 'teleterm/services/config';
import {
  SearchContextProvider,
  useSearchContext,
} from 'teleterm/ui/Search/SearchContext';
import {
  useKeyboardShortcutFormatters,
  useKeyboardShortcuts,
} from 'teleterm/ui/services/keyboardShortcuts';

import { useAppContext } from '../appContextProvider';
import { useStoreSelector } from '../hooks/useStoreSelector';

const OPEN_SEARCH_BAR_SHORTCUT_ACTION: KeyboardShortcutAction = 'openSearchBar';

export function SearchBarConnected() {
  const rootClusterUri = useStoreSelector(
    'workspacesService',
    useCallback(state => state.rootClusterUri, [])
  );

  if (!rootClusterUri) {
    return null;
  }

  return (
    <SearchContextProvider>
      <SearchBar />
    </SearchContextProvider>
  );
}

function SearchBar() {
  const containerRef = useRef<HTMLDivElement>();
  const { getAccelerator } = useKeyboardShortcutFormatters();
  const {
    activePicker,
    inputValue,
    setInputValue,
    inputRef,
    isOpen,
    open,
    close,
    closeWithoutRestoringFocus,
    addWindowEventListener,
    makeEventListener,
  } = useSearchContext();
  const ctx = useAppContext();
  ctx.clustersService.useState();

  useKeyboardShortcuts({
    [OPEN_SEARCH_BAR_SHORTCUT_ACTION]: () => {
      open();
    },
  });

  // Handle outside click when the search bar is open.
  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const onClickOutside = (e: MouseEvent) => {
      if (
        !(
          e.composedPath().includes(containerRef.current) ||
          // Prevents closing the search bar
          // when the advanced search tooltip is opened.
          document.querySelector('#predicate-documentation')
        )
      ) {
        close();
      }
    };

    const { cleanup } = addWindowEventListener('click', onClickOutside, {
      capture: true,
    });
    return cleanup;
  }, [close, isOpen, addWindowEventListener]);

  // closeIfAnotherElementReceivedFocus handles a scenario where the focus shifts from the search
  // input to another element on page. It does nothing if there's no other element that receives
  // focus, i.e. the user clicks on an unfocusable element (for example, the empty space between the
  // search bar and the profile selector).
  //
  // If that element is present though, onBlur takes precedence over onClickOutside. For example,
  // clicking on a button outside of the search bar will trigger onBlur and will not trigger
  // onClickOutside.
  const closeIfAnotherElementReceivedFocus = makeEventListener(
    (event: React.FocusEvent) => {
      const elementReceivingFocus = event.relatedTarget;

      if (!(elementReceivingFocus instanceof Node)) {
        // event.relatedTarget might be undefined if the user clicked on an element that is not
        // focusable. The element might or might not be inside the search bar, however we have no way
        // of knowing that. Instead of closing the search bar, we defer this responsibility to the
        // onClickOutside handler and return early.
        //
        return;
      }

      const isElementReceivingFocusOutsideOfSearchBar =
        !containerRef.current.contains(elementReceivingFocus);

      if (isElementReceivingFocusOutsideOfSearchBar) {
        closeWithoutRestoringFocus(); // without restoring focus
      }
    }
  );

  const defaultInputProps = {
    ref: inputRef,
    role: 'searchbox',
    placeholder: activePicker.placeholder,
    value: inputValue,
    onChange: e => {
      setInputValue(e.target.value);
    },
    onFocus: (e: React.FocusEvent) => {
      open(e.relatedTarget);
    },
    onBlur: closeIfAnotherElementReceivedFocus,
    spellCheck: false,
  };

  return (
    <Flex
      css={`
        position: relative;
        flex: 4;
        flex-shrink: 1;
        height: 100%;
        border: 1px ${props => props.theme.colors.buttons.border.border} solid;
        border-radius: ${props => props.theme.radii[2]}px;

        &:hover {
          background: ${props => props.theme.colors.spotBackground[0]};
        }
      `}
      justifyContent="center"
      ref={containerRef}
    >
      {!isOpen && (
        <Flex alignItems="center" flex={1}>
          <Input
            {...defaultInputProps}
            // Adds `text-overflow: ellipsis` only to the closed state.
            // Generally, ellipsis does not work when the input is focused.
            // This causes flickering when an item is selected by clicking -
            // the input loses focus, the ellipsis activates for a moment,
            // and after a fraction of a second is removed when the input receives focus back.
            css={`
              text-overflow: ellipsis;
            `}
          />
          <Shortcut>{getAccelerator(OPEN_SEARCH_BAR_SHORTCUT_ACTION)}</Shortcut>
        </Flex>
      )}
      {isOpen && (
        <activePicker.picker
          // When the search bar transitions from closed to open state, `inputRef.current` within
          // the `open` function refers to the input element from when the search bar was closed.
          //
          // Thus, calling `focus()` on it would have no effect. Instead, we add `autoFocus` on the
          // input when the search bar is open.
          input={<Input {...defaultInputProps} autoFocus={true} />}
        />
      )}
    </Flex>
  );
}

const Input = styled.input`
  height: 38px;
  width: 100%;
  // min-width causes the filters and the actual input text to be broken into
  // two lines when there is no space
  min-width: calc(${props => props.theme.space[8]}px * 2);
  background: transparent;
  color: inherit;
  box-sizing: border-box;
  outline: none;
  border: none;
  font-size: 14px;
  border-radius: ${props => props.theme.radii[2]}px;
  padding-inline: ${props => props.theme.space[2]}px;

  &::placeholder {
    color: ${props => props.theme.colors.text.slightlyMuted};
  }
`;

const Shortcut = styled(Box).attrs({ p: 1, mr: 2 })`
  color: ${({ theme }) => theme.colors.text.slightlyMuted};
  background-color: ${({ theme }) => theme.colors.spotBackground[0]};
  line-height: 12px;
  font-size: 12px;
  border-radius: ${props => props.theme.radii[2]}px;
`;
