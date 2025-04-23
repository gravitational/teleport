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

import { ChangeEvent, forwardRef, ReactNode } from 'react';
import styled from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';
import LabelInput from 'design/LabelInput';
import { RadioButton, RadioButtonSize } from 'design/RadioButton';
import { SpaceProps } from 'design/system';
import Text from 'design/Text';

interface FieldRadioProps extends SpaceProps {
  name?: string;
  label?: ReactNode;
  helperText?: ReactNode;
  checked?: boolean;
  defaultChecked?: boolean;
  disabled?: boolean;
  size?: RadioButtonSize;
  value?: string;
  autoFocus?: boolean;
  onChange?: (e: ChangeEvent<HTMLInputElement>) => void;
}

/** Renders a radio button with an associated label and an optional helper text. */
export const FieldRadio = forwardRef<HTMLInputElement, FieldRadioProps>(
  (
    {
      name,
      label,
      helperText,
      checked,
      defaultChecked,
      disabled,
      size,
      value,
      autoFocus,
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
            {/* Nudge the small-size radio button to better align with the
                label. */}
            <Box mt={size === 'small' ? '2px' : '0px'}>
              <RadioButton
                size={size}
                ref={ref}
                checked={checked}
                defaultChecked={defaultChecked}
                disabled={disabled}
                name={name}
                value={value}
                autoFocus={autoFocus}
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
  // acts as a click target for a radio button. To avoid this, we use
  // inline-flex to wrap the label around its content.
  display: inline-flex;
  width: auto;
  gap: ${props => props.theme.space[2]}px;
  margin-bottom: 0;
  line-height: 0;
  cursor: ${props => (props.disabled ? 'not-allowed' : 'pointer')};
`;
