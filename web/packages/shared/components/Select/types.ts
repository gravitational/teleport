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

import {
  GroupBase,
  OnChangeValue,
  OptionProps,
  Props as ReactSelectProps,
  StylesConfig,
} from 'react-select';
import { AsyncProps as ReactSelectAsyncProps } from 'react-select/async';
import { AsyncCreatableProps as ReactSelectAsyncCreatableProps } from 'react-select/async-creatable';
import { CreatableProps as ReactSelectCreatableProps } from 'react-select/creatable';

export type SelectSize = 'large' | 'medium' | 'small';

export type CommonProps<Opt, IsMulti extends boolean> = {
  size?: SelectSize;
  hasError?: boolean;
  /**
   * customProps are any props that are not react-select
   * default or option props and need to be accessed through a
   * react-select custom component. `customProps` can be accessed
   * through react-select prop `selectProps`.
   * eg: `selectProps.customProps.<the-prop-name>`
   */
  customProps?: Record<string, any>;
  /** Whether or not the element is on an elevated platform (such as a dialog). */
  elevated?: boolean;
  stylesConfig?: StylesConfig;
  // We redeclare the `value` field to narrow its type a bit. The permissive
  // definition in react-select is mainly for historical compatibility, and
  // having this more strict (depending on `IsMulti`) helps us to pass the
  // correct type to the validation rules in `FieldSelect`.
  value?: OnChangeValue<Opt, IsMulti>;
};

export type Props<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
> = ReactSelectProps<Opt, IsMulti, Group> & CommonProps<Opt, IsMulti>;

export type AsyncProps<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
> = ReactSelectAsyncProps<Opt, IsMulti, Group> & CommonProps<Opt, IsMulti>;

/**
 * Properties specific to `react-select`'s Creatable widget.
 */
export type CreatableProps<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
> = ReactSelectCreatableProps<Opt, IsMulti, Group> & CommonProps<Opt, IsMulti>;

export type AsyncCreatableProps<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
> = ReactSelectAsyncCreatableProps<Opt, IsMulti, Group> &
  CommonProps<Opt, IsMulti>;

// Option defines the data type for select dropdown list.
export type Option<T = string, S = string> = {
  // value is the actual value used inlieu of label.
  value: T;
  // label is the value user sees in the select options dropdown.
  label: S;
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
  IsMulti extends boolean = false,
  Group extends GroupBase<CustomOption> = GroupBase<CustomOption>,
> = OptionProps<CustomOption, IsMulti, Group> & CustomOption & {
  /**
   * selectProps is the field to use to access the props that were
   * passed down to react-select's component.
   *
   * Use `customProps` field to easily identify non react-select props
   * that are intended to be used in custom components.
   */
  selectProps: { customProps: CustomProps };
};
