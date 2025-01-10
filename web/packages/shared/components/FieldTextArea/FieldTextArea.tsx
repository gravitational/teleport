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

import React, {
  forwardRef,
  HTMLInputAutoCompleteAttribute,
  useId,
} from 'react';

import { Box, LabelInput, TextArea } from 'design';
import { BoxProps } from 'design/Box';
import { TextAreaSize } from 'design/TextArea';
import { IconTooltip } from 'design/Tooltip';
import { useRule } from 'shared/components/Validation';

import { HelperTextLine } from '../FieldInput/FieldInput';

export type FieldTextAreaProps = BoxProps & {
  id?: string;
  name?: string;
  value?: string;
  label?: string;
  helperText?: React.ReactNode;
  size?: TextAreaSize;
  placeholder?: string;
  autoFocus?: boolean;
  autoComplete?: HTMLInputAutoCompleteAttribute;
  spellCheck?: boolean;
  rule?: (options: unknown) => () => unknown;
  onChange?: React.ChangeEventHandler<HTMLInputElement>;
  onKeyPress?: React.KeyboardEventHandler<HTMLInputElement>;
  onKeyDown?: React.KeyboardEventHandler<HTMLInputElement>;
  onFocus?: React.FocusEventHandler<HTMLInputElement>;
  onBlur?: React.FocusEventHandler<HTMLInputElement>;
  readonly?: boolean;
  defaultValue?: string;
  toolTipContent?: React.ReactNode;
  disabled?: boolean;
  // markAsError is a flag to highlight an
  // input box as error color before validator
  // runs (which marks it as error)
  markAsError?: boolean;
  textAreaCss?: string;
  resizable?: boolean;
};

export const FieldTextArea = forwardRef<
  HTMLTextAreaElement,
  FieldTextAreaProps
>(
  (
    {
      id,
      label,
      helperText,
      size,
      value,
      onChange,
      onKeyPress,
      onKeyDown,
      onFocus,
      onBlur,
      placeholder,
      defaultValue,
      rule = defaultRule,
      name,
      autoFocus = false,
      autoComplete = 'off',
      spellCheck,
      readonly = false,
      toolTipContent = null,
      disabled = false,
      markAsError = false,
      resizable = true,
      textAreaCss,
      ...styles
    },
    ref
  ) => {
    const { valid, message } = useRule(rule(value));
    const helperTextId = useId();

    const hasError = !valid;
    const $textAreaElement = (
      <TextArea
        ref={ref}
        id={id}
        name={name}
        hasError={hasError || markAsError}
        placeholder={placeholder}
        autoFocus={autoFocus}
        value={value}
        autoComplete={autoComplete}
        onChange={onChange}
        onKeyPress={onKeyPress}
        onKeyDown={onKeyDown}
        onFocus={onFocus}
        onBlur={onBlur}
        readOnly={readonly}
        spellCheck={spellCheck}
        defaultValue={defaultValue}
        disabled={disabled}
        size={size}
        aria-invalid={hasError || markAsError}
        aria-describedby={helperTextId}
        css={textAreaCss}
        resizable={resizable}
      />
    );

    return (
      <Box mb="3" {...styles}>
        {label ? (
          <LabelInput mb={0}>
            <Box mb={1}>
              {toolTipContent ? (
                <>
                  <span
                    css={{
                      marginRight: '4px',
                      verticalAlign: 'middle',
                    }}
                  >
                    {label}
                  </span>
                  <IconTooltip children={toolTipContent} />
                </>
              ) : (
                <>{label}</>
              )}
            </Box>
            {$textAreaElement}
          </LabelInput>
        ) : (
          $textAreaElement
        )}
        <HelperTextLine
          hasError={hasError}
          helperTextId={helperTextId}
          helperText={helperText}
          errorMessage={message}
        />
      </Box>
    );
  }
);

const defaultRule = () => () => ({ valid: true });
