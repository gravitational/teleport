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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import styled from 'styled-components';
import { debounce } from 'lodash';
import { Flex } from 'design';
import { color, height, space, width } from 'styled-system';
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
  const measuringInputRef = useRef<HTMLSpanElement>();
  const refList = useRef<HTMLElement>();
  const refContainer = useRef<HTMLElement>();
  const [measuredInputTextWidth, setMeasuredInputTextWidth] =
    useState<number>();

  const handleInputChange = useMemo(() => {
    return debounce(() => {
      props.onInputChange(refInput.current.value);
      measureInputTextWidth();
    }, 100);
  }, []);

  // Update input value if it changed outside of this component. This happens when the user pick an
  // autocomplete suggestion.
  useEffect(() => {
    if (refInput.current.value !== inputValue) {
      refInput.current.value = inputValue;
      measureInputTextWidth();
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

  const measureInputTextWidth = () => {
    const width = measuringInputRef.current?.getBoundingClientRect().width || 0;
    setMeasuredInputTextWidth(width);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    const keyCode = e.which;
    switch (keyCode) {
      case KeyEnum.RETURN:
        e.stopPropagation();
        e.preventDefault();

        props.onEnter(activeSuggestion);
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
        width: '100%',
        height: '100%',
      }}
      flex={1}
      ref={refContainer}
      onFocus={handleOnFocus}
      onBlur={handleOnBlur}
    >
      <MeasuringInput ref={measuringInputRef}>{inputValue}</MeasuringInput>
      <Input
        ref={refInput}
        spellCheck={false}
        placeholder="Enter a command and press enter"
        onChange={handleInputChange}
        onKeyDown={handleKeyDown}
        isOpened={visible}
      />
      {visible && hasSuggestions && (
        <QuickInputList
          ref={refList}
          position={measuredInputTextWidth}
          items={autocompleteResult.suggestions}
          activeItem={activeSuggestion}
          onPick={props.onEnter}
        />
      )}
    </Flex>
  );
}

const MeasuringInput = styled.span`
  z-index: -1;
  font-size: 14px;
  padding-left: 8px;
  position: absolute;
  visibility: hidden;
`;

const Input = styled.input(props => {
  const { theme } = props;
  return {
    height: '100%',
    background: 'inherit',
    display: 'flex',
    flex: '1',
    zIndex: '0',
    boxSizing: 'border-box',
    color: theme.colors.text.primary,
    width: '100%',
    fontSize: '14px',
    border: `0.5px ${theme.colors.action.disabledBackground} solid`,
    borderRadius: '4px',
    outline: 'none',
    padding: '2px 8px',
    '::placeholder': {
      color: theme.colors.text.secondary,
    },
    '&:hover, &:focus': {
      color: theme.colors.primary.contrastText,
      borderColor: theme.colors.light,
    },
    '&:focus': {
      borderColor: theme.colors.secondary.main,
      '::placeholder': {
        color: theme.colors.text.placeholder,
      },
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
