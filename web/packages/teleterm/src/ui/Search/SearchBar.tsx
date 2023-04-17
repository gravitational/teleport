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
    onInputValueChange,
    inputRef,
    opened,
    open,
    close,
  } = useSearchContext();
  const ctx = useAppContext();
  ctx.clustersService.useState();

  useKeyboardShortcuts({
    [OPEN_SEARCH_BAR_SHORTCUT_ACTION]: () => {
      open();
    },
  });

  useEffect(() => {
    const onClickOutside = e => {
      if (!e.composedPath().includes(containerRef.current)) {
        close();
      }
    };
    if (opened) {
      window.addEventListener('click', onClickOutside);
      return () => window.removeEventListener('click', onClickOutside);
    }
  }, [close, opened]);

  function handleOnFocus(e: React.FocusEvent) {
    open(e.relatedTarget);
  }

  const defaultInputProps = {
    ref: inputRef,
    role: 'searchbox',
    placeholder: activePicker.placeholder,
    value: inputValue,
    onChange: e => {
      onInputValueChange(e.target.value);
    },
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
        background: ${props => props.theme.colors.levels.sunkenSecondary};
        border: 1px ${props => props.theme.colors.action.disabledBackground}
          solid;
        border-radius: ${props => props.theme.radii[2]}px;

        &:hover {
          color: ${props => props.theme.colors.levels.contrast};
          background: ${props => props.theme.colors.levels.surface};
        }
      `}
      justifyContent="center"
      ref={containerRef}
      onFocus={handleOnFocus}
    >
      {!opened && (
        <>
          <Input {...defaultInputProps} />
          <Shortcut>{getAccelerator(OPEN_SEARCH_BAR_SHORTCUT_ACTION)}</Shortcut>
        </>
      )}
      {opened && (
        <activePicker.picker
          // autofocusing cannot be done in `open` function as it would focus the input from closed state
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
  background: inherit;
  color: inherit;
  box-sizing: border-box;
  outline: none;
  border: none;
  font-size: 14px;
  border-radius: ${props => props.theme.radii[2]}px;
  padding-inline: ${props => props.theme.space[2]}px;

  ::placeholder {
    color: ${props => props.theme.colors.text.secondary};
  }
`;

const Shortcut = styled(Box).attrs({ p: 1 })`
  position: absolute;
  right: ${props => props.theme.space[2]}px;
  top: 50%;
  transform: translate(0, -50%);
  color: ${({ theme }) => theme.colors.text.secondary};
  background-color: ${({ theme }) => theme.colors.levels.surface};
  line-height: 12px;
  font-size: 12px;
  border-radius: ${props => props.theme.radii[2]}px;
`;
