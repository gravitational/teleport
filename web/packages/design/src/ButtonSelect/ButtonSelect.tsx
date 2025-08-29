/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import styled from 'styled-components';

import { Button } from 'design/Button';
import Flex, { FlexProps } from 'design/Flex';
import { HoverTooltip } from 'design/Tooltip';

type OptionValue = string | number | bigint;

type Option<T> = {
  value: T;
  label: string;
  disabled?: boolean;
  tooltip?: string;
};

type ButtonSelectProps<T extends readonly Option<OptionValue>[]> = {
  options: T;
  activeValue: T[number]['value'];
  onChange: (selectedValue: T[number]['value']) => void;
  fullWidth?: boolean;
  disabled?: boolean;
};

/**
 * ButtonSelect is a segmented button that allows users to select one of the provided options.
 *
 * Each option must have a unique `value` and a `label` that will be displayed on the button.
 *
 * @property options - The options to display in the button select. Each option should have a unique `value` and a `label` to display on the button.
 * @property activeValue - The value of the currently active option.
 * @property onChange - Callback function that is called when the active button changes. Receives the value of the newly selected button.
 *
 * @example
 * const options = [
 *   { value: '1', label: 'Option 1' },
 *   { value: '2', label: 'Option 2' },
 * ];
 * const [activeValue, setActiveValue] = useState('1');
 * return (
 *   <ButtonSelect
 *     options={options}
 *     activeValue={activeValue}
 *     onChange={setactiveValue}
 *   />
 * );
 */
export const ButtonSelect = <T extends readonly Option<OptionValue>[]>({
  options,
  activeValue,
  onChange,
  disabled = false,
  fullWidth = false,
}: ButtonSelectProps<T> & FlexProps) => {
  const updateValue = (newValue: T[number]['value']) => {
    if (activeValue !== newValue) {
      onChange(newValue);
    }
  };

  return (
    <Wrapper $fullWidth={fullWidth}>
      {options.map(option => {
        const isActive = activeValue === option.value;
        return (
          <HoverTooltip tipContent={option.tooltip} key={option.label}>
            <ButtonSelectButton
              aria-label={option.label}
              aria-checked={isActive}
              onClick={() =>
                !(option.disabled || disabled) && updateValue(option.value)
              }
              intent={isActive ? 'primary' : 'neutral'}
              disabled={option.disabled || disabled}
            >
              {option.label}
            </ButtonSelectButton>
          </HoverTooltip>
        );
      })}
    </Wrapper>
  );
};

const Wrapper = styled(Flex)<{ $fullWidth?: boolean }>`
  ${({ $fullWidth }) => !$fullWidth && 'width: min-content;'}
  & > * {
    min-width: fit-content;
  }
`;

const ButtonSelectButton = styled(Button)`
  flex: 1 1 0;
  border-radius: 0px;

  &:focus-visible {
    border-radius: 0px;
  }

  &:first-child,
  &:first-child:focus-visible {
    border-top-left-radius: 4px;
    border-bottom-left-radius: 4px;
  }

  &:last-child,
  &:last-child:focus-visible {
    border-top-right-radius: 4px;
    border-bottom-right-radius: 4px;
  }
`;
