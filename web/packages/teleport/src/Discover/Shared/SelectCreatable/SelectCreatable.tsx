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
import ReactSelectCreatable from 'react-select/creatable';
import { useTheme } from 'styled-components';

import { Cross } from 'design/Icon';

export const styles = theme => ({
  multiValue: (base, state) => {
    return state.data.isFixed
      ? { ...base, backgroundColor: `${theme.colors.spotBackground[2]}` }
      : { ...base, backgroundColor: `${theme.colors.spotBackground[0]}` };
  },
  multiValueLabel: (base, state) => {
    if (state.data.isFixed) {
      return { ...base, color: theme.colors.text.main, paddingRight: 6 };
    }

    if (state.isDisabled) {
      return { ...base, paddingRight: 6, color: theme.colors.text.main };
    }

    return { ...base, color: theme.colors.text.primary };
  },
  multiValueRemove: (base, state) => {
    return state.data.isFixed || state.isDisabled
      ? { ...base, display: 'none' }
      : {
          ...base,
          cursor: 'pointer',
          color: theme.colors.text.primary,
        };
  },
  menuList: base => {
    return {
      ...base,
      color: theme.colors.text.primary,
      backgroundColor: theme.colors.spotBackground[0],
    };
  },

  control: base => ({
    ...base,
    backgroundColor: theme.colors.levels.surface,
  }),

  input: base => ({
    ...base,
    color: theme.colors.text.primary,
  }),

  menu: base => ({ ...base, backgroundColor: theme.colors.levels.elevated }),

  option: (base, state) => {
    if (state.isFocused) {
      return {
        ...base,
        backgroundColor: theme.colors.spotBackground[1],
      };
    }
    return base;
  },
});

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

// TODO(bl-nero): There's no need for this to be a separate component. Migrate
// it to the shared component.
export const SelectCreatable = ({
  isMulti = true,
  isClearable = true,
  isDisabled = false,
  autoFocus = false,
  ...rest
}: SelectCreatableProps) => {
  const theme = useTheme();
  return (
    <ReactSelectCreatable
      className="react-select"
      components={{
        DropdownIndicator: null,
        CrossIcon: () => <Cross />,
      }}
      styles={styles(theme)}
      {...rest}
      isMulti={isMulti}
      isClearable={isClearable}
      isDisabled={isDisabled}
      autoFocus={autoFocus}
    />
  );
};

export type Option = {
  // value is the actual value used inlieu of label.
  value: string;
  // label is the value user sees in the select options dropdown.
  label: string;
  // isFixed is a flag that when true doesn't allow this option
  // to be removable.
  isFixed?: boolean;
};
