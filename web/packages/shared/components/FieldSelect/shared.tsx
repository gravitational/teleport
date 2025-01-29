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

import React, { useId } from 'react';
import { GroupBase, OnChangeValue, OptionsOrGroups } from 'react-select';

import Box, { BoxProps } from 'design/Box';
import Flex from 'design/Flex';
import LabelInput from 'design/LabelInput';
import { IconTooltip } from 'design/Tooltip';

import { HelperTextLine } from '../FieldInput/FieldInput';
import {
  AsyncProps as AsyncSelectProps,
  Props as SelectProps,
} from '../Select';
import { useRule } from '../Validation';

export const defaultRule = () => () => ({ valid: true });

export const LabelTip = ({ text }) => (
  <span
    css={{ fontWeight: 'normal', textTransform: 'none' }}
  >{` - ${text}`}</span>
);

type FieldSelectWrapperPropsBase<Opt, IsMulti extends boolean> = {
  label?: string;
  toolTipContent?: React.ReactNode;
  helperText?: React.ReactNode;
  value?: OnChangeValue<Opt, IsMulti>;
  rule?: (options: OnChangeValue<Opt, IsMulti>) => () => unknown;
  inputId?: string;
  markAsError?: boolean;
};

type FieldSelectWrapperProps<
  Opt,
  IsMulti extends boolean,
> = FieldSelectWrapperPropsBase<Opt, IsMulti> & {
  children: React.ReactElement<
    // Note: I have no idea why `aria-invalid` is mentioned in the types, but
    // `aria-describedby` is not. As this attribute actually gets applied, I
    // suppose it's a type system bug.
    SelectProps<Opt, IsMulti> & { 'aria-describedby'?: string }
  >;
} & BoxProps;

/**
 * This component contains common validation and ID wrangling logic for all the
 * select fields.
 */
export const FieldSelectWrapper = <Opt, IsMulti extends boolean>({
  label,
  toolTipContent,
  helperText,
  value,
  rule,
  inputId,
  markAsError,
  children,
  ...styles
}: FieldSelectWrapperProps<Opt, IsMulti>) => {
  const { valid, message } = useRule((rule ?? defaultRule)(value));
  // We can't generate a random ID only when it's needed; this would break the
  // expectation that hooks need to be called exactly the same way on every
  // rendering pass.
  const randomFieldId = useId();
  const helperId = useId();

  const id = inputId || randomFieldId;
  const hasError = !valid;

  return (
    <Box mb="3" {...styles}>
      {label && (
        <LabelInput htmlFor={id}>
          {toolTipContent ? (
            <Flex gap={1} alignItems="center">
              {label}
              <IconTooltip children={toolTipContent} />
            </Flex>
          ) : (
            label
          )}
        </LabelInput>
      )}
      {React.cloneElement(children, {
        inputId: id,
        hasError,
        'aria-invalid': markAsError || hasError,
        'aria-describedby': helperId,
      })}
      <HelperTextLine
        hasError={markAsError || hasError}
        helperTextId={helperId}
        helperText={helperText}
        errorMessage={message}
      />
    </Box>
  );
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

export type FieldProps<Opt, IsMulti extends boolean> = BoxProps & {
  autoFocus?: boolean;
  label?: string;
  toolTipContent?: React.ReactNode;
  helperText?: React.ReactNode;
  rule?: (options: OnChangeValue<Opt, IsMulti>) => () => unknown;
  markAsError?: boolean;
  ariaLabel?: string;
};

/**
 * Select fields have a metric ton of props, all of which need to be properly
 * forwarded to the underlying components. The existing interface is a flat list
 * of props, which makes this task quite tricky. Therefore, we offload it to this
 * separate function for consistency.
 */
export function splitSelectProps<
  Opt,
  IsMulti extends boolean,
  Group extends GroupBase<Opt>,
  Props extends SelectProps<Opt, IsMulti, Group> & FieldProps<Opt, IsMulti>,
>(
  props: Props,
  defaults: Partial<Props>
): {
  /** Props that should go to the underlying select component. */
  base: SelectProps<Opt, IsMulti, Group>;
  /** Props that should go to `FieldSelectWrapper`. */
  wrapper: FieldSelectWrapperPropsBase<Opt, IsMulti>;
  /** Rest of the props. It's up to the caller to decide what to do with these. */
  others: Omit<Props, KeysRemovedFromOthers>;
} {
  const propsWithDefaults = { ...defaults, ...props };
  const {
    ariaLabel,
    autoFocus,
    components,
    customProps,
    defaultValue,
    elevated,
    helperText,
    inputId,
    inputValue,
    isClearable,
    isDisabled,
    isMulti,
    isSearchable,
    label,
    markAsError,
    maxMenuHeight,
    menuIsOpen,
    onMenuOpen,
    onMenuClose,
    closeMenuOnSelect,
    hideSelectedOptions,
    menuPosition,
    name,
    noOptionsMessage,
    onBlur,
    onChange,
    onInputChange,
    onKeyDown,
    openMenuOnClick,
    options,
    placeholder,
    rule,
    stylesConfig,
    toolTipContent,
    value,
    ...others
  } = propsWithDefaults;
  return {
    // hasError and inputId are deliberately excluded from the base, since they
    // are set by the wrapper component.
    base: {
      'aria-label': ariaLabel,
      autoFocus,
      components,
      customProps,
      defaultValue,
      elevated,
      inputValue,
      isClearable,
      isDisabled,
      isMulti,
      isSearchable,
      maxMenuHeight,
      menuPosition,
      menuIsOpen,
      onMenuOpen,
      onMenuClose,
      closeMenuOnSelect,
      hideSelectedOptions,
      name,
      noOptionsMessage,
      onBlur,
      onChange,
      onInputChange,
      onKeyDown,
      options,
      openMenuOnClick,
      placeholder,
      stylesConfig,
      value,
    },
    wrapper: {
      helperText,
      inputId,
      label,
      markAsError,
      rule,
      toolTipContent,
      value,
    },
    others,
  };
}

type KeysRemovedFromOthers =
  | 'ariaLabel'
  | 'autoFocus'
  | 'components'
  | 'customProps'
  | 'defaultValue'
  | 'elevated'
  | 'helperText'
  | 'inputId'
  | 'inputValue'
  | 'isClearable'
  | 'isDisabled'
  | 'isMulti'
  | 'isSearchable'
  | 'label'
  | 'markAsError'
  | 'maxMenuHeight'
  | 'menuIsOpen'
  | 'menuPosition'
  | 'onMenuOpen'
  | 'onMenuClose'
  | 'closeMenuOnSelect'
  | 'hideSelectedOptions'
  | 'name'
  | 'noOptionsMessage'
  | 'onBlur'
  | 'onChange'
  | 'onInputChange'
  | 'onKeyDown'
  | 'openMenuOnClick'
  | 'options'
  | 'placeholder'
  | 'rule'
  | 'stylesConfig'
  | 'toolTipContent'
  | 'value';
