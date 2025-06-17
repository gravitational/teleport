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

import Flex from 'design/Flex';

import { Button } from '../Button/Button';

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
export const ButtonSelect = ({
  options,
  activeValue,
  onChange,
}: {
  options: { value: string; label: string }[];
  activeValue: string;
  onChange: (selectedvalue: string) => void;
}) => {
  const updateValue = (newValue: string) => {
    if (activeValue !== newValue) {
      onChange(newValue);
    }
  };

  return (
    <Wrapper>
      {options.map(option => {
        const isActive = activeValue === option.value;
        return (
          <ButtonSelectButton
            key={option.value}
            aria-label={option.label}
            aria-checked={isActive}
            onClick={() => updateValue(option.value)}
            intent={isActive ? 'primary' : 'neutral'}
          >
            {option.label}
          </ButtonSelectButton>
        );
      })}
    </Wrapper>
  );
};

const Wrapper = styled(Flex)`
  & > * {
    min-width: fit-content;
  }
`;

const ButtonSelectButton = styled(Button)`
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
