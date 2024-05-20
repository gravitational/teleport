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

import React from 'react';
import { Box, LabelInput, Text, TextArea } from 'design';

import { useRule } from 'shared/components/Validation';

export interface FieldTextAreaProps
  extends Pick<
    React.TextareaHTMLAttributes<HTMLTextAreaElement>,
    'onChange' | 'placeholder' | 'value' | 'readOnly' | 'autoFocus' | 'name'
  > {
  label?: string;
  labelTip?: string;
  autoComplete?: 'off' | 'on';
  textAreaCss?: string;
  rule?: (options: unknown) => () => unknown;
  resizable?: boolean;

  // TS: temporary handles ...styles
  [key: string]: any;
}

export function FieldTextArea({
  label,
  labelTip,
  value,
  onChange,
  placeholder,
  rule = defaultRule,
  autoFocus,
  autoComplete = 'off',
  readOnly,
  textAreaCss,
  name,
  resizable = true,
  ...styles
}: FieldTextAreaProps) {
  const { valid, message } = useRule(rule(value));
  const hasError = !valid;
  const labelText = hasError ? message : label;

  const $textAreaElement = (
    <TextArea
      mt={1}
      name={name}
      css={textAreaCss}
      hasError={hasError}
      placeholder={placeholder}
      value={value}
      autoComplete={autoComplete}
      autoFocus={autoFocus}
      onChange={onChange}
      readOnly={readOnly}
      resizable={resizable}
    />
  );

  return (
    <Box mb="4" {...styles}>
      {label ? (
        <LabelInput mb={0} hasError={hasError}>
          {labelText}
          {labelTip && <LabelTip text={labelTip} />}
          {$textAreaElement}
        </LabelInput>
      ) : (
        $textAreaElement
      )}
    </Box>
  );
}

const defaultRule = () => () => ({ valid: true });

const LabelTip = ({ text }) => (
  <Text as="span" style={{ fontWeight: 'normal' }}>{` - ${text}`}</Text>
);
