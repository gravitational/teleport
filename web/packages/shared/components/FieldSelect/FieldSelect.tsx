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

import { GroupBase } from 'react-select';

import { useAsync } from 'shared/hooks/useAsync';

import Select, {
  Props as SelectProps,
  SelectAsync,
  AsyncProps as AsyncSelectProps,
  Option,
} from '../Select';

import {
  FieldProps,
  FieldSelectWrapper,
  resolveUndefinedOptions,
  splitSelectProps,
} from './shared';

export function FieldSelect<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
>(props: SelectProps<Opt, IsMulti, Group> & FieldProps<Opt, IsMulti>) {
  const { base, wrapper, others } = splitSelectProps<
    Opt,
    IsMulti,
    Group,
    typeof props
  >(props, {});
  return (
    <FieldSelectWrapper {...wrapper} {...others}>
      <Select<Opt, IsMulti, Group> {...base} />
    </FieldSelectWrapper>
  );
}

/** @deprecated Use the named export `{ FieldSelect }`. */
export default FieldSelect;

/**
 * A select field that asynchronously loads options.
 * The prop `loadOptions` accepts a callback that provides
 * an `input` value used for filtering options on the call site.
 *
 * If `loadOptions` returns an error, the user can retry loading options by
 * changing the input.
 * Note: It is not possible to re-fetch the initial call for options.
 * ReactSelect fetches them when the component mounts and then keeps in memory.
 */
export function FieldSelectAsync<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
>(props: AsyncSelectProps<Opt, IsMulti, Group> & FieldProps<Opt, IsMulti>) {
  const { base, wrapper, others } = splitSelectProps<
    Opt,
    IsMulti,
    Group,
    typeof props
  >(props, {
    defaultOptions: true,
  });
  const { defaultOptions, loadOptions, ...styles } = others;
  const [attempt, runAttempt] = useAsync(resolveUndefinedOptions(loadOptions));
  return (
    <FieldSelectWrapper {...wrapper} {...styles}>
      <SelectAsync<Opt, IsMulti, Group>
        {...base}
        defaultOptions={defaultOptions}
        loadOptions={async (value, callback) => {
          console.log('loading');
          const [options, error] = await runAttempt(value, callback);
          if (error) {
            return [];
          }
          return options;
        }}
        noOptionsMessage={obj => {
          if (attempt.status === 'error') {
            return `Could not load options: ${attempt.statusText}`;
          }
          return base.noOptionsMessage?.(obj) ?? 'No options';
        }}
      />
    </FieldSelectWrapper>
  );
}
