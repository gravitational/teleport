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

import { ReactNode, useRef } from 'react';
import styled, { useTheme } from 'styled-components';

import Box from 'design/Box';
import { ButtonSecondary } from 'design/Button';
import ButtonIcon from 'design/ButtonIcon';
import Flex from 'design/Flex';
import * as Icon from 'design/Icon';
import { LabelContent } from 'design/LabelInput/LabelInput';
import { useRule } from 'shared/components/Validation';
import {
  precomputed,
  Rule,
  ValidationResult,
} from 'shared/components/Validation/rules';

import FieldInput from '../FieldInput';

type StringListValidationResult = ValidationResult & {
  /**
   * A list of validation results, one per list item. Note: results are
   * optional just because `useRule` by default returns only
   * `ValidationResult`. For the actual validation, it's not optional; if it's
   * undefined, or there are fewer results in this list than the list items,
   * the corresponding items will be treated as valid.
   */
  results?: ValidationResult[];
};

export type FieldMultiInputProps = {
  label?: string;
  value: string[];
  disabled?: boolean;
  /** Adds a required field indicator to the label. */
  required?: boolean;
  tooltipContent?: ReactNode;
  tooltipSticky?: boolean;
  onChange?(val: string[]): void;
  rule?: Rule<string[], StringListValidationResult>;
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
  required,
  tooltipContent,
  tooltipSticky,
  onChange,
  rule = defaultRule,
}: FieldMultiInputProps) {
  // It's important to first validate, and then treat an empty array as a
  // single-item list with an empty string, since this "synthetic" empty
  // string is technically not a part of the model and should not be
  // validated.
  const validationResult: StringListValidationResult = useRule(rule(value));
  if (value.length === 0) {
    value = [''];
  }

  const theme = useTheme();
  // Index of the input to be focused after the next rendering.
  const toFocus = useRef<number | undefined>();

  const setFocus = (element: HTMLInputElement) => {
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
        {label && (
          <Legend>
            <LabelContent
              required={required}
              tooltipContent={tooltipContent}
              tooltipSticky={tooltipSticky}
            >
              {label}
            </LabelContent>
          </Legend>
        )}
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
              <FieldInput
                value={val}
                rule={precomputed(
                  validationResult.results?.[i] ?? { valid: true }
                )}
                ref={toFocus.current === i ? setFocus : null}
                onChange={e =>
                  onChange?.(
                    value.map((v, j) => (j === i ? e.target.value : v))
                  )
                }
                onKeyDown={e => handleKeyDown(i, e)}
                mb={0}
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

const defaultRule = () => () => ({ valid: true });

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
