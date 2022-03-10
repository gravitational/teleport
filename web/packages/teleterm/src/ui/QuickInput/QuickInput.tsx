/**
 * Copyright 2021 Gravitational, Inc.
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

import React, { useRef, useEffect, useMemo } from 'react';
import styled from 'styled-components';
import { debounce } from 'lodash';
import { Box, Flex } from 'design';
import { space, width, color, height } from 'styled-system';
import useQuickInput, { State } from './useQuickInput';
import QuickInputList from './QuickInputList';

export default function Container() {
  const state = useQuickInput();
  return <QuickInput {...state} />;
}

export function QuickInput(props: State) {
  const { visible, activeSuggestion, autocompleteResult, inputValue } = props;
  const hasSuggestions =
    autocompleteResult.kind === 'autocomplete.partial-match';
  const refInput = useRef<HTMLInputElement>();
  const refList = useRef<HTMLElement>();
  const refContainer = useRef<HTMLElement>();

  const handleInputChange = useMemo(() => {
    return debounce(() => {
      props.onInputChange(refInput.current.value);
    }, 100);
  }, []);

  // Update input value if it changed outside of this component. This happens when the user pick an
  // autocomplete suggestion.
  useEffect(() => {
    if (refInput.current.value !== inputValue) {
      refInput.current.value = inputValue;
    }
  }, [inputValue]);

  function handleOnFocus(e: React.SyntheticEvent) {
    // trigger a callback when focus is coming from external element
    if (!refContainer.current.contains(e['relatedTarget'])) {
      props.onFocus(e);
    }

    // ensure that
    if (!visible) {
      props.onShow();
    }
  }

  function handleOnBlur(e: any) {
    const inside =
      e?.relatedTarget?.contains(refInput.current) ||
      e?.relatedTarget?.contains(refList.current);

    if (inside) {
      refInput.current.focus();
      return;
    }

    props.onHide();
  }

  const handleArrowKey = (e: React.KeyboardEvent, nudge = 0) => {
    e.stopPropagation();
    if (!hasSuggestions) {
      return;
    }
    const next = getNext(
      activeSuggestion + nudge,
      autocompleteResult.suggestions.length
    );
    props.onActiveSuggestion(next);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    const keyCode = e.which;
    switch (keyCode) {
      case KeyEnum.RETURN:
        e.stopPropagation();
        e.preventDefault();

        props.onPickSuggestion(activeSuggestion);
        return;
      case KeyEnum.ESC:
        props.onBack();
        return;
      case KeyEnum.TAB:
        return;
      case KeyEnum.UP:
        e.stopPropagation();
        e.preventDefault();
        handleArrowKey(e, -1);
        return;
      case KeyEnum.DOWN:
        e.stopPropagation();
        e.preventDefault();
        handleArrowKey(e, 1);
        return;
    }
  };

  useEffect(() => {
    if (visible) {
      refInput.current.focus();
    }

    return () => handleInputChange.cancel();
  }, [visible]);

  return (
    <Flex
      style={{
        position: 'relative',
      }}
      justifyContent="center"
      ref={refContainer}
      onFocus={handleOnFocus}
      onBlur={handleOnBlur}
    >
      <Box width="600px" mx="auto">
        <Input
          ref={refInput}
          placeholder="Enter a command and press enter"
          onChange={handleInputChange}
          onKeyDown={handleKeyDown}
        />
      </Box>
      {visible && hasSuggestions && (
        <QuickInputList
          ref={refList}
          items={autocompleteResult.suggestions}
          activeItem={activeSuggestion}
          onPick={props.onPickSuggestion}
        />
      )}
    </Flex>
  );
}

const Input = styled.input(props => {
  const { theme } = props;
  return {
    height: '32px',
    background: theme.colors.primary.lighter,
    boxSizing: 'border-box',
    color: theme.colors.text.primary,
    width: '100%',
    border: 'none',
    outline: 'none',
    padding: '2px 8px',
    '&:hover, &:focus': {
      color: theme.colors.primary.contrastText,
      background: theme.colors.primary.lighter,

      opacity: 1,
    },

    ...space(props),
    ...width(props),
    ...height(props),
    ...color(props),
  };
});

const KeyEnum = {
  BACKSPACE: 8,
  TAB: 9,
  RETURN: 13,
  ALT: 18,
  ESC: 27,
  SPACE: 32,
  PAGE_UP: 33,
  PAGE_DOWN: 34,
  END: 35,
  HOME: 36,
  LEFT: 37,
  UP: 38,
  RIGHT: 39,
  DOWN: 40,
  DELETE: 46,
  COMMA: 188,
  PERIOD: 190,
  A: 65,
  Z: 90,
  ZERO: 48,
  NUMPAD_0: 96,
  NUMPAD_9: 105,
};

function getNext(selectedIndex = 0, max = 0) {
  let index = selectedIndex % max;
  if (index < 0) {
    index += max;
  }
  return index;
}
