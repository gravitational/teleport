/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useRef, useEffect } from 'react';
import { Box, Input, LabelInput, Text } from 'design';

import { useRule } from 'shared/components/Validation';

export default function FieldInput({
  label,
  labelTip,
  value,
  onChange,
  onKeyPress,
  placeholder,
  defaultValue,
  min,
  max,
  rule = defaultRule,
  type = 'text',
  autoFocus = false,
  transitionPropertyName = '',
  refocusIndicator = '',
  autoComplete = 'off',
  inputMode = 'text',
  readonly = false,
  ...styles
}: Props) {
  const { valid, message } = useRule(rule(value));
  const hasError = !valid;
  const labelText = hasError ? message : label;

  const inputRef = useRef<HTMLInputElement>();

  useEffect(() => {
    if (!autoFocus) return;

    if (!transitionPropertyName) {
      inputRef.current.focus();
      return;
    }

    // autoFocusOnTransitionEnd focus's the input element after transition property name
    // defined by 'transitionPropertyName' has ended. This prevents auto focusing during
    // transitioning which causes transition to be jumpy caused by trying to bring focused
    // element into view. This also prevents prematurely showing the browser password
    // manager icons and tooltips while transitioing.
    function autoFocusOnTransitionEnd(e: TransitionEvent) {
      if (e.propertyName !== transitionPropertyName) return;
      inputRef.current.focus();
      // Since we only need to auto focus one time, the listener's are no longer needed.
      removeListeners();
    }

    // autoFocusOnTransitionCancel is fallback to autoFocusOnTransitionEnd when the transition
    // we are expecting gets canceled (sometimes happens in chrome, but strangely not in firefox).
    function autoFocusOnTransitionCancel(e: TransitionEvent) {
      if (e.propertyName !== transitionPropertyName) return;
      inputRef.current.focus();
      // Since we only need to auto focus one time, the listener is no longer needed.
      removeListeners();
    }

    function removeListeners() {
      window.removeEventListener('transitionend', autoFocusOnTransitionEnd);
      window.removeEventListener(
        'transitioncancel',
        autoFocusOnTransitionCancel
      );
    }

    window.addEventListener('transitionend', autoFocusOnTransitionEnd);
    window.addEventListener('transitioncancel', autoFocusOnTransitionCancel);

    return () => {
      removeListeners();
    };
  }, [refocusIndicator]);

  const $inputElement = (
    <Input
      mt={1}
      ref={inputRef}
      type={type}
      hasError={hasError}
      placeholder={placeholder}
      value={value}
      min={min}
      max={max}
      autoComplete={autoComplete}
      onChange={onChange}
      onKeyPress={onKeyPress}
      readOnly={readonly}
      inputMode={inputMode}
      defaultValue={defaultValue}
    />
  );

  return (
    <Box mb="4" {...styles}>
      {label ? (
        <LabelInput mb={0} hasError={hasError}>
          {labelText}
          {labelTip && <LabelTip text={labelTip} />}
          {$inputElement}
        </LabelInput>
      ) : (
        $inputElement
      )}
    </Box>
  );
}

const defaultRule = () => () => ({ valid: true });

const LabelTip = ({ text }) => (
  <Text as="span" style={{ fontWeight: 'normal' }}>{` - ${text}`}</Text>
);

type Props = {
  value?: string;
  label?: string;
  labelTip?: string;
  placeholder?: string;
  autoFocus?: boolean;
  autoComplete?: 'off' | 'on' | 'one-time-code';
  // transitionPropertyName if defined with flag 'autoFocus', is used
  // to determine if input element should be auto focused after
  // a transition has ended.
  transitionPropertyName?: string;
  // refocusIndicator is used as a listener for change (with any text value)
  // for the useEffect that handles the auto-focusing.
  refocusIndicator?: string;
  type?: 'email' | 'text' | 'password' | 'number' | 'date' | 'week';
  inputMode?: 'text' | 'numeric';
  rule?: (options: unknown) => () => unknown;
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onKeyPress?: (e: React.KeyboardEvent<HTMLInputElement>) => void;
  readonly?: boolean;
  defaultValue?: string;
  min?: number;
  max?: number;
  // TS: temporary handles ...styles
  [key: string]: any;
};
