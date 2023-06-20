/**
 * Copyright 2023 Gravitational, Inc.
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

import React, { useRef, useEffect } from 'react';
import styled from 'styled-components';
import { Box, Flex } from 'design';

import {
  SearchContextProvider,
  useSearchContext,
} from 'teleterm/ui/Search/SearchContext';
import { KeyboardShortcutAction } from 'teleterm/services/config';
import {
  useKeyboardShortcutFormatters,
  useKeyboardShortcuts,
} from 'teleterm/ui/services/keyboardShortcuts';

import { useAppContext } from '../appContextProvider';

const OPEN_SEARCH_BAR_SHORTCUT_ACTION: KeyboardShortcutAction = 'openSearchBar';

export function SearchBarConnected() {
  const { workspacesService } = useAppContext();
  workspacesService.useState();

  if (!workspacesService.getRootClusterUri()) {
    return null;
  }

  return (
    <SearchContextProvider>
      <SearchBar />
    </SearchContextProvider>
  );
}

function SearchBar() {
  const containerRef = useRef<HTMLElement>();
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

    const onClickOutside = e => {
      if (!e.composedPath().includes(containerRef.current)) {
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
    (event: FocusEvent) => {
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
        min-width: calc(${props => props.theme.space[7]}px * 2);
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
  min-width: calc(${props => props.theme.space[9]}px * 2);
  background: transparent;
  color: inherit;
  box-sizing: border-box;
  outline: none;
  border: none;
  font-size: 14px;
  border-radius: ${props => props.theme.radii[2]}px;
  padding-inline: ${props => props.theme.space[2]}px;

  ::placeholder {
    color: ${props => props.theme.colors.text.slightlyMuted};
  }
`;

const Shortcut = styled(Box).attrs({ p: 1, mr: 2 })`
  color: ${({ theme }) => theme.colors.text.slightlyMuted};
  background-color: ${({ theme }) => theme.colors.levels.elevated};
  line-height: 12px;
  font-size: 12px;
  border-radius: ${props => props.theme.radii[2]}px;
`;
