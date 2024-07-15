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

import React, { FocusEvent } from 'react';

import { StylesConfig } from 'react-select';

export type Props = {
  inputId?: string;
  hasError?: boolean;
  isClearable?: boolean;
  closeMenuOnSelect?: boolean;
  isSimpleValue?: boolean;
  isSearchable?: boolean;
  isDisabled?: boolean;
  menuIsOpen?: boolean;
  hideSelectedOptions?: boolean;
  controlShouldRenderValue?: boolean;
  maxMenuHeight?: number;
  onChange(e: Option<any, any> | Option<any, any>[], action?: ActionMeta): void;
  onKeyDown?(e: KeyboardEvent | React.KeyboardEvent): void;
  value: null | Option<any, any> | Option<any, any>[];
  isMulti?: boolean;
  autoFocus?: boolean;
  label?: string;
  placeholder?: string;
  options?: Option<any, any>[] | GroupOption[];
  width?: string | number;
  menuPlacement?: string;
  name?: string;
  minMenuHeight?: number;
  components?: any;
  /**
   * customProps are any props that are not react-select
   * default or option props and need to be accessed through a
   * react-select custom component. `customProps` can be accessed
   * through react-select prop `selectProps`.
   * eg: `selectProps.customProps.<the-prop-name>`
   */
  customProps?: Record<string, any>;
  menuPosition?: 'fixed' | 'absolute';
  inputValue?: string;
  filterOption?(): null | boolean;
  onInputChange?(value: string, actionMeta: ActionMeta): void;
  // Whether or not the element is on an elevated platform (such as a dialog).
  elevated?: boolean;
  stylesConfig?: StylesConfig;
  formatCreateLabel?: (i: string) => string;
};

export type AsyncProps = Omit<Props, 'options'> & {
  defaultOptions?: true | Option;
  cacheOptions?: boolean;
  defaultMenuIsOpen?: boolean;
  loadOptions(input: string, o?: Option[]): Promise<Option[] | void>;
  noOptionsMessage(): string;
};

/**
 * Properties specific to `react-select`'s Creatable widget.
 */
export type CreatableProps = Props & {
  onBlur?(e: FocusEvent): void;
};

// Option defines the data type for select dropdown list.
export type Option<T = string, S = string> = {
  // value is the actual value used inlieu of label.
  value: T;
  // label is the value user sees in the select options dropdown.
  label: S;
};

export type GroupOption = {
  label: string;
  options: Option[];
};

export type ActionMeta = {
  action: 'set-value' | 'input-change' | 'input-blur' | 'menu-close' | 'clear';
};

/**
 * CustomSelectComponentProps defines a prop type for the custom
 * components you define for react-select's `components` prop.
 *
 * @template CustomProps - type defining all the custom props being passed
 * down to custom component.
 *
 * @template CustomOption - the data type used for react-select `options`
 */
// prettier-ignore
export type CustomSelectComponentProps<
  CustomProps,
  CustomOption = Option,
> = CustomOption & {
  /**
   * selectProps is the field to use to access the props that were
   * passed down to react-select's component.
   *
   * Use `customProps` field to easily identify non react-select props
   * that are intended to be used in custom components.
   */
  selectProps: { customProps: CustomProps };
};
