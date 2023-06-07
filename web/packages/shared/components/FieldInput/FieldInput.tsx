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

import React, { forwardRef } from 'react';
import { Box, Input, LabelInput, Text } from 'design';

import { ToolTipInfo } from 'shared/components/ToolTip';
import { useRule } from 'shared/components/Validation';

const FieldInput = forwardRef<HTMLInputElement, Props>(
  (
    {
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
      name,
      type = 'text',
      autoFocus = false,
      autoComplete = 'off',
      inputMode = 'text',
      readonly = false,
      toolTipContent = null,
      disabled = false,
      markAsError = false,
      ...styles
    },
    ref
  ) => {
    const { valid, message } = useRule(rule(value));
    const hasError = !valid;
    const labelText = hasError ? message : label;
    const $inputElement = (
      <Input
        mt={1}
        ref={ref}
        type={type}
        name={name}
        hasError={hasError || markAsError}
        placeholder={placeholder}
        autoFocus={autoFocus}
        value={value}
        min={min}
        max={max}
        autoComplete={autoComplete}
        onChange={onChange}
        onKeyPress={onKeyPress}
        readOnly={readonly}
        inputMode={inputMode}
        defaultValue={defaultValue}
        disabled={disabled}
      />
    );

    return (
      <Box mb="4" {...styles}>
        {label ? (
          <LabelInput mb={0} hasError={hasError}>
            {toolTipContent ? (
              <>
                <span
                  css={{
                    marginRight: '4px',
                    verticalAlign: 'middle',
                  }}
                >
                  {labelText}
                  {labelTip && <LabelTip text={labelTip} />}
                </span>
                <ToolTipInfo children={toolTipContent} />
              </>
            ) : (
              <>
                {labelText}
                {labelTip && <LabelTip text={labelTip} />}
              </>
            )}
            {$inputElement}
          </LabelInput>
        ) : (
          $inputElement
        )}
      </Box>
    );
  }
);

const defaultRule = () => () => ({ valid: true });

const LabelTip = ({ text }) => (
  <Text as="span" style={{ fontWeight: 'normal' }}>{` - ${text}`}</Text>
);

export default FieldInput;

type Props = {
  value?: string;
  label?: string;
  labelTip?: string;
  placeholder?: string;
  autoFocus?: boolean;
  autoComplete?: 'off' | 'on' | 'one-time-code';
  type?: 'email' | 'text' | 'password' | 'number' | 'date' | 'week';
  inputMode?: 'text' | 'numeric';
  rule?: (options: unknown) => () => unknown;
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onKeyPress?: (e: React.KeyboardEvent<HTMLInputElement>) => void;
  readonly?: boolean;
  defaultValue?: string;
  min?: number;
  max?: number;
  toolTipContent?: React.ReactNode;
  disabled?: boolean;
  // markAsError is a flag to highlight an
  // input box as error color before validator
  // runs (which marks it as error)
  markAsError?: boolean;
  // TS: temporary handles ...styles
  [key: string]: any;
};
