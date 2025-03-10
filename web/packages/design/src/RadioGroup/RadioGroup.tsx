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

import { ReactNode } from 'react';

import { Flex } from 'design';
import { FieldRadio } from 'design/FieldRadio';
import { FlexProps } from 'design/Flex';
import { RadioButton, RadioButtonSize } from 'design/RadioButton';

interface RadioObjectOption {
  value: string;
  label: ReactNode;
  disabled?: boolean;
  helperText?: ReactNode;
}

type RadioOption = RadioObjectOption | string;

interface RadioGroupProps extends FlexProps {
  options: RadioOption[];
  onChange?: (value: string) => void;
  value?: string;
  /** Sets focus on the first radio input element */
  autoFocus?: boolean;
  /** The name property of radio input elements */
  name: string;
  size?: RadioButtonSize;
}

export function RadioGroup({
  options,
  value,
  onChange,
  autoFocus,
  name,
  size,
  ...styles
}: RadioGroupProps) {
  return (
    <Flex gap={3} flexDirection="column" {...styles}>
      {options.map((option, index) => {
        const optionValue = isRadioObjectOption(option) ? option.value : option;
        const optionLabel = isRadioObjectOption(option) ? option.label : option;
        const optionDisabled = isRadioObjectOption(option)
          ? option.disabled
          : undefined;
        const optionHelperText = isRadioObjectOption(option)
          ? option.helperText
          : undefined;
        return (
          <FieldRadio
            key={optionValue}
            name={name}
            label={optionLabel}
            helperText={optionHelperText}
            checked={value !== undefined ? value === optionValue : undefined}
            disabled={optionDisabled}
            size={size}
            value={optionValue}
            autoFocus={index === 0 && autoFocus}
            onChange={() => onChange?.(optionValue)}
            mb={0}
          />
        );
      })}
    </Flex>
  );
}

interface RadioProps {
  option: RadioOption;
  name: string;
  checked: boolean;
  autoFocus?: boolean;
  onChange?: (value: string) => void;
}

export function Radio(props: RadioProps) {
  const optionValue = isRadioObjectOption(props.option)
    ? props.option.value
    : props.option;
  const optionLabel = isRadioObjectOption(props.option)
    ? props.option.label
    : props.option;
  const optionDisabled = isRadioObjectOption(props.option)
    ? props.option.disabled
    : undefined;

  return (
    <label
      css={`
        display: flex;
        align-items: center;
        cursor: ${optionDisabled ? 'not-allowed' : 'pointer'};
      `}
    >
      <RadioButton
        autoFocus={props.autoFocus}
        name={props.name}
        checked={props.checked}
        onChange={() => props.onChange?.(optionValue)}
        value={optionValue}
        disabled={optionDisabled}
      />
      <span css={{ opacity: optionDisabled ? 0.5 : 1 }}>{optionLabel}</span>
    </label>
  );
}

function isRadioObjectOption(option: RadioOption): option is RadioObjectOption {
  return typeof option === 'object';
}
