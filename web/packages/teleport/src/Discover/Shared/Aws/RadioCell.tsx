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

import { Flex } from 'design';

import { DisableableCell } from './DisableableCell';

export function RadioCell<T>({
  item,
  value,
  isChecked,
  onChange,
  disabled,
  disabledText,
}: {
  item: T;
  value: string;
  isChecked: boolean;
  onChange(selectedItem: T): void;
  disabled: boolean;
  disabledText: string;
}) {
  return (
    <DisableableCell
      width="20px"
      disabled={disabled}
      disabledText={disabledText}
    >
      <Flex alignItems="center" my={2} justifyContent="center">
        <input
          // eslint-disable-next-line react/no-unknown-property
          css={`
            margin: 0 ${props => props.theme.space[2]}px 0 0;
            accent-color: ${props => props.theme.colors.brand.accent};
            cursor: pointer;

            &:disabled {
              cursor: not-allowed;
            }
          `}
          type="radio"
          name={value}
          checked={isChecked}
          onChange={() => onChange(item)}
          value={value}
          disabled={disabled}
        />
      </Flex>
    </DisableableCell>
  );
}
