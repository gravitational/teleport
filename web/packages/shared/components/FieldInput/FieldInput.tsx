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

import React, { forwardRef } from 'react';

import { Box, Input, LabelInput, Text } from 'design';
import { ToolTipInfo } from 'shared/components/ToolTip';
import { useRule } from 'shared/components/Validation';

const FieldInput = forwardRef<HTMLInputElement, FieldInputProps>(
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
      tooltipSticky = false,
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
                <ToolTipInfo sticky={tooltipSticky} children={toolTipContent} />
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

export type FieldInputProps = {
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
  tooltipSticky?: boolean;
  disabled?: boolean;
  // markAsError is a flag to highlight an
  // input box as error color before validator
  // runs (which marks it as error)
  markAsError?: boolean;
  // TS: temporary handles ...styles
  [key: string]: any;
};
