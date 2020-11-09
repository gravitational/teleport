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

import React from 'react';
import { Box, Input, LabelInput } from 'design';
import { useRule } from 'shared/components/Validation';

export default function FieldInput({
  label,
  value,
  onChange,
  onKeyPress,
  placeholder,
  rule = defaultRule,
  type = 'text',
  autoFocus = false,
  autoComplete = 'off',
  readonly = false,
  ...styles
}: Props) {
  const { valid, message } = useRule(rule(value));
  const hasError = !valid;
  const labelText = hasError ? message : label;
  return (
    <Box mb="4" {...styles}>
      {label && <LabelInput hasError={hasError}>{labelText}</LabelInput>}
      <Input
        type={type}
        autoFocus={autoFocus}
        hasError={hasError}
        placeholder={placeholder}
        value={value}
        autoComplete={autoComplete}
        onChange={onChange}
        onKeyPress={onKeyPress}
        readOnly={readonly}
      />
    </Box>
  );
}

const defaultRule = () => () => ({ valid: true });

type Props = {
  value?: string;
  label?: string;
  placeholder?: string;
  autoFocus?: boolean;
  autoComplete?: 'off' | 'on';
  type?: 'email' | 'text' | 'password' | 'number' | 'date' | 'week';
  rule?: Function;
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onKeyPress?: (e: React.KeyboardEvent<HTMLInputElement>) => void;
  readonly?: boolean;
  // TS: temporary handles ...styles
  [key: string]: any;
};
