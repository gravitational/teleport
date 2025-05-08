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
import { GroupBase, OnChangeValue } from 'react-select';

import { BoxProps } from 'design/Box';
import { useAsync } from 'shared/hooks/useAsync';

import {
  AsyncProps,
  Option,
  SelectCreatable,
  CreatableProps as SelectCreatableProps,
} from '../Select';
import { SelectCreatableAsync } from '../Select/Select';
import { Rule } from '../Validation/rules';
import {
  FieldProps,
  FieldSelectWrapper,
  resolveUndefinedOptions,
  splitSelectProps,
} from './shared';

/**
 * Returns a styled SelectCreatable with label, input validation rule and error handling.
 * @param {() => void} onChange - change handler.
 * @param {defaultRule} rule - rules for the select component.
 * @param {boolean} markAsError - manually mark the component as error.
 * @param {string} placeholder - placeholder value.
 * @param {string} formatCreateLabel - custom formatting for create label.
 * @param {StylesConfig} stylesConfig - custom styles for the inner select component.
 * @returns SelectCreatable
 */
export function FieldSelectCreatable<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
>(props: SelectCreatableProps<Opt, IsMulti, Group> & FieldProps<Opt, IsMulti>) {
  const { base, wrapper, others } = splitSelectProps<
    Opt,
    IsMulti,
    Group,
    typeof props
  >(props, {});
  const { formatCreateLabel, ...styles } = others;
  return (
    <FieldSelectWrapper {...wrapper} {...styles}>
      <SelectCreatable<Opt, IsMulti, Group>
        {...base}
        formatCreateLabel={formatCreateLabel}
      />
    </FieldSelectWrapper>
  );
}

/**
 * A select creatable field that asynchronously loads options.
 * The prop `loadOptions` accepts a callback that provides
 * an `input` value used for filtering options on the call site.
 *
 * If `loadOptions` returns an error, the user can retry loading options by
 * changing the input.
 * Note: It is not possible to re-fetch the initial call for options.
 * ReactSelect fetches them when the component mounts and then keeps in memory.
 */
export function FieldSelectCreatableAsync<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
>(
  props: AsyncProps<Opt, IsMulti, Group> & CreatableProps<Opt, IsMulti, Group>
) {
  const { base, wrapper, others } = splitSelectProps<
    Opt,
    IsMulti,
    Group,
    typeof props
  >(props, {
    defaultOptions: true,
  });
  const { defaultOptions, loadOptions, formatCreateLabel, ...styles } = others;
  const [attempt, runAttempt] = useAsync(resolveUndefinedOptions(loadOptions));
  return (
    <FieldSelectWrapper {...wrapper} {...styles}>
      <SelectCreatableAsync
        {...base}
        defaultOptions={defaultOptions}
        formatCreateLabel={formatCreateLabel}
        loadOptions={async (input, option) => {
          const [options, error] = await runAttempt(input, option);
          if (error) {
            return [];
          }
          return options;
        }}
        noOptionsMessage={obj => {
          if (attempt.status === 'error') {
            return `Could not load options: ${attempt.error}`;
          }
          return base.noOptionsMessage?.(obj) ?? 'No options';
        }}
      />
    </FieldSelectWrapper>
  );
}

type CreatableProps<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
> = SelectCreatableProps<Opt, IsMulti, Group> &
  BoxProps & {
    autoFocus?: boolean;
    label?: string;
    toolTipContent?: React.ReactNode;
    rule?: Rule<OnChangeValue<Opt, IsMulti>>;
    markAsError?: boolean;
    ariaLabel?: string;
  };
