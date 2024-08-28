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

import { Box, LabelInput } from 'design';

import { GroupBase, OnChangeValue, OptionsOrGroups } from 'react-select';

import { BoxProps } from 'design/Box';

import { useRule } from 'shared/components/Validation';
import { useAsync } from 'shared/hooks/useAsync';

import Select, {
  Props as SelectProps,
  SelectAsync,
  AsyncProps as AsyncSelectProps,
  Option,
} from '../Select';

import { LabelTip, defaultRule } from './shared';

export function FieldSelect<
  Opt = Option,
  IsMulti extends boolean = false,
  Group extends GroupBase<Opt> = GroupBase<Opt>,
>({
  components,
  label,
  labelTip,
  value,
  options,
  name,
  onChange,
  placeholder,
  maxMenuHeight,
  isClearable,
  isMulti,
  menuPosition,
  rule = defaultRule,
  stylesConfig,
  isSearchable = false,
  autoFocus = false,
  isDisabled = false,
  elevated = false,
  inputId = 'select',
  defaultValue,
  ...styles
}: SelectProps<Opt, IsMulti, Group> & FieldProps<Opt, IsMulti>) {
  const { valid, message } = useRule(rule(value));
  const hasError = Boolean(!valid);
  const labelText = hasError ? message : label;
  return (
    <Box mb="4" {...styles}>
      {label && (
        <LabelInput htmlFor={inputId} hasError={hasError}>
          {labelText}
          {labelTip && <LabelTip text={labelTip} />}
        </LabelInput>
      )}
      <Select<Opt, IsMulti, Group>
        components={components}
        stylesConfig={stylesConfig}
        inputId={inputId}
        name={name}
        menuPosition={menuPosition}
        hasError={hasError}
        isSearchable={isSearchable}
        isClearable={isClearable}
        value={value}
        onChange={onChange}
        options={options}
        maxMenuHeight={maxMenuHeight}
        placeholder={placeholder}
        isMulti={isMulti}
        autoFocus={autoFocus}
        isDisabled={isDisabled}
        elevated={elevated}
        defaultValue={defaultValue}
      />
    </Box>
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
>({
  components,
  label,
  labelTip,
  value,
  name,
  onChange,
  placeholder,
  maxMenuHeight,
  isClearable,
  isMulti,
  menuPosition,
  rule = defaultRule,
  stylesConfig,
  isSearchable,
  autoFocus,
  isDisabled,
  elevated,
  noOptionsMessage,
  loadOptions,
  inputId = 'select',
  defaultValue,
  ...styles
}: AsyncSelectProps<Opt, IsMulti, Group> & FieldProps<Opt, IsMulti>) {
  const [attempt, runAttempt] = useAsync(resolveUndefinedOptions(loadOptions));
  const { valid, message } = useRule(rule(value));
  const hasError = Boolean(!valid);
  const labelText = hasError ? message : label;
  return (
    <Box mb="4" {...styles}>
      <LabelInput htmlFor={inputId} hasError={hasError}>
        {labelText}
        {labelTip && <LabelTip text={labelTip} />}
      </LabelInput>
      <SelectAsync<Opt, IsMulti, Group>
        components={components}
        stylesConfig={stylesConfig}
        inputId={inputId}
        name={name}
        menuPosition={menuPosition}
        hasError={hasError}
        isSearchable={isSearchable}
        isClearable={isClearable}
        value={value}
        onChange={onChange}
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
          return noOptionsMessage?.(obj) ?? 'No options';
        }}
        maxMenuHeight={maxMenuHeight}
        defaultOptions={true}
        placeholder={placeholder}
        isMulti={isMulti}
        autoFocus={autoFocus}
        isDisabled={isDisabled}
        elevated={elevated}
        defaultValue={defaultValue}
      />
    </Box>
  );
}

type FieldProps<Opt, IsMulti extends boolean> = BoxProps & {
  autoFocus?: boolean;
  label?: string;
  labelTip?: string;
  rule?: (options: OnChangeValue<Opt, IsMulti>) => () => unknown;
};

/**
 * Returns an option loader that wraps given function and returns a promise to
 * an empty array if the wrapped function returns `undefined`. This wrapper is
 * useful for using the `loadingOptions` callback in context where a promise is
 * strictly required, while the declaration of the `loadingOptions` attribute
 * allows a `void` return type.
 */
export const resolveUndefinedOptions =
  <Opt, Group extends GroupBase<Opt>>(
    loadOptions: AsyncSelectProps<Opt, false, Group>['loadOptions']
  ) =>
  (
    value: string,
    callback?: (options: OptionsOrGroups<Opt, Group>) => void
  ) => {
    const result = loadOptions(value, callback);
    if (!result) {
      return Promise.resolve([] as Opt[]);
    }
    return result;
  };
