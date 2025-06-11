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

import { ButtonPrimary } from '../Button/Button';

interface ButtonSelectProps {
  // The options to display in the button select. Each option should have a unique `key` and a `label` to display on the button.
  options: { key: string; label: string }[];
  // The index of the currently active button
  activeIndex: number;
  // Callback function that is called when the active button changes. It receives the index of the newly selected button.
  onChange: (selectedIndex: number) => void;
}

/**
 * ButtonSelect is a segmented button that allows users to select one of the provided options.
 *
 * Each option must have a unique `key` and a `label` that will be displayed on the button.
 *
 * @example
 * const options = [
 *   { key: 'customer', label: 'All Clusters (4)' },
 *   { key: 'cluster', label: 'Current Cluster' },
 * ];
 *
 * const [activeIndex, setActiveIndex] = useState(0);
 * return (
 *   <ButtonSelect
 *     options={options}
 *     activeIndex={activeIndex}
 *     onChange={(selectedIndex) => setActiveIndex(selectedIndex)}
 *   />
 * );
 */

export const ButtonSelect = ({
  options,
  activeIndex = 0,
  onChange,
}: ButtonSelectProps) => {
  const updateValue = (newValue: number) => {
    if (activeIndex !== newValue) {
      onChange(newValue);
    }
  };

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'ArrowRight' && activeIndex < options.length - 1) {
      onChange(activeIndex + 1);
    }
    if (e.key === 'ArrowLeft' && activeIndex > 0) {
      onChange(activeIndex - 1);
    }
  }

  return (
    <ButtonSelectWrapper
      tabIndex={0}
      onKeyDown={handleKeyDown}
      data-testid="button-select"
    >
      {options.map((option, index) => {
        const isActive = activeIndex === index;
        return (
          <ButtonSelectButton
            key={option.key}
            onClick={() => updateValue(index)}
            data-testid={`button-toggle-option-${index}`}
            tabIndex={-1}
            isActive={isActive}
            data-active={isActive}
            intent={isActive ? 'primary' : 'neutral'}
          >
            {option.label}
          </ButtonSelectButton>
        );
      })}
    </ButtonSelectWrapper>
  );
};

const ButtonSelectButton = styled(ButtonPrimary)<{ isActive: boolean }>`
  // middle button(s) style
  border-radius: 0px;

  // left button style
  &:first-child {
    border-top-left-radius: 4px;
    border-bottom-left-radius: 4px;
  }

  // right button style
  &:last-child {
    border-top-right-radius: 4px;
    border-bottom-right-radius: 4px;
  }
`;

const ButtonSelectWrapper = styled.div`
  display: flex;
  width: fit-content;
  border-radius: 4px;

  &:focus-visible {
    outline: 2px solid
      ${({ theme }) => theme.colors.interactive.solid.primary.default};
    outline-offset: 2px;
  }
`;
