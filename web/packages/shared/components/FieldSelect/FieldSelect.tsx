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

import { useRule } from 'shared/components/Validation';
import { useAsync } from 'shared/hooks/useAsync';

import Select, {
  Props as SelectProps,
  SelectAsync,
  AsyncProps as AsyncSelectProps,
} from '../Select';

import { LabelTip, defaultRule } from './shared';

export function FieldSelect({
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
  isSimpleValue = false,
  autoFocus = false,
  isDisabled = false,
  elevated = false,
  inputId = 'select',
  ...styles
}: SelectProps & FieldProps) {
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
      <Select
        components={components}
        stylesConfig={stylesConfig}
        inputId={inputId}
        name={name}
        menuPosition={menuPosition}
        hasError={hasError}
        isSimpleValue={isSimpleValue}
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
export function FieldSelectAsync({
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
  isSimpleValue,
  autoFocus,
  isDisabled,
  elevated,
  noOptionsMessage,
  loadOptions,
  inputId = 'select',
  ...styles
}: AsyncSelectProps & FieldProps) {
  const [attempt, runAttempt] = useAsync(loadOptions);
  const { valid, message } = useRule(rule(value));
  const hasError = Boolean(!valid);
  const labelText = hasError ? message : label;
  return (
    <Box mb="4" {...styles}>
      <LabelInput htmlFor={inputId} hasError={hasError}>
        {labelText}
        {labelTip && <LabelTip text={labelTip} />}
      </LabelInput>
      <SelectAsync
        components={components}
        stylesConfig={stylesConfig}
        inputId={inputId}
        name={name}
        menuPosition={menuPosition}
        hasError={hasError}
        isSimpleValue={isSimpleValue}
        isSearchable={isSearchable}
        isClearable={isClearable}
        value={value}
        onChange={onChange}
        loadOptions={async (input, option) => {
          const [options, error] = await runAttempt(input, option);
          if (error) {
            return [];
          }
          return options;
        }}
        noOptionsMessage={() => {
          if (attempt.status === 'error') {
            return `Could not load options: ${attempt.statusText}`;
          }
          return noOptionsMessage();
        }}
        maxMenuHeight={maxMenuHeight}
        defaultOptions={true}
        placeholder={placeholder}
        isMulti={isMulti}
        autoFocus={autoFocus}
        isDisabled={isDisabled}
        elevated={elevated}
      />
    </Box>
  );
}

type FieldProps = {
  autoFocus?: boolean;
  label?: string;
  labelTip?: string;
  rule?: (options: unknown) => () => unknown;
  // styles
  [key: string]: any;
};
