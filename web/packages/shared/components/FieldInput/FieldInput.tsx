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
import styled, { useTheme } from 'styled-components';

import { Box, Input, LabelInput, Text } from 'design';
import { BoxProps } from 'design/Box';
import { IconProps } from 'design/Icon/Icon';
import { InputMode, InputSize, InputType } from 'design/Input';
import { IconTooltip } from 'design/Tooltip';
import { useRule } from 'shared/components/Validation';

const FieldInput = forwardRef<HTMLInputElement, FieldInputProps>(
  (
    {
      id,
      label,
      helperText,
      icon,
      size,
      value,
      onChange,
      onKeyPress,
      onKeyDown,
      onFocus,
      onBlur,
      placeholder,
      defaultValue,
      min,
      max,
      rule = defaultRule,
      name,
      type = 'text',
      autoFocus = false,
      autoComplete = 'off',
      inputMode = 'text',
      spellCheck,
      readonly = false,
      toolTipContent = null,
      tooltipSticky = false,
      disabled = false,
      markAsError = false,
      required = false,
      ...styles
    },
    ref
  ) => {
    const { valid, message } = useRule(rule(value));
    const helperTextId = useId();

    const hasError = !valid;
    const $inputElement = (
      <Input
        ref={ref}
        id={id}
        type={type}
        name={name}
        hasError={hasError || markAsError}
        placeholder={placeholder}
        autoFocus={autoFocus}
        value={value}
        min={min}
        max={max}
        autoComplete={autoComplete}
        onChange={onChange}
        onKeyPress={onKeyPress}
        onKeyDown={onKeyDown}
        onFocus={onFocus}
        onBlur={onBlur}
        readOnly={readonly}
        inputMode={inputMode}
        spellCheck={spellCheck}
        defaultValue={defaultValue}
        disabled={disabled}
        icon={icon}
        size={size}
        aria-invalid={hasError || markAsError}
        aria-describedby={helperTextId}
        required={required}
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
                  <IconTooltip
                    sticky={tooltipSticky}
                    children={toolTipContent}
                  />
                </>
              ) : (
                <>{label}</>
              )}
            </Box>
            {$inputElement}
          </LabelInput>
        ) : (
          $inputElement
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

/**
 * Renders a line that, depending on situation, shows either a helper text or
 * an error message. Since we want the text line to appear dynamically in
 * response to input validation, we introduce height animation here and
 * constrain the amount of text to a single line. This limitation can be lifted
 * after the `calc-size` CSS function gets widely adopted.
 */
export const HelperTextLine = ({
  hasError,
  helperTextId,
  helperText,
  errorMessage,
}: {
  hasError: boolean;
  /**
   * ID of the helper text element, used to connect it to the input control for
   * better accessibility.
   */
  helperTextId: string;
  helperText?: React.ReactNode;
  errorMessage?: string;
}) => {
  const theme = useTheme();
  return (
    <HelperTextContainer
      expanded={(hasError && !!errorMessage) || !!helperText}
      id={helperTextId}
    >
      {!hasError && (
        <HelperText
          color={theme.colors.text.slightlyMuted}
          style={{ display: hasError ? 'none' : undefined }}
          typography="body3"
          pt={1}
        >
          {helperText}
        </HelperText>
      )}
      {/* For the live region to work and announce validation errors in screen
          readers, it needs to be rendered prior to the validation error itself.
          The screen reader will only announce changes to the live region
          content, and not its initial value. We therefore use separate regions
          for showing helper text and error messages. */}
      <div aria-live="polite">
        {hasError && (
          <HelperText
            color={theme.colors.interactive.solid.danger.default}
            style={{ display: hasError ? undefined : 'none' }}
            typography="body3"
            pt={1}
          >
            {errorMessage}
          </HelperText>
        )}
      </div>
    </HelperTextContainer>
  );
};

const HelperTextContainer = styled.div<{
  expanded?: boolean;
}>`
  // Constrain the height to be able to animate it.
  height: ${props =>
    props.expanded
      ? `calc(${props.theme.space[1]}px + ${props.theme.typography.body3.lineHeight})`
      : '0'};
  opacity: ${props => (props.expanded ? 1 : 0)};
  transition:
    height 200ms ease-in,
    opacity 200ms ease-in;
`;

const HelperText = styled(Text)`
  white-space: nowrap;
`;

export default FieldInput;

export type FieldInputProps = BoxProps & {
  id?: string;
  name?: string;
  value?: string;
  label?: React.ReactNode;
  helperText?: React.ReactNode;
  icon?: React.ComponentType<IconProps>;
  size?: InputSize;
  placeholder?: string;
  autoFocus?: boolean;
  autoComplete?: HTMLInputAutoCompleteAttribute;
  type?: InputType;
  inputMode?: InputMode;
  spellCheck?: boolean;
  rule?: (options: unknown) => () => unknown;
  onChange?: React.ChangeEventHandler<HTMLInputElement>;
  onKeyPress?: React.KeyboardEventHandler<HTMLInputElement>;
  onKeyDown?: React.KeyboardEventHandler<HTMLInputElement>;
  onFocus?: React.FocusEventHandler<HTMLInputElement>;
  onBlur?: React.FocusEventHandler<HTMLInputElement>;
  readonly?: boolean;
  defaultValue?: string;
  min?: number;
  max?: number;
  toolTipContent?: React.ReactNode;
  tooltipSticky?: boolean;
  disabled?: boolean;
  // markAsError is a flag to highlight an
  // input box as error color before validator
  // runs (which marks it as error)
  markAsError?: boolean;
  required?: boolean;
};
