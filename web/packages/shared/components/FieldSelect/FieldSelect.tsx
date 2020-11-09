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
import Select, { Props as SelectProps } from './../Select';
import { Box, LabelInput } from 'design';
import { useRule } from 'shared/components/Validation';

export default function FieldSelect({
  label,
  value,
  options,
  onChange,
  placeholder,
  maxMenuHeight,
  clearable,
  isMulti,
  rule = defaultRule,
  isSearchable = false,
  isSimpleValue = false,
  autoFocus = false,
  isDisabled = false,
  ...styles
}: Props) {
  const { valid, message } = useRule(rule(value));
  const hasError = Boolean(!valid);
  const labelText = hasError ? message : label;
  return (
    <Box mb="4" {...styles}>
      {label && <LabelInput hasError={hasError}>{labelText}</LabelInput>}
      <Select
        hasError={hasError}
        isSimpleValue={isSimpleValue}
        isSearchable={isSearchable}
        clearable={clearable}
        value={value}
        onChange={onChange}
        options={options}
        maxMenuHeight={maxMenuHeight}
        placeholder={placeholder}
        isMulti={isMulti}
        autoFocus={autoFocus}
        isDisabled={isDisabled}
      />
    </Box>
  );
}

const defaultRule = () => () => ({ valid: true });

type Props = SelectProps & {
  autoFocus?: boolean;
  label?: string;
  rule?: Function;
};
