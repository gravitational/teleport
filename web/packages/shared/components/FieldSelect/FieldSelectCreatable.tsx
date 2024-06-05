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

import { Box, Flex, LabelInput } from 'design';

import { useAsync } from 'shared/hooks/useAsync';
import { ToolTipInfo } from 'shared/components/ToolTip';
import { useRule } from 'shared/components/Validation';

import {
  AsyncProps,
  SelectCreatable,
  CreatableProps as SelectCreatableProps,
} from '../Select';
import { SelectCreatableAsync } from '../Select/Select';

import { LabelTip, defaultRule } from './shared';

/**
 * Returns a styled SelectCreatable with label, input validation rule and error handling.
 * @param {() => void} onChange - change handler.
 * @param {defaultRule} rule - rules for the select component.
 * @param {boolean} markAsError - manually mark the component as error.
 * @param {string} placeholder - placeholder value.
 * @param {string} formatCreateLabel - custom formatting for create label.
 * @returns SelectCreatable
 */
export function FieldSelectCreatable({
  components,
  toolTipContent = null,
  label,
  labelTip,
  value,
  name,
  onChange,
  placeholder,
  maxMenuHeight,
  isClearable,
  isMulti,
  menuIsOpen,
  menuPosition,
  inputValue,
  onKeyDown,
  onInputChange,
  onBlur,
  options,
  formatCreateLabel,
  ariaLabel,
  rule = defaultRule,
  stylesConfig,
  isSearchable = false,
  isSimpleValue = false,
  autoFocus = false,
  isDisabled = false,
  elevated = false,
  inputId = 'select',
  markAsError = false,
  customProps,
  ...styles
}: CreatableProps) {
  const { valid, message } = useRule(rule(value));
  const hasError = Boolean(!valid);
  const labelText = hasError ? message : label;
  const $inputElement = (
    <SelectCreatable
      components={components}
      inputId={inputId}
      name={name}
      menuPosition={menuPosition}
      hasError={hasError || markAsError}
      isSimpleValue={isSimpleValue}
      isSearchable={isSearchable}
      isClearable={isClearable}
      value={value}
      onChange={onChange}
      onKeyDown={onKeyDown}
      onInputChange={onInputChange}
      onBlur={onBlur}
      inputValue={inputValue}
      maxMenuHeight={maxMenuHeight}
      placeholder={placeholder}
      isMulti={isMulti}
      autoFocus={autoFocus}
      isDisabled={isDisabled}
      elevated={elevated}
      menuIsOpen={menuIsOpen}
      stylesConfig={stylesConfig}
      options={options}
      formatCreateLabel={formatCreateLabel}
      aria-label={ariaLabel}
      customProps={customProps}
    />
  );

  return (
    <Box mb="4" {...styles}>
      {label ? (
        <>
          <LabelInput mb={0} htmlFor={inputId} hasError={hasError}>
            {toolTipContent ? (
              <Flex gap={1} alignItems="center">
                {labelText}
                {labelTip && <LabelTip text={labelTip} />}
                <ToolTipInfo children={toolTipContent} />
              </Flex>
            ) : (
              <>
                {labelText}
                {labelTip && <LabelTip text={labelTip} />}
              </>
            )}
          </LabelInput>
          {$inputElement}
        </>
      ) : (
        $inputElement
      )}
    </Box>
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
export function FieldSelectCreatableAsync({
  components,
  toolTipContent = null,
  label,
  labelTip,
  value,
  name,
  onChange,
  placeholder,
  maxMenuHeight,
  isClearable,
  isMulti,
  menuIsOpen,
  menuPosition,
  inputValue,
  onKeyDown,
  onInputChange,
  onBlur,
  options,
  formatCreateLabel,
  ariaLabel,
  rule = defaultRule,
  stylesConfig,
  isSearchable = false,
  isSimpleValue = false,
  autoFocus = false,
  isDisabled = false,
  elevated = false,
  inputId = 'select',
  markAsError = false,
  customProps,
  loadOptions,
  noOptionsMessage,
  defaultOptions,
  ...styles
}: AsyncProps & CreatableProps) {
  const [attempt, runAttempt] = useAsync(loadOptions);
  const { valid, message } = useRule(rule(value));
  const hasError = Boolean(!valid);
  const labelText = hasError ? message : label;
  const $inputElement = (
    <SelectCreatableAsync
      components={components}
      defaultOptions={defaultOptions}
      inputId={inputId}
      name={name}
      menuPosition={menuPosition}
      hasError={hasError || markAsError}
      isSimpleValue={isSimpleValue}
      isSearchable={isSearchable}
      isClearable={isClearable}
      value={value}
      onChange={onChange}
      onKeyDown={onKeyDown}
      onInputChange={onInputChange}
      onBlur={onBlur}
      inputValue={inputValue}
      maxMenuHeight={maxMenuHeight}
      placeholder={placeholder}
      isMulti={isMulti}
      autoFocus={autoFocus}
      isDisabled={isDisabled}
      elevated={elevated}
      menuIsOpen={menuIsOpen}
      stylesConfig={stylesConfig}
      options={options}
      formatCreateLabel={formatCreateLabel}
      aria-label={ariaLabel}
      customProps={customProps}
      loadOptions={async (input, option) => {
        const [options, error] = await runAttempt(input, option);
        if (error) {
          return [];
        }
        return options;
      }}
      noOptionsMessage={() => {
        if (attempt.status === 'error') {
          return `Could not load options: ${attempt.error}`;
        }
        return noOptionsMessage();
      }}
    />
  );

  return (
    <Box mb="4" {...styles}>
      {label ? (
        <>
          <LabelInput mb={0} htmlFor={inputId} hasError={hasError}>
            {toolTipContent ? (
              <Flex gap={1} alignItems="center">
                {labelText}
                {labelTip && <LabelTip text={labelTip} />}
                <ToolTipInfo children={toolTipContent} />
              </Flex>
            ) : (
              <>
                {labelText}
                {labelTip && <LabelTip text={labelTip} />}
              </>
            )}
          </LabelInput>
          {$inputElement}
        </>
      ) : (
        $inputElement
      )}
    </Box>
  );
}

type CreatableProps = SelectCreatableProps & {
  autoFocus?: boolean;
  label?: string;
  rule?: (options: unknown) => () => unknown;
  markAsError?: boolean;
  ariaLabel?: string;
  // styles
  [key: string]: any;
};
