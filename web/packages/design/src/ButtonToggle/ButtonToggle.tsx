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
import { useState } from 'react';

import { ButtonPrimary } from '../Button/Button';
import Flex from '../Flex';

interface ButtonToggleProps {
  leftLabel: string;
  rightLabel: string;
  initialValue: boolean;
  rightIsTrue?: boolean;
  onChange: (value: boolean) => void;
}

/**
 * A toggle component for controlling a boolean value in the parent component. It displays two buttons side by side, allowing the user to switch between two states.
 *
 * The `onChange` prop is called with the new value whenever the toggle changes, so you can update state or perform side effects in the parent component.
 *
 * By default, the left button represents `false` and the right button represents `true`. You can use the optional `rightIsTrue` prop to swap this behavior, making the right button represent `true`.
 *
 * @example
 * const [toggleValue, setToggleValue] = useState<boolean>(false);
 * <ButtonToggle
 *   leftLabel="OFF"
 *   rightLabel="ON"
 *   rightIsTrue // right button represents `true`
 *   initialValue={toggleValue}
 *   onChange={(value: boolean) => {
 *     setToggleValue(value);
 *   }}
 * />
 */

export const ButtonToggle = ({
  leftLabel,
  rightLabel,
  initialValue,
  rightIsTrue = false,
  onChange,
}: ButtonToggleProps) => {
  const [value, setValue] = useState<boolean>(initialValue);

  // returns if the given button is currently selected based on the current value
  const isActive = (position: 'left' | 'right') =>
    value === getValueForPosition(position);

  // returns the boolean value associated with the given position
  const getValueForPosition = (position: 'left' | 'right') =>
    position === (rightIsTrue ? 'right' : 'left');

  const handleClick = (position: 'left' | 'right') => {
    const newValue = getValueForPosition(position);
    if (value !== newValue) {
      setValue(newValue);
      onChange(newValue); // handle the change in the parent component
    }
  };

  return (
    <Flex>
      <ButtonPrimary
        style={{
          borderRight: 'none',
          borderTopRightRadius: 0,
          borderBottomRightRadius: 0,
        }}
        intent={isActive('left') ? 'primary' : 'neutral'}
        onClick={() => handleClick('left')}
      >
        {leftLabel}
      </ButtonPrimary>
      <ButtonPrimary
        style={{
          borderLeft: 'none',
          borderTopLeftRadius: 0,
          borderBottomLeftRadius: 0,
        }}
        intent={isActive('right') ? 'primary' : 'neutral'}
        onClick={() => handleClick('right')}
      >
        {rightLabel}
      </ButtonPrimary>
    </Flex>
  );
};
