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

import { GroupBase, OptionsOrGroups } from 'react-select';

import { useAsync } from 'shared/hooks/useAsync';

import Select, {
  AsyncProps as AsyncSelectProps,
  Option,
  SelectAsync,
  Props as SelectProps,
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
>(
  props: AsyncSelectProps<Opt, IsMulti, Group> &
    FieldProps<Opt, IsMulti> & {
      /**
       * A function that sets the initial options, after the initial options are
       * finished fetching (triggered by when user clicks on the select component
       * that renders the dropdown menu).
       *
       * Select async doesn't provide an option for "on menu open, load options".
       * There is only "on render load, or provide default array of options".
       * There are some cases where there can be many select async components rendered
       * (eg: bulk adding kube clusters to an access request) and users may not be
       * required to select anything from the select async dropdown, so this provides
       * a way to load options only on need (menu open) and save wasteful api calls.
       *
       * Requires:
       *   - base.onMenuOpen to be defined
       *   - defaultOptions to be an array
       */
      initOptionsOnMenuOpen?(options: OptionsOrGroups<Opt, Group>): void;
    }
) {
  const { base, wrapper, others } = splitSelectProps<
    Opt,
    IsMulti,
    Group,
    typeof props
  >(props, {
    defaultOptions: true,
  });
  const { defaultOptions, loadOptions, initOptionsOnMenuOpen, ...styles } =
    others;
  const [attempt, runAttempt] = useAsync(resolveUndefinedOptions(loadOptions));

  async function onMenuOpen() {
    if (!base.onMenuOpen) return;

    base.onMenuOpen();

    if (
      initOptionsOnMenuOpen &&
      defaultOptions &&
      Array.isArray(defaultOptions) &&
      defaultOptions.length == 0
    ) {
      const [options, error] = await runAttempt('', null);
      if (!error) {
        return others.initOptionsOnMenuOpen(options);
      }
    }
  }

  return (
    <FieldSelectWrapper {...wrapper} {...styles}>
      <SelectAsync<Opt, IsMulti, Group>
        {...base}
        onMenuOpen={onMenuOpen}
        defaultOptions={defaultOptions}
        loadOptions={async (value, callback) => {
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
          if (attempt.status === 'processing') {
            return 'Loading...';
          }
          return base.noOptionsMessage?.(obj) ?? 'No options';
        }}
      />
    </FieldSelectWrapper>
  );
}
