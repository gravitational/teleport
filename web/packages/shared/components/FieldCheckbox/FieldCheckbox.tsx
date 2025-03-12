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

import React, { forwardRef } from 'react';
import styled from 'styled-components';

import Box from 'design/Box';
import { CheckboxInput, CheckboxSize } from 'design/Checkbox';
import Flex from 'design/Flex';
import LabelInput from 'design/LabelInput';
import { SpaceProps } from 'design/system';
import Text from 'design/Text';

interface FieldCheckboxProps extends SpaceProps {
  name?: string;
  label?: React.ReactNode;
  helperText?: React.ReactNode;
  checked?: boolean;
  defaultChecked?: boolean;
  disabled?: boolean;
  size?: CheckboxSize;
  onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
}

/** Renders a checkbox with an associated label and an optional helper text. */
export const FieldCheckbox = forwardRef<HTMLInputElement, FieldCheckboxProps>(
  (
    {
      name,
      label,
      helperText,
      checked,
      defaultChecked,
      disabled,
      size,
      onChange,
      ...styles
    },
    ref
  ) => {
    const labelColor = disabled ? 'text.disabled' : 'text.main';
    const helperColor = disabled ? 'text.disabled' : 'text.slightlyMuted';
    const labelTypography = size === 'small' ? 'body2' : 'body1';
    const helperTypography = size === 'small' ? 'body3' : 'body2';
    return (
      <Box mb={3} lineHeight={0} {...styles}>
        <StyledLabel disabled={disabled}>
          <Flex flexDirection="row" gap={2}>
            {/* Nudge the small-size checkbox to better align with the
                label. */}
            <Box mt={size === 'small' ? '2px' : '0px'}>
              <CheckboxInput
                size={size}
                ref={ref}
                checked={checked}
                defaultChecked={defaultChecked}
                disabled={disabled}
                name={name}
                onChange={onChange}
              />
            </Box>
            <Box>
              <Text typography={labelTypography} color={labelColor}>
                {label}
              </Text>
              <Text typography={helperTypography} color={helperColor}>
                {helperText}
              </Text>
            </Box>
          </Flex>
        </StyledLabel>
      </Box>
    );
  }
);

const StyledLabel = styled(LabelInput)<{ disabled?: boolean }>`
  // Typically, a short label in a wide container means a lot of whitespace that
  // acts as a click target for a checkbox. To avoid this, we use inline-flex to
  // wrap the label around its content.
  display: inline-flex;
  width: auto;
  gap: ${props => props.theme.space[2]}px;
  margin-bottom: 0;
  line-height: 0;
  cursor: ${props => (props.disabled ? 'not-allowed' : 'pointer')};
`;
