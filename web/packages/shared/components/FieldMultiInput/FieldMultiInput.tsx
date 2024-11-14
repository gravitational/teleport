/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import Box from 'design/Box';
import { ButtonSecondary } from 'design/Button';
import ButtonIcon from 'design/ButtonIcon';
import Flex from 'design/Flex';
import * as Icon from 'design/Icon';
import Input from 'design/Input';
import { useRef } from 'react';
import styled, { useTheme } from 'styled-components';

export type FieldMultiInputProps = {
  label?: string;
  value: string[];
  disabled?: boolean;
  onChange?(val: string[]): void;
};

/**
 * Allows editing a list of strings, one value per row. Use instead of
 * `FieldSelectCreatable` when:
 *
 * - There are no predefined values to be picked from.
 * - Values are expected to be relatively long and would be unreadable after
 *   being truncated.
 */
export function FieldMultiInput({
  label,
  value,
  disabled,
  onChange,
}: FieldMultiInputProps) {
  if (value.length === 0) {
    value = [''];
  }

  const theme = useTheme();
  // Index of the input to be focused after the next rendering.
  const toFocus = useRef<number | undefined>();

  const setFocus = element => {
    element?.focus();
    toFocus.current = undefined;
  };

  function insertItem(index: number) {
    onChange?.(value.toSpliced(index, 0, ''));
  }

  function removeItem(index: number) {
    onChange?.(value.toSpliced(index, 1));
  }

  function handleKeyDown(index: number, e: React.KeyboardEvent) {
    if (e.key === 'Enter') {
      insertItem(index + 1);
      toFocus.current = index + 1;
    }
  }

  return (
    <Box>
      <Fieldset>
        {label && <Legend>{label}</Legend>}
        {value.map((val, i) => (
          // Note on keys: using index as a key is an anti-pattern in general,
          // but here, we can safely assume that even though the list is
          // editable, we don't rely on any unmanaged HTML element state other
          // than focus, which we deal with separately anyway. The alternatives
          // would be either to require an array with keys generated
          // synthetically and injected from outside (which would make the API
          // difficult to use) or to keep the array with generated IDs as local
          // state (which would require us to write a prop/state reconciliation
          // procedure whose complexity would probably outweigh the benefits).
          <Flex key={i} alignItems="center" gap={2}>
            <Box flex="1">
              <Input
                value={val}
                ref={toFocus.current === i ? setFocus : null}
                onChange={e =>
                  onChange?.(
                    value.map((v, j) => (j === i ? e.target.value : v))
                  )
                }
                onKeyDown={e => handleKeyDown(i, e)}
              />
            </Box>
            <ButtonIcon
              size="0"
              title="Remove Item"
              onClick={() => removeItem(i)}
              disabled={disabled}
            >
              <Icon.Cross size="small" color={theme.colors.text.muted} />
            </ButtonIcon>
          </Flex>
        ))}
        <ButtonSecondary
          alignSelf="start"
          onClick={() => insertItem(value.length)}
        >
          <Icon.Plus size="small" mr={2} />
          Add More
        </ButtonSecondary>
      </Fieldset>
    </Box>
  );
}

const Fieldset = styled.fieldset`
  border: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: ${props => props.theme.space[2]}px;
`;

const Legend = styled.legend`
  margin: 0 0 ${props => props.theme.space[1]}px 0;
  padding: 0;
  ${props => props.theme.typography.body3}
`;
