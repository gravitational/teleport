/**
 Copyright 2022 Gravitational, Inc.

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

import React from 'react';
import { Box, LabelInput, Text, TextArea } from 'design';
import { TextAreaProps } from 'design/TextArea';

import { useRule } from 'shared/components/Validation';

export interface FieldTextAreaProps
  extends Pick<
    TextAreaProps,
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
