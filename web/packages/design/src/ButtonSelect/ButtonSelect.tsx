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

interface ButtonSelectProps {
  // The options to display in the button select.
  // Each option should have a unique `key` and a `label` to display on the button.
  options: { key: string; label: string }[];
  // The key of the active option
  activeOption: string;
  // Callback function that is called when the active button changes. It receives the key of the newly selected button.
  onChange: (selectedKey: string) => void;
}

/**
 * ButtonSelect is a segmented button that allows users to select one of the provided options.
 *
 * Each option must have a unique `key` and a `label` that will be displayed on the button.
 *
 * @example
 * const options = [
 *   { key: '1', label: 'Option 1' },
 *   { key: '2', label: 'Option 2' },
 * ];
 * const [activeOption, setActiveOption] = useState('1');
 * return (
 *   <ButtonSelect
 *     options={options}
 *     activeOption={activeOption}
 *     onChange={setActiveOption}
 *   />
 * );
 */
export const ButtonSelect = ({
  options,
  activeOption,
  onChange,
}: ButtonSelectProps) => {
  const updateValue = (newOption: string) => {
    if (activeOption !== newOption) {
      onChange(newOption);
    }
  };

  return (
    <Wrapper>
      {options.map(option => {
        const isActive = activeOption === option.key;
        return (
          <ButtonSelectButton
            key={option.key}
            onClick={() => updateValue(option.key)}
            data-active={isActive}
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
