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

import { Box, LabelInput } from 'design';

import { useRule } from 'shared/components/Validation';

import {
  SelectCreatable,
  CreatableProps as SelectCreatableProps,
} from '../Select';

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
  return (
    <Box mb="4" {...styles}>
      {label && (
        <LabelInput htmlFor={inputId} hasError={hasError}>
          {labelText}
          {labelTip && <LabelTip text={labelTip} />}
        </LabelInput>
      )}
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
