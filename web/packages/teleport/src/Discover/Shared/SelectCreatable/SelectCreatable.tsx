/**
 * Copyright 2022 Gravitational, Inc.
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

import React from 'react';
import ReactSelectCreatable from 'react-select/creatable';
import theme from 'design/theme';

const styles = {
  multiValue: (base, state) => {
    return state.data.isFixed ? { ...base, backgroundColor: 'gray' } : base;
  },
  multiValueLabel: (base, state) => {
    if (state.data.isFixed) {
      return { ...base, color: theme.colors.text.main, paddingRight: 6 };
    }

    if (state.isDisabled) {
      return { ...base, paddingRight: 6 };
    }

    return { ...base, color: theme.colors.text.primaryInverse };
  },
  multiValueRemove: (base, state) => {
    return state.data.isFixed || state.isDisabled
      ? { ...base, display: 'none' }
      : {
          ...base,
          cursor: 'pointer',
          color: theme.colors.text.primaryInverse,
        };
  },
  menuList: base => {
    return { ...base, color: theme.colors.text.primaryInverse };
  },
};

export type SelectCreatableProps = {
  inputValue: string;
  placeholder?: string;
  isDisabled?: boolean;
  // isClearable removes all selections
  // defined by field `value`.
  isClearable?: boolean;
  // isMulti allows users to select more
  // than one option.
  isMulti?: boolean;
  // value is the current set of selected options.
  value: Option[];
  // options are the drop down list of selectable
  // options.
  options: Option[];
  onChange(value, action): void;
  onInputChange?(i: string): void;
  onKeyDown?(e: React.KeyboardEvent): void;
  autoFocus?: boolean;
};

export const SelectCreatable = ({
  isMulti = true,
  isClearable = true,
  isDisabled = false,
  autoFocus = false,
  ...rest
}: SelectCreatableProps) => (
  <ReactSelectCreatable
    className="react-select"
    components={{
      DropdownIndicator: null,
    }}
    styles={styles}
    {...rest}
    isMulti={isMulti}
    isClearable={isClearable}
    isDisabled={isDisabled}
    autoFocus={autoFocus}
  />
);

export type Option = {
  // value is the actual value used inlieu of label.
  value: string;
  // label is the value user sees in the select options dropdown.
  label: string;
  // isFixed is a flag that when true doesn't allow this option
  // to be removable.
  isFixed?: boolean;
};
